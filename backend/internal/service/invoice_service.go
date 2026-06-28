package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

// =============================================================================
// InvoiceService — business rules for Invoicing & Payments
// Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md
//
// Layering:
//   - The repository (B2) owns SQL and data integrity.
//   - This service owns: cross-module orchestration, idempotency dedup,
//     refund authorization, void-audit enforcement, and the
//     booking-lifecycle integration (BR-INV-001/007/008).
//   - Handlers (B4) own HTTP and request/response shape.
//
// PDF generation is called via the PDFGenerator interface below. The
// interface is declared now and implemented in B5; if nil, PDF is
// skipped (invoice persists with pdf_url NULL per spec §8.1).
// =============================================================================

// PDFGenerator produces a PDF for an invoice and returns its storage URL.
// Implementation deferred to B5; the service treats this as best-effort
// (a nil generator or a failed call leaves pdf_url NULL but the invoice
// remains valid — spec §8.1, R-04 in Ratification Log).
//
// locale is a BCP-47-like tag (e.g. "en", "es", "id"). The generator
// uses it to pick the user-facing copy in the PDF. An empty or unknown
// tag falls back to English inside the generator.
type PDFGenerator interface {
	Generate(ctx context.Context, invoiceID uuid.UUID, locale string) (url string, err error)
}

// UserRole is the role string used to authorize refunds. Values: "owner",
// "receptionist". Anything else is denied for refund operations.
type UserRole string

const (
	RoleOwner        UserRole = "owner"
	RoleReceptionist UserRole = "receptionist"
)

// Refund authorization error code (mapped to 403 in handlers).
const CodeRefundForbidden = "REFUND_FORBIDDEN"

type InvoiceService struct {
	db          *pgxpool.Pool
	invoiceRepo *repository.InvoiceRepository
	bookingRepo *repository.BookingRepository
	pdfGen      PDFGenerator
}

func NewInvoiceService(
	db *pgxpool.Pool,
	invoiceRepo *repository.InvoiceRepository,
	bookingRepo *repository.BookingRepository,
	pdfGen PDFGenerator,
) *InvoiceService {
	return &InvoiceService{db: db, invoiceRepo: invoiceRepo, bookingRepo: bookingRepo, pdfGen: pdfGen}
}

// SetPDFGenerator wires the PDF generator after construction (so B5 can
// inject it once the implementation is in place).
func (s *InvoiceService) SetPDFGenerator(g PDFGenerator) { s.pdfGen = g }

// =============================================================================
// Booking lifecycle hooks (called by BookingService)
// =============================================================================

