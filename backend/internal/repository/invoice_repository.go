package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

// =============================================================================
// InvoiceRepository — pgx raw SQL for the Invoicing & Payments module.
//
// Conventions:
//   - Methods that MUST run inside a transaction accept pgx.Tx as the
//     FIRST parameter. The service layer owns tx creation/rollback.
//   - Read-only methods use the connection pool directly.
//   - The effective payment status (unpaid/partial/paid/overpaid) is
//     computed in SQL via CASE WHEN ... using SUM(payments.amount).
//   - All amounts are stored as NUMERIC(12,2) in DB; Go side uses float64.
// =============================================================================

// Sentinel errors. The handler layer (B4) maps these to HTTP status codes
// following spec §9.
var (
	ErrInvoiceNotFound   = errors.New("invoice not found")
	ErrInvoiceVoid       = errors.New("invoice is void")
	ErrPaymentExceeds    = errors.New("payment exceeds balance")
	ErrOverpaid          = errors.New("invoice is overpaid")
	ErrInvalidPayment    = errors.New("invalid payment")
	ErrReferenceRequired = errors.New("reference required for non-cash payment")
	ErrInvalidInput      = errors.New("invalid input")
)

// InvoiceRepository is the data access layer for invoices, line items and
// payments. It also exposes daily / monthly aggregation queries.
type InvoiceRepository struct {
	db *pgxpool.Pool
}

func NewInvoiceRepository(db *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

// =============================================================================
// SQL fragments
// =============================================================================

// paymentAggCTE is a WITH clause that aggregates payments once per invoice.
// Reused by every query that needs TotalPaid / TotalRefunded / EffectiveStatus.
const paymentAggCTE = `
payments_agg AS (
    SELECT
        invoice_id,
        COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0) AS total_paid,
        COALESCE(ABS(SUM(CASE WHEN is_reversal = TRUE AND amount < 0 THEN amount ELSE 0 END)), 0) AS total_refunded
    FROM payments
    GROUP BY invoice_id
)
`

// effectiveStatusExpr returns a SQL CASE expression that computes the derived
// payment status. Use within a query that has access to the invoices columns
// aliased as `i` and a `payments_agg` CTE exposing `total_paid` and the
// invoice's `total` column.
func effectiveStatusExpr() string {
	return `CASE
        WHEN i.status = 'void' THEN 'void'
        WHEN COALESCE(agg.total_paid, 0) = 0 THEN 'unpaid'
        WHEN COALESCE(agg.total_paid, 0) < i.total THEN 'partial'
        WHEN COALESCE(agg.total_paid, 0) = i.total THEN 'paid'
        ELSE 'overpaid'
    END`
}

// =============================================================================
// PPN rate lookup
// =============================================================================

// GetPPNRate returns the PPN rate configured on the property's settings
// JSONB column, falling back to 0.11 (Indonesian standard rate) if absent.
// Spec §3 BR-INV-001.
func (r *InvoiceRepository) GetPPNRate(ctx context.Context, propertyID uuid.UUID) (float64, error) {
	var settings *[]byte
	err := r.db.QueryRow(ctx, `
		SELECT settings FROM properties WHERE id = $1
	`, propertyID).Scan(&settings)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0.11, nil
		}
		return 0, err
	}
	if settings == nil {
		return 0.11, nil
	}
	var m map[string]any
	if err := json.Unmarshal(*settings, &m); err != nil {
		return 0.11, nil
	}
	v, ok := m["ppn_rate"]
	if !ok {
		return 0.11, nil
	}
	switch t := v.(type) {
	case float64:
		if t <= 0 || t > 1 {
			return 0.11, nil
		}
		return t, nil
	default:
		return 0.11, nil
	}
}

// =============================================================================
// Create
// =============================================================================

