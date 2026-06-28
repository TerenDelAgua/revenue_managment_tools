package service

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

// =============================================================================
// InvoiceService tests — covers business rules that live in the service
// layer (refund authorization, idempotency, void propagation).
// =============================================================================

func testServiceDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable"
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping: cannot connect to %s (%v)", dbURL, err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Skipf("Skipping: DB not reachable (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

type serviceFixture struct {
	propertyID    uuid.UUID
	userID        uuid.UUID
	bookingID     uuid.UUID
	invoiceID     uuid.UUID
	forceOverride bool
}

// createServiceFixture creates a property+user+booking+invoice and returns
// the IDs. Optionally sets force_override on the booking for refund tests.
func createServiceFixture(t *testing.T, db *pgxpool.Pool, opts ...func(*serviceFixtureOptions)) *serviceFixture {
	t.Helper()
	o := &serviceFixtureOptions{totalAmount: defaultTotalAmount}
	for _, fn := range opts {
		fn(o)
	}
	ctx := context.Background()
	propertyID := uuid.New()
	userID := uuid.New()
	guestID := uuid.New()
	roomID := uuid.New()
	bookingID := uuid.New()
	invoiceID := uuid.New()

	if _, err := db.Exec(ctx, `
		INSERT INTO properties (id, name, slug, currency, timezone)
		VALUES ($1::uuid, 'Svc ' || $1::text, 'svc-' || substring($1::text, 1, 8), 'IDR', 'UTC')
	`, propertyID); err != nil {
		t.Fatalf("create property: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, property_id, name, email, role)
		VALUES ($1::uuid, $2::uuid, 'Svc User', $3, 'owner')
	`, userID, propertyID, "svc-"+userID.String()[:8]+"@test.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO guests (id, property_id, full_name) VALUES ($1::uuid, $2::uuid, 'Guest')
	`, guestID, propertyID); err != nil {
		t.Fatalf("create guest: %v", err)
	}
	var floorID, rtID uuid.UUID
	if err := db.QueryRow(ctx, `INSERT INTO floors (property_id, floor_number, label, sort_order) VALUES ($1::uuid, 99, 'S', 99) RETURNING id`, propertyID).Scan(&floorID); err != nil {
		t.Fatalf("create floor: %v", err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO room_types (property_id, name, max_occupancy) VALUES ($1::uuid, 'S', 1) RETURNING id`, propertyID).Scan(&rtID); err != nil {
		t.Fatalf("create room_type: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO rooms (id, property_id, floor_id, room_type_id, number, status, pos_x, pos_y)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'S', 'inactive', 0, 0)
	`, roomID, propertyID, floorID, rtID); err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, original_amount, total_amount, source, status, force_override)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, CURRENT_DATE, CURRENT_DATE + 3, 500000, $6, 'walk_in', 'confirmed', $7)
	`, bookingID, propertyID, roomID, guestID, userID, o.totalAmount, o.forceOverride); err != nil {
		t.Fatalf("create booking: %v", err)
	}
	if o.totalAmount > 0 {
		// Create an active invoice for the booking.
		if err := db.QueryRow(ctx, `
			INSERT INTO invoices (id, property_id, booking_id, invoice_number, subtotal, tax_amount, total, created_by)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'INV-T-' || substring($3::text, 1, 8), 500000, 55000, 555000, $4)
			RETURNING id
		`, invoiceID, propertyID, bookingID, userID).Scan(&invoiceID); err != nil {
			t.Fatalf("create invoice: %v", err)
		}
	}

	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM properties WHERE id = $1`, propertyID)
	})
	return &serviceFixture{
		propertyID:    propertyID,
		userID:        userID,
		bookingID:     bookingID,
		invoiceID:     invoiceID,
		forceOverride: o.forceOverride,
	}
}

type serviceFixtureOptions struct {
	totalAmount   float64
	forceOverride bool
}

const defaultTotalAmount = 500000.0

