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
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type GuestHandler struct {
	svc *service.GuestService
}

func NewGuestHandler(svc *service.GuestService) *GuestHandler {
	return &GuestHandler{svc: svc}
}

func (h *GuestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateGuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	guest, err := h.svc.CreateGuest(r.Context(), &req)
	if err != nil {
		api.InternalServerError(w, "Failed to create guest")
		return
	}

	api.JSON(w, http.StatusCreated, guest)
}

func (h *GuestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid guest ID")
		return
	}

	guest, err := h.svc.GetGuestByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.NotFound(w, "Guest not found")
			return
		}
		api.InternalServerError(w, "Failed to fetch guest details")
		return
	}

	api.JSON(w, http.StatusOK, guest)
}

func (h *GuestHandler) List(w http.ResponseWriter, r *http.Request) {
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

	search := r.URL.Query().Get("search")
	page := 1
	limit := 50

	guests, total, err := h.svc.ListGuests(r.Context(), propertyID, search, page, limit)
	if err != nil {
		api.InternalServerError(w, "Failed to fetch guests list")
		return
	}

	// pagination wrapper
	api.JSON(w, http.StatusOK, map[string]interface{}{
		"guests": guests,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func (h *GuestHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid guest ID")
		return
	}

	var req models.UpdateGuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	guest, err := h.svc.UpdateGuest(r.Context(), id, req)
	if err != nil {
		api.InternalServerError(w, "Failed to update guest")
		return
	}

	api.JSON(w, http.StatusOK, guest)
}
