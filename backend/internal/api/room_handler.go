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

type RoomHandler struct {
	repo *repository.RoomRepository
}

func NewRoomHandler(repo *repository.RoomRepository) *RoomHandler {
	return &RoomHandler{repo: repo}
}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	room, err := h.repo.Create(r.Context(), &req)
	if err != nil {
		api.InternalServerError(w, "Failed to create room")
		return
	}

	api.JSON(w, http.StatusCreated, room)
}

func (h *RoomHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.BadRequest(w, "Invalid room ID")
		return
	}

	room, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		api.InternalServerError(w, "Failed to get room")
		return
	}
	if room == nil {
		api.NotFound(w, "Room not found")
		return
	}

	api.JSON(w, http.StatusOK, room)
}

func (h *RoomHandler) ListByFloor(w http.ResponseWriter, r *http.Request) {
	floorIDStr := chi.URLParam(r, "floorID")
	floorID, err := uuid.Parse(floorIDStr)
	if err != nil {
		api.BadRequest(w, "Invalid floor ID")
		return
	}

	rooms, err := h.repo.ListByFloor(r.Context(), floorID)
	if err != nil {
		api.InternalServerError(w, "Failed to list rooms")
		return
	}

	api.JSON(w, http.StatusOK, rooms)
}

func (h *RoomHandler) UpdatePosition(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
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

	room, err := h.repo.UpdatePosition(r.Context(), id, req.PosX, req.PosY)
	if err != nil {
		api.InternalServerError(w, "Failed to update room position")
		return
	}
	if room == nil {
		api.NotFound(w, "Room not found")
		return
	}

	api.JSON(w, http.StatusOK, room)
}