// CreateInvoiceForBooking is the BR-INV-001 entry point. It runs INSIDE
// the caller's transaction so the booking + invoice creation is atomic.
//
// The caller passes the freshly-inserted Booking to avoid a round-trip
// (the booking would not be visible via the pool's GetByID until the
// tx commits).
//
// Behavior matrix (per spec §3 + §8.1):
//   - subtotal > 0: create invoice with tax + line items
//   - subtotal = 0: create a void invoice with notes='Courtesy booking'
//     (BR-INV-012, MVP behavior; Phase 2 will use a
//     dedicated courtesies table)
//
// Returns the created invoice. PDF generation is best-effort and runs
// after commit if a generator is wired.
func (s *InvoiceService) CreateInvoiceForBooking(
	ctx context.Context,
	tx pgx.Tx,
	booking *models.Booking,
) (models.Invoice, error) {
	if booking.PropertyID == uuid.Nil {
		return models.Invoice{}, &BusinessError{
			Code:    "BOOKING_PROPERTY_REQUIRED",
			Message: "Booking must have a property_id",
		}
	}

	// Build line items from the booking.
	// For MVP: 1 line "Room X - N nights" (room may be nil if unassigned).
	detail := &models.BookingDetail{Booking: *booking}
	lineItems := []models.NewLineItem{{
		Description: lineDescription(detail),
		Quantity:    1,
		UnitPrice:   booking.TotalAmount,
		SortOrder:   0,
	}}

	created, err := s.invoiceRepo.CreateInvoiceWithTx(ctx, tx, models.CreateInvoiceInput{
		PropertyID: booking.PropertyID,
		BookingID:  booking.ID,
		Subtotal:   booking.TotalAmount,
		LineItems:  lineItems,
		CreatedBy:  booking.CreatedBy,
	})
	if err != nil {
		return models.Invoice{}, fmt.Errorf("create invoice: %w", err)
	}

	// BR-INV-012: subtotal = 0 → automatically void the invoice (no fiscal
	// document for a courtesy booking). Done in the same tx so it's atomic.
	if booking.TotalAmount == 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE invoices
			SET status = 'void', voided_by = $2, voided_at = NOW(),
			    void_reason = 'Courtesy booking (subtotal=0)'
			WHERE id = $1
		`, created.ID, booking.CreatedBy); err != nil {
			return models.Invoice{}, fmt.Errorf("void courtesy invoice: %w", err)
		}
		created.Status = models.InvoiceStatusVoid
		created.VoidReason = ptr("Courtesy booking (subtotal=0)")
		now := time.Now()
		created.VoidedAt = &now
		created.VoidedBy = &booking.CreatedBy
	}

	return created.Invoice, nil
}

// VoidInvoiceForBooking is called when a booking is cancelled. Per BR-INV-007
// + spec §8.2: the invoice becomes void (status='void', voided_by, voided_at,
// void_reason). PDF is regenerated on a best-effort basis with VOID watermark.
func (s *InvoiceService) VoidInvoiceForBooking(
	ctx context.Context,
	bookingID, userID uuid.UUID,
	reason string,
) error {
	// Find the invoice for the booking. Not found is not an error here:
	// a booking may have been a courtesy (subtotal=0 → already void) or
	// the invoice might have been voided manually.
	inv, err := s.invoiceRepo.GetInvoiceByBookingID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			return nil
		}
		return err
	}
	if inv.Status == models.InvoiceStatusVoid {
		return nil // already void; nothing to do
	}
	if reason == "" {
		reason = "Booking cancelled"
	}
	if _, err := s.invoiceRepo.VoidInvoice(ctx, inv.ID, models.VoidInvoiceInput{
		VoidedBy:   userID,
		VoidReason: reason,
	}); err != nil {
		return err
	}
	// Best-effort PDF regeneration with VOID watermark. Failures are
	// logged and ignored (spec §8.1, R-04).
	s.regeneratePDFQuietly(ctx, inv.ID)
	return nil
}

// SetBookingPaymentStatus is the BR-INV-004 update: after a check-out, the
// booking's payment_status is derived from the invoice's balance. This
// keeps both sides of the model in sync.
func (s *InvoiceService) SetBookingPaymentStatus(ctx context.Context, bookingID uuid.UUID) error {
	inv, err := s.invoiceRepo.GetInvoiceByBookingID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			return nil
		}
		return err
	}
	// Map effective status → booking payment_status.
	var paymentStatus string
	switch inv.EffectiveStatus {
	case models.PaymentStatusPaid:
		paymentStatus = "paid"
	case models.PaymentStatusPartial:
		paymentStatus = "partial"
	case models.PaymentStatusOverpaid:
		paymentStatus = "overpaid"
	case models.PaymentStatusUnpaid, models.PaymentStatusVoid:
		fallthrough
	default:
		// For MVP we keep it simple: unpaid/void → unpaid. Future Phase 2
		// dashboard alert if balance > 0 at check-out.
		paymentStatus = "unpaid"
	}
	// Update the booking directly via the repo. We accept the column
	// update as a "set" operation to avoid diffing the full booking row.
	_, err = s.db.Exec(ctx, `UPDATE bookings SET payment_status = $2 WHERE id = $1`, bookingID, paymentStatus)
	return err
}

// =============================================================================
// Public API (called by handlers in B4)
// =============================================================================

func (s *InvoiceService) GetByID(ctx context.Context, id uuid.UUID) (models.InvoiceDetail, error) {
	return s.invoiceRepo.GetInvoiceByID(ctx, id)
}

func (s *InvoiceService) GetByBookingID(ctx context.Context, bookingID uuid.UUID) (models.InvoiceDetail, error) {
	return s.invoiceRepo.GetInvoiceByBookingID(ctx, bookingID)
}

func (s *InvoiceService) List(ctx context.Context, f models.ListInvoicesFilter) ([]models.InvoiceSummary, int, error) {
	return s.invoiceRepo.ListInvoices(ctx, f)
}

func (s *InvoiceService) DailySummary(ctx context.Context, propertyID uuid.UUID, date time.Time, tz string) (models.DailySummary, error) {
	return s.invoiceRepo.DailySummary(ctx, propertyID, date, tz)
}

func (s *InvoiceService) MonthlyTaxReport(ctx context.Context, propertyID uuid.UUID, year, month int) (models.MonthlyTaxReport, error) {
	return s.invoiceRepo.MonthlyTaxReport(ctx, propertyID, year, month)
}

func (s *InvoiceService) UpdateNotes(ctx context.Context, invoiceID uuid.UUID, notes string) (models.Invoice, error) {
	return s.invoiceRepo.UpdateNotes(ctx, invoiceID, notes)
}

// =============================================================================
// Payments
// =============================================================================

// RegisterPayment is the BR-INV-005/010 entry point. It applies:
//   - Idempotency dedup (R-06): if the same Idempotency-Key has been
//     seen in the last 24h, return the cached response without
//     reprocessing.
//   - Refund authorization (BR-INV-010): refunds (amount < 0) require
//     role=owner OR booking.force_override=true.
//
// On success, persists the payment and stores the cached response for
// future idempotent replays.
func (s *InvoiceService) RegisterPayment(
	ctx context.Context,
	input models.RegisterPaymentInput,
	idemKey *uuid.UUID,
	userID uuid.UUID,
	userRole UserRole,
) (models.Payment, error) {
	// 1. Idempotency replay short-circuit.
	if idemKey != nil {
		cached, found, err := s.invoiceRepo.GetIdempotentResponse(ctx, *idemKey)
		if err != nil {
			return models.Payment{}, fmt.Errorf("idempotency lookup: %w", err)
		}
		if found {
			// Replay: the cached response body is the JSON of the original
			// payment. We re-hydrate the Payment struct from the body
			// (caller gets the same payment object back).
			// For MVP we re-fetch the most recent payment for this invoice
			// matching the request: simpler than re-marshalling the JSON.
			return s.fetchPaymentForReplay(ctx, input, cached.Endpoint, cached.UserID)
		}
	}

	// 2. Refund authorization (BR-INV-010 + R-02).
	if input.Amount < 0 {
		ok, err := s.canRefund(ctx, input.InvoiceID, userID, userRole)
		if err != nil {
			return models.Payment{}, err
		}
		if !ok {
			return models.Payment{}, &BusinessError{
				Code:    CodeRefundForbidden,
				Message: "Refunds require role=owner or booking.force_override=true",
			}
		}
	}

	// 3. Register the payment via the repo. Repo enforces amount > 0 <=
	// balance, reference required for non-cash, etc.
	payment, err := s.invoiceRepo.RegisterPayment(ctx, input)
	if err != nil {
		return models.Payment{}, err
	}

	// 4. Idempotency: cache the response for future replays.
	if idemKey != nil {
		// Build a minimal response envelope (the handler will shape it).
		body := []byte(fmt.Sprintf(`{"id":%q,"invoice_id":%q,"amount":%v,"is_reversal":%v}`,
			payment.ID.String(), payment.InvoiceID.String(), payment.Amount, payment.IsReversal))
		_ = s.invoiceRepo.SaveIdempotentResponse(ctx, *idemKey, userID, "POST /invoices/:id/payments", body)
	}

	// 5. Best-effort: refresh the booking payment_status (BR-INV-004).
	// Look up the booking_id from the invoice; SetBookingPaymentStatus
	// needs a booking ID, not an invoice ID.
	var bookingID uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT booking_id FROM invoices WHERE id = $1`, payment.InvoiceID).Scan(&bookingID); err == nil {
		if sErr := s.SetBookingPaymentStatus(ctx, bookingID); sErr != nil {
			log.Printf("SetBookingPaymentStatus after payment %s: %v", payment.ID, sErr)
		}
	} else {
		log.Printf("lookup booking_id for invoice %s: %v", payment.InvoiceID, err)
	}

	return payment, nil
}