// CreateInvoiceWithTx atomically creates the invoice header + line items
// and returns the full detail. The transaction MUST be passed in by the
// service so that booking + invoice creation can share the same tx (spec §8.1).
func (r *InvoiceRepository) CreateInvoiceWithTx(
	ctx context.Context,
	tx pgx.Tx,
	input models.CreateInvoiceInput,
) (models.InvoiceDetail, error) {
	// 1. Determine PPN rate: explicit input overrides; else property default.
	ppnRate := input.PPNRate
	if ppnRate == 0 {
		// Use the pool to fetch the rate (the tx is for writes, reads on
		// the pool are fine since settings are stable within a tx).
		var err error
		ppnRate, err = r.GetPPNRate(ctx, input.PropertyID)
		if err != nil {
			return models.InvoiceDetail{}, fmt.Errorf("get ppn rate: %w", err)
		}
	}

	// 2. Compute totals. Round to 2 decimals to match NUMERIC(12,2).
	taxAmount := round2(input.Subtotal * ppnRate)
	total := round2(input.Subtotal + taxAmount)

	// 3. Reserve invoice_number (UPSERT atomic, see migration 006 §7).
	var invoiceNumber string
	if err := tx.QueryRow(ctx, `SELECT get_next_invoice_number($1)`, input.PropertyID).Scan(&invoiceNumber); err != nil {
		return models.InvoiceDetail{}, fmt.Errorf("next invoice number: %w", err)
	}

	// 4. INSERT the header. RETURNING everything we need.
	var inv models.Invoice
	if err := tx.QueryRow(ctx, `
		INSERT INTO invoices (
			property_id, booking_id, invoice_number,
			subtotal, tax_amount, ppn_rate_snapshot, total,
			original_currency, exchange_rate,
			status, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'IDR', 1.0, 'active', $8)
		RETURNING
			id, property_id, booking_id, invoice_number,
			subtotal, tax_amount, ppn_rate_snapshot, total,
			original_currency, exchange_rate, status, issued_at, paid_at,
			voided_at, voided_by, void_reason, created_by, pdf_url, notes,
			created_at, updated_at
	`,
		input.PropertyID, input.BookingID, invoiceNumber,
		input.Subtotal, taxAmount, ppnRate, total,
		input.CreatedBy,
	).Scan(
		&inv.ID, &inv.PropertyID, &inv.BookingID, &inv.InvoiceNumber,
		&inv.Subtotal, &inv.TaxAmount, &inv.PPNRateSnapshot, &inv.Total,
		&inv.OriginalCurrency, &inv.ExchangeRate, &inv.Status, &inv.IssuedAt, &inv.PaidAt,
		&inv.VoidedAt, &inv.VoidedBy, &inv.VoidReason, &inv.CreatedBy, &inv.PDFURL, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		return models.InvoiceDetail{}, fmt.Errorf("insert invoice: %w", err)
	}

	// 5. INSERT line items (batch).
	lineItems := make([]models.InvoiceLineItem, 0, len(input.LineItems))
	for _, li := range input.LineItems {
		total := round2(li.Quantity * li.UnitPrice)
		var row models.InvoiceLineItem
		if err := tx.QueryRow(ctx, `
			INSERT INTO invoice_line_items (invoice_id, description, quantity, unit_price, total, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, invoice_id, description, quantity, unit_price, total, sort_order, created_at
		`, inv.ID, li.Description, li.Quantity, li.UnitPrice, total, li.SortOrder).Scan(
			&row.ID, &row.InvoiceID, &row.Description, &row.Quantity, &row.UnitPrice, &row.Total, &row.SortOrder, &row.CreatedAt,
		); err != nil {
			return models.InvoiceDetail{}, fmt.Errorf("insert line item: %w", err)
		}
		lineItems = append(lineItems, row)
	}

	// 6. Build the detail. No payments yet → unpaid.
	return models.InvoiceDetail{
		Invoice:         inv,
		LineItems:       lineItems,
		Payments:        []models.Payment{},
		TotalPaid:       0,
		TotalRefunded:   0,
		Balance:         inv.Total,
		EffectiveStatus: models.PaymentStatusUnpaid,
	}, nil
}

// =============================================================================
// Read
// =============================================================================

