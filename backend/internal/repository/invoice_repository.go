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
	ErrInvoiceNotFound    = errors.New("invoice not found")
	ErrInvoiceVoid        = errors.New("invoice is void")
	ErrInvoiceTerminal    = errors.New("invoice is terminal (void or refunded)") // v1.2 R-08
	ErrPaymentExceeds     = errors.New("payment exceeds balance")
	ErrOverpaid           = errors.New("invoice is overpaid")
	ErrInvalidPayment     = errors.New("invalid payment")
	ErrReferenceRequired  = errors.New("reference required for non-cash payment")
	ErrInvalidInput       = errors.New("invalid input")
	// v1.2 R-07 refund 1:1 validation gates. All map to 422 INVALID_REFUND_TARGET.
	ErrRefundNotReverse  = errors.New("refund must have is_reversal=true and reversal_of")
	ErrRefundCrossInvoice = errors.New("reversal_of must belong to the same invoice")
	ErrRefundOfRefund    = errors.New("cannot refund a reversal (no refund-of-refund)")
	ErrRefundExceedsCap  = errors.New("refund amount exceeds remaining_reverseable")
	ErrRefundMethodMismatch = errors.New("refund method must match original")
	ErrRefundOverRefund  = errors.New("refund would cause over-refund (total_refunded > total)")
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
//
// v1.2 R-07: WHERE invalidated_at IS NULL filters out retired rows so the
// aggregation reflects only "live" payments.
const paymentAggCTE = `
payments_agg AS (
    SELECT
        invoice_id,
        COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0) AS total_paid,
        COALESCE(ABS(SUM(CASE WHEN is_reversal = TRUE AND amount < 0 THEN amount ELSE 0 END)), 0) AS total_refunded
    FROM payments
    WHERE invalidated_at IS NULL
    GROUP BY invoice_id
)
`