// RefundAll is the bulk-refund endpoint (v1.2 R-07, spec §4.12). It refunds
// every positive, non-invalidated payment on the invoice atomically.
// Same authorization as RegisterPayment for refunds (role=owner OR
// booking.force_override=true).
//
// On success, the underlying DB trigger flips invoices.status to 'refunded'
// when total_refunded >= total. Otherwise the invoice stays 'active' with
// effective_status='partial' or similar.
func (s *InvoiceService) RefundAll(
	ctx context.Context,
	input models.RefundAllInput,
	userRole UserRole,
) (repository.RefundAllResult, error) {
	if input.Reason == "" {
		return repository.RefundAllResult{}, &BusinessError{
			Code:    "REFUND_REASON_REQUIRED",
			Message: "reason is required for refund-all",
		}
	}
	// Refund authorization (BR-INV-010 + R-02).
	ok, err := s.canRefund(ctx, input.InvoiceID, input.InitiatedBy, userRole)
	if err != nil {
		return repository.RefundAllResult{}, err
	}
	if !ok {
		return repository.RefundAllResult{}, &BusinessError{
			Code:    CodeRefundForbidden,
			Message: "Refunds require role=owner or booking.force_override=true",
		}
	}

	result, err := s.invoiceRepo.RefundAll(ctx, input)
	if err != nil {
		// Map domain errors to BusinessError for the handler.
		switch {
		case errors.Is(err, repository.ErrInvoiceTerminal):
			return repository.RefundAllResult{}, &BusinessError{
				Code:    "INVOICE_TERMINAL",
				Message: "This invoice is in a terminal state (void or refunded).",
			}
		case errors.Is(err, repository.ErrInvoiceNotFound):
			return repository.RefundAllResult{}, &BusinessError{
				Code:    "INVOICE_NOT_FOUND",
				Message: "Invoice not found.",
			}
		case errors.Is(err, repository.ErrRefundNotReverse):
			return repository.RefundAllResult{}, &BusinessError{
				Code:    "NO_PAYMENTS_TO_REFUND",
				Message: "No positive payments available to refund on this invoice.",
			}
		default:
			return repository.RefundAllResult{}, err
		}
	}
	return result, nil
}

