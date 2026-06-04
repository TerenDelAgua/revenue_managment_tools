package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

type GuestService struct {
	guestRepo *repository.GuestRepository
}

func NewGuestService(guestRepo *repository.GuestRepository) *GuestService {
	return &GuestService{guestRepo: guestRepo}
}

func (s *GuestService) ListGuests(ctx context.Context, propertyID uuid.UUID, search string, page int, limit int) ([]*models.GuestListDTO, int, error) {
	return s.guestRepo.List(ctx, propertyID, search, page, limit)
}

func (s *GuestService) GetGuestByID(ctx context.Context, id uuid.UUID) (*models.GuestDetail, error) {
	return s.guestRepo.GetByID(ctx, id)
}

func (s *GuestService) CreateGuest(ctx context.Context, req *models.CreateGuestRequest) (*models.Guest, error) {
	return s.guestRepo.Create(ctx, req)
}

func (s *GuestService) UpdateGuest(ctx context.Context, id uuid.UUID, req models.UpdateGuestRequest) (*models.Guest, error) {
	params := make(map[string]interface{})
	if req.FullName != nil {
		params["full_name"] = *req.FullName
	}
	if req.IdNumber != nil {
		params["id_number"] = *req.IdNumber
	}
	if req.Phone != nil {
		params["phone"] = *req.Phone
	}
	if req.Email != nil {
		params["email"] = *req.Email
	}
	if req.Nationality != nil {
		params["nationality"] = *req.Nationality
	}
	if req.Notes != nil {
		params["notes"] = *req.Notes
	}
	return s.guestRepo.Update(ctx, id, params)
}
