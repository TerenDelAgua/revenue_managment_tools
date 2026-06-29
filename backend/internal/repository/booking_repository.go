package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

// Querier is the minimal subset of pgx methods shared by *pgxpool.Pool
// and pgx.Tx. Repos accept this so the service layer can pass either
// (non-tx reads use the pool; tx-wrapped writes use the tx).
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type BookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{db: db}
}

func scanBooking(row pgx.Row, b *models.Booking) error {
	return row.Scan(
		&b.ID, &b.PropertyID, &b.RoomID, &b.GuestID, &b.CreatedBy,
		&b.CheckIn, &b.CheckOut, &b.Adults, &b.Children, &b.OriginalAmount,
		&b.OriginalCurrency, &b.ExchangeRate, &b.TotalAmount, &b.PaymentStatus,
		&b.Source, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt,
	)
}

func (r *BookingRepository) Create(ctx context.Context, req *models.CreateBookingRequest) (*models.Booking, error) {
	return r.CreateWithTx(ctx, r.db, req)
}

// CreateWithTx inserts a booking using the provided querier (pool or tx).
// Used by BookingService to share a transaction with InvoiceService
// (BR-INV-001: booking + invoice creation is atomic).
func (r *BookingRepository) CreateWithTx(ctx context.Context, q Querier, req *models.CreateBookingRequest) (*models.Booking, error) {
	if req.CreatedBy == uuid.Nil {
		var userID uuid.UUID
		err := q.QueryRow(ctx, "SELECT id FROM users WHERE property_id = $1 LIMIT 1", req.PropertyID).Scan(&userID)
		if err == nil {
			req.CreatedBy = userID
		} else {
			// Si no hay usuarios en la DB (raro), podemos intentar crear uno por defecto o buscar el primero globalmente
			err = q.QueryRow(ctx, "SELECT id FROM users LIMIT 1").Scan(&userID)
			if err == nil {
				req.CreatedBy = userID
			}
		}
	}

	var booking models.Booking
	err := q.QueryRow(ctx, `
		INSERT INTO bookings (property_id, room_id, guest_id, created_by, check_in, check_out, adults, children, original_amount, original_currency, exchange_rate, total_amount, payment_status, source, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, property_id, room_id, guest_id, created_by, check_in, check_out, adults, children, original_amount, original_currency, exchange_rate, total_amount, payment_status, source, status, notes, created_at, updated_at
	`, req.PropertyID, req.RoomID, req.GuestID, req.CreatedBy, req.CheckIn, req.CheckOut, req.Adults, req.Children, req.OriginalAmount, req.OriginalCurrency, req.ExchangeRate, req.TotalAmount, req.PaymentStatus, req.Source, req.Status, req.Notes).Scan(
		&booking.ID, &booking.PropertyID, &booking.RoomID, &booking.GuestID, &booking.CreatedBy,
		&booking.CheckIn, &booking.CheckOut, &booking.Adults, &booking.Children, &booking.OriginalAmount,
		&booking.OriginalCurrency, &booking.ExchangeRate, &booking.TotalAmount, &booking.PaymentStatus,
		&booking.Source, &booking.Status, &booking.Notes, &booking.CreatedAt, &booking.UpdatedAt,
	)
	return &booking, err
}

// GetByID recupera los detalles de un booking por ID, incluyendo relaciones
func (r *BookingRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.BookingDetail, error) {
	var detail models.BookingDetail
	var checkIn, checkOut time.Time
	err := r.db.QueryRow(ctx, `
		SELECT 
			b.id, b.property_id, b.room_id, b.guest_id, b.created_by,
			b.check_in, b.check_out, b.adults, b.children, b.original_amount,
			b.original_currency, b.exchange_rate, b.total_amount, b.payment_status,
			b.source, b.status, b.notes, b.created_at, b.updated_at,
			g.full_name, g.phone, g.email, g.nationality, g.id_number, g.notes,
			r.number, rt.name,
			u.name
		FROM bookings b
		JOIN guests g ON b.guest_id = g.id
		LEFT JOIN rooms r ON b.room_id = r.id
		LEFT JOIN room_types rt ON r.room_type_id = rt.id
		LEFT JOIN users u ON b.created_by = u.id
		WHERE b.id = $1
	`, id).Scan(
		&detail.ID, &detail.PropertyID, &detail.RoomID, &detail.GuestID, &detail.CreatedBy,
		&checkIn, &checkOut, &detail.Adults, &detail.Children, &detail.OriginalAmount,
		&detail.OriginalCurrency, &detail.ExchangeRate, &detail.TotalAmount, &detail.PaymentStatus,
		&detail.Source, &detail.Status, &detail.Notes, &detail.CreatedAt, &detail.UpdatedAt,
		&detail.GuestName, &detail.GuestPhone, &detail.GuestEmail, &detail.GuestNationality, &detail.GuestIdNumber, &detail.GuestNotes,
		&detail.RoomNumber, &detail.RoomTypeName,
		&detail.CreatedByName,
	)
	if err != nil {
		return nil, err
	}
	detail.CheckIn = checkIn
	detail.CheckOut = checkOut
	return &detail, nil
}