// GetInvoiceByID returns the full invoice detail (header + line items +
// payments + computed fields). Returns ErrInvoiceNotFound if absent.
func (r *InvoiceRepository) GetInvoiceByID(ctx context.Context, invoiceID uuid.UUID) (models.InvoiceDetail, error) {
	var d models.InvoiceDetail
	if err := r.db.QueryRow(ctx, `
		WITH `+paymentAggCTE+`
		SELECT
			i.id, i.property_id, i.booking_id, i.invoice_number,
			i.subtotal, i.tax_amount, i.ppn_rate_snapshot, i.total,
			i.original_currency, i.exchange_rate, i.status, i.issued_at, i.paid_at,
			i.voided_at, i.voided_by, i.void_reason, i.created_by, i.pdf_url, i.notes,
			i.created_at, i.updated_at,
			COALESCE(agg.total_paid, 0)     AS total_paid,
			COALESCE(agg.total_refunded, 0) AS total_refunded,
			i.total - COALESCE(agg.total_paid, 0) AS balance,
			`+effectiveStatusExpr()+` AS effective_status
		FROM invoices i
		LEFT JOIN payments_agg agg ON agg.invoice_id = i.id
		WHERE i.id = $1
	`, invoiceID).Scan(
		&d.ID, &d.PropertyID, &d.BookingID, &d.InvoiceNumber,
		&d.Subtotal, &d.TaxAmount, &d.PPNRateSnapshot, &d.Total,
		&d.OriginalCurrency, &d.ExchangeRate, &d.Status, &d.IssuedAt, &d.PaidAt,
		&d.VoidedAt, &d.VoidedBy, &d.VoidReason, &d.CreatedBy, &d.PDFURL, &d.Notes,
		&d.CreatedAt, &d.UpdatedAt,
		&d.TotalPaid, &d.TotalRefunded, &d.Balance, &d.EffectiveStatus,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.InvoiceDetail{}, ErrInvoiceNotFound
		}
		return models.InvoiceDetail{}, err
	}

	// Line items
	rows, err := r.db.Query(ctx, `
		SELECT id, invoice_id, description, quantity, unit_price, total, sort_order, created_at
		FROM invoice_line_items WHERE invoice_id = $1 ORDER BY sort_order
	`, invoiceID)
	if err != nil {
		return models.InvoiceDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var li models.InvoiceLineItem
		if err := rows.Scan(&li.ID, &li.InvoiceID, &li.Description, &li.Quantity, &li.UnitPrice, &li.Total, &li.SortOrder, &li.CreatedAt); err != nil {
			return models.InvoiceDetail{}, err
		}
		d.LineItems = append(d.LineItems, li)
	}
	if d.LineItems == nil {
		d.LineItems = []models.InvoiceLineItem{}
	}

	// Payments
	payRows, err := r.db.Query(ctx, `
		SELECT id, invoice_id, property_id, method, amount,
		       original_currency, exchange_rate, reference, notes,
		       is_reversal, reversal_of, received_by, received_at, created_at
		FROM payments WHERE invoice_id = $1 ORDER BY received_at
	`, invoiceID)
	if err != nil {
		return models.InvoiceDetail{}, err
	}
	defer payRows.Close()
	for payRows.Next() {
		var p models.Payment
		if err := payRows.Scan(&p.ID, &p.InvoiceID, &p.PropertyID, &p.Method, &p.Amount,
			&p.OriginalCurrency, &p.ExchangeRate, &p.Reference, &p.Notes,
			&p.IsReversal, &p.ReversalOf, &p.ReceivedBy, &p.ReceivedAt, &p.CreatedAt); err != nil {
			return models.InvoiceDetail{}, err
		}
		d.Payments = append(d.Payments, p)
	}
	if d.Payments == nil {
		d.Payments = []models.Payment{}
	}

	return d, nil
}

// GetInvoiceByBookingID is the convenience alias used by the booking detail view
// (spec §4.1). invoices.booking_id is UNIQUE → at most one row.
func (r *InvoiceRepository) GetInvoiceByBookingID(ctx context.Context, bookingID uuid.UUID) (models.InvoiceDetail, error) {
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, `SELECT id FROM invoices WHERE booking_id = $1`, bookingID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.InvoiceDetail{}, ErrInvoiceNotFound
		}
		return models.InvoiceDetail{}, err
	}
	return r.GetInvoiceByID(ctx, id)
}

// =============================================================================
// Update — payments
// =============================================================================