// VoidInvoice is the explicit owner/admin action (BR-INV-008 + §4.4).
func (s *InvoiceService) VoidInvoice(
	ctx context.Context,
	invoiceID, userID uuid.UUID,
	reason string,
) (models.Invoice, error) {
	if reason == "" {
		return models.Invoice{}, &BusinessError{
			Code:    "VOID_REASON_REQUIRED",
			Message: "void_reason is required",
		}
	}
	inv, err := s.invoiceRepo.VoidInvoice(ctx, invoiceID, models.VoidInvoiceInput{
		VoidedBy:   userID,
		VoidReason: reason,
	})
	if err != nil {
		return models.Invoice{}, err
	}
	// Sync the booking payment_status (now unpaid/void).
	if sErr := s.SetBookingPaymentStatus(ctx, inv.BookingID); sErr != nil {
		log.Printf("SetBookingPaymentStatus after void %s: %v", inv.ID, sErr)
	}
	// Best-effort PDF regeneration with VOID watermark.
	s.regeneratePDFQuietly(ctx, inv.ID)
	return inv, nil
}

// RegeneratePDF is the public hook for §4.6. Tries the configured PDF
// generator and stores the URL on success. Returns the new URL.
//
// locale (B7-validation): the SPA's current language is read from the
// Accept-Language header by the handler and forwarded here, so the
// generated PDF matches whatever the user is reading the dashboard in.
// Empty locale → English fallback inside the generator.
func (s *InvoiceService) RegeneratePDF(ctx context.Context, invoiceID uuid.UUID, locale string) (string, error) {
	if s.pdfGen == nil {
		return "", &BusinessError{
			Code:    "PDF_NOT_CONFIGURED",
			Message: "PDF generator is not configured (B5 pending)",
		}
	}
	url, err := s.pdfGen.Generate(ctx, invoiceID, locale)
	if err != nil {
		return "", &BusinessError{
			Code:    "PDF_STORAGE_FAILED",
			Message: "Failed to regenerate PDF. You can retry from the invoice list.",
		}
	}
	if err := s.invoiceRepo.UpdatePDFURL(ctx, invoiceID, url); err != nil {
		return "", fmt.Errorf("store pdf url: %w", err)
	}
	return url, nil
}

// =============================================================================
// Internals
// =============================================================================