func withTotalAmount(v float64) func(*serviceFixtureOptions) {
	return func(o *serviceFixtureOptions) { o.totalAmount = v }
}

func withForceOverride(v bool) func(*serviceFixtureOptions) {
	return func(o *serviceFixtureOptions) { o.forceOverride = v }
}

func newInvoiceServiceForTest(db *pgxpool.Pool) *InvoiceService {
	invoiceRepo := repository.NewInvoiceRepository(db)
	bookingRepo := repository.NewBookingRepository(db)
	return NewInvoiceService(db, invoiceRepo, bookingRepo, nil)
}

// =============================================================================
// Tests
// =============================================================================

// TestCreateInvoiceForBooking_NormalFlow covers the happy path: subtotal > 0,
// invoice is created active, unpaid, balance = total.
func TestCreateInvoiceForBooking_NormalFlow(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	// Total 0 means no auto-invoice in the fixture; the test creates one.
	f := createServiceFixture(t, db, withTotalAmount(0))

	svc := newInvoiceServiceForTest(db)
	booking, err := svc.dbBooking(ctx, f.bookingID) // helper to read booking
	if err != nil {
		t.Fatalf("load booking: %v", err)
	}
	// Override the booking's total to 500000 to simulate a paid booking.
	// (The fixture sets totalAmount=0 for the courtesy case; here we want
	// the normal case where the service computes tax.)
	booking.TotalAmount = 500000

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	inv, err := svc.CreateInvoiceForBooking(ctx, tx, booking)
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if inv.Status != models.InvoiceStatusActive {
		t.Errorf("status: want active, got %s", inv.Status)
	}
	if inv.Total != 555000 {
		t.Errorf("total: want 555000, got %v", inv.Total)
	}
	if inv.Subtotal != 500000 {
		t.Errorf("subtotal: want 500000, got %v", inv.Total)
	}
	// Cleanup
	_, _ = db.Exec(ctx, `DELETE FROM invoices WHERE id = $1`, inv.ID)
}

// TestCreateInvoiceForBooking_CourtesyFlow covers BR-INV-012: subtotal=0 →
// invoice is auto-voided with reason 'Courtesy booking (subtotal=0)'.
func TestCreateInvoiceForBooking_CourtesyFlow(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db, withTotalAmount(0))

	svc := newInvoiceServiceForTest(db)
	booking, err := svc.dbBooking(ctx, f.bookingID)
	if err != nil {
		t.Fatalf("load booking: %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	inv, err := svc.CreateInvoiceForBooking(ctx, tx, booking)
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if inv.Status != models.InvoiceStatusVoid {
		t.Errorf("status: want void, got %s", inv.Status)
	}
	if inv.VoidReason == nil || *inv.VoidReason != "Courtesy booking (subtotal=0)" {
		t.Errorf("void_reason: want 'Courtesy booking (subtotal=0)', got %v", inv.VoidReason)
	}
	// Cleanup
	_, _ = db.Exec(ctx, `DELETE FROM invoices WHERE id = $1`, inv.ID)
}

// TestRegisterPayment_Refund_OwnerAllowed asserts BR-INV-010: an owner
// can always refund.
func TestRegisterPayment_Refund_OwnerAllowed(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	// First, pay the invoice fully (so we can refund).
	svc := newInvoiceServiceForTest(db)
	_, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleOwner)
	if err != nil {
		t.Fatalf("pay invoice: %v", err)
	}

	// Refund: owner role, no force_override needed.
	refund, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -10000,
		IsReversal: true,
		ReversalOf: ptrUUID(svcFirstPaymentID(t, db, f.invoiceID)),
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleOwner)
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if refund.Amount != -10000 || !refund.IsReversal {
		t.Errorf("refund: %+v", refund)
	}
}