// RegisterPayment inserts a single payment (cobro or refund) and returns it.
// It is atomic: the new TotalPaid is computed via the SQL CASE, and the
// caller can verify the result was the intended effective status.
//
// Business rules enforced HERE (data-integrity layer):
//   - amount != 0
//   - invoice must exist and not be void
//   - For amounts > 0: must not exceed remaining balance (Balance)
//   - For non-cash methods: reference is required (BR-INV-005)
//   - For refunds (amount < 0): is_reversal=true and reversal_of set
//
// Business rules enforced in the SERVICE layer (not here):
//   - Refunds require role owner OR booking.force_override=true
//   - Idempotency-Key dedup
func (r *InvoiceRepository) RegisterPayment(ctx context.Context, input models.RegisterPaymentInput) (models.Payment, error) {
	if input.Amount == 0 {
		return models.Payment{}, fmt.Errorf("%w: amount must not be zero", ErrInvalidPayment)
	}
	// Reference required for non-cash (BR-INV-005 + R-01)
	if input.Method != models.PaymentMethodCash && input.Reference == "" {
		return models.Payment{}, ErrReferenceRequired
	}
	// Refunds must have reversal_of
	if input.Amount < 0 && (input.IsReversal == false || input.ReversalOf == nil) {
		return models.Payment{}, fmt.Errorf("%w: refund requires is_reversal=true and reversal_of", ErrInvalidPayment)
	}

	// Wrap the check + insert in a tx so Balance is consistent across the
	// two operations (no race between concurrent payments on the same invoice).
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.Payment{}, err
	}
	defer tx.Rollback(ctx) // safe to call even after Commit

	// Read current state inside the tx
	var (
		status models.InvoiceStatus
		total  float64
	)
	if err := tx.QueryRow(ctx, `SELECT status, total FROM invoices WHERE id = $1 FOR UPDATE`, input.InvoiceID).Scan(&status, &total); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Payment{}, ErrInvoiceNotFound
		}
		return models.Payment{}, err
	}
	if status == models.InvoiceStatusVoid {
		return models.Payment{}, ErrInvoiceVoid
	}

	// Compute current total_paid and remaining balance
	var currentPaid float64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM payments
		WHERE invoice_id = $1 AND amount > 0 AND is_reversal = FALSE
	`, input.InvoiceID).Scan(&currentPaid); err != nil {
		return models.Payment{}, err
	}

	// Validate amount
	if input.Amount > 0 {
		remaining := total - currentPaid
		if input.Amount > remaining+0.0001 { // tolerate float drift
			return models.Payment{}, fmt.Errorf("%w: amount=%.2f > remaining=%.2f", ErrPaymentExceeds, input.Amount, remaining)
		}
	}

	// Insert payment
	var p models.Payment
	if err := tx.QueryRow(ctx, `
		INSERT INTO payments (
			invoice_id, property_id, method, amount,
			original_currency, exchange_rate,
			reference, notes, is_reversal, reversal_of, received_by
		)
		VALUES ($1, $2, $3, $4, 'IDR', 1.0, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9)
		RETURNING
			id, invoice_id, property_id, method, amount,
			original_currency, exchange_rate, reference, notes,
			is_reversal, reversal_of, received_by, received_at, created_at
	`,
		input.InvoiceID, input.PropertyID, input.Method, input.Amount,
		input.Reference, input.Notes, input.IsReversal, input.ReversalOf, input.ReceivedBy,
	).Scan(
		&p.ID, &p.InvoiceID, &p.PropertyID, &p.Method, &p.Amount,
		&p.OriginalCurrency, &p.ExchangeRate, &p.Reference, &p.Notes,
		&p.IsReversal, &p.ReversalOf, &p.ReceivedBy, &p.ReceivedAt, &p.CreatedAt,
	); err != nil {
		return models.Payment{}, err
	}

	// If this payment makes the invoice fully paid, stamp paid_at
	if input.Amount > 0 {
		newTotalPaid := currentPaid + input.Amount
		if newTotalPaid >= total-0.0001 {
			if _, err := tx.Exec(ctx, `
				UPDATE invoices SET paid_at = COALESCE(paid_at, NOW())
				WHERE id = $1 AND paid_at IS NULL
			`, input.InvoiceID); err != nil {
				return models.Payment{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Payment{}, err
	}
	return p, nil
}

// =============================================================================
// Update — void / notes / PDF
// =============================================================================

// VoidInvoice marks the invoice as void. The DB trigger trg_invoice_void_audit
// enforces that voided_by + void_reason are present and stamps voided_at.
func (r *InvoiceRepository) VoidInvoice(ctx context.Context, invoiceID uuid.UUID, input models.VoidInvoiceInput) (models.Invoice, error) {
	var inv models.Invoice
	err := r.db.QueryRow(ctx, `
		UPDATE invoices
		SET status = 'void', voided_by = $2, voided_at = NOW(), void_reason = $3
		WHERE id = $1 AND status = 'active'
		RETURNING
			id, property_id, booking_id, invoice_number,
			subtotal, tax_amount, ppn_rate_snapshot, total,
			original_currency, exchange_rate, status, issued_at, paid_at,
			voided_at, voided_by, void_reason, created_by, pdf_url, notes,
			created_at, updated_at
	`, invoiceID, input.VoidedBy, input.VoidReason).Scan(
		&inv.ID, &inv.PropertyID, &inv.BookingID, &inv.InvoiceNumber,
		&inv.Subtotal, &inv.TaxAmount, &inv.PPNRateSnapshot, &inv.Total,
		&inv.OriginalCurrency, &inv.ExchangeRate, &inv.Status, &inv.IssuedAt, &inv.PaidAt,
		&inv.VoidedAt, &inv.VoidedBy, &inv.VoidReason, &inv.CreatedBy, &inv.PDFURL, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either the invoice doesn't exist OR it's already void.
			// Disambiguate to give the right error.
			var status string
			if err2 := r.db.QueryRow(ctx, `SELECT status FROM invoices WHERE id = $1`, invoiceID).Scan(&status); err2 == nil {
				return models.Invoice{}, ErrInvoiceVoid
			}
			return models.Invoice{}, ErrInvoiceNotFound
		}
		return models.Invoice{}, err
	}
	return inv, nil
}

// UpdateNotes sets the (only mutable) free-text field on the invoice.
func (r *InvoiceRepository) UpdateNotes(ctx context.Context, invoiceID uuid.UUID, notes string) (models.Invoice, error) {
	var inv models.Invoice
	err := r.db.QueryRow(ctx, `
		UPDATE invoices SET notes = $2
		WHERE id = $1 AND status = 'active'
		RETURNING
			id, property_id, booking_id, invoice_number,
			subtotal, tax_amount, ppn_rate_snapshot, total,
			original_currency, exchange_rate, status, issued_at, paid_at,
			voided_at, voided_by, void_reason, created_by, pdf_url, notes,
			created_at, updated_at
	`, invoiceID, notes).Scan(
		&inv.ID, &inv.PropertyID, &inv.BookingID, &inv.InvoiceNumber,
		&inv.Subtotal, &inv.TaxAmount, &inv.PPNRateSnapshot, &inv.Total,
		&inv.OriginalCurrency, &inv.ExchangeRate, &inv.Status, &inv.IssuedAt, &inv.PaidAt,
		&inv.VoidedAt, &inv.VoidedBy, &inv.VoidReason, &inv.CreatedBy, &inv.PDFURL, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either not found or void (we don't allow notes on void invoices)
			return models.Invoice{}, ErrInvoiceNotFound
		}
		return models.Invoice{}, err
	}
	return inv, nil
}

// UpdatePDFURL stores the R2 URL after PDF generation (B5 responsibility).
// Pass empty string to clear.
func (r *InvoiceRepository) UpdatePDFURL(ctx context.Context, invoiceID uuid.UUID, url string) error {
	var pdfURL *string
	if url != "" {
		pdfURL = &url
	}
	_, err := r.db.Exec(ctx, `UPDATE invoices SET pdf_url = $2 WHERE id = $1`, invoiceID, pdfURL)
	return err
}

// =============================================================================
// List
// =============================================================================

// ListInvoices returns a paginated, filterable list of invoice summaries.
// Returns (rows, totalCount, error).
func (r *InvoiceRepository) ListInvoices(ctx context.Context, f models.ListInvoicesFilter) ([]models.InvoiceSummary, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Page < 1 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit

	// Build WHERE dynamically. Each filter is optional.
	conds := []string{"i.property_id = $1"}
	args := []any{f.PropertyID}
	argIdx := 2

	if f.DateFrom != nil {
		conds = append(conds, fmt.Sprintf("i.issued_at >= $%d", argIdx))
		args = append(args, *f.DateFrom)
		argIdx++
	}
	if f.DateTo != nil {
		conds = append(conds, fmt.Sprintf("i.issued_at < $%d", argIdx))
		args = append(args, *f.DateTo)
		argIdx++
	}
	if f.Search != "" {
		// Match invoice_number exact, room number, or guest name (ILIKE)
		conds = append(conds, fmt.Sprintf(`(
			i.invoice_number ILIKE $%d
			OR EXISTS (SELECT 1 FROM bookings bk JOIN rooms r ON r.id = bk.room_id
			           WHERE bk.id = i.booking_id AND r.number ILIKE $%d)
			OR EXISTS (SELECT 1 FROM bookings bk JOIN guests g ON g.id = bk.guest_id
			           WHERE bk.id = i.booking_id AND g.full_name ILIKE $%d)
		)`, argIdx, argIdx, argIdx))
		args = append(args, "%"+f.Search+"%")
		argIdx++
	}

	// Effective status filter is applied AFTER computing the aggregation.
	// We pre-compute effective_status in a CTE and filter on it.
	// Because pgx doesn't always carry the parameter type hint cleanly
	// for CASE-derived columns, we inline the value as a string literal
	// (safe: it's a strict enum, not user input — see BR-INV-003).
	effectiveStatusCond := ""
	if f.Status != "" {
		effectiveStatusCond = fmt.Sprintf(" AND es = '%s'", string(f.Status))
	}

	where := strings.Join(conds, " AND ")

	// Count query — uses a shared CTE that includes effective_status as a
	// pre-computed column (avoids re-typing the CASE expression).
	// We explicitly cast the CASE result to text so the parameter type
	// in the WHERE clause can be inferred by Postgres at prepare time.
	countSQL := fmt.Sprintf(`
		WITH `+paymentAggCTE+`,
		enriched AS (
			SELECT
				i.id,
				CAST(`+effectiveStatusExpr()+` AS text) AS es
			FROM invoices i
			LEFT JOIN payments_agg agg ON agg.invoice_id = i.id
			WHERE %s
		)
		SELECT COUNT(*) FROM enriched WHERE TRUE %s
	`, where, effectiveStatusCond)
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List query — uses the same `enriched` CTE as the count query so the
	// status filter applies identically. (COUNT(*) and SELECT * share the
	// same predicate against `es`.)
	listSQL := fmt.Sprintf(`
		WITH `+paymentAggCTE+`,
		enriched AS (
			SELECT
				i.id, i.invoice_number, i.booking_id,
				i.subtotal, i.tax_amount, i.total,
				i.status, i.issued_at, i.paid_at, i.voided_at,
				COALESCE(agg.total_paid, 0) AS total_paid,
				i.total - COALESCE(agg.total_paid, 0) AS balance,
				`+effectiveStatusExpr()+` AS es,
				`+effectiveStatusExpr()+` AS effective_status,
				(SELECT g.full_name FROM bookings bk JOIN guests g ON g.id = bk.guest_id
				 WHERE bk.id = i.booking_id LIMIT 1) AS guest_name,
				(SELECT r.number FROM bookings bk JOIN rooms r ON r.id = bk.room_id
				 WHERE bk.id = i.booking_id LIMIT 1) AS room_number
			FROM invoices i
			LEFT JOIN payments_agg agg ON agg.invoice_id = i.id
			WHERE %s
		),
		page AS (
			SELECT * FROM enriched WHERE TRUE %s
			ORDER BY issued_at DESC
			LIMIT $%d OFFSET $%d
		)
		SELECT * FROM page
	`, where, effectiveStatusCond, argIdx, argIdx+1)
	args = append(args, f.Limit, offset)

	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []models.InvoiceSummary{}
	for rows.Next() {
		var s models.InvoiceSummary
		// The enriched CTE exposes 16 columns; we skip the duplicate `es`
		// (used for the WHERE filter) and read the rest.
		var esDiscard string
		if err := rows.Scan(
			&s.ID, &s.InvoiceNumber, &s.BookingID,
			&s.Subtotal, &s.TaxAmount, &s.Total,
			&s.Status, &s.IssuedAt, &s.PaidAt, &s.VoidedAt,
			&s.TotalPaid, &s.Balance, &esDiscard, &s.EffectiveStatus,
			&s.GuestName, &s.RoomNumber,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, nil
}

// =============================================================================
// Aggregations (spec §4.9, §4.11)
// =============================================================================

// DailySummary returns the end-of-day cash-closing numbers for a property.
// The "day" is interpreted in the property's timezone (passed as the second
// param) so aggregations respect local time.
func (r *InvoiceRepository) DailySummary(ctx context.Context, propertyID uuid.UUID, date time.Time, tz string) (models.DailySummary, error) {
	out := models.DailySummary{
		Date:       date,
		PropertyID: propertyID,
		ByMethod:   map[models.PaymentMethod]float64{},
	}

	// Load offset (minutes) for the timezone; pgx parses the IANA name via
	// PostgreSQL's AT TIME ZONE.
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	// Status counts (invoices issued today)
	row := r.db.QueryRow(ctx, `
		WITH `+paymentAggCTE+`
		SELECT
			COUNT(*) FILTER (WHERE TRUE) AS issued,
			COUNT(*) FILTER (WHERE i.status='active' AND `+effectiveStatusExpr()+` = 'paid') AS paid,
			COUNT(*) FILTER (WHERE i.status='active' AND `+effectiveStatusExpr()+` = 'partial') AS partial,
			COUNT(*) FILTER (WHERE i.status='active' AND `+effectiveStatusExpr()+` = 'unpaid') AS unpaid,
			COUNT(*) FILTER (WHERE i.status='active' AND `+effectiveStatusExpr()+` = 'overpaid') AS overpaid,
			COUNT(*) FILTER (WHERE i.status='void') AS void,
			COALESCE(SUM(i.total) FILTER (WHERE i.status='active'), 0) AS total_revenue,
			COALESCE(SUM(agg.total_paid) FILTER (WHERE i.status='active'), 0) AS total_collected,
			COALESCE(SUM(agg.total_refunded), 0) AS total_refunded,
			COALESCE(SUM(i.total - COALESCE(agg.total_paid, 0)) FILTER (WHERE i.status='active'), 0) AS total_pending,
			COALESCE(SUM(i.tax_amount) FILTER (WHERE i.status='active' AND `+effectiveStatusExpr()+` IN ('paid','overpaid')), 0) AS tax_collected
		FROM invoices i
		LEFT JOIN payments_agg agg ON agg.invoice_id = i.id
		WHERE i.property_id = $1
		  AND i.issued_at >= $2 AND i.issued_at < $3
	`, propertyID, dayStart, dayEnd)

	if err := row.Scan(
		&out.InvoicesIssued,
		&out.InvoicesPaid, &out.InvoicesPartial, &out.InvoicesUnpaid,
		&out.InvoicesOverpaid, &out.InvoicesVoid,
		&out.TotalRevenue, &out.TotalCollected, &out.TotalRefunded, &out.TotalPending,
		&out.TaxCollected,
	); err != nil {
		return models.DailySummary{}, err
	}

	// Per-method totals (positive payments received today)
	methodRows, err := r.db.Query(ctx, `
		SELECT method, COALESCE(SUM(amount), 0) AS total
		FROM payments
		WHERE property_id = $1
		  AND amount > 0
		  AND is_reversal = FALSE
		  AND received_at >= $2 AND received_at < $3
		GROUP BY method
	`, propertyID, dayStart, dayEnd)
	if err != nil {
		return models.DailySummary{}, err
	}
	defer methodRows.Close()
	for methodRows.Next() {
		var m models.PaymentMethod
		var total float64
		if err := methodRows.Scan(&m, &total); err != nil {
			return models.DailySummary{}, err
		}
		out.ByMethod[m] = total
	}

	// Staff breakdown
	staffRows, err := r.db.Query(ctx, `
		SELECT u.id, u.name, COUNT(p.id) AS payments_count, COALESCE(SUM(p.amount), 0) AS amount_collected
		FROM payments p
		JOIN users u ON u.id = p.received_by
		WHERE p.property_id = $1
		  AND p.amount > 0
		  AND p.is_reversal = FALSE
		  AND p.received_at >= $2 AND p.received_at < $3
		GROUP BY u.id, u.name
		ORDER BY amount_collected DESC
	`, propertyID, dayStart, dayEnd)
	if err != nil {
		return models.DailySummary{}, err
	}
	defer staffRows.Close()
	for staffRows.Next() {
		var s models.StaffPaymentSummary
		if err := staffRows.Scan(&s.UserID, &s.UserName, &s.PaymentsCount, &s.AmountCollected); err != nil {
			return models.DailySummary{}, err
		}
		out.StaffBreakdown = append(out.StaffBreakdown, s)
	}
	if out.StaffBreakdown == nil {
		out.StaffBreakdown = []models.StaffPaymentSummary{}
	}

	return out, nil
}

// MonthlyTaxReport returns the PPN report for a property for a given period.
// If month is 0, returns the entire year.
func (r *InvoiceRepository) MonthlyTaxReport(ctx context.Context, propertyID uuid.UUID, year, month int) (models.MonthlyTaxReport, error) {
	out := models.MonthlyTaxReport{
		PropertyID: propertyID,
		Year:       year,
		Month:      month,
	}

	var start, end time.Time
	if month > 0 {
		start = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	} else {
		start = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(1, 0, 0)
	}

	row := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(subtotal) FILTER (WHERE status = 'active'), 0) AS total_subtotal,
			COALESCE(SUM(tax_amount) FILTER (WHERE status = 'active'), 0) AS total_tax,
			COUNT(*) FILTER (WHERE status = 'active') AS invoices_count,
			COUNT(*) FILTER (WHERE status = 'void') AS void_count,
			COALESCE((SELECT ABS(SUM(amount)) FROM payments
			          WHERE property_id = $1
			            AND amount < 0
			            AND is_reversal = TRUE
			            AND received_at >= $2 AND received_at < $3), 0) AS refunds_total
		FROM invoices
		WHERE property_id = $1
		  AND issued_at >= $2 AND issued_at < $3
	`, propertyID, start, end)
	if err := row.Scan(&out.TotalSubtotal, &out.TotalTax, &out.InvoicesCount, &out.VoidCount, &out.RefundsTotal); err != nil {
		return models.MonthlyTaxReport{}, err
	}
	out.NetTaxCollected = out.TotalTax
	return out, nil
}

