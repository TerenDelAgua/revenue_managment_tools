package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/api/middleware"
	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
)

// =============================================================================
// HTTP handler tests — uses httptest with a real DB (skipped if unreachable).
// =============================================================================

func testHandlerDB(t *testing.T) *pgxpool.Pool {
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

// handlerFixture creates property + user + booking + invoice for the test.
type handlerFixture struct {
	propertyID uuid.UUID
	userID     uuid.UUID
	bookingID  uuid.UUID
	invoiceID  uuid.UUID
}

func newHandlerFixture(t *testing.T, db *pgxpool.Pool) handlerFixture {
	t.Helper()
	ctx := context.Background()
	propertyID := uuid.New()
	userID := uuid.New()
	guestID := uuid.New()
	roomID := uuid.New()
	bookingID := uuid.New()
	invoiceID := uuid.New()

	if _, err := db.Exec(ctx, `
		INSERT INTO properties (id, name, slug, currency, timezone)
		VALUES ($1::uuid, 'H ' || $1::text, 'h-' || substring($1::text, 1, 8), 'IDR', 'UTC')
	`, propertyID); err != nil {
		t.Fatalf("property: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, property_id, name, email, role)
		VALUES ($1::uuid, $2::uuid, 'H User', $3, 'owner')
	`, userID, propertyID, "hu-"+userID.String()[:8]+"@test.com"); err != nil {
		t.Fatalf("user: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO guests (id, property_id, full_name) VALUES ($1::uuid, $2::uuid, 'Guest')
	`, guestID, propertyID); err != nil {
		t.Fatalf("guest: %v", err)
	}
	var floorID, rtID uuid.UUID
	if err := db.QueryRow(ctx, `INSERT INTO floors (property_id, floor_number, label, sort_order) VALUES ($1::uuid, 99, 'H', 99) RETURNING id`, propertyID).Scan(&floorID); err != nil {
		t.Fatalf("floor: %v", err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO room_types (property_id, name, max_occupancy) VALUES ($1::uuid, 'H', 1) RETURNING id`, propertyID).Scan(&rtID); err != nil {
		t.Fatalf("room_type: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO rooms (id, property_id, floor_id, room_type_id, number, status, pos_x, pos_y)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'H', 'inactive', 0, 0)
	`, roomID, propertyID, floorID, rtID); err != nil {
		t.Fatalf("room: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, original_amount, total_amount, source, status, force_override)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, CURRENT_DATE, CURRENT_DATE + 3, 500000, 500000, 'walk_in', 'confirmed', FALSE)
	`, bookingID, propertyID, roomID, guestID, userID); err != nil {
		t.Fatalf("booking: %v", err)
	}
	if err := db.QueryRow(ctx, `
		INSERT INTO invoices (id, property_id, booking_id, invoice_number, subtotal, tax_amount, total, created_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'INV-T-' || substring($3::text, 1, 8), 500000, 55000, 555000, $4)
		RETURNING id
	`, invoiceID, propertyID, bookingID, userID).Scan(&invoiceID); err != nil {
		t.Fatalf("invoice: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM properties WHERE id = $1`, propertyID)
	})
	return handlerFixture{propertyID: propertyID, userID: userID, bookingID: bookingID, invoiceID: invoiceID}
}

// newRouter builds a chi router with the invoice routes wired, plus the
// AuthContext + IdempotencyKey middleware.
func newRouter(svc *service.InvoiceService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.AuthContext)
	r.Use(middleware.IdempotencyKey)

	h := NewInvoiceHandler(svc)
	r.Route("/api/v1/invoices", func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/daily-summary", h.DailySummary)
		r.Get("/tax-report", h.MonthlyTaxReport)
		r.Get("/by-booking/{bookingId}", h.GetByBookingID)
		r.Get("/{id}", h.GetByID)
		r.Patch("/{id}/notes", h.UpdateNotes)
		r.Post("/{id}/void", h.Void)
		r.Post("/{id}/payments", h.RegisterPayment)
		r.Post("/{id}/regenerate-pdf", h.RegeneratePDF)
	})
	return r
}

// doRequest sends a request through the test router. It returns the
// response recorder for assertions.
func doRequest(t *testing.T, r http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		bb, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(bb)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newInvoiceService(t *testing.T, db *pgxpool.Pool) *service.InvoiceService {
	return service.NewInvoiceService(
		db,
		repository.NewInvoiceRepository(db),
		repository.NewBookingRepository(db),
		nil,
	)
}

// =============================================================================
// Tests
// =============================================================================

func TestGetByID_ReturnsInvoice(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/"+f.invoiceID.String(), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got models.InvoiceDetail
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != f.invoiceID {
		t.Errorf("id: want %s, got %s", f.invoiceID, got.ID)
	}
	if got.InvoiceNumber != "INV-T-"+f.bookingID.String()[:8] {
		t.Errorf("invoice_number: %s", got.InvoiceNumber)
	}
	if got.Total != 555000 {
		t.Errorf("total: want 555000, got %v", got.Total)
	}
}

func TestGetByID_NotFoundReturns404(t *testing.T) {
	db := testHandlerDB(t)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/"+uuid.NewString(), nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", w.Code)
	}
}

func TestGetByID_InvalidIDReturns400(t *testing.T) {
	db := testHandlerDB(t)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/not-a-uuid", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestRegisterPayment_HappyPath(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments",
		map[string]any{
			"method":    "cash",
			"amount":    200000,
			"reference": "",
			"notes":     "Test payment",
		},
		map[string]string{"X-User-ID": f.userID.String(), "X-User-Role": "owner"},
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var p models.Payment
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Amount != 200000 {
		t.Errorf("amount: %v", p.Amount)
	}
}