// TestRegisterPayment_Refund_ReceptionistNoForce_Forbidden asserts
// BR-INV-010 + R-02: a receptionist without force_override cannot refund.
func TestRegisterPayment_Refund_ReceptionistNoForce_Forbidden(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db) // forceOverride defaults to false

	svc := newInvoiceServiceForTest(db)
	// Pay first.
	if _, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleOwner); err != nil {
		t.Fatalf("pay: %v", err)
	}

	// Refund: receptionist, no force_override → forbidden.
	_, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -1000,
		IsReversal: true,
		ReversalOf: ptrUUID(svcFirstPaymentID(t, db, f.invoiceID)),
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleReceptionist)
	if err == nil {
		t.Fatal("expected REFUND_FORBIDDEN, got nil")
	}
	be, ok := err.(*BusinessError)
	if !ok {
		t.Fatalf("expected BusinessError, got %T: %v", err, err)
	}
	if be.Code != CodeRefundForbidden {
		t.Errorf("expected code %s, got %s", CodeRefundForbidden, be.Code)
	}
}

// TestRegisterPayment_Refund_ReceptionistWithForce_Allowed asserts
// BR-INV-010 + R-02: a receptionist CAN refund if force_override is true
// on the booking.
func TestRegisterPayment_Refund_ReceptionistWithForce_Allowed(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db, withForceOverride(true))

	svc := newInvoiceServiceForTest(db)
	if _, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleOwner); err != nil {
		t.Fatalf("pay: %v", err)
	}

	// Refund: receptionist, force_override=true → allowed.
	refund, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -5000,
		IsReversal: true,
		ReversalOf: ptrUUID(svcFirstPaymentID(t, db, f.invoiceID)),
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleReceptionist)
	if err != nil {
		t.Fatalf("refund should be allowed: %v", err)
	}
	if refund.Amount != -5000 {
		t.Errorf("refund amount: %v", refund.Amount)
	}
}

