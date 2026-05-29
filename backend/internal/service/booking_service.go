package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

type BookingService struct {
	db           *pgxpool.Pool
	bookingRepo  *repository.BookingRepository
	inventorySvc *InventoryService // Reutilizamos para verificar disponibilidad
}

func NewBookingService(db *pgxpool.Pool, bookingRepo *repository.BookingRepository, inventorySvc *InventoryService) *BookingService {
	return &BookingService{db, bookingRepo, inventorySvc}
}

// CreateBooking valida disponibilidad antes de crear.
func (s *BookingService) CreateBooking(ctx context.Context, req models.CreateBookingRequest) (*models.Booking, error) {
	// 1. Verificar disponibilidad (BR-04)
	available, err := s.inventorySvc.IsRoomAvailableForBooking(ctx, req.RoomID, req.CheckIn, req.CheckOut)
	if err != nil {
		return nil, fmt.Errorf("failed to check availability: %w", err)
	}
	if !available {
		return nil, &BusinessError{Code: "ROOM_UNAVAILABLE", Message: "This room is already booked or blocked for the selected dates."}
	}

	// 2. Crear reserva
	return s.bookingRepo.Create(ctx, &req)
}

// CheckIn ejecuta el flujo de llegada.
func (s *BookingService) CheckIn(ctx context.Context, bookingID uuid.UUID) error {
	return s.bookingRepo.CheckIn(ctx, bookingID)
}

// CheckOut ejecuta el flujo de salida.
func (s *BookingService) CheckOut(ctx context.Context, bookingID uuid.UUID) error {
	return s.bookingRepo.CheckOut(ctx, bookingID)
}

func (s *BookingService) GetPendingBookings(ctx context.Context, propertyID uuid.UUID) ([]*repository.PendingBookingDTO, error) {
	return s.bookingRepo.GetPendingByProperty(ctx, propertyID)
}

func (s *BookingService) AssignRoom(ctx context.Context, bookingID, roomID uuid.UUID) error {
	return s.bookingRepo.AssignRoom(ctx, bookingID, roomID)
}


