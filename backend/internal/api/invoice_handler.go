package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/api/middleware"
	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

// =============================================================================
// InvoiceHandler — HTTP layer for the Invoicing & Payments module
// Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4
//
// Mapping (B4 responsibility):
//   - HTTP request  → service DTO + auth context (X-User-ID / X-User-Role)
//   - Service error → JSON response with code/message/hint (Tone of Voice)
//   - Service DTO   → HTTP response (mostly pass-through; some reshaping)
//
// Idempotency-Key: read from request via the IdempotencyKey middleware,
// passed to the service. Service handles the dedup logic (R-06).
// =============================================================================

type InvoiceHandler struct {
	svc *service.InvoiceService
}

func NewInvoiceHandler(svc *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{svc: svc}
}

// =============================================================================
// Read endpoints
// =============================================================================

// GetByID — spec §4.2
func (h *InvoiceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid invoice ID"})
		return
	}
	d, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			api.JSON(w, http.StatusNotFound, api.Error{Code: "INVOICE_NOT_FOUND", Message: "Invoice not found"})
			return
		}
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, d)
}

// GetByBookingID — spec §4.1 (alternative URL form; the canonical
// /bookings/:bookingId/invoice lives in the booking router)
func (h *InvoiceHandler) GetByBookingID(w http.ResponseWriter, r *http.Request) {
	bookingID, err := uuid.Parse(chi.URLParam(r, "bookingId"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid booking ID"})
		return
	}
	d, err := h.svc.GetByBookingID(r.Context(), bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			api.JSON(w, http.StatusNotFound, api.Error{Code: "INVOICE_NOT_FOUND", Message: "Invoice not found"})
			return
		}
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, d)
}

// List — spec §4.7
func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	propertyIDStr := r.URL.Query().Get("property_id")
	if propertyIDStr == "" {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "MISSING_PARAM", Message: "property_id is required"})
		return
	}
	propertyID, err := uuid.Parse(propertyIDStr)
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid property_id"})
		return
	}

	f := models.ListInvoicesFilter{PropertyID: propertyID}
	if v := r.URL.Query().Get("status"); v != "" {
		f.Status = models.PaymentStatus(v)
	}
	if v := r.URL.Query().Get("date_from"); v != "" {
		if t, err := parseDate(v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		if t, err := parseDate(v); err == nil {
			f.DateTo = &t
		}
	}
	if v := r.URL.Query().Get("search"); v != "" {
		f.Search = v
	}
	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			f.Page = p
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			f.Limit = l
		}
	}

	rows, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Spec §4.7 — headers X-Total-Count, X-Total-Collected, X-Total-Pending, X-Total-Tax
	totalCollected, totalPending, totalTax := aggregateHeaders(rows)
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Total-Collected", formatFloat(totalCollected))
	w.Header().Set("X-Total-Pending", formatFloat(totalPending))
	w.Header().Set("X-Total-Tax", formatFloat(totalTax))
	api.JSON(w, http.StatusOK, map[string]any{
		"invoices": rows,
		"pagination": map[string]any{
			"page":  f.Page,
			"limit": f.Limit,
			"total": total,
		},
	})
}

// DailySummary — spec §4.9
func (h *InvoiceHandler) DailySummary(w http.ResponseWriter, r *http.Request) {
	propertyID, err := uuid.Parse(r.URL.Query().Get("property_id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "MISSING_PARAM", Message: "property_id is required"})
		return
	}
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	if dateStr == "" {
		date = time.Now().UTC()
	} else if d, err := parseDate(dateStr); err == nil {
		date = d
	} else {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_DATE", Message: "date must be YYYY-MM-DD or RFC3339"})
		return
	}
	tz := r.URL.Query().Get("tz")
	if tz == "" {
		tz = "UTC"
	}
	summary, err := h.svc.DailySummary(r.Context(), propertyID, date, tz)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, summary)
}