// canRefund returns true if the user is allowed to issue a refund for the
// given invoice. Per BR-INV-010 + R-02:
//   - Role "owner" → always allowed.
//   - Role "receptionist" → allowed if the linked booking has
//     force_override = TRUE.
//   - Any other role → denied.
func (s *InvoiceService) canRefund(ctx context.Context, invoiceID uuid.UUID, userID uuid.UUID, role UserRole) (bool, error) {
	if role == RoleOwner {
		return true, nil
	}
	if role != RoleReceptionist {
		return false, nil
	}
	// Look up the booking for the invoice and check its force_override flag.
	var bookingID uuid.UUID
	if err := s.db.QueryRow(ctx,
		`SELECT booking_id FROM invoices WHERE id = $1`, invoiceID).Scan(&bookingID); err != nil {
		return false, err
	}
	var forceOverride bool
	if err := s.db.QueryRow(ctx,
		`SELECT force_override FROM bookings WHERE id = $1`, bookingID).Scan(&forceOverride); err != nil {
		return false, err
	}
	return forceOverride, nil
}

// fetchPaymentForReplay re-fetches the payment that matches the original
// request. Simpler than re-marshalling the cached JSON.
func (s *InvoiceService) fetchPaymentForReplay(
	ctx context.Context,
	input models.RegisterPaymentInput,
	endpoint string,
	originalUserID uuid.UUID,
) (models.Payment, error) {
	// Look up the latest payment for this invoice that matches the
	// method+amount+is_reversal (cheap heuristic for "the original").
	row := s.db.QueryRow(ctx, `
		SELECT id, invoice_id, property_id, method, amount,
		       original_currency, exchange_rate, reference, notes,
		       is_reversal, reversal_of, received_by, received_at, created_at
		FROM payments
		WHERE invoice_id = $1 AND method = $2 AND amount = $3 AND is_reversal = $4
		ORDER BY received_at DESC
		LIMIT 1
	`, input.InvoiceID, input.Method, input.Amount, input.IsReversal)
	var p models.Payment
	if err := row.Scan(&p.ID, &p.InvoiceID, &p.PropertyID, &p.Method, &p.Amount,
		&p.OriginalCurrency, &p.ExchangeRate, &p.Reference, &p.Notes,
		&p.IsReversal, &p.ReversalOf, &p.ReceivedBy, &p.ReceivedAt, &p.CreatedAt); err != nil {
		return models.Payment{}, fmt.Errorf("idempotent replay lookup: %w", err)
	}
	return p, nil
}

// regeneratePDFQuietly tries to regenerate the PDF and silently logs
// failures. Used after void / cancel so the new VOID-watermarked PDF
// eventually replaces the old one (spec §6, §8.2).
func (s *InvoiceService) regeneratePDFQuietly(ctx context.Context, invoiceID uuid.UUID) {
	if s.pdfGen == nil {
		return
	}
	go func() {
		// Detached context: don't tie to the request lifecycle.
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		url, err := s.pdfGen.Generate(bgCtx, invoiceID, "") // locale empty → English fallback for async regen
		if err != nil {
			log.Printf("PDF regen failed for invoice %s: %v", invoiceID, err)
			return
		}
		if uerr := s.invoiceRepo.UpdatePDFURL(bgCtx, invoiceID, url); uerr != nil {
			log.Printf("PDF URL store failed for invoice %s: %v", invoiceID, uerr)
		}
	}()
}

// lineDescription builds the default line item description for a booking.
// It uses the room number when assigned; otherwise a generic description.
func lineDescription(b *models.BookingDetail) string {
	nights := 1
	if b.CheckOut.After(b.CheckIn) {
		n := int(b.CheckOut.Sub(b.CheckIn).Hours() / 24)
		if n > 0 {
			nights = n
		}
	}
	// Room number is not on the Booking model directly; use the booking ID
	// suffix as a stable but unique-enough label for MVP. Phase 2 can
	// join to rooms for a richer description.
	if b.RoomID != nil {
		short := b.RoomID.String()[:8]
		return fmt.Sprintf("Room %s - %d night(s)", short, nights)
	}
	return fmt.Sprintf("Booking %s - %d night(s)", b.ID.String()[:8], nights)
}

func ptr(s string) *string { return &s }