// effectiveStatusExpr returns a SQL CASE expression that computes the derived
// payment status. Use within a query that has access to the invoices columns
// aliased as `i` and a `payments_agg` CTE exposing `total_paid` and the
// invoice's `total` column.
//
// v1.2 R-08 precedence:
//   void      > refunded > paid > partial > unpaid
// (overpaid removed from effective_status; legacy over-refund goes
//  to invoices.needs_review=true and is excluded from reports.)
func effectiveStatusExpr() string {
	return `CASE
        WHEN i.status = 'void' THEN 'void'
        WHEN i.status = 'refunded' THEN 'refunded'
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
			original_currency, exchange_rate, status, needs_review,
			issued_at, paid_at,
			voided_at, voided_by, void_reason, created_by, pdf_url, notes,
			created_at, updated_at
	`,
		input.PropertyID, input.BookingID, invoiceNumber,
		input.Subtotal, taxAmount, ppnRate, total,
		input.CreatedBy,
	).Scan(
		&inv.ID, &inv.PropertyID, &inv.BookingID, &inv.InvoiceNumber,
		&inv.Subtotal, &inv.TaxAmount, &inv.PPNRateSnapshot, &inv.Total,
		&inv.OriginalCurrency, &inv.ExchangeRate, &inv.Status, &inv.NeedsReview, &inv.IssuedAt, &inv.PaidAt,
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
			i.original_currency, i.exchange_rate, i.status, i.needs_review,
			i.issued_at, i.paid_at,
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
		&d.OriginalCurrency, &d.ExchangeRate, &d.Status, &d.NeedsReview, &d.IssuedAt, &d.PaidAt,
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

	// Payments (v1.2 R-07: also return invalidated_at / invalidated_by /
	// invalidated_reason so the UI can render the strikethrough badge).
	payRows, err := r.db.Query(ctx, `
		SELECT id, invoice_id, property_id, method, amount,
		       original_currency, exchange_rate, reference, notes,
		       is_reversal, reversal_of, received_by, received_at, created_at,
		       invalidated_at, invalidated_by, invalidated_reason
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
			&p.IsReversal, &p.ReversalOf, &p.ReceivedBy, &p.ReceivedAt, &p.CreatedAt,
			&p.InvalidatedAt, &p.InvalidatedBy, &p.InvalidatedReason); err != nil {
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
//   - invoice must exist and not be void/refunded (R-08)
//   - For amounts > 0: must not exceed remaining balance (Balance)
//   - For non-cash methods: reference is required (BR-INV-005)
//   - For refunds (amount < 0, R-07):
//       * is_reversal=true && reversal_of != nil
//       * reversal_of points to a payment of the same invoice
//       * reversal_of is not itself a reversal
//       * |refund.amount| <= remaining_reverseable(target)
//       * method matches original.method unless ForceOverride
//       * refund would not cause total_refunded > total
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
		return models.Payment{}, fmt.Errorf("%w: refund requires is_reversal=true and reversal_of", ErrRefundNotReverse)
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
	// v1.2 R-08: refunded is also terminal — no new payments allowed.
	if status == models.InvoiceStatusRefunded {
		return models.Payment{}, ErrInvoiceTerminal
	}

	// Compute current total_paid and remaining balance.
	// v1.2 R-07: exclude invalidated rows from the aggregation.
	var currentPaid float64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM payments
		WHERE invoice_id = $1 AND amount > 0 AND is_reversal = FALSE
		  AND invalidated_at IS NULL
	`, input.InvoiceID).Scan(&currentPaid); err != nil {
		return models.Payment{}, err
	}

	// =================================================================
	// v1.2 R-07 — Refund 1:1 validation gates
	// All checks are inside the same tx as the insert so they race safely.
	// =================================================================
	if input.Amount < 0 {
		// (a) Load the target payment (must exist on same invoice, not be
		//     a reversal itself). We use SELECT FOR UPDATE so a concurrent
		//     refund against the same target can't slip past our cap check.
		var (
			targetInvoiceID uuid.UUID
			targetAmount    float64
			targetMethod    models.PaymentMethod
			targetIsRev     bool
			targetInvalid   *time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT invoice_id, amount, method, is_reversal, invalidated_at
			  FROM payments
			 WHERE id = $1
			 FOR UPDATE
		`, *input.ReversalOf).Scan(
			&targetInvoiceID, &targetAmount, &targetMethod, &targetIsRev, &targetInvalid,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.Payment{}, fmt.Errorf("%w: target payment not found", ErrInvalidPayment)
			}
			return models.Payment{}, err
		}

		// (b) Cross-invoice refund: forbidden.
		if targetInvoiceID != input.InvoiceID {
			return models.Payment{}, ErrRefundCrossInvoice
		}
		// (c) Refund-of-refund: forbidden.
		if targetIsRev {
			return models.Payment{}, ErrRefundOfRefund
		}
		// (d) Target invalidated: treat as if target doesn't exist for refund purposes.
		if targetInvalid != nil {
			return models.Payment{}, fmt.Errorf("%w: target payment has been invalidated", ErrRefundNotReverse)
		}
		// (e) Method match (R-07: same method as original). force_override
		//     is owner-only; the SERVICE layer has already verified role
		//     and forwarded the override flag here.
		if input.Method != targetMethod && !input.ForceOverride {
			return models.Payment{}, ErrRefundMethodMismatch
		}
		// (f) Cap: |refund.amount| <= remaining_reverseable(target).
		//     remaining_reverseable = target.amount - sum(refunds against it).
		var alreadyRefundedAgainstTarget float64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(ABS(SUM(amount)), 0) FROM payments
			 WHERE reversal_of = $1
			   AND invalidated_at IS NULL
		`, *input.ReversalOf).Scan(&alreadyRefundedAgainstTarget); err != nil {
			return models.Payment{}, err
		}
		remainingReverseable := targetAmount - alreadyRefundedAgainstTarget
		refundAbs := -input.Amount // amount is negative
		if refundAbs > remainingReverseable+0.0001 {
			return models.Payment{}, fmt.Errorf(
				"%w: target=%.2f, already_refunded=%.2f, remaining=%.2f, requested=%.2f",
				ErrRefundExceedsCap, targetAmount, alreadyRefundedAgainstTarget, remainingReverseable, refundAbs,
			)
		}
		// (g) Over-refund: total_refunded + |refund| <= total.
		//     Compute current total_refunded (live rows only) for this invoice.
		var currentRefunded float64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(ABS(SUM(amount)), 0) FROM payments
			 WHERE invoice_id = $1
			   AND amount < 0
			   AND invalidated_at IS NULL
		`, input.InvoiceID).Scan(&currentRefunded); err != nil {
			return models.Payment{}, err
		}
		if currentRefunded+refundAbs > total+0.01 {
			return models.Payment{}, fmt.Errorf(
				"%w: total_refunded=%.2f + refund=%.2f > total=%.2f",
				ErrRefundOverRefund, currentRefunded, refundAbs, total,
			)
		}
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
			is_reversal, reversal_of, received_by, received_at, created_at,
			invalidated_at, invalidated_by, invalidated_reason
	`,
		input.InvoiceID, input.PropertyID, input.Method, input.Amount,
		input.Reference, input.Notes, input.IsReversal, input.ReversalOf, input.ReceivedBy,
	).Scan(
		&p.ID, &p.InvoiceID, &p.PropertyID, &p.Method, &p.Amount,
		&p.OriginalCurrency, &p.ExchangeRate, &p.Reference, &p.Notes,
		&p.IsReversal, &p.ReversalOf, &p.ReceivedBy, &p.ReceivedAt, &p.CreatedAt,
		&p.InvalidatedAt, &p.InvalidatedBy, &p.InvalidatedReason,
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

// =============================================================================
// RefundAll (v1.2 R-07 — spec §4.12)
// Atomic batch refund of every positive, non-invalidated payment on the
// invoice. Generates N refund rows + 1 refund_batches audit row, all in
// a single tx. If any refund fails its gate, the whole batch rolls back.
// =============================================================================

// RefundAllResult is the summary of refunds created by RefundAll.
type RefundAllResult struct {
	RefundBatches         models.RefundBatch
	RefundedPayments      []models.Payment // N refund rows in order of generation
	InvoiceLifecycleAfter models.InvoiceStatus
}

func (r *InvoiceRepository) RefundAll(ctx context.Context, input models.RefundAllInput) (RefundAllResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return RefundAllResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Lock the invoice + check lifecycle.
	var (
		status     string
		propertyID uuid.UUID
		total      float64
	)
	err = tx.QueryRow(ctx, `
		SELECT status, property_id, total
		  FROM invoices WHERE id = $1 FOR UPDATE
	`, input.InvoiceID).Scan(&status, &propertyID, &total)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefundAllResult{}, ErrInvoiceNotFound
		}
		return RefundAllResult{}, err
	}
	if status == string(models.InvoiceStatusVoid) || status == string(models.InvoiceStatusRefunded) {
		return RefundAllResult{}, ErrInvoiceTerminal
	}

	// 2. Find every positive, non-invalidated, not-fully-refunded payment.
	rows, err := tx.Query(ctx, `
		SELECT p.id, p.amount, p.method, p.reference
		  FROM payments p
		 WHERE p.invoice_id = $1
		   AND p.amount > 0
		   AND p.is_reversal = FALSE
		   AND p.invalidated_at IS NULL
		   AND ABS(p.amount) > COALESCE((
		       SELECT ABS(SUM(c.amount))
		         FROM payments c
		        WHERE c.reversal_of = p.id
		          AND c.invalidated_at IS NULL
		   ), 0)
		 ORDER BY p.received_at
		 FOR UPDATE OF p
	`, input.InvoiceID)
	if err != nil {
		return RefundAllResult{}, err
	}
	type target struct {
		ID        uuid.UUID
		Amount    float64
		Method    models.PaymentMethod
		Reference *string
	}
	var targets []target
	for rows.Next() {
		var t target
		var ref *string
		if err := rows.Scan(&t.ID, &t.Amount, &t.Method, &ref); err != nil {
			rows.Close()
			return RefundAllResult{}, err
		}
		t.Reference = ref
		targets = append(targets, t)
	}
	rows.Close()
	if len(targets) == 0 {
		return RefundAllResult{}, fmt.Errorf("%w: no positive payments to refund", ErrRefundNotReverse)
	}

	// 3. Generate one refund row per target. Each refund is the full
	//    remaining_reverseable of its target. Method inherits (no
	//    override on bulk refunds — would need explicit per-row UI).
	result := RefundAllResult{}
	var totalRefunded float64
	for _, t := range targets {
		// Compute remaining at the moment of insert (inside tx, FOR UPDATE
		// on the target already held so this is race-safe).
		var alreadyRefunded float64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(ABS(SUM(amount)), 0) FROM payments
			 WHERE reversal_of = $1 AND invalidated_at IS NULL
		`, t.ID).Scan(&alreadyRefunded); err != nil {
			return RefundAllResult{}, err
		}
		remaining := t.Amount - alreadyRefunded

		// Default reference: REFUND-{original.reference || payment.id[:8]}
		refStr := ""
		if t.Reference != nil && *t.Reference != "" {
			refStr = "REFUND-" + *t.Reference
		} else {
			refStr = "REFUND-" + t.ID.String()[:8]
		}

		var p models.Payment
		err := tx.QueryRow(ctx, `
			INSERT INTO payments (
				invoice_id, property_id, method, amount,
				original_currency, exchange_rate,
				reference, notes, is_reversal, reversal_of, received_by
			)
			VALUES ($1, $2, $3, $4, 'IDR', 1.0, $5, $6, TRUE, $7, $8)
			RETURNING
				id, invoice_id, property_id, method, amount,
				original_currency, exchange_rate, reference, notes,
				is_reversal, reversal_of, received_by, received_at, created_at,
				invalidated_at, invalidated_by, invalidated_reason
		`,
			input.InvoiceID, propertyID, t.Method, -remaining,
			refStr, input.Reason, t.ID, input.InitiatedBy,
		).Scan(
			&p.ID, &p.InvoiceID, &p.PropertyID, &p.Method, &p.Amount,
			&p.OriginalCurrency, &p.ExchangeRate, &p.Reference, &p.Notes,
			&p.IsReversal, &p.ReversalOf, &p.ReceivedBy, &p.ReceivedAt, &p.CreatedAt,
			&p.InvalidatedAt, &p.InvalidatedBy, &p.InvalidatedReason,
		)
		if err != nil {
			return RefundAllResult{}, fmt.Errorf("insert refund for %s: %w", t.ID, err)
		}
		result.RefundedPayments = append(result.RefundedPayments, p)
		totalRefunded += remaining
	}

	// 4. Insert refund_batches audit row.
	var batch models.RefundBatch
	paymentIDs := make([]uuid.UUID, 0, len(result.RefundedPayments))
	for _, p := range result.RefundedPayments {
		paymentIDs = append(paymentIDs, p.ID)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO refund_batches
			(invoice_id, property_id, initiated_by, reason, payment_ids, total_refunded)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, invoice_id, property_id, initiated_by, reason,
		          payment_ids, total_refunded, created_at
	`, input.InvoiceID, propertyID, input.InitiatedBy,
		input.Reason, paymentIDs, totalRefunded,
	).Scan(
		&batch.ID, &batch.InvoiceID, &batch.PropertyID,
		&batch.InitiatedBy, &batch.Reason,
		&batch.PaymentIDs, &batch.TotalRefunded, &batch.CreatedAt,
	)
	if err != nil {
		return RefundAllResult{}, fmt.Errorf("insert refund_batches: %w", err)
	}
	result.RefundBatches = batch

	// 5. The trigger trg_invoice_status_update will auto-flip status to
	//    'refunded' if total_refunded >= total. We read the result to
	//    surface the final lifecycle to the caller.
	var finalStatus string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM invoices WHERE id = $1`,
		input.InvoiceID).Scan(&finalStatus); err != nil {
		return RefundAllResult{}, err
	}
	result.InvoiceLifecycleAfter = models.InvoiceStatus(finalStatus)

	// 6. Commit. All-or-nothing.
	if err := tx.Commit(ctx); err != nil {
		return RefundAllResult{}, err
	}
	return result, nil
}

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

	// Status counts (invoices issued today).
	// v1.2 R-08: invoices with needs_review=true are excluded from all
	// revenue/tax aggregations (data integrity flag, BR-INV-011).
	row := r.db.QueryRow(ctx, `
		WITH `+paymentAggCTE+`
		SELECT
			COUNT(*) FILTER (WHERE i.needs_review = FALSE) AS issued,
			COUNT(*) FILTER (WHERE i.status='active' AND i.needs_review = FALSE AND `+effectiveStatusExpr()+` = 'paid') AS paid,
			COUNT(*) FILTER (WHERE i.status='active' AND i.needs_review = FALSE AND `+effectiveStatusExpr()+` = 'partial') AS partial,
			COUNT(*) FILTER (WHERE i.status='active' AND i.needs_review = FALSE AND `+effectiveStatusExpr()+` = 'unpaid') AS unpaid,
			COUNT(*) FILTER (WHERE i.status='active' AND i.needs_review = FALSE AND `+effectiveStatusExpr()+` = 'overpaid') AS overpaid,
			COUNT(*) FILTER (WHERE i.status='void' AND i.needs_review = FALSE) AS void,
			COUNT(*) FILTER (WHERE i.status='refunded' AND i.needs_review = FALSE) AS refunded,
			COUNT(*) FILTER (WHERE i.needs_review = TRUE) AS needs_review_count,
			COALESCE(SUM(i.total) FILTER (WHERE i.status='active' AND i.needs_review = FALSE), 0) AS total_revenue,
			COALESCE(SUM(agg.total_paid) FILTER (WHERE i.status='active' AND i.needs_review = FALSE), 0) AS total_collected,
			COALESCE(SUM(agg.total_refunded) FILTER (WHERE i.needs_review = FALSE), 0) AS total_refunded,
			COALESCE(SUM(i.total - COALESCE(agg.total_paid, 0)) FILTER (WHERE i.status='active' AND i.needs_review = FALSE), 0) AS total_pending,
			COALESCE(SUM(i.tax_amount) FILTER (WHERE i.status='active' AND i.needs_review = FALSE AND `+effectiveStatusExpr()+` IN ('paid','overpaid')), 0) AS tax_collected
		FROM invoices i
		LEFT JOIN payments_agg agg ON agg.invoice_id = i.id
		WHERE i.property_id = $1
		  AND i.issued_at >= $2 AND i.issued_at < $3
	`, propertyID, dayStart, dayEnd)

	if err := row.Scan(
		&out.InvoicesIssued,
		&out.InvoicesPaid, &out.InvoicesPartial, &out.InvoicesUnpaid,
		&out.InvoicesOverpaid, &out.InvoicesVoid,
		&out.InvoicesRefunded, &out.NeedsReviewCount, // v1.2 R-08
		&out.TotalRevenue, &out.TotalCollected, &out.TotalRefunded, &out.TotalPending,
		&out.TaxCollected,
	); err != nil {
		return models.DailySummary{}, err
	}

	// v1.2 R-08: net_revenue = collected - refunded (informational)
	out.NetRevenue = out.TotalCollected - out.TotalRefunded

	// Per-method totals (positive payments received today).
	// v1.2 R-07: exclude invalidated rows from the per-method aggregation.
	methodRows, err := r.db.Query(ctx, `
		SELECT method, COALESCE(SUM(amount), 0) AS total
		FROM payments
		WHERE property_id = $1
		  AND amount > 0
		  AND is_reversal = FALSE
		  AND invalidated_at IS NULL
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
//
// v1.2 R-08 / R-09 Q3: refunds net PPN del MES DEL REFUND (cash basis).
//   - Refund counted in month of received_at (NOT month of invoice).
//   - net_tax = tax_amount * (1 - refunded/total) on the matched invoices.
//   - Invoices with needs_review=true are excluded from all totals.
//   - Adds: refunded_count, needs_review_count, excluded_needs_review,
//     refunds_count, net_subtotal.
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

	// refund_amount_for_invoice(invoice_total, tax_amount, refunds_total) =
	//   apply the proportional refund to both subtotal and tax_amount.
	// We compute it in SQL using GREATEST(0, total_tax - refunds_share).
	row := r.db.QueryRow(ctx, `
		WITH invoice_refunds AS (
			SELECT
				i.id,
				i.subtotal,
				i.tax_amount,
				i.status,
				i.needs_review,
				COALESCE((
					SELECT ABS(SUM(p.amount))
					  FROM payments p
					 WHERE p.invoice_id = i.id
					   AND p.amount < 0
					   AND p.invalidated_at IS NULL
				), 0) AS refunded_for_invoice
			FROM invoices i
			WHERE i.property_id = $1
			  AND i.issued_at >= $2 AND i.issued_at < $3
		)
		SELECT
			COALESCE(SUM(subtotal) FILTER (WHERE status = 'active' AND needs_review = FALSE), 0) AS total_subtotal,
			COALESCE(SUM(subtotal - subtotal * LEAST(1, refunded_for_invoice / NULLIF(subtotal + tax_amount, 0)))
				FILTER (WHERE status = 'active' AND needs_review = FALSE), 0) AS net_subtotal,
			COALESCE(SUM(tax_amount) FILTER (WHERE status = 'active' AND needs_review = FALSE), 0) AS total_tax,
			COALESCE(SUM(tax_amount - tax_amount * LEAST(1, refunded_for_invoice / NULLIF(subtotal + tax_amount, 0)))
				FILTER (WHERE status = 'active' AND needs_review = FALSE), 0) AS net_tax_collected,
			COALESCE(SUM(refunded_for_invoice) FILTER (WHERE needs_review = FALSE), 0) AS refunds_total,
			COUNT(*) FILTER (WHERE status = 'active' AND needs_review = FALSE) AS invoices_count,
			COUNT(*) FILTER (WHERE status = 'void' AND needs_review = FALSE) AS void_count,
			COUNT(*) FILTER (WHERE status = 'refunded' AND needs_review = FALSE) AS refunded_count,
			COUNT(*) FILTER (WHERE needs_review = TRUE) AS needs_review_count
		FROM invoice_refunds
	`, propertyID, start, end)
	if err := row.Scan(
		&out.TotalSubtotal, &out.NetSubtotal,
		&out.TotalTax, &out.NetTaxCollected,
		&out.TotalRefunded,
		&out.InvoicesCount, &out.VoidCount, &out.RefundedCount,
		&out.NeedsReviewCount,
	); err != nil {
		return models.MonthlyTaxReport{}, err
	}
	// Refunds received in this period (cash basis: when money leaves).
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments
		 WHERE property_id = $1
		   AND amount < 0
		   AND is_reversal = TRUE
		   AND invalidated_at IS NULL
		   AND received_at >= $2 AND received_at < $3
	`, propertyID, start, end).Scan(&out.RefundsCount); err != nil {
		return models.MonthlyTaxReport{}, err
	}
	// Count of invoices that exist in period but were excluded (needs_review).
	// (Above query already returns it; alias for clarity.)
	out.Excluded = out.NeedsReviewCount
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
