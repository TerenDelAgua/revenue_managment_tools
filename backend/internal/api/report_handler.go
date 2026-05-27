package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/api"
)

type ReportHandler struct {
	svc *service.ReportService
}

func NewReportHandler(svc *service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// GetMetrics - GET /api/v1/reports/metrics?date_from=YYYY-MM-DD&date_to=YYYY-MM-DD
// Devuelve RevPAR, ADR y Occupancy Rate para el rango solicitado.
func (h *ReportHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	dateFromStr := r.URL.Query().Get("date_from")
	dateToStr := r.URL.Query().Get("date_to")

	if dateFromStr == "" || dateToStr == "" {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{
			Code: "MISSING_DATES", Message: "date_from and date_to are required",
		})
		return
	}

	dateFrom, err := time.Parse("2006-01-02", dateFromStr)
	if err != nil {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATE", Message: "Invalid date_from format"})
		return
	}

	dateTo, err := time.Parse("2006-01-02", dateToStr)
	if err != nil {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATE", Message: "Invalid date_to format"})
		return
	}

	// TODO: En producción, extraer de JWT claims: claims := r.Context().Value("claims").(*jwt.CustomClaims)
	// propertyID := claims.PropertyID
	propertyIDStr := r.URL.Query().Get("property_id")
	if propertyIDStr == "" {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "MISSING_PROPERTY", Message: "property_id required"})
		return
	}

	propertyID, err := uuid.Parse(propertyIDStr)
	if err != nil {
		api.BadRequest(w, "Invalid property ID")
		return
	}

	req := models.ReportRequest{
		PropertyID: propertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	}

	metrics, err := h.svc.GetMetrics(r.Context(), req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to generate report")
		return
	}

	api.JSON(w, http.StatusOK, metrics)
}

// GetDailyBreakdown - GET /api/v1/reports/daily?date_from=YYYY-MM-DD&date_to=YYYY-MM-DD
// Devuelve métricas día a día para gráficos de tendencia.
func (h *ReportHandler) GetDailyBreakdown(w http.ResponseWriter, r *http.Request) {
	dateFromStr := r.URL.Query().Get("date_from")
	dateToStr := r.URL.Query().Get("date_to")

	if dateFromStr == "" || dateToStr == "" {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{
			Code: "MISSING_DATES", Message: "date_from and date_to are required",
		})
		return
	}

	dateFrom, err := time.Parse("2006-01-02", dateFromStr)
	if err != nil {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATE", Message: "Invalid date_from format"})
		return
	}

	dateTo, err := time.Parse("2006-01-02", dateToStr)
	if err != nil {
		api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: "INVALID_DATE", Message: "Invalid date_to format"})
		return
	}

	propertyIDStr := r.URL.Query().Get("property_id")
	if propertyIDStr == "" {
		api.JSON(w, http.StatusBadRequest, api.Error{Code: "MISSING_PROPERTY", Message: "property_id required"})
		return
	}

	propertyID, err := uuid.Parse(propertyIDStr)
	if err != nil {
		api.BadRequest(w, "Invalid property ID")
		return
	}

	req := models.ReportRequest{
		PropertyID: propertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	}

	breakdown, err := h.svc.GetDailyBreakdown(r.Context(), req)
	if err != nil {
		var bizErr *service.BusinessError
		if errors.As(err, &bizErr) {
			api.JSON(w, http.StatusUnprocessableEntity, api.Error{Code: bizErr.Code, Message: bizErr.Message})
			return
		}
		api.InternalServerError(w, "Failed to generate daily breakdown")
		return
	}

	api.JSON(w, http.StatusOK, breakdown)
}
