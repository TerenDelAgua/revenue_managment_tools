package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type RoomBlockHandler struct {
	svc *service.InventoryService
}

func NewRoomBlockHandler(svc *service.InventoryService) *RoomBlockHandler {
	return &RoomBlockHandler{svc: svc}
}

// Create - POST /room-blocks (Spec §5)
// Aplica BR-03 (conflicto con checked_in), validación de fechas y detección de duplicados.
func (h *RoomBlockHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRoomBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "Invalid request body")
		return
	}
	defer r.Body.Close()

	// TODO: Fase Auth. Extraer de JWT claims:
	// claims := r.Context().Value("claims").(*jwt.CustomClaims)
	// req.CreatedBy = claims.UserID

	block, err := h.svc.BlockRoom(r.Context(), req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusConflict, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to create room block")
		return
	}

	api.JSON(w, http.StatusCreated, block)
}

// Delete - DELETE /room-blocks/{id} (Spec §5)
// Elimina bloqueo. Si no existe o falla, retorna 404/500 respectivamente.
func (h *RoomBlockHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.BadRequest(w, "Invalid block ID")
		return
	}

	if err := h.svc.RemoveBlock(r.Context(), id); err != nil {
		// Opcional: diferenciar "not found" vs "db error" según evolución del repo
		api.InternalServerError(w, "Failed to remove room block")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
