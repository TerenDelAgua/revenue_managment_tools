package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

// =============================================================================
// Test harness — fixture builders shared across B2 tests
// =============================================================================

func testDB(t *testing.T) *pgxpool.Pool {
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

type fixture struct {
	propertyID uuid.UUID
	userID     uuid.UUID
	bookingID  uuid.UUID
	bookings   []uuid.UUID
}

// createFixture creates a property + user + a single booking. For tests
// that need multiple invoices per property, use createFixtureWithBookings
// to control the number of bookings.
func createFixture(ctx context.Context, t *testing.T, db *pgxpool.Pool) fixture {
	return createFixtureWithBookings(ctx, t, db, 1)
}

// createFixtureWithBookings creates a property + user + N bookings. Each
// booking can host one invoice (FK UNIQUE booking_id). The returned
// fixture's `bookingID` is set to `bookings[0]` so simple tests work.
func createFixtureWithBookings(ctx context.Context, t *testing.T, db *pgxpool.Pool, n int) fixture {
	t.Helper()
	propertyID := uuid.New()
	userID := uuid.New()
	guestID := uuid.New()
	roomID := uuid.New()
	bookings := make([]uuid.UUID, n)

	if _, err := db.Exec(ctx, `
		INSERT INTO properties (id, name, slug, currency, timezone)
		VALUES ($1::uuid, 'Test ' || $1::text, 'test-' || substring($1::text, 1, 8), 'IDR', 'UTC')
	`, propertyID); err != nil {
		t.Fatalf("create property: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, property_id, name, email, role)
		VALUES ($1::uuid, $2::uuid, 'Test User', $3, 'owner')
	`, userID, propertyID, "u-"+userID.String()[:8]+"@test.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO guests (id, property_id, full_name) VALUES ($1::uuid, $2::uuid, 'Guest')
	`, guestID, propertyID); err != nil {
		t.Fatalf("create guest: %v", err)
	}
	// Floor + room type required by rooms FK NOT NULL constraints
	var floorID, rtID uuid.UUID
	if err := db.QueryRow(ctx, `
		INSERT INTO floors (property_id, floor_number, label, sort_order)
		VALUES ($1::uuid, 99, 'Test Floor', 99) RETURNING id
	`, propertyID).Scan(&floorID); err != nil {
		t.Fatalf("create floor: %v", err)
	}
	if err := db.QueryRow(ctx, `
		INSERT INTO room_types (property_id, name, max_occupancy)
		VALUES ($1::uuid, 'Test RT', 1) RETURNING id
	`, propertyID).Scan(&rtID); err != nil {
		t.Fatalf("create room_type: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO rooms (id, property_id, floor_id, room_type_id, number, status, pos_x, pos_y)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'TEST', 'inactive', 0, 0)
	`, roomID, propertyID, floorID, rtID); err != nil {
		t.Fatalf("create room: %v", err)
	}
	for i := 0; i < n; i++ {
		bid := uuid.New()
		bookings[i] = bid
		if _, err := db.Exec(ctx, `
			INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, CURRENT_DATE, CURRENT_DATE + 3, 500000, 'walk_in', 'confirmed')
		`, bid, propertyID, roomID, guestID, userID); err != nil {
			t.Fatalf("create booking %d: %v", i, err)
		}
	}

	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM properties WHERE id = $1`, propertyID)
	})
	// Set bookingID to the first booking so simple tests (using f.bookingID) work
	var primaryBookingID uuid.UUID
	if n > 0 {
		primaryBookingID = bookings[0]
	}
	return fixture{propertyID: propertyID, userID: userID, bookingID: primaryBookingID, bookings: bookings}
}

// createInvoice opens a tx, calls the repo's create method, commits, and
// returns the resulting detail. Cleans up at end of test. Uses the
// default booking from the fixture.
func createInvoice(t *testing.T, db *pgxpool.Pool, f fixture, ppnRate float64) models.InvoiceDetail {
	return createInvoiceForBooking(t, db, f, f.bookingID, ppnRate)
}