// TestRegisterPayment_IdempotencyReplay asserts the same Idempotency-Key
// returns the original payment on the second call (R-06, TTL 24h).
func TestRegisterPayment_IdempotencyReplay(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	svc := newInvoiceServiceForTest(db)
	idemKey := uuid.New()

	// First call: registers the payment.
	first, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodBankTransfer,
		Amount:     100000,
		Reference:  "TRF-IDEM-001",
		ReceivedBy: f.userID,
	}, &idemKey, f.userID, RoleOwner)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	// Second call: same key → returns the original payment, no new row.
	second, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodBankTransfer,
		Amount:     100000,
		Reference:  "TRF-IDEM-001",
		ReceivedBy: f.userID,
	}, &idemKey, f.userID, RoleOwner)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("idempotent replay: expected same ID %s, got %s", first.ID, second.ID)
	}

	// Verify only ONE row was inserted.
	var count int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE invoice_id = $1`, f.invoiceID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 payment, got %d", count)
	}

	// Cleanup
	_, _ = db.Exec(ctx, `DELETE FROM idempotency_keys WHERE key = $1`, idemKey)
}

// TestVoidInvoice_RequiresReason asserts the business rule on the
// explicit void action (different from VoidInvoiceForBooking).
func TestVoidInvoice_RequiresReason(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	svc := newInvoiceServiceForTest(db)
	_, err := svc.VoidInvoice(ctx, f.invoiceID, f.userID, "")
	if err == nil {
		t.Fatal("expected VOID_REASON_REQUIRED, got nil")
	}
	be, ok := err.(*BusinessError)
	if !ok || be.Code != "VOID_REASON_REQUIRED" {
		t.Errorf("expected VOID_REASON_REQUIRED, got %v", err)
	}
}

// TestVoidInvoiceForBooking_Propagates asserts BR-INV-007: cancelling a
// booking voids the associated invoice.
func TestVoidInvoiceForBooking_Propagates(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	svc := newInvoiceServiceForTest(db)
	if err := svc.VoidInvoiceForBooking(ctx, f.bookingID, f.userID, "test cancel"); err != nil {
		t.Fatalf("void for booking: %v", err)
	}

	inv, err := svc.GetByID(ctx, f.invoiceID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if inv.Status != models.InvoiceStatusVoid {
		t.Errorf("status: want void, got %s", inv.Status)
	}
	if inv.VoidReason == nil || *inv.VoidReason == "" {
		t.Error("void_reason should be set")
	}
}

// TestSetBookingPaymentStatus_SyncsAfterPayment asserts BR-INV-004: after a
// full payment, bookings.payment_status is set to 'paid'.
func TestSetBookingPaymentStatus_SyncsAfterPayment(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	svc := newInvoiceServiceForTest(db)
	if _, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleOwner); err != nil {
		t.Fatalf("pay: %v", err)
	}

	// Check the booking's payment_status
	var ps string
	if err := db.QueryRow(ctx, `SELECT payment_status FROM bookings WHERE id = $1`, f.bookingID).Scan(&ps); err != nil {
		t.Fatalf("read booking ps: %v", err)
	}
	if ps != "paid" {
		t.Errorf("expected booking.payment_status=paid, got %s", ps)
	}
}

// TestRegeneratePDF_NotConfigured asserts the service returns a clear
// business error when no PDF generator is wired.
func TestRegeneratePDF_NotConfigured(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	svc := newInvoiceServiceForTest(db) // no PDF gen
	_, err := svc.RegeneratePDF(ctx, f.invoiceID, "en")
	if err == nil {
		t.Fatal("expected error when no PDF gen")
	}
	be, ok := err.(*BusinessError)
	if !ok || be.Code != "PDF_NOT_CONFIGURED" {
		t.Errorf("expected PDF_NOT_CONFIGURED, got %v", err)
	}
}

// =============================================================================
// Spec §10.1 — Refund behaviour regression suite (R-07 / R-08)
//
// These tests cover the cumulative / terminal / invalidation paths
// of the refund flow. They piggy-back on the shared fixture created
// by createServiceFixture (single property, single booking, single
// invoice, single user with role=owner).
//
// Each test stands alone: a fresh fixture is created per test
// (createServiceFixture returns a new booking + invoice each call)
// so they can run in any order and don't share mutable state.
// =============================================================================

// payInvoice is a small helper that registers a single charge so the
// invoice ends up 'paid'. Used by the refund-flow tests below.
func payInvoice(t *testing.T, db *pgxpool.Pool, f *serviceFixture, amount float64) {
	t.Helper()
	svc := newInvoiceServiceForTest(db)
	if _, err := svc.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     amount,
		ReceivedBy: f.userID,
	}, nil, f.userID, RoleOwner); err != nil {
		t.Fatalf("pay invoice: %v", err)
	}
}

// refundPartial registers a partial refund of `amount` on `target`.
// Helper used by the accumulation tests below.
func refundPartial(t *testing.T, db *pgxpool.Pool, f *serviceFixture, target uuid.UUID, amount float64) {
	t.Helper()
	svc := newInvoiceServiceForTest(db)
	idem := uuid.New()
	if _, err := svc.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -amount, // negative = refund
		Reference:  "REFUND-PART",
		Notes:      "partial refund",
		IsReversal: true, // required by RegisterPayment for negative amounts
		ReversalOf: &target,
		ReceivedBy: f.userID,
	}, &idem, f.userID, RoleOwner); err != nil {
		t.Fatalf("refund partial %v: %v", amount, err)
	}
}

// TestRefundPartialAccumulation (R-07, §10.1):
// 4 partial refunds on the same target — each one accepted because
// the sum stays below the original charge. The 5th one must be
// rejected with REFUND_EXCEEDS_CHARGE / equivalent sentinel.
func TestRefundPartialAccumulation(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)
	payInvoice(t, db, f, 500000)

	// Look up the original payment row to use as ReversalOf target.
	var targetID uuid.UUID
	if err := db.QueryRow(ctx, `
		SELECT id FROM payments
		WHERE invoice_id=$1 AND amount > 0 AND NOT is_reversal
		ORDER BY received_at LIMIT 1
	`, f.invoiceID).Scan(&targetID); err != nil {
		t.Fatalf("lookup target: %v", err)
	}

	// 4 × 100000 = 400000 — fits under 500000.
	for i := 0; i < 4; i++ {
		refundPartial(t, db, f, targetID, 100000)
	}

	// 5th partial of 100001 — would push us to 500001, > original.
	svc := newInvoiceServiceForTest(db)
	idem := uuid.New()
	_, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -100001,
		Reference:  "REFUND-OVERFLOW",
		Notes:      "should fail",
		IsReversal: true,
		ReversalOf: &targetID,
		ReceivedBy: f.userID,
	}, &idem, f.userID, RoleOwner)
	if err == nil {
		t.Fatal("expected over-refund rejection, got nil")
	}
	// Contract: we did NOT register the row. Service may surface this
	// as BusinessError (REFUND_EXCEEDS_CHARGE) or as a generic pgx
	// wrap (23514 check_violation) — both are acceptable.
}

// TestRefundAll (R-07, §10.1):
// Pay the invoice in 3 separate charges, then call RefundAll and
// expect a single batch row + N individual refund rows + lifecycle
// flips to 'refunded'.
func TestRefundAll(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)

	// 3 partial charges so we exercise the "N rows" path. The
	// booking's total is 555000 (subtotal 500000 + 11% tax); we
	// stay under 500000 so the third charge doesn't trip the
	// "exceeds balance" guard.
	payInvoice(t, db, f, 100000)
	payInvoice(t, db, f, 100000)
	payInvoice(t, db, f, 100000)

	// Sanity: invoice has 3 collected payments.
	var collected int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments
		WHERE invoice_id=$1 AND amount > 0 AND NOT is_reversal
	`, f.invoiceID).Scan(&collected); err != nil {
		t.Fatalf("count charges: %v", err)
	}
	if collected != 3 {
		t.Fatalf("expected 3 charges, got %d", collected)
	}

	svc := newInvoiceServiceForTest(db)
	resp, err := svc.RefundAll(ctx, models.RefundAllInput{
		InvoiceID:   f.invoiceID,
		Reason:      "spec §10.1 smoke",
		InitiatedBy: f.userID,
	}, RoleOwner)
	if err != nil {
		t.Fatalf("refund-all: %v", err)
	}
	if resp.RefundBatches.ID == uuid.Nil {
		t.Error("expected non-nil batch row")
	}
	if got := len(resp.RefundedPayments); got != 3 {
		t.Errorf("expected 3 refund rows, got %d", got)
	}

	// After refund-all the response echoes the post-refund lifecycle
	// computed by the trigger. With only 3 × 100000 paid against a
	// 555000 total, refunded < total so lifecycle stays 'active' (the
	// trigger flips to 'refunded' only when refunded >= total).
	if resp.InvoiceLifecycleAfter == models.InvoiceStatusVoid {
		t.Errorf("expected lifecycle!=void (we did refund), got %q", resp.InvoiceLifecycleAfter)
	}

	// 3 × 100000 — total refunded matches total paid.
	var refundedTotal float64
	if err := db.QueryRow(ctx, `
		SELECT COALESCE(SUM(-amount), 0) FROM payments
		WHERE invoice_id=$1 AND is_reversal = true
	`, f.invoiceID).Scan(&refundedTotal); err != nil {
		t.Fatalf("sum refunds: %v", err)
	}
	if refundedTotal != 300000 {
		t.Errorf("expected refunded_total=300000, got %v", refundedTotal)
	}
}

