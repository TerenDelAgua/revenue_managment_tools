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

type PropertyHandler struct {
	repo *repository.PropertyRepository
}

func NewPropertyHandler(repo *repository.PropertyRepository) *PropertyHandler {
	return &PropertyHandler{repo: repo}
}

func (h *PropertyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePropertyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	property, err := h.repo.Create(r.Context(), &req)
	if err != nil {
		api.InternalServerError(w, "Failed to create property")
		return
	}

	api.JSON(w, http.StatusCreated, property)
}

func (h *PropertyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.BadRequest(w, "Invalid property ID")
		return
	}

	property, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		api.InternalServerError(w, "Failed to get property")
		return
	}
	if property == nil {
		api.NotFound(w, "Property not found")
		return
	}

	api.JSON(w, http.StatusOK, property)
}

func (h *PropertyHandler) List(w http.ResponseWriter, r *http.Request) {
	properties, err := h.repo.List(r.Context())
	if err != nil {
		api.InternalServerError(w, "Failed to list properties")
		return
	}

	api.JSON(w, http.StatusOK, properties)
}
