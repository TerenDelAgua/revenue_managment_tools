package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type BookingHandler struct {
	svc *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	res, err := h.svc.CreateBooking(r.Context(), req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			if bizErr.Code == "ROOM_NOT_AVAILABLE" {
				api.JSON(w, http.StatusConflict, map[string]interface{}{
					"code":         bizErr.Code,
					"message":      bizErr.Message,
					"can_override": true, // Para desarrollo local, permitir forzar
				})
				return
			}
			api.JSON(w, http.StatusBadRequest, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to create booking: "+err.Error())
		return
	}

	api.JSON(w, http.StatusCreated, res)
}

func (h *BookingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	detail, err := h.svc.GetBookingByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.NotFound(w, "Booking not found")
			return
		}
		api.InternalServerError(w, "Failed to fetch booking details")
		return
	}

	api.JSON(w, http.StatusOK, detail)
}

func (h *BookingHandler) List(w http.ResponseWriter, r *http.Request) {
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

	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	bookings, total, err := h.svc.ListBookings(r.Context(), propertyID, status, search, page, limit)
	if err != nil {
		api.InternalServerError(w, "Failed to list bookings")
		return
	}

	api.JSON(w, http.StatusOK, map[string]interface{}{
		"bookings": bookings,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func (h *BookingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	var req models.UpdateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	booking, err := h.svc.UpdateBooking(r.Context(), id, req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			if bizErr.Code == "ROOM_NOT_AVAILABLE" {
				api.JSON(w, http.StatusConflict, map[string]interface{}{
					"code":         bizErr.Code,
					"message":      bizErr.Message,
					"can_override": true,
				})
				return
			}
			api.JSON(w, http.StatusBadRequest, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to update booking")
		return
	}

	api.JSON(w, http.StatusOK, booking)
}

func (h *BookingHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	if err := h.svc.CheckIn(r.Context(), id); err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusBadRequest, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to check in")
		return
	}

	api.JSON(w, http.StatusOK, map[string]string{"status": "checked_in"})
}

func (h *BookingHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	if err := h.svc.CheckOut(r.Context(), id); err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusBadRequest, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to check out")
		return
	}

	api.JSON(w, http.StatusOK, map[string]string{"status": "checked_out"})
}

func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid booking ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	if err := h.svc.CancelBooking(r.Context(), id, req.Reason); err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusBadRequest, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to cancel booking")
		return
	}

	api.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

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
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusBadRequest, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to assign room")
		return
	}

	api.JSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (h *BookingHandler) GetPending(w http.ResponseWriter, r *http.Request) {
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