// MonthlyTaxReport — spec §4.11
func (h *InvoiceHandler) MonthlyTaxReport(w http.ResponseWriter, r *http.Request) {
	propertyID, err := uuid.Parse(r.URL.Query().Get("property_id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "MISSING_PARAM", Message: "property_id is required"})
		return
	}
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 2000 || year > 2100 {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_YEAR", Message: "year is required (4-digit)"})
		return
	}
	month := 0 // 0 → whole year
	if v := r.URL.Query().Get("month"); v != "" {
		if m, err := strconv.Atoi(v); err == nil && m >= 1 && m <= 12 {
			month = m
		}
	}
	report, err := h.svc.MonthlyTaxReport(r.Context(), propertyID, year, month)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, report)
}

// =============================================================================
// Write endpoints
// =============================================================================

// RegisterPayment — spec §4.3 + R-01 + R-02 + R-06
func (h *InvoiceHandler) RegisterPayment(w http.ResponseWriter, r *http.Request) {
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid invoice ID"})
		return
	}
	var req struct {
		Method        string  `json:"method"`
		Amount        float64 `json:"amount"`
		Reference     string  `json:"reference"`
		Notes         string  `json:"notes"`
		IsReversal    bool    `json:"is_reversal"`
		ReversalOf    *string `json:"reversal_of"`
		ForceOverride bool    `json:"force_override"` // v1.2 R-07
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_BODY", Message: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		api.JSON(w, http.StatusUnauthorized, api.Error{Code: "UNAUTHENTICATED", Message: "Missing X-User-ID header (dev auth)"})
		return
	}
	roleStr, _ := middleware.UserRoleFromContext(r.Context())
	role := service.UserRole(roleStr)
	if roleStr == "" {
		role = service.RoleOwner // dev default
	}
	idemKey, _ := middleware.IdempotencyKeyFromContext(r.Context())

	// Look up property_id from the invoice (1 quick read).
	propertyID, ok := h.propertyIDForInvoice(r.Context(), invoiceID, w)
	if !ok {
		return
	}

	input := models.RegisterPaymentInput{
		InvoiceID:  invoiceID,
		PropertyID: propertyID,
		Method:     models.PaymentMethod(req.Method),
		Amount:     req.Amount,
		Reference:  req.Reference,
		Notes:      req.Notes,
		IsReversal: req.IsReversal,
		ReceivedBy: userID,
	}
	if req.ReversalOf != nil {
		rid, err := uuid.Parse(*req.ReversalOf)
		if err != nil {
			api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "reversal_of must be a UUID"})
			return
		}
		input.ReversalOf = &rid
	}
	input.ForceOverride = req.ForceOverride

	p, err := h.svc.RegisterPayment(r.Context(), input, idemKey, userID, role)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentExceeds) {
			api.JSON(w, http.StatusConflict, map[string]any{
				"code":    "PAYMENT_EXCEEDS_BALANCE",
				"message": "El importe excede el balance pendiente.",
				"hint":    "Reduce el importe o registra un refund primero.",
			})
			return
		}
		// v1.2 R-08: refunded invoices can't accept new payments.
		if errors.Is(err, repository.ErrInvoiceTerminal) {
			api.JSON(w, http.StatusConflict, map[string]any{
				"code":    "INVOICE_TERMINAL",
				"message": "Esta factura está en estado terminal y no acepta más cambios.",
				"hint":    "Las facturas anuladas o reembolsadas íntegramente son terminales.",
			})
			return
		}
		// v1.2 R-07: refund 1:1 validation gates.
		if errors.Is(err, repository.ErrRefundCrossInvoice) ||
			errors.Is(err, repository.ErrRefundOfRefund) ||
			errors.Is(err, repository.ErrRefundExceedsCap) ||
			errors.Is(err, repository.ErrRefundMethodMismatch) ||
			errors.Is(err, repository.ErrRefundOverRefund) ||
			errors.Is(err, repository.ErrRefundNotReverse) ||
			(errors.Is(err, repository.ErrInvalidPayment) && input.Amount < 0) {
			api.JSON(w, http.StatusUnprocessableEntity, map[string]any{
				"code":    "INVALID_REFUND_TARGET",
				"message": "No se puede reembolsar este pago. Verifica que sea un cobro original y pertenezca a esta factura.",
				"hint":    "Los reembolsos son siempre de 1 pago a la vez. Si necesitas reembolsar todo, usa 'Refund all'.",
			})
			return
		}
		if errors.Is(err, repository.ErrReferenceRequired) {
			api.JSON(w, http.StatusUnprocessableEntity, map[string]any{
				"code":    "REFERENCE_REQUIRED",
				"message": "Necesitamos una referencia para pagos con " + req.Method + ".",
				"hint":    "Lo encuentras en el comprobante del banco o del adquirente.",
			})
			return
		}
		if errors.Is(err, repository.ErrInvalidPayment) {
			api.JSON(w, http.StatusBadRequest, map[string]any{
				"code":    "INVALID_PAYMENT",
				"message": err.Error(),
			})
			return
		}
		if errors.Is(err, repository.ErrInvoiceVoid) {
			api.JSON(w, http.StatusConflict, map[string]any{
				"code":    "INVOICE_VOID",
				"message": "Esta factura fue anulada. No se pueden registrar pagos nuevos.",
				"hint":    "Consulta el historial si necesitas ver el detalle.",
			})
			return
		}
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusCreated, p)
}

