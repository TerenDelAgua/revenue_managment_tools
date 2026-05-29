package api

import (
	"errors"
	"log"
	"net/http"
	"time"

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
