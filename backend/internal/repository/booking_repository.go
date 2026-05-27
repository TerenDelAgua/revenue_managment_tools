package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type BookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{db: db}
}

// Create reserva una habitación.
func (r *BookingRepository) Create(ctx context.Context, req *models.CreateBookingRequest) (*models.Booking, error) {
	var booking models.Booking
	err := r.db.QueryRow(ctx, `
		INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status, notes, created_at, updated_at
	`, req.PropertyID, req.RoomID, req.GuestID, req.CreatedBy, req.CheckIn, req.CheckOut, req.TotalAmount, req.Source, req.Status, req.Notes).Scan(
		&booking.ID, &booking.PropertyID, &booking.RoomID, &booking.GuestID, &booking.CreatedBy,
		&booking.CheckIn, &booking.CheckOut, &booking.TotalAmount, &booking.Source, &booking.Status,
		&booking.Notes, &booking.CreatedAt, &booking.UpdatedAt,
	)
	return &booking, err
}

// CheckIn cambia el estado a 'checked_in'.
func (r *BookingRepository) CheckIn(ctx context.Context, bookingID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE bookings SET status = 'checked_in', updated_at = NOW() WHERE id = $1 AND status = 'confirmed'`, bookingID)
	return err
}

// CheckOut cambia el estado a 'checked_out'.
func (r *BookingRepository) CheckOut(ctx context.Context, bookingID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE bookings SET status = 'checked_out', updated_at = NOW() WHERE id = $1 AND status = 'checked_in'`, bookingID)
	return err
}

// AssignRoom asigna un ID de habitación a una reserva.
func (r *BookingRepository) AssignRoom(ctx context.Context, bookingID, roomID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE bookings SET room_id = $1, updated_at = NOW() WHERE id = $2`, roomID, bookingID)
	return err
}

// GetPendingByRoom devuelve reservas confirmadas pero no checked-in para una habitación en un rango de fechas.
func (r *BookingRepository) GetPendingByRoom(ctx context.Context, roomID uuid.UUID, start, end time.Time) ([]*models.Booking, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status, notes, created_at, updated_at
		FROM bookings
		WHERE room_id = $1 AND status = 'confirmed' AND check_in < $3 AND check_out > $2
		ORDER BY check_in ASC
	`, roomID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*models.Booking
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.PropertyID, &b.RoomID, &b.GuestID, &b.CreatedBy, &b.CheckIn, &b.CheckOut, &b.TotalAmount, &b.Source, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		bookings = append(bookings, &b)
	}
	return bookings, rows.Err()
}

type PendingBookingDTO struct {
	ID          uuid.UUID `json:"id"`
	GuestName   string    `json:"guest_name"`
	CheckIn     string    `json:"check_in"`
	CheckOut    string    `json:"check_out"`
	Source      string    `json:"source"`
	Adults      int       `json:"adults"`
	TotalAmount int       `json:"total_amount"`
}

func (r *BookingRepository) GetPendingByProperty(ctx context.Context, propertyID uuid.UUID) ([]*PendingBookingDTO, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.id, g.full_name, b.check_in, b.check_out, b.source, b.total_amount
		FROM bookings b
		JOIN guests g ON b.guest_id = g.id
		WHERE b.property_id = $1 AND b.status = 'confirmed'
		ORDER BY b.check_in ASC
	`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*PendingBookingDTO
	for rows.Next() {
		var b PendingBookingDTO
		var checkIn, checkOut time.Time
		if err := rows.Scan(&b.ID, &b.GuestName, &checkIn, &checkOut, &b.Source, &b.TotalAmount); err != nil {
			return nil, err
		}
		b.CheckIn = checkIn.Format("2006-01-02")
		b.CheckOut = checkOut.Format("2006-01-02")
		b.Adults = 2 // Hardcoded as it's not in DB schema currently
		bookings = append(bookings, &b)
	}
	return bookings, rows.Err()
}
