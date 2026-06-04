package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

type BookingService struct {
	db           *pgxpool.Pool
	bookingRepo  *repository.BookingRepository
	guestRepo    *repository.GuestRepository
	inventorySvc *InventoryService
}

func NewBookingService(db *pgxpool.Pool, bookingRepo *repository.BookingRepository, guestRepo *repository.GuestRepository, inventorySvc *InventoryService) *BookingService {
	return &BookingService{db, bookingRepo, guestRepo, inventorySvc}
}

func (s *BookingService) CreateBooking(ctx context.Context, req models.CreateBookingRequest) (*models.CreateBookingResponse, error) {
	// 1. Validaciones básicas de fecha
	if !req.CheckOut.After(req.CheckIn) {
		return nil, &BusinessError{Code: "INVALID_DATES", Message: "check_out must be after check_in"}
	}
	if req.Adults < 1 {
		req.Adults = 1
	}

	// 2. Comprobar disponibilidad de habitación si está asignada
	if req.RoomID != nil {
		overlapBookings, err := s.bookingRepo.GetOverlapCount(ctx, *req.RoomID, req.CheckIn, req.CheckOut, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to check overlapping bookings: %w", err)
		}
		overlapBlocks, err := s.bookingRepo.GetActiveBlockOverlapCount(ctx, *req.RoomID, req.CheckIn, req.CheckOut)
		if err != nil {
			return nil, fmt.Errorf("failed to check overlapping blocks: %w", err)
		}

		if (overlapBookings > 0 || overlapBlocks > 0) && !req.ForceOverride {
			return nil, &BusinessError{Code: "ROOM_NOT_AVAILABLE", Message: "Room is not available for selected dates"}
		}
	}

	// 3. Resolver Huésped (Guest Reuse Logic BR-BOOKING-06)
	var guestID uuid.UUID
	var guestReused bool

	if req.GuestID != nil {
		guestID = *req.GuestID
		guestReused = true
	} else if req.Guest != nil {
		// Buscar por email o teléfono en la propiedad
		matches, err := s.guestRepo.FindByPhoneOrEmail(ctx, req.PropertyID, req.Guest.Phone, req.Guest.Email)
		if err != nil {
			return nil, fmt.Errorf("failed searching existing guests: %w", err)
		}

		if len(matches) > 0 {
			// Auto-vincular al primero encontrado
			guestID = matches[0].ID
			guestReused = true
		} else {
			// Crear nuevo huésped
			newGuest, err := s.guestRepo.Create(ctx, req.Guest)
			if err != nil {
				return nil, fmt.Errorf("failed to create guest: %w", err)
			}
			guestID = newGuest.ID
			guestReused = false
		}
	} else {
		return nil, &BusinessError{Code: "GUEST_REQUIRED", Message: "Guest details or guest_id must be provided"}
	}

	// 4. Asignar Huésped a la petición
	req.GuestID = &guestID

	// 5. Configurar defaults MVP para multi-currency
	if req.OriginalCurrency == "" {
		req.OriginalCurrency = "IDR"
	}
	if req.ExchangeRate == 0 {
		req.ExchangeRate = 1.0
	}
	// total_amount = original_amount * exchange_rate
	req.TotalAmount = req.OriginalAmount * req.ExchangeRate

	if req.Status == "" {
		req.Status = "confirmed"
	}
	if req.PaymentStatus == "" {
		req.PaymentStatus = "pending"
	}

	// Si hay override de disponibilidad por el usuario, dejar constancia en las notas
	if req.RoomID != nil && req.ForceOverride {
		req.Notes = "[OVERRIDE] Forzado de disponibilidad por el usuario.\n" + req.Notes
	}

	// 6. Crear la reserva
	booking, err := s.bookingRepo.Create(ctx, &req)
	if err != nil {
		return nil, err
	}

	return &models.CreateBookingResponse{
		Booking:     *booking,
		GuestReused: guestReused,
	}, nil
}

func (s *BookingService) ListBookings(ctx context.Context, propertyID uuid.UUID, status string, search string, page int, limit int) ([]*repository.BookingListDTO, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	return s.bookingRepo.List(ctx, propertyID, status, search, page, limit)
}

func (s *BookingService) GetBookingByID(ctx context.Context, id uuid.UUID) (*models.BookingDetail, error) {
	return s.bookingRepo.GetByID(ctx, id)
}