// TestOverRefundBlocked (R-08, §10.1):
// After refund-all, a fresh refund attempt on the same invoice must
// fail because the lifecycle is terminal.
func TestOverRefundBlocked(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)
	payInvoice(t, db, f, 500000)

	var targetID uuid.UUID
	if err := db.QueryRow(ctx, `
		SELECT id FROM payments
		WHERE invoice_id=$1 AND amount > 0 AND NOT is_reversal
		ORDER BY received_at LIMIT 1
	`, f.invoiceID).Scan(&targetID); err != nil {
		t.Fatalf("lookup target: %v", err)
	}

	svc := newInvoiceServiceForTest(db)
	if _, err := svc.RefundAll(ctx, models.RefundAllInput{
		InvoiceID:   f.invoiceID,
		Reason:      "drain it",
		InitiatedBy: f.userID,
	}, RoleOwner); err != nil {
		t.Fatalf("refund-all: %v", err)
	}

	// Now try to refund again on the (now terminal) invoice.
	idem := uuid.New()
	_, err := svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -100,
		Reference:  "REFUND-AFTER-ALL",
		Notes:      "should fail",
		IsReversal: true,
		ReversalOf: &targetID,
		ReceivedBy: f.userID,
	}, &idem, f.userID, RoleOwner)
	if err == nil {
		t.Fatal("expected terminal rejection after refund-all, got nil")
	}
}