// Void — spec §4.4
func (h *InvoiceHandler) Void(w http.ResponseWriter, r *http.Request) {
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid invoice ID"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_BODY", Message: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		api.JSON(w, http.StatusUnauthorized, api.Error{Code: "UNAUTHENTICATED", Message: "Missing X-User-ID header"})
		return
	}

	inv, err := h.svc.VoidInvoice(r.Context(), invoiceID, userID, req.Reason)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			api.JSON(w, http.StatusNotFound, api.Error{Code: "INVOICE_NOT_FOUND", Message: "Invoice not found"})
			return
		}
		if errors.Is(err, repository.ErrInvoiceVoid) {
			api.JSON(w, http.StatusConflict, api.Error{Code: "INVOICE_VOID", Message: "Invoice already void"})
			return
		}
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, inv)
}

// UpdateNotes — spec §4.5
func (h *InvoiceHandler) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid invoice ID"})
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_BODY", Message: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	inv, err := h.svc.UpdateNotes(r.Context(), invoiceID, req.Notes)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			api.JSON(w, http.StatusNotFound, api.Error{Code: "INVOICE_NOT_FOUND", Message: "Invoice not found"})
			return
		}
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, inv)
}

// RegeneratePDF — spec §4.6
//
// Reads Accept-Language so the PDF comes out in the same language the
// user is reading the dashboard in. Empty/missing header → English
// fallback inside the generator.
func (h *InvoiceHandler) RegeneratePDF(w http.ResponseWriter, r *http.Request) {
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid invoice ID"})
		return
	}
	locale := primaryLanguage(r.Header.Get("Accept-Language"))
	url, err := h.svc.RegeneratePDF(r.Context(), invoiceID, locale)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, map[string]any{"pdf_url": url})
}