func (s *BookingService) UpdateBooking(ctx context.Context, id uuid.UUID, req models.UpdateBookingRequest) (*models.Booking, error) {
	// Obtener booking actual para validaciones de negocio
	current, err := s.bookingRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	params := make(map[string]interface{})

	// BR-BOOKING-03: No cambiar habitación o fechas si está en check-in
	if current.Status == "checked_in" {
		if req.RoomID != nil && (current.RoomID == nil || *req.RoomID != *current.RoomID) {
			return nil, &BusinessError{Code: "INVALID_OPERATION", Message: "Cannot change room for checked-in guests"}
		}
		if (!req.CheckIn.IsZero() && !req.CheckIn.Equal(current.CheckIn)) || (!req.CheckOut.IsZero() && !req.CheckOut.Equal(current.CheckOut)) {
			return nil, &BusinessError{Code: "INVALID_OPERATION", Message: "Cannot change dates for checked-in guests"}
		}
	}

	// Validar disponibilidad si cambia habitación o fechas
	var newRoomID *uuid.UUID = current.RoomID
	if req.RoomID != nil {
		newRoomID = req.RoomID
	}
	var newCheckIn time.Time = current.CheckIn
	if !req.CheckIn.IsZero() {
		newCheckIn = req.CheckIn
	}
	var newCheckOut time.Time = current.CheckOut
	if !req.CheckOut.IsZero() {
		newCheckOut = req.CheckOut
	}

	if (req.RoomID != nil && (current.RoomID == nil || *req.RoomID != *current.RoomID)) || !req.CheckIn.IsZero() || !req.CheckOut.IsZero() {
		if newRoomID != nil {
			overlapCount, err := s.bookingRepo.GetOverlapCount(ctx, *newRoomID, newCheckIn, newCheckOut, &id)
			if err != nil {
				return nil, err
			}
			blockCount, err := s.bookingRepo.GetActiveBlockOverlapCount(ctx, *newRoomID, newCheckIn, newCheckOut)
			if err != nil {
				return nil, err
			}
			if (overlapCount > 0 || blockCount > 0) && !req.ForceOverride {
				return nil, &BusinessError{Code: "ROOM_NOT_AVAILABLE", Message: "Room is not available for selected dates"}
			}
		}
	}

	if req.RoomID != nil {
		params["room_id"] = *req.RoomID
	}
	if !req.CheckIn.IsZero() {
		params["check_in"] = req.CheckIn
	}
	if !req.CheckOut.IsZero() {
		params["check_out"] = req.CheckOut
	}
	if req.Adults > 0 {
		params["adults"] = req.Adults
	}
	if req.Children >= 0 {
		params["children"] = req.Children
	}
	if req.OriginalAmount > 0 {
		params["original_amount"] = req.OriginalAmount
		params["total_amount"] = req.OriginalAmount * current.ExchangeRate
	}
	if req.Notes != "" {
		params["notes"] = req.Notes
	}

	if req.RoomID != nil && req.ForceOverride {
		currentNotesVal := ""
		if current.Notes != nil {
			currentNotesVal = *current.Notes
		}
		params["notes"] = "[OVERRIDE] Cambio de habitación forzado por el usuario.\n" + currentNotesVal
	}

	return s.bookingRepo.Update(ctx, id, params)
}

func (s *BookingService) CheckIn(ctx context.Context, bookingID uuid.UUID) error {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.Status != "confirmed" {
		return &BusinessError{Code: "INVALID_STATUS", Message: "Only confirmed bookings can be checked in"}
	}
	if booking.RoomID == nil {
		return &BusinessError{Code: "ROOM_REQUIRED", Message: "Room must be assigned before check-in"}
	}
	return s.bookingRepo.CheckIn(ctx, bookingID)
}

func (s *BookingService) CheckOut(ctx context.Context, bookingID uuid.UUID) error {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.Status != "checked_in" {
		return &BusinessError{Code: "INVALID_STATUS", Message: "Only checked-in bookings can be checked out"}
	}
	if err := s.bookingRepo.CheckOut(ctx, bookingID); err != nil {
		return err
	}

	// Auto-cleaning: tras check-out la habitación pasa automáticamente a estado
	// operacional `cleaning` (housekeeping en curso). El check-out ya está hecho
	// (el huésped se fue físicamente), así que un fallo de cleaning NO debe
	// revertirlo: lo logueamos y continuamos. Caso esperado de fallo: la room
	// fue marcada inactive entre check-in y check-out (SetRoomCleaning devuelve
	// ROOM_INACTIVE en ese caso, BR-TEREN-16).
	if booking.RoomID != nil {
		if _, cerr := s.inventorySvc.SetRoomCleaning(ctx, *booking.RoomID, true); cerr != nil {
			log.Printf(
				"auto-cleaning skipped for room %s after checkout of booking %s: %v",
				*booking.RoomID, bookingID, cerr,
			)
		}
	}
	return nil
}

func (s *BookingService) CancelBooking(ctx context.Context, bookingID uuid.UUID, reason string) error {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.Status != "confirmed" && booking.Status != "checked_in" {
		return &BusinessError{Code: "INVALID_STATUS", Message: "Only confirmed or checked-in bookings can be cancelled"}
	}
	return s.bookingRepo.Cancel(ctx, bookingID, reason)
}

func (s *BookingService) GetPendingBookings(ctx context.Context, propertyID uuid.UUID) ([]*repository.PendingBookingDTO, error) {
	return s.bookingRepo.GetPendingByProperty(ctx, propertyID)
}

func (s *BookingService) AssignRoom(ctx context.Context, bookingID, roomID uuid.UUID) error {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.Status != "confirmed" {
		return &BusinessError{Code: "INVALID_STATUS", Message: "Only confirmed bookings can have a room assigned"}
	}

	// Validar disponibilidad
	overlapCount, err := s.bookingRepo.GetOverlapCount(ctx, roomID, booking.CheckIn, booking.CheckOut, &bookingID)
	if err != nil {
		return err
	}
	blockCount, err := s.bookingRepo.GetActiveBlockOverlapCount(ctx, roomID, booking.CheckIn, booking.CheckOut)
	if err != nil {
		return err
	}
	if overlapCount > 0 || blockCount > 0 {
		return &BusinessError{Code: "ROOM_NOT_AVAILABLE", Message: "Room is not available for selected dates"}
	}

	return s.bookingRepo.AssignRoom(ctx, bookingID, roomID)
}