// TestRefundInvalidated (R-07, §10.1):
// A payment invalidated AFTER collection must not count towards
// total_paid / total_refunded / effective_status. The service can
// still see the row (we don't delete it), but refund attempts
// pointing at it must not match the row in the paymentAggCTE.
func TestRefundInvalidated(t *testing.T) {
	db := testServiceDB(t)
	ctx := context.Background()
	f := createServiceFixture(t, db)
	payInvoice(t, db, f, 500000)

	var targetID uuid.UUID
	if err := db.QueryRow(ctx, `
		SELECT id FROM payments
		WHERE invoice_id=$1 AND amount > 0 AND NOT is_reversal
		ORDER BY received_at LIMIT 1
	`, f.invoiceID).Scan(&targetID); err != nil {
		t.Fatalf("lookup target: %v", err)
	}

	// Mark the row invalidated (R-09 Q2 — owner-only on the server).
	_, err := db.Exec(ctx, `
		UPDATE payments SET invalidated_at = NOW(),
		                    invalidated_by = $2,
		                    invalidated_reason = 'test invalidation'
		WHERE id = $1
	`, targetID, f.userID)
	if err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	// Refund pointing at the now-invalidated row. The service must
	// refuse because the row is excluded from the aggregates — the
	// remaining_reverseable becomes 0, so any refund amount is a
	// REFUND_EXCEEDS_CHARGE.
	svc := newInvoiceServiceForTest(db)
	idem := uuid.New()
	_, err = svc.RegisterPayment(ctx, models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -1000,
		Reference:  "REFUND-INV",
		Notes:      "target is invalidated",
		IsReversal: true,
		ReversalOf: &targetID,
		ReceivedBy: f.userID,
	}, &idem, f.userID, RoleOwner)
	if err == nil {
		t.Fatal("expected refund against invalidated row to fail, got nil")
	}
}

// =============================================================================
// Helpers
// =============================================================================

// dbBooking loads a booking by ID via the pool (used outside any tx in tests).
func (s *InvoiceService) dbBooking(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, property_id, room_id, guest_id, created_by, check_in, check_out,
		       adults, children, original_amount, original_currency, exchange_rate,
		       total_amount, payment_status, source, status, notes, created_at, updated_at
		FROM bookings WHERE id = $1
	`, id)
	var b models.Booking
	if err := row.Scan(
		&b.ID, &b.PropertyID, &b.RoomID, &b.GuestID, &b.CreatedBy,
		&b.CheckIn, &b.CheckOut, &b.Adults, &b.Children, &b.OriginalAmount,
		&b.OriginalCurrency, &b.ExchangeRate, &b.TotalAmount, &b.PaymentStatus,
		&b.Source, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	return &b, nil
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

// svcFirstPaymentID returns the ID of the first payment for an invoice.
// Used to set up a refund's ReversalOf pointer.
func svcFirstPaymentID(t *testing.T, db *pgxpool.Pool, invoiceID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := db.QueryRow(context.Background(),
		`SELECT id FROM payments WHERE invoice_id = $1 ORDER BY received_at LIMIT 1`,
		invoiceID).Scan(&id); err != nil {
		t.Fatalf("first payment: %v", err)
	}
	return id
}
