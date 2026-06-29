package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

// InventoryService maneja disponibilidad, bloqueos y detección de conflictos.
// No contiene lógica de routing ni mapeo HTTP (eso es responsabilidad del Handler).
type InventoryService struct {
	db            *pgxpool.Pool
	roomRepo      *repository.RoomRepository
	roomBlockRepo *repository.RoomBlockRepository
}

func NewInventoryService(db *pgxpool.Pool, roomRepo *repository.RoomRepository, roomBlockRepo *repository.RoomBlockRepository) *InventoryService {
	return &InventoryService{db, roomRepo, roomBlockRepo}
}

// BusinessError permite mapeo limpio a 409 Conflict en handlers.
type BusinessError struct {
	Code    string
	Message string
}

func (e *BusinessError) Error() string { return e.Message }

// GetMap delega al repo. Aquí podríamos añadir cacheo (Redis) o post-procesado futuro.
func (s *InventoryService) GetMap(ctx context.Context, req models.MapAvailabilityRequest) (*models.MapResponse, error) {
	return s.roomRepo.GetMapWithAvailability(ctx, req)
}

// BlockRoom aplica BR-03 y validación de fechas antes de persistir.
func (s *InventoryService) BlockRoom(ctx context.Context, req models.CreateRoomBlockRequest) (*models.RoomBlock, error) {
	// Validación de fechas (BR implícito + Spec §2.3)
	if !req.EndDate.After(req.StartDate) {
		return nil, &BusinessError{Code: "INVALID_DATES", Message: "end_date must be strictly after start_date"}
	}

	// BR-03: Hard block si hay booking checked_in solapando
	hasActive, err := s.hasCheckedInOverlap(ctx, req.RoomID, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to check active bookings: %w", err)
	}
	if hasActive {
		return nil, &BusinessError{Code: "ACTIVE_BOOKING_CONFLICT", Message: "Cannot block a room with an active checked-in booking overlapping this period."}
	}

	// Evitar duplicados de bloqueo en el mismo rango
	overlaps, err := s.roomBlockRepo.GetOverlapping(ctx, req.RoomID, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if len(overlaps) > 0 {
		return nil, &BusinessError{Code: "BLOCK_CONFLICT", Message: "Room already blocked for this date range."}
	}

	return s.roomBlockRepo.Create(ctx, &req)
}

// RemoveBlock elimina un bloqueo. Podría añadir auditoría aquí.
func (s *InventoryService) RemoveBlock(ctx context.Context, blockID uuid.UUID) error {
	return s.roomBlockRepo.Delete(ctx, blockID)
}

// IsRoomAvailableForBooking helper para BR-04 (Booking Service futuro).
// Reglas: un room NO es reservable si (a) tiene un room_block solapando, (b) tiene
// un booking checked_in solapando, o (c) está en estado persistente "cleaning".
func (s *InventoryService) IsRoomAvailableForBooking(ctx context.Context, roomID uuid.UUID, start, end time.Time) (bool, error) {
	hasBlock, err := s.hasBlockingOverlap(ctx, roomID, start, end)
	if err != nil || hasBlock {
		return !hasBlock, err
	}
	hasActive, err := s.hasCheckedInOverlap(ctx, roomID, start, end)
	if err != nil {
		return false, err
	}
	if hasActive {
		return false, nil
	}
	// Estado persistente: housekeeping en curso. No vendible.
	cleaning, err := s.isRoomCleaning(ctx, roomID)
	if err != nil {
		return false, err
	}
	return !cleaning, nil
}

// SetRoomCleaning marca/desmarca el estado operacional "cleaning" de una habitación.
// Reglas de dominio:
//   - No se puede limpiar una habitación "inactive" (devuelve BusinessError).
//   - Idempotente: aplicar el mismo estado dos veces es no-op.
//   - Al terminar la limpieza, el estado vuelve a "active" (vendible de nuevo).
func (s *InventoryService) SetRoomCleaning(ctx context.Context, roomID uuid.UUID, isCleaning bool) (*models.Room, error) {
	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to load room: %w", err)
	}
	if room == nil {
		return nil, &BusinessError{Code: "ROOM_NOT_FOUND", Message: "Room not found."}
	}
	if room.Status == "inactive" {
		return nil, &BusinessError{
			Code:    "ROOM_INACTIVE",
			Message: "An inactive room cannot be marked as cleaning. Reactivate it first.",
		}
	}

	target := "active"
	if isCleaning {
		target = "cleaning"
		if room.Status == "cleaning" {
			return room, nil // idempotente
		}
	} else {
		if room.Status != "cleaning" {
			return room, nil // ya estaba en estado vendible
		}
	}

	return s.roomRepo.Update(ctx, roomID, &models.UpdateRoomRequest{Status: &target})
}

// === Consultas de conflicto (BR-03 / BR-04) ===
// Usamos queries directas para mantener el servicio autocontenido en Fase 1.
// Se refactorizarán a BookingRepository en Fase 2.

func (s *InventoryService) hasCheckedInOverlap(ctx context.Context, roomID uuid.UUID, start, end time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(1) FROM bookings
		WHERE room_id = $1 AND status = 'checked_in'
		AND check_in < $3 AND check_out > $2
	`, roomID, start, end).Scan(&count)
	return count > 0, err
}

func (s *InventoryService) hasBlockingOverlap(ctx context.Context, roomID uuid.UUID, start, end time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(1) FROM room_blocks
		WHERE room_id = $1 AND start_date < $3 AND end_date > $2
	`, roomID, start, end).Scan(&count)
	return count > 0, err
}

func (s *InventoryService) isRoomCleaning(ctx context.Context, roomID uuid.UUID) (bool, error) {
	var status string
	err := s.db.QueryRow(ctx, `SELECT status FROM rooms WHERE id = $1`, roomID).Scan(&status)
	if err != nil {
		return false, err
	}
	return status == "cleaning", nil
}
