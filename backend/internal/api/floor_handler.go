package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type FloorHandler struct {
	repo *repository.FloorRepository
}

func NewFloorHandler(repo *repository.FloorRepository) *FloorHandler {
	return &FloorHandler{repo: repo}
}

func (h *FloorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateFloorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	floor, err := h.repo.Create(r.Context(), &req)
	if err != nil {
		api.InternalServerError(w, "Failed to create floor")
		return
	}

	api.JSON(w, http.StatusCreated, floor)
}

func (h *FloorHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.BadRequest(w, "Invalid floor ID")
		return
	}

	floor, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		api.InternalServerError(w, "Failed to get floor")
		return
	}
	if floor == nil {
		api.NotFound(w, "Floor not found")
		return
	}

	api.JSON(w, http.StatusOK, floor)
}

func (h *FloorHandler) ListByProperty(w http.ResponseWriter, r *http.Request) {
	propertyIDStr := chi.URLParam(r, "propertyID")
	propertyID, err := uuid.Parse(propertyIDStr)
	if err != nil {
		api.BadRequest(w, "Invalid property ID")
		return
	}

	floors, err := h.repo.ListByProperty(r.Context(), propertyID)
	if err != nil {
		api.InternalServerError(w, "Failed to list floors")
		return
	}

	api.JSON(w, http.StatusOK, floors)
}