// createInvoiceForBooking creates an invoice for a specific booking ID.
func createInvoiceForBooking(t *testing.T, db *pgxpool.Pool, f fixture, bookingID uuid.UUID, ppnRate float64) models.InvoiceDetail {
	t.Helper()
	repo := NewInvoiceRepository(db)
	ctx := context.Background()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	detail, err := repo.CreateInvoiceWithTx(ctx, tx, models.CreateInvoiceInput{
		PropertyID: f.propertyID,
		BookingID:  bookingID,
		Subtotal:   500000,
		PPNRate:    ppnRate,
		LineItems: []models.NewLineItem{
			{Description: "Room TEST - 3 nights", Quantity: 3, UnitPrice: 166666.67, SortOrder: 0},
		},
		CreatedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM invoices WHERE id = $1`, detail.ID)
	})
	return detail
}

// =============================================================================
// Tests
// =============================================================================

// TestGetPPNRate_DefaultReturns11 verifies the default PPN rate is 0.11
// when the property has no settings.
func TestGetPPNRate_DefaultReturns11(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewInvoiceRepository(db)
	f := createFixture(ctx, t, db)

	rate, err := repo.GetPPNRate(ctx, f.propertyID)
	if err != nil {
		t.Fatalf("GetPPNRate: %v", err)
	}
	if rate != 0.11 {
		t.Errorf("expected 0.11, got %v", rate)
	}
}

// TestGetPPNRate_CustomRate verifies a property-level override is honored.
func TestGetPPNRate_CustomRate(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewInvoiceRepository(db)
	f := createFixture(ctx, t, db)

	if _, err := db.Exec(ctx, `UPDATE properties SET settings = $1 WHERE id = $2`,
		[]byte(`{"ppn_rate": 0.12}`), f.propertyID); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	rate, err := repo.GetPPNRate(ctx, f.propertyID)
	if err != nil {
		t.Fatalf("GetPPNRate: %v", err)
	}
	if rate != 0.12 {
		t.Errorf("expected 0.12, got %v", rate)
	}
}

// TestCreateInvoiceWithTx_ComputesTotalsAndPersists verifies the math
// (subtotal + tax = total) and persistence (line items, invoice_number).
func TestCreateInvoiceWithTx_ComputesTotalsAndPersists(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)

	detail := createInvoice(t, db, f, 0.11)

	// Math: 500000 + 11% = 500000 + 55000 = 555000
	if detail.Subtotal != 500000 {
		t.Errorf("subtotal: want 500000, got %v", detail.Subtotal)
	}
	if detail.TaxAmount != 55000 {
		t.Errorf("tax: want 55000, got %v", detail.TaxAmount)
	}
	if detail.Total != 555000 {
		t.Errorf("total: want 555000, got %v", detail.Total)
	}
	if detail.PPNRateSnapshot != 0.11 {
		t.Errorf("ppn_rate_snapshot: want 0.11, got %v", detail.PPNRateSnapshot)
	}
	if detail.EffectiveStatus != models.PaymentStatusUnpaid {
		t.Errorf("effective_status: want unpaid, got %s", detail.EffectiveStatus)
	}
	if detail.Balance != 555000 {
		t.Errorf("balance: want 555000, got %v", detail.Balance)
	}
	if len(detail.LineItems) != 1 {
		t.Errorf("line items: want 1, got %d", len(detail.LineItems))
	}
	if detail.InvoiceNumber == "" {
		t.Error("invoice_number not set")
	}
	if !startsWith(detail.InvoiceNumber, "INV-") {
		t.Errorf("invoice_number should start with INV-, got %s", detail.InvoiceNumber)
	}

	// Re-fetch and verify
	got, err := repo.GetInvoiceByID(context.Background(), detail.ID)
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if got.Total != 555000 || got.InvoiceNumber != detail.InvoiceNumber {
		t.Errorf("refetch mismatch: %+v vs %+v", got, detail)
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestRegisterPayment_ValidatesBalance asserts that an amount > remaining
// returns ErrPaymentExceeds and doesn't insert.
func TestRegisterPayment_ValidatesBalance(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	detail := createInvoice(t, db, f, 0.11)

	// First payment: 300000 (partial). Should succeed.
	p1, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodBankTransfer,
		Amount:     300000,
		Reference:  "TRF-001",
		ReceivedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("first payment: %v", err)
	}
	if p1.Amount != 300000 {
		t.Errorf("payment amount: want 300000, got %v", p1.Amount)
	}

	// Second payment: 300000 → 600000 paid of 555000 → exceeds.
	_, err = repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     300000,
		ReceivedBy: f.userID,
	})
	if !errors.Is(err, ErrPaymentExceeds) {
		t.Errorf("expected ErrPaymentExceeds, got %v", err)
	}

	// Third payment: 255000 (final). Should succeed and stamp paid_at.
	p3, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     255000,
		ReceivedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("final payment: %v", err)
	}
	if p3.Amount != 255000 {
		t.Errorf("payment amount: want 255000, got %v", p3.Amount)
	}

	// Verify state via re-fetch
	got, err := repo.GetInvoiceByID(context.Background(), detail.ID)
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if got.EffectiveStatus != models.PaymentStatusPaid {
		t.Errorf("status: want paid, got %s", got.EffectiveStatus)
	}
	if got.Balance != 0 {
		t.Errorf("balance: want 0, got %v", got.Balance)
	}
	if got.PaidAt == nil {
		t.Error("paid_at should be set")
	}
}

// TestRegisterPayment_RequiresReferenceForNonCash asserts BR-INV-005 / R-01.
func TestRegisterPayment_RequiresReferenceForNonCash(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	detail := createInvoice(t, db, f, 0.11)

	cases := []models.PaymentMethod{
		models.PaymentMethodBankTransfer,
		models.PaymentMethodQris,
		models.PaymentMethodCard,
	}
	for _, m := range cases {
		_, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
			InvoiceID:  detail.ID,
			PropertyID: f.propertyID,
			Method:     m,
			Amount:     1000,
			Reference:  "", // missing
			ReceivedBy: f.userID,
		})
		if !errors.Is(err, ErrReferenceRequired) {
			t.Errorf("method=%s: expected ErrReferenceRequired, got %v", m, err)
		}
	}

	// Cash with no reference is fine.
	_, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     1000,
		ReceivedBy: f.userID,
	})
	if err != nil {
		t.Errorf("cash without ref should be ok, got %v", err)
	}
}

// TestRegisterPayment_RefundFlow asserts refund validations and linkage.
func TestRegisterPayment_RefundFlow(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	detail := createInvoice(t, db, f, 0.11)

	// Original payment (covers the whole invoice)
	original, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("original: %v", err)
	}

	// Refund: amount < 0 with is_reversal=true, reversal_of set
	refund, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -50000,
		IsReversal: true,
		ReversalOf: &original.ID,
		ReceivedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if refund.Amount != -50000 || !refund.IsReversal {
		t.Errorf("refund: %+v", refund)
	}

	// Refund without is_reversal=true → rejected
	_, err = repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     -100,
		ReceivedBy: f.userID,
	})
	if err == nil {
		t.Error("expected error: refund without is_reversal=true")
	}

	// Verify total_refunded includes the 50k
	got, _ := repo.GetInvoiceByID(context.Background(), detail.ID)
	if got.TotalRefunded != 50000 {
		t.Errorf("total_refunded: want 50000, got %v", got.TotalRefunded)
	}
	if got.EffectiveStatus != models.PaymentStatusPaid {
		t.Errorf("paid - 50k refund still = paid, got %s", got.EffectiveStatus)
	}
}

// TestVoidInvoice_RequiresAudit asserts the trigger behavior + service-level
// check (we return ErrInvoiceVoid on already-voided).
func TestVoidInvoice_RequiresAudit(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	detail := createInvoice(t, db, f, 0.11)

	// First void: success
	voided, err := repo.VoidInvoice(context.Background(), detail.ID, models.VoidInvoiceInput{
		VoidedBy:   f.userID,
		VoidReason: "Test void",
	})
	if err != nil {
		t.Fatalf("first void: %v", err)
	}
	if voided.Status != models.InvoiceStatusVoid {
		t.Errorf("status: want void, got %s", voided.Status)
	}
	if voided.VoidedBy == nil || *voided.VoidedBy != f.userID {
		t.Errorf("voided_by not set")
	}
	if voided.VoidedAt == nil {
		t.Error("voided_at not set")
	}

	// Second void on the same invoice → ErrInvoiceVoid
	_, err = repo.VoidInvoice(context.Background(), detail.ID, models.VoidInvoiceInput{
		VoidedBy:   f.userID,
		VoidReason: "Second void",
	})
	if !errors.Is(err, ErrInvoiceVoid) {
		t.Errorf("expected ErrInvoiceVoid, got %v", err)
	}

	// Payments on a voided invoice → ErrInvoiceVoid
	_, err = repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     1000,
		ReceivedBy: f.userID,
	})
	if !errors.Is(err, ErrInvoiceVoid) {
		t.Errorf("payment on void: expected ErrInvoiceVoid, got %v", err)
	}
}

// TestListInvoices_FiltersByStatus asserts the effective_status filter.
func TestListInvoices_FiltersByStatus(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixtureWithBookings(context.Background(), t, db, 3)

	// Create 3 invoices in different states (one per booking)
	inv1 := createInvoiceForBooking(t, db, f, f.bookings[0], 0.11) // unpaid
	inv2 := createInvoiceForBooking(t, db, f, f.bookings[1], 0.11) // will be paid
	inv3 := createInvoiceForBooking(t, db, f, f.bookings[2], 0.11) // will be partial

	// Pay inv2 fully
	if _, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  inv2.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}); err != nil {
		t.Fatalf("pay inv2: %v", err)
	}

	// Partially pay inv3
	if _, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  inv3.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     100000,
		ReceivedBy: f.userID,
	}); err != nil {
		t.Fatalf("partial inv3: %v", err)
	}

	// List with status=paid
	rows, total, err := repo.ListInvoices(context.Background(), models.ListInvoicesFilter{
		PropertyID: f.propertyID,
		Status:     models.PaymentStatusPaid,
	})
	if err != nil {
		t.Fatalf("list paid: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 paid, got %d", total)
	}
	for _, r := range rows {
		if r.EffectiveStatus != models.PaymentStatusPaid {
			t.Errorf("row %s should be paid, got %s", r.InvoiceNumber, r.EffectiveStatus)
		}
	}

	// List with status=unpaid
	rows, total, err = repo.ListInvoices(context.Background(), models.ListInvoicesFilter{
		PropertyID: f.propertyID,
		Status:     models.PaymentStatusUnpaid,
	})
	if err != nil {
		t.Fatalf("list unpaid: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 unpaid, got %d", total)
	}
	_ = inv1
}

// TestDailySummary_Aggregates asserts the end-of-day cash-closing numbers.
func TestDailySummary_Aggregates(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixtureWithBookings(context.Background(), t, db, 2)

	// 2 invoices: one paid, one partial
	inv1 := createInvoiceForBooking(t, db, f, f.bookings[0], 0.11)
	inv2 := createInvoiceForBooking(t, db, f, f.bookings[1], 0.11)

	if _, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  inv1.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}); err != nil {
		t.Fatalf("pay inv1: %v", err)
	}
	if _, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  inv2.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodQris,
		Amount:     200000,
		Reference:  "QR-001",
		ReceivedBy: f.userID,
	}); err != nil {
		t.Fatalf("partial inv2: %v", err)
	}

	summary, err := repo.DailySummary(context.Background(), f.propertyID, time.Now().UTC(), "UTC")
	if err != nil {
		t.Fatalf("DailySummary: %v", err)
	}

	if summary.InvoicesIssued < 2 {
		t.Errorf("invoices_issued: want >= 2, got %d", summary.InvoicesIssued)
	}
	if summary.InvoicesPaid < 1 {
		t.Errorf("invoices_paid: want >= 1, got %d", summary.InvoicesPaid)
	}
	if summary.InvoicesPartial < 1 {
		t.Errorf("invoices_partial: want >= 1, got %d", summary.InvoicesPartial)
	}
	if summary.TotalCollected < 755000 {
		t.Errorf("total_collected: want >= 755000, got %v", summary.TotalCollected)
	}
	if summary.ByMethod[models.PaymentMethodCash] < 555000 {
		t.Errorf("cash total: want >= 555000, got %v", summary.ByMethod[models.PaymentMethodCash])
	}
	if summary.ByMethod[models.PaymentMethodQris] < 200000 {
		t.Errorf("qris total: want >= 200000, got %v", summary.ByMethod[models.PaymentMethodQris])
	}
	if len(summary.StaffBreakdown) < 1 {
		t.Errorf("staff_breakdown: want >= 1, got %d", len(summary.StaffBreakdown))
	}
}

// TestMonthlyTaxReport_ComputesPPN asserts the PPN aggregation.
func TestMonthlyTaxReport_ComputesPPN(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	_ = createInvoice(t, db, f, 0.11) // 500000 subtotal, 55000 PPN

	now := time.Now().UTC()
	report, err := repo.MonthlyTaxReport(context.Background(), f.propertyID, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("MonthlyTaxReport: %v", err)
	}
	if report.TotalSubtotal < 500000 {
		t.Errorf("total_subtotal: want >= 500000, got %v", report.TotalSubtotal)
	}
	if report.TotalTax < 55000 {
		t.Errorf("total_tax: want >= 55000, got %v", report.TotalTax)
	}
	if report.InvoicesCount < 1 {
		t.Errorf("invoices_count: want >= 1, got %d", report.InvoicesCount)
	}
	if report.NetTaxCollected != report.TotalTax {
		t.Errorf("net_tax_collected should equal total_tax in MVP")
	}
}

// TestIdempotency_DedupeByKey asserts the same key returns the same response.
func TestIdempotency_DedupeByKey(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	ctx := context.Background()
	f := createFixture(ctx, t, db) // gives us a valid user_id

	key := uuid.New()
	// Use a payload that jsonb stores as-is (no whitespace normalization
	// happens because jsonb only canonicalizes structure, not the body
	// itself for non-numeric keys). We compare by content equality.
	body := []byte(`{"id":"abc","status":"paid"}`)

	// First save: stores the body
	if err := repo.SaveIdempotentResponse(ctx, key, f.userID, "POST /invoices/x/payments", body); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Read: returns the body
	resp, found, err := repo.GetIdempotentResponse(ctx, key)
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	// jsonb may reorder keys but bytes should be equivalent
	if !jsonBytesEqual(resp.ResponseBody, body) {
		t.Errorf("body mismatch: want %s, got %s", body, resp.ResponseBody)
	}

	// Save again with same key: ON CONFLICT DO NOTHING, so still the original body
	body2 := []byte(`{"different":"body"}`)
	if err := repo.SaveIdempotentResponse(ctx, key, f.userID, "POST", body2); err != nil {
		t.Fatalf("second save: %v", err)
	}
	resp, _, _ = repo.GetIdempotentResponse(ctx, key)
	if !jsonBytesEqual(resp.ResponseBody, body) {
		t.Errorf("expected original body to be preserved, got %s", resp.ResponseBody)
	}

	// Cleanup
	_, _ = db.Exec(ctx, `DELETE FROM idempotency_keys WHERE key = $1`, key)
}

// TestUpdateNotes_OnlyOnActiveInvoice asserts voided invoices can't be edited.
func TestUpdateNotes_OnlyOnActiveInvoice(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	detail := createInvoice(t, db, f, 0.11)

	// Active: notes update works
	_, err := repo.UpdateNotes(context.Background(), detail.ID, "Some notes")
	if err != nil {
		t.Fatalf("update notes: %v", err)
	}

	// Void it
	if _, err := repo.VoidInvoice(context.Background(), detail.ID, models.VoidInvoiceInput{
		VoidedBy: f.userID, VoidReason: "test",
	}); err != nil {
		t.Fatalf("void: %v", err)
	}

	// Voided: notes update should fail
	_, err = repo.UpdateNotes(context.Background(), detail.ID, "Other notes")
	if !errors.Is(err, pgx.ErrNoRows) && err == nil {
		t.Errorf("expected error on voided invoice notes update, got %v", err)
	}
}

// TestRegisterPayment_AcceptsRoundedTotal covers the bug from the
// last smoke session: paying the exact total (722) on an invoice whose
// stored total is 721.50 (subtotal 650 + 11% tax) used to 422 with
// "amount exceeds the remaining balance". The fix rounds both sides
// to 2 decimals before comparing and widens the drift tolerance
// from 0.0001 to 0.01, which is the smallest unit a human enters
// on a calculator for IDR amounts.
//
// This test forces the stored invoice.total to 721.50 directly via
// UPDATE (the createInvoice helper hard-codes subtotal=500000 and
// would otherwise land at 555000) and then pays the rounded 722.
func TestRegisterPayment_AcceptsRoundedTotal(t *testing.T) {
	db := testDB(t)
	repo := NewInvoiceRepository(db)
	f := createFixture(context.Background(), t, db)
	detail := createInvoice(t, db, f, 0.11)

	// Force the stored total to 721.50 (650 + 11% tax). This mimics
	// the bug fixture from INV-2026-0006.
	if _, err := db.Exec(context.Background(),
		`UPDATE invoices SET subtotal = 650, tax_amount = 71.50, total = 721.50 WHERE id = $1`,
		detail.ID); err != nil {
		t.Fatalf("force total: %v", err)
	}

	// Sanity re-fetch: stored total is now 721.50.
	got, err := repo.GetInvoiceByID(context.Background(), detail.ID)
	if err != nil {
		t.Fatalf("refetch after force: %v", err)
	}
	if got.Total != 721.50 {
		t.Fatalf("expected total=721.50, got %v", got.Total)
	}

	// Pay the rounded-up total. This is what the frontend sends when
	// the user clicks "MAX" or types 722 against a UI that shows the
	// total rounded to the nearest integer.
	p, err := repo.RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  detail.ID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     722,
		ReceivedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("expected rounded payment 722 to be accepted, got %v", err)
	}
	if p.Amount != 722 {
		t.Errorf("expected stored amount=722, got %v", p.Amount)
	}

	// Re-fetch and verify the invoice is paid (or overpaid by the
	// 0.50 round-up — that's a feature, not a bug, since the user
	// was shown the rounded total on the UI).
	got, err = repo.GetInvoiceByID(context.Background(), detail.ID)
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if got.EffectiveStatus != models.PaymentStatusPaid &&
		got.EffectiveStatus != models.PaymentStatusOverpaid {
		t.Errorf("expected status=paid|overpaid, got %s", got.EffectiveStatus)
	}
}

// jsonBytesEqual compares two JSON byte slices for semantic equality,
// ignoring whitespace and key order. Used for tests where jsonb may
// canonicalize the stored payload.
func jsonBytesEqual(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}
