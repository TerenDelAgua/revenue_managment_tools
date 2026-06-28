package api

import (
	"errors"
	"net/http"

	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

// businessErrorStatus maps a BusinessError.Code to its HTTP status.
// Default: 400 Bad Request. Codes not listed here are treated as 400
// unless they look like "not found" (404) or "forbidden" (403).
func businessErrorStatus(code string) int {
	switch code {
	// 400 — invalid input from the client
	case "INVALID_INPUT", "INVALID_STATUS", "INVALID_PAYMENT",
		"REFERENCE_REQUIRED", "VOID_REASON_REQUIRED",
		"BOOKING_PROPERTY_REQUIRED", "BOOKING_PROPERTY_MISMATCH":
		return http.StatusBadRequest
	// 403 — auth/authorization failures
	case "REFUND_FORBIDDEN":
		return http.StatusForbidden
	// 404 — resource not found
	case "INVOICE_NOT_FOUND", "BOOKING_NOT_FOUND":
		return http.StatusNotFound
	// 409 — state conflict (already void, overpaid, balance exceeded)
	case "INVOICE_VOID", "INVOICE_OVERPAID", "PAYMENT_EXCEEDS_BALANCE",
		"ROOM_NOT_AVAILABLE", "ROOM_INACTIVE", "NO_OVERLAP":
		return http.StatusConflict
	// 422 — semantic validation failure (e.g. coords out of bounds)
	case "OUT_OF_BOUNDS", "OVERLAPPING_DATES":
		return http.StatusUnprocessableEntity
	// 500 — downstream failure (PDF storage, R2, etc.)
	case "PDF_STORAGE_FAILED":
		return http.StatusInternalServerError
	// 501 — feature not yet implemented (B5 PDF generator)
	case "PDF_NOT_CONFIGURED":
		return http.StatusNotImplemented
	default:
		return http.StatusBadRequest
	}
}

// writeServiceError is a small helper that maps service errors to HTTP
// responses consistently. It also adds an optional `hint` field per the
// spec §9 tone-of-voice contract (helpful, solution-oriented messages).
func writeServiceError(w http.ResponseWriter, err error) {
	var bizErr *service.BusinessError
	if errors.As(err, &bizErr) {
		status := businessErrorStatus(bizErr.Code)
		// Tone of Voice: include hint when the business code carries one.
		// Spec §9 lists hint examples (PAYMENT_EXCEEDS_BALANCE, REFERENCE_REQUIRED).
		// Keep the hint short and actionable.
		body := map[string]any{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		}
		if hint, ok := hintForCode(bizErr.Code); ok {
			body["hint"] = hint
		}
		api.JSON(w, status, body)
		return
	}
	api.JSON(w, http.StatusInternalServerError, map[string]any{
		"message": "Internal server error",
	})
}

// hintForCode returns a user-friendly hint for selected business codes.
// Spec §9 — Tone of Voice Manifesto: empathetic, solution-oriented.
func hintForCode(code string) (string, bool) {
	switch code {
	case "PAYMENT_EXCEEDS_BALANCE":
		return "Reduce the amount or register a refund first.", true
	case "REFERENCE_REQUIRED":
		return "Find the reference in the bank/adquirente receipt.", true
	case "INVOICE_VOID":
		return "Check the invoice history if you need the details.", true
	case "INVOICE_OVERPAID":
		return "Only the owner can resolve an overpayment.", true
	case "REFUND_FORBIDDEN":
		return "Refunds require the owner role or booking.force_override=true.", true
	case "PDF_NOT_CONFIGURED":
		return "The PDF feature is being rolled out. Try again later.", true
	case "PDF_STORAGE_FAILED":
		return "You can retry from the invoice list.", true
	}
	return "", false
}