// Update actualiza campos específicos de un booking.
func (r *BookingRepository) Update(ctx context.Context, id uuid.UUID, params map[string]interface{}) (*models.Booking, error) {
	if len(params) == 0 {
		// Fetch existing
		var b models.Booking
		err := r.db.QueryRow(ctx, `SELECT id, property_id, room_id, guest_id, created_by, check_in, check_out, adults, children, original_amount, original_currency, exchange_rate, total_amount, payment_status, source, status, notes, created_at, updated_at FROM bookings WHERE id = $1`, id).Scan(
			&b.ID, &b.PropertyID, &b.RoomID, &b.GuestID, &b.CreatedBy,
			&b.CheckIn, &b.CheckOut, &b.Adults, &b.Children, &b.OriginalAmount,
			&b.OriginalCurrency, &b.ExchangeRate, &b.TotalAmount, &b.PaymentStatus,
			&b.Source, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt,
		)
		return &b, err
	}

	query := "UPDATE bookings SET "
	args := []interface{}{id}
	idx := 2

	for k, v := range params {
		query += fmt.Sprintf("%s = $%d, ", k, idx)
		args = append(args, v)
		idx++
	}
	query = strings.TrimSuffix(query, ", ") + " WHERE id = $1 RETURNING id, property_id, room_id, guest_id, created_by, check_in, check_out, adults, children, original_amount, original_currency, exchange_rate, total_amount, payment_status, source, status, notes, created_at, updated_at"

	var b models.Booking
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&b.ID, &b.PropertyID, &b.RoomID, &b.GuestID, &b.CreatedBy,
		&b.CheckIn, &b.CheckOut, &b.Adults, &b.Children, &b.OriginalAmount,
		&b.OriginalCurrency, &b.ExchangeRate, &b.TotalAmount, &b.PaymentStatus,
		&b.Source, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt,
	)
	return &b, err
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

// Cancel cancela un booking.
func (r *BookingRepository) Cancel(ctx context.Context, bookingID uuid.UUID, reason string) error {
	notesUpdate := "[CANCELLED] reason: " + reason
	_, err := r.db.Exec(ctx, `UPDATE bookings SET status = 'cancelled', notes = COALESCE(notes, '') || '\n' || $2, updated_at = NOW() WHERE id = $1 AND status IN ('confirmed', 'checked_in')`, bookingID, notesUpdate)
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
		SELECT id, property_id, room_id, guest_id, created_by, check_in, check_out, adults, children, original_amount, original_currency, exchange_rate, total_amount, payment_status, source, status, notes, created_at, updated_at
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
		if err := scanBooking(rows, &b); err != nil {
			return nil, err
		}
		bookings = append(bookings, &b)
	}
	return bookings, rows.Err()
}

type PendingBookingDTO struct {
	ID          uuid.UUID  `json:"id"`
	GuestName   string     `json:"guest_name"`
	CheckIn     string     `json:"check_in"`
	CheckOut    string     `json:"check_out"`
	Source      string     `json:"source"`
	Adults      int        `json:"adults"`
	TotalAmount float64    `json:"total_amount"`
	RoomID      *uuid.UUID `json:"room_id"`
	Status      string     `json:"status"`
}