// =============================================================================
// Idempotency (spec §4.3 + R-06)
// =============================================================================

// GetIdempotentResponse returns the cached response for a key, or
// (nil, false, nil) if the key has never been seen or has expired.
func (r *InvoiceRepository) GetIdempotentResponse(ctx context.Context, key uuid.UUID) (*models.IdempotentResponse, bool, error) {
	var resp models.IdempotentResponse
	err := r.db.QueryRow(ctx, `
		SELECT key, user_id, endpoint, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE key = $1 AND expires_at > NOW()
	`, key).Scan(&resp.Key, &resp.UserID, &resp.Endpoint, &resp.ResponseBody, &resp.CreatedAt, &resp.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &resp, true, nil
}

// SaveIdempotentResponse stores the response under the given key. The DB
// enforces uniqueness on `key` so a duplicate insert returns a unique
// violation — the caller should treat that as "replay" and re-read.
func (r *InvoiceRepository) SaveIdempotentResponse(ctx context.Context, key, userID uuid.UUID, endpoint string, body []byte) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO idempotency_keys (key, user_id, endpoint, response_body)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (key) DO NOTHING
	`, key, userID, endpoint, body)
	return err
}

// =============================================================================
// helpers
// =============================================================================

// round2 rounds to 2 decimals using banker's rounding-free math.
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
