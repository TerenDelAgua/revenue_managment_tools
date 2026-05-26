package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type RoomHandler struct {
	repo *repository.RoomRepository
}

func NewRoomHandler(repo *repository.RoomRepository) *RoomHandler {
	return &RoomHandler{repo: repo}
}

// mapDBError centraliza el mapeo de errores pgx/PostgreSQL a HTTP
func mapDBError(w http.ResponseWriter, err error, msgConflict, msgValidation string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		api.NotFound(w, "Room not found")
		return true
	}
	if strings.Contains(err.Error(), "unique_violation") || strings.Contains(err.Error(), "_unique") {
		api.JSON(w, http.StatusConflict, api.Error{Code: "CONFLICT", Message: msgConflict})
		return true
	}
	if strings.Contains(err.Error(), "check_violation") {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_INPUT", Message: msgValidation})
		return true
	}
	api.InternalServerError(w, "Failed to process room")
	return true
}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	// BR-06: Validación estricta de grid
	if req.PosX < 0 || req.PosX > 11 || req.PosY < 0 || req.PosY > 19 {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "OUT_OF_BOUNDS", Message: "pos_x must be 0-11, pos_y must be 0-19"})
		return
	}

	room, err := h.repo.Create(r.Context(), &req)
	if mapDBError(w, err, "Room number or position already exists in this floor/property", "Invalid room configuration") {
		return
	}
	api.JSON(w, http.StatusCreated, room)
}

func (h *RoomHandler) UpdatePosition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.BadRequest(w, "Invalid room ID")
		return
	}

	var req models.UpdateRoomPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.PosX < 0 || req.PosX > 11 || req.PosY < 0 || req.PosY > 19 {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "OUT_OF_BOUNDS", Message: "pos_x must be 0-11, pos_y must be 0-19"})
		return
	}

	room, err := h.repo.UpdatePosition(r.Context(), id, req.PosX, req.PosY)
	if mapDBError(w, err, "Another room already occupies this cell", "Invalid coordinates") {
		return
	}
	if room == nil {
		api.NotFound(w, "Room not found")
		return
	}
	api.JSON(w, http.StatusOK, room)
}

// BatchUpdatePositions - Spec 5.2 (Save layout)
func (h *RoomHandler) BatchUpdatePositions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Positions []models.RoomPositionUpdate `json:"positions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validación previa rápida
	for _, p := range req.Positions {
		if p.PosX < 0 || p.PosX > 11 || p.PosY < 0 || p.PosY > 19 {
			api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "OUT_OF_BOUNDS", Message: "One or more positions exceed grid limits"})
			return
		}
	}

	updated, err := h.repo.BatchUpdatePositions(r.Context(), req.Positions)
	if mapDBError(w, err, "Position conflict detected during batch save", "Invalid layout configuration") {
		return
	}
	api.JSON(w, http.StatusOK, map[string]int{"updated": updated})
}
