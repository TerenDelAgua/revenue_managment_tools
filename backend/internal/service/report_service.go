package service

import (
	"context"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

type ReportService struct {
	repo *repository.ReportRepository
}

func NewReportService(repo *repository.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
}

// GetMetrics valida el rango y delega al repositorio.
func (s *ReportService) GetMetrics(ctx context.Context, req models.ReportRequest) (*models.ReportResponse, error) {
	if !req.DateTo.After(req.DateFrom) {
		return nil, &BusinessError{Code: "INVALID_RANGE", Message: "date_to must be strictly after date_from"}
	}

	// Opcional: Limitar rango máximo para evitar queries pesadas (ej. 365 días)
	if req.DateTo.Sub(req.DateFrom).Hours() > 8760 {
		return nil, &BusinessError{Code: "RANGE_TOO_LARGE", Message: "Maximum report range is 1 year"}
	}

	return s.repo.GetMetrics(ctx, req)
}

func (s *ReportService) GetDailyBreakdown(ctx context.Context, req models.ReportRequest) (*models.DailyBreakdownResponse, error) {
	if !req.DateTo.After(req.DateFrom) {
		return nil, &BusinessError{Code: "INVALID_RANGE", Message: "date_to must be strictly after date_from"}
	}

	// Limitar rango máximo a 90 días para el breakdown (evitar queries pesadas)
	if req.DateTo.Sub(req.DateFrom).Hours() > 2160 { // 90 días
		return nil, &BusinessError{Code: "RANGE_TOO_LARGE", Message: "Maximum daily breakdown range is 90 days"}
	}

	return s.repo.GetDailyBreakdown(ctx, req)
}