// RefundAll — spec §4.12 (v1.2 R-07).
//
// Atomic batch refund of every positive, non-invalidated payment on the
// invoice. Generates N refund rows + 1 refund_batches audit row.
// If any refund would fail its R-07 gate, the whole batch rolls back.
func (h *InvoiceHandler) RefundAll(w http.ResponseWriter, r *http.Request) {
	invoiceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_ID", Message: "Invalid invoice ID"})
		return
	}
	var req struct {
		Reason        string `json:"reason"`
		ForceOverride bool   `json:"force_override"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "INVALID_BODY", Message: "Invalid request body"})
		return
	}
	defer r.Body.Close()

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		api.JSON(w, http.StatusUnauthorized, api.Error{Code: "UNAUTHENTICATED", Message: "Missing X-User-ID header"})
		return
	}
	roleStr, _ := middleware.UserRoleFromContext(r.Context())
	role := service.UserRole(roleStr)
	if roleStr == "" {
		role = service.RoleOwner // dev default
	}

	result, err := h.svc.RefundAll(r.Context(), models.RefundAllInput{
		InvoiceID:     invoiceID,
		Reason:        req.Reason,
		ForceOverride: req.ForceOverride,
		InitiatedBy:   userID,
	}, role)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// Surface N refund rows + the audit batch.
	type refundedItem struct {
		OriginalPaymentID uuid.UUID `json:"original_payment_id"`
		RefundPaymentID   uuid.UUID `json:"refund_payment_id"`
		Method            string    `json:"method"`
		AmountRefunded    float64   `json:"amount_refunded"`
	}
	items := make([]refundedItem, 0, len(result.RefundedPayments))
	for _, p := range result.RefundedPayments {
		originalID := uuid.Nil
		if p.ReversalOf != nil {
			originalID = *p.ReversalOf
		}
		items = append(items, refundedItem{
			OriginalPaymentID: originalID,
			RefundPaymentID:   p.ID,
			Method:            string(p.Method),
			AmountRefunded:    -p.Amount,
		})
	}
	api.JSON(w, http.StatusOK, map[string]any{
		"invoice_id":              result.RefundBatches.InvoiceID,
		"refunded_payments":       items,
		"refund_batch_id":         result.RefundBatches.ID,
		"total_refunded":          result.RefundBatches.TotalRefunded,
		"invoice_lifecycle_after": string(result.InvoiceLifecycleAfter),
	})
}

// =============================================================================
// Helpers
// =============================================================================

// propertyIDForInvoice resolves the property_id for an invoice.
// Writes the 404 / 500 response on failure and returns ok=false.
func (h *InvoiceHandler) propertyIDForInvoice(ctx context.Context, invoiceID uuid.UUID, w http.ResponseWriter) (uuid.UUID, bool) {
	detail, err := h.svc.GetByID(ctx, invoiceID)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			api.JSON(w, http.StatusNotFound, api.Error{Code: "INVOICE_NOT_FOUND", Message: "Invoice not found"})
			return uuid.Nil, false
		}
		writeServiceError(w, err)
		return uuid.Nil, false
	}
	return detail.PropertyID, true
}

// aggregateHeaders computes X-Total-Count summary headers from the page
// of invoice summaries. Per spec §4.7 the response carries totals in
// the body and the headers for quick client-side aggregation.
func aggregateHeaders(rows []models.InvoiceSummary) (collected, pending, tax float64) {
	for _, r := range rows {
		collected += r.TotalPaid
		pending += r.Balance
		tax += r.TaxAmount
	}
	return
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// parseDate accepts YYYY-MM-DD (preferred) or RFC3339.
func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// primaryLanguage picks the most-preferred language from an
// Accept-Language header. The format is a comma-separated list of
// tags with optional q-values:
//
//	Accept-Language: en-US,en;q=0.9,id;q=0.8
//
// We don't pull in golang.org/x/text/language — we only ship three
// locales (en/es/id) so a tiny splitter is enough. Returns "" when
// the header is missing or empty.
func primaryLanguage(header string) string {
	if header == "" {
		return ""
	}
	best := ""
	bestQ := -1.0
	for _, raw := range strings.Split(header, ",") {
		tag, q := tagAndQ(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if q > bestQ {
			bestQ = q
			best = tag
		}
	}
	// Strip region/script suffix: "en-US" → "en", "id-ID" → "id".
	if i := strings.IndexByte(best, '-'); i > 0 {
		best = best[:i]
	}
	return strings.ToLower(best)
}

// tagAndQ splits one entry of an Accept-Language list into its tag
// and q-value (default 1.0 when missing).
func tagAndQ(entry string) (string, float64) {
	parts := strings.SplitN(entry, ";", 2)
	tag := strings.TrimSpace(parts[0])
	q := 1.0
	if len(parts) == 2 {
		// parts[1] looks like "q=0.8"
		qStr := strings.TrimSpace(parts[1])
		if strings.HasPrefix(strings.ToLower(qStr), "q=") {
			if v, err := strconv.ParseFloat(qStr[2:], 64); err == nil {
				q = v
			}
		}
	}
	return tag, q
}