func (r *BookingRepository) GetPendingByProperty(ctx context.Context, propertyID uuid.UUID) ([]*PendingBookingDTO, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.id, g.full_name, b.check_in, b.check_out, b.source, b.adults, b.total_amount, b.room_id, b.status
		FROM bookings b
		JOIN guests g ON b.guest_id = g.id
		WHERE b.property_id = $1 AND b.status = 'confirmed' AND b.room_id IS NULL
		ORDER BY b.check_in ASC
	`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := make([]*PendingBookingDTO, 0)
	for rows.Next() {
		var b PendingBookingDTO
		var checkIn, checkOut time.Time
		if err := rows.Scan(&b.ID, &b.GuestName, &checkIn, &checkOut, &b.Source, &b.Adults, &b.TotalAmount, &b.RoomID, &b.Status); err != nil {
			return nil, err
		}
		b.CheckIn = checkIn.Format("2006-01-02")
		b.CheckOut = checkOut.Format("2006-01-02")
		bookings = append(bookings, &b)
	}
	return bookings, rows.Err()
}

type BookingListDTO struct {
	ID               uuid.UUID  `json:"id"`
	RoomID           *uuid.UUID `json:"room_id"`
	RoomNumber       *string    `json:"room_number"`
	GuestName        string     `json:"guest_name"`
	GuestPhone       string     `json:"guest_phone"`
	CheckIn          string     `json:"check_in"`
	CheckOut         string     `json:"check_out"`
	Nights           int        `json:"nights"`
	OriginalAmount   float64    `json:"original_amount"`
	OriginalCurrency string     `json:"original_currency"`
	TotalAmount      float64    `json:"total_amount"`
	Status           string     `json:"status"`
	Source           string     `json:"source"`
}

func (r *BookingRepository) List(ctx context.Context, propertyID uuid.UUID, status string, search string, roomID *uuid.UUID, page int, limit int) ([]*BookingListDTO, int, error) {
	offset := (page - 1) * limit

	whereClause := "WHERE b.property_id = $1 "
	args := []interface{}{propertyID}
	paramIdx := 2

	if roomID != nil {
		whereClause += fmt.Sprintf("AND b.room_id = $%d ", paramIdx)
		args = append(args, *roomID)
		paramIdx++
	}

	if status != "" {
		whereClause += fmt.Sprintf("AND b.status = $%d ", paramIdx)
		args = append(args, status)
		paramIdx++
	}
	if search != "" {
		whereClause += fmt.Sprintf("AND (g.full_name ILIKE $%d OR g.phone ILIKE $%d OR r.number ILIKE $%d) ", paramIdx, paramIdx, paramIdx)
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern)
		paramIdx++
	}
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM bookings b 
		JOIN guests g ON b.guest_id = g.id
		LEFT JOIN rooms r ON b.room_id = r.id
		%s
	`, whereClause)
	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	query := fmt.Sprintf(`
		SELECT b.id, b.room_id, r.number, g.full_name, g.phone, b.check_in, b.check_out, 
		       (b.check_out - b.check_in) as nights, b.original_amount, b.original_currency, b.total_amount, b.status, b.source
		FROM bookings b
		JOIN guests g ON b.guest_id = g.id
		LEFT JOIN rooms r ON b.room_id = r.id
		%s
		ORDER BY b.check_in DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, paramIdx, paramIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]*BookingListDTO, 0)
	for rows.Next() {
		var b BookingListDTO
		var checkIn, checkOut time.Time
		if err := rows.Scan(&b.ID, &b.RoomID, &b.RoomNumber, &b.GuestName, &b.GuestPhone, &checkIn, &checkOut, &b.Nights, &b.OriginalAmount, &b.OriginalCurrency, &b.TotalAmount, &b.Status, &b.Source); err != nil {
			return nil, 0, err
		}
		b.CheckIn = checkIn.Format("2006-01-02")
		b.CheckOut = checkOut.Format("2006-01-02")
		list = append(list, &b)
	}
	return list, total, rows.Err()
}

func (r *BookingRepository) GetOverlapCount(ctx context.Context, roomID uuid.UUID, checkIn, checkOut time.Time, excludeBookingID *uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM bookings
		WHERE room_id = $1 
		  AND status IN ('confirmed', 'checked_in')
		  AND check_in < $3 
		  AND check_out > $2
	`
	args := []interface{}{roomID, checkIn, checkOut}
	if excludeBookingID != nil {
		query += " AND id != $4"
		args = append(args, *excludeBookingID)
	}
	var count int
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *BookingRepository) GetActiveBlockOverlapCount(ctx context.Context, roomID uuid.UUID, checkIn, checkOut time.Time) (int, error) {
	query := `
		SELECT COUNT(*) FROM room_blocks
		WHERE room_id = $1
		  AND start_date < $3
		  AND end_date > $2
	`
	var count int
	err := r.db.QueryRow(ctx, query, roomID, checkIn, checkOut).Scan(&count)
	return count, err
}

func (r *BookingRepository) GetActiveBookingsByGuest(ctx context.Context, guestID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM bookings WHERE guest_id = $1 AND status != 'cancelled'`, guestID).Scan(&count)
	return count, err
}
