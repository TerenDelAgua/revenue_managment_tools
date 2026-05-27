package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api" // Tu paquete de helpers HTTP
)

type BookingHandler struct {
	svc *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

// Create - POST /api/v1/bookings
// Crea una nueva reserva. Valida disponibilidad antes de guardar.
func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validación básica de fechas en el handler (opcional si ya está en el service)
	if !req.CheckOut.After(req.CheckIn) {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATES", Message: "check_out must be after check_in"})
		return
	}

	booking, err := h.svc.CreateBooking(r.Context(), req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusConflict, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to create booking")
		return
	}

	api.JSON(w, http.StatusCreated, booking)
}

// GetPending - GET /api/v1/bookings/pending
// Devuelve la lista de reservas confirmadas sin habitación asignada o pendientes de check-in.
// Usado por el RoomDrawer para el selector inline.
func (h *BookingHandler) GetPending(w http.ResponseWriter, r *http.Request) {
	// En producción, extraer property_id del Contexto (JWT)
	propertyIDStr := r.URL.Query().Get("property_id")
	if propertyIDStr == "" {
		api.BadRequest(w, "property_id is required")
		return
	}

	propertyID, err := uuid.Parse(propertyIDStr)
	if err != nil {
		api.BadRequest(w, "Invalid property_id")
		return
	}

	bookings, err := h.svc.GetPendingBookings(r.Context(), propertyID)
	if err != nil {
		api.InternalServerError(w, "Failed to fetch pending bookings")
		return
	}

	api.JSON(w, http.StatusOK, bookings)
}

// CheckIn - POST /api/v1/bookings/{id}/checkin
// Marca una reserva como checked_in. Libera la habitación de 'pending' a 'occupied'.
func (h *BookingHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	if err := h.svc.CheckIn(r.Context(), id); err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusConflict, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		// Caso común: Booking not found
		if errors.Is(err, pgx.ErrNoRows) {
			api.NotFound(w, "Booking not found")
			return
		}
		api.InternalServerError(w, "Failed to check in")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CheckOut - POST /api/v1/bookings/{id}/checkout
// Marca una reserva como checked_out. Libera la habitación a 'available'.
func (h *BookingHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	if err := h.svc.CheckOut(r.Context(), id); err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusConflict, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			api.NotFound(w, "Booking not found")
			return
		}
		api.InternalServerError(w, "Failed to check out")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Assign - PATCH /api/v1/bookings/{id}
// Asigna una habitación a una reserva pendiente.
func (h *BookingHandler) Assign(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	var req struct {
		RoomID uuid.UUID `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	if err := h.svc.AssignRoom(r.Context(), id, req.RoomID); err != nil {
		api.InternalServerError(w, "Failed to assign room")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