func TestRegisterPayment_MissingAuthHeaderReturns401(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments",
		map[string]any{"method": "cash", "amount": 1000},
		nil, // no X-User-ID
	)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: want 401, got %d", w.Code)
	}
}

func TestRegisterPayment_NonCashRequiresReference(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments",
		map[string]any{
			"method":    "qris",
			"amount":    1000,
			"reference": "", // missing — required for QRIS per R-01
		},
		map[string]string{"X-User-ID": f.userID.String(), "X-User-Role": "owner"},
	)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: want 422, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "REFERENCE_REQUIRED") {
		t.Errorf("body should contain REFERENCE_REQUIRED code, got %s", w.Body.String())
	}
}

func TestRegisterPayment_IdempotencyReplay(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	idemKey := uuid.NewString()
	body := map[string]any{
		"method":    "bank_transfer",
		"amount":    50000,
		"reference": "TRF-IDEM-1",
	}
	headers := map[string]string{
		"X-User-ID":       f.userID.String(),
		"X-User-Role":     "owner",
		"Idempotency-Key": idemKey,
	}

	// First call
	w1 := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments", body, headers)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first: want 201, got %d, body=%s", w1.Code, w1.Body.String())
	}
	var p1 models.Payment
	_ = json.Unmarshal(w1.Body.Bytes(), &p1)

	// Second call (same key) → same payment ID
	w2 := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments", body, headers)
	if w2.Code != http.StatusCreated {
		t.Fatalf("second: want 201, got %d", w2.Code)
	}
	var p2 models.Payment
	_ = json.Unmarshal(w2.Body.Bytes(), &p2)

	if p1.ID != p2.ID {
		t.Errorf("idempotent replay: expected same ID %s, got %s", p1.ID, p2.ID)
	}

	// Verify only 1 payment was created
	var count int
	if err := db.QueryRow(context.Background(), `SELECT COUNT(*) FROM payments WHERE invoice_id = $1`, f.invoiceID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 payment, got %d", count)
	}
}

func TestRegisterPayment_InvalidIdempotencyKeyReturns400(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments",
		map[string]any{"method": "cash", "amount": 1000},
		map[string]string{
			"X-User-ID":       f.userID.String(),
			"Idempotency-Key": "not-a-uuid",
		},
	)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestVoid_RequiresReason(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/void",
		map[string]any{"reason": ""},
		map[string]string{"X-User-ID": f.userID.String(), "X-User-Role": "owner"},
	)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateNotes_HappyPath(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "PATCH", "/api/v1/invoices/"+f.invoiceID.String()+"/notes",
		map[string]any{"notes": "Some context"},
		map[string]string{"X-User-ID": f.userID.String(), "X-User-Role": "owner"},
	)
	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestList_RequiresPropertyID(t *testing.T) {
	db := testHandlerDB(t)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestList_ReturnsInvoicesAndHeaders(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/?property_id="+f.propertyID.String(), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Total-Count") == "" {
		t.Error("X-Total-Count header missing")
	}
	if w.Header().Get("X-Total-Collected") == "" {
		t.Error("X-Total-Collected header missing")
	}
}

func TestDailySummary_RequiresPropertyID(t *testing.T) {
	db := testHandlerDB(t)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/daily-summary", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestTaxReport_RequiresYear(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	w := doRequest(t, r, "GET", "/api/v1/invoices/tax-report?property_id="+f.propertyID.String(), nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestRegeneratePDF_NotConfiguredReturns501(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db)) // no PDF gen

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/regenerate-pdf", nil, nil)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status: want 501, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestRefundForbiddenReturns403(t *testing.T) {
	db := testHandlerDB(t)
	f := newHandlerFixture(t, db)
	r := newRouter(newInvoiceService(t, db))

	// First, pay the invoice fully (as owner) to create something to refund.
	_, err := service.NewInvoiceService(db,
		repository.NewInvoiceRepository(db),
		repository.NewBookingRepository(db),
		nil,
	).RegisterPayment(context.Background(), models.RegisterPaymentInput{
		InvoiceID:  f.invoiceID,
		PropertyID: f.propertyID,
		Method:     models.PaymentMethodCash,
		Amount:     555000,
		ReceivedBy: f.userID,
	}, nil, f.userID, service.RoleOwner)
	if err != nil {
		t.Fatalf("setup payment: %v", err)
	}

	// Now try a refund as a receptionist (no force_override on the booking).
	var paymentID uuid.UUID
	if err := db.QueryRow(context.Background(),
		`SELECT id FROM payments WHERE invoice_id = $1 ORDER BY received_at LIMIT 1`,
		f.invoiceID).Scan(&paymentID); err != nil {
		t.Fatalf("find payment: %v", err)
	}

	w := doRequest(t, r, "POST", "/api/v1/invoices/"+f.invoiceID.String()+"/payments",
		map[string]any{
			"method":      "cash",
			"amount":      -1000,
			"is_reversal": true,
			"reversal_of": paymentID.String(),
		},
		map[string]string{
			"X-User-ID":   f.userID.String(),
			"X-User-Role": "receptionist",
		},
	)
	if w.Code != http.StatusForbidden {
		t.Errorf("status: want 403, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "REFUND_FORBIDDEN") {
		t.Errorf("body should contain REFUND_FORBIDDEN, got %s", w.Body.String())
	}
}
