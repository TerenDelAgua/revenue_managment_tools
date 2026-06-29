package api

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type InventoryHandler struct {
	svc *service.InventoryService
}

func NewInventoryHandler(svc *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

// GetMap - GET /api/v1/map?date_from=YYYY-MM-DD&date_to=YYYY-MM-DD
// Devuelve el estado de disponibilidad del mapa para un rango de fechas.
// Spec §5.1 · AC-03 · AC-09
func (h *InventoryHandler) GetMap(w http.ResponseWriter, r *http.Request) {
	dateFromStr := r.URL.Query().Get("date_from")
	dateToStr := r.URL.Query().Get("date_to")

	if dateFromStr == "" || dateToStr == "" {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{
			Code:    "MISSING_DATES",
			Message: "date_from and date_to query parameters are required (format: YYYY-MM-DD)",
		})
		return
	}

	dateFrom, err := time.Parse("2006-01-02", dateFromStr)
	if err != nil {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATE", Message: "Invalid date_from format. Use YYYY-MM-DD"})
		return
	}

	dateTo, err := time.Parse("2006-01-02", dateToStr)
	if err != nil {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATE", Message: "Invalid date_to format. Use YYYY-MM-DD"})
		return
	}

	if !dateTo.After(dateFrom) {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_RANGE", Message: "date_to must be strictly after date_from"})
		return
	}

	// TODO: Fase Auth. Extraer property_id de JWT claims:
	// claims := r.Context().Value("claims").(*jwt.CustomClaims)
	// propID := claims.PropertyID
	// Placeholder para desarrollo local (middleware de auth pendiente):
	propIDStr := r.Header.Get("X-Property-ID")
	if propIDStr == "" {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "MISSING_PROPERTY", Message: "property_id required via auth context or X-Property-ID header"})
		return
	}

	propID, err := uuid.Parse(propIDStr)
	if err != nil {
		api.BadRequest(w, "Invalid property ID format")
		return
	}

	req := models.MapAvailabilityRequest{
		PropertyID: propID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	}

	mapData, err := h.svc.GetMap(r.Context(), req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusConflict, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		// Log error for debugging
		log.Printf("GetMap error: %v", err)
		api.InternalServerError(w, "Failed to load floor map")
		return
	}

	api.JSON(w, http.StatusOK, mapData)
}

// SetCleaning - POST /api/v1/rooms/{id}/cleaning
// Marca la habitación como "cleaning" (housekeeping en curso).
// Es idempotente: aplicarlo dos veces no rompe nada.
func (h *InventoryHandler) SetCleaning(w http.ResponseWriter, r *http.Request) {
	room, err := h.transitionCleaning(r, true)
	if err != nil {
		h.writeCleaningErr(w, err)
		return
	}
	api.JSON(w, http.StatusOK, room)
}

// ClearCleaning - DELETE /api/v1/rooms/{id}/cleaning
// Marca la habitación como disponible de nuevo (sale del estado cleaning).
// Es idempotente.
func (h *InventoryHandler) ClearCleaning(w http.ResponseWriter, r *http.Request) {
	room, err := h.transitionCleaning(r, false)
	if err != nil {
		h.writeCleaningErr(w, err)
		return
	}
	api.JSON(w, http.StatusOK, room)
}

func (h *InventoryHandler) transitionCleaning(r *http.Request, isCleaning bool) (*models.Room, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, &service.BusinessError{Code: "INVALID_ID", Message: "Invalid room ID."}
	}
	return h.svc.SetRoomCleaning(r.Context(), id, isCleaning)
}

func (h *InventoryHandler) writeCleaningErr(w http.ResponseWriter, err error) {
	var bizErr *service.BusinessError
	if errors.As(err, &bizErr) {
		status := http.StatusConflict
		if bizErr.Code == "ROOM_NOT_FOUND" || bizErr.Code == "INVALID_ID" {
			status = http.StatusNotFound
		}
		api.JSON(w, status, api.Error{Code: bizErr.Code, Message: bizErr.Message})
		return
	}
	log.Printf("cleaning transition error: %v", err)
	api.InternalServerError(w, "Failed to update room cleaning state")
}
