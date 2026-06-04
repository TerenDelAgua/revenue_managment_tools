package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type GuestRepository struct {
	db *pgxpool.Pool
}

func NewGuestRepository(db *pgxpool.Pool) *GuestRepository {
	return &GuestRepository{db: db}
}

func (r *GuestRepository) Create(ctx context.Context, req *models.CreateGuestRequest) (*models.Guest, error) {
	var g models.Guest
	err := r.db.QueryRow(ctx, `
		INSERT INTO guests (property_id, full_name, id_number, phone, email, nationality, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, property_id, full_name, id_number, phone, email, nationality, notes, created_at, updated_at
	`, req.PropertyID, req.FullName, req.IdNumber, req.Phone, req.Email, req.Nationality, req.Notes).Scan(
		&g.ID, &g.PropertyID, &g.FullName, &g.IdNumber, &g.Phone, &g.Email, &g.Nationality, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
	)
	return &g, err
}

func (r *GuestRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GuestDetail, error) {
	var detail models.GuestDetail
	
	// 1. Get Guest details
	err := r.db.QueryRow(ctx, `
		SELECT id, property_id, full_name, id_number, phone, email, nationality, notes, created_at, updated_at
		FROM guests WHERE id = $1
	`, id).Scan(
		&detail.ID, &detail.PropertyID, &detail.FullName, &detail.IdNumber, &detail.Phone, &detail.Email, &detail.Nationality, &detail.Notes, &detail.CreatedAt, &detail.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	// 2. Get Guest Stats (total bookings, total revenue, last visit)
	var lastVisitTime *time.Time
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(id) as total_bookings,
			COALESCE(SUM(total_amount), 0) as total_revenue,
			MAX(check_out) as last_visit
		FROM bookings
		WHERE guest_id = $1 AND status != 'cancelled'
	`, id).Scan(&detail.TotalBookings, &detail.TotalRevenue, &lastVisitTime)
	if err != nil {
		return nil, err
	}
	if lastVisitTime != nil {
		str := lastVisitTime.Format("2006-01-02")
		detail.LastVisit = &str
	}
	
	// 3. Get Guest Bookings list
	rows, err := r.db.Query(ctx, `
		SELECT b.id, b.check_in, b.check_out, r.number, b.status, b.total_amount
		FROM bookings b
		LEFT JOIN rooms r ON b.room_id = r.id
		WHERE b.guest_id = $1
		ORDER BY b.check_in DESC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	detail.Bookings = make([]models.GuestBookingDTO, 0)
	for rows.Next() {
		var b models.GuestBookingDTO
		var checkIn, checkOut time.Time
		if err := rows.Scan(&b.ID, &checkIn, &checkOut, &b.RoomNumber, &b.Status, &b.TotalAmount); err != nil {
			return nil, err
		}
		b.CheckIn = checkIn.Format("2006-01-02")
		b.CheckOut = checkOut.Format("2006-01-02")
		detail.Bookings = append(detail.Bookings, b)
	}
	
	return &detail, rows.Err()
}

func (r *GuestRepository) Update(ctx context.Context, id uuid.UUID, params map[string]interface{}) (*models.Guest, error) {
	if len(params) == 0 {
		var g models.Guest
		err := r.db.QueryRow(ctx, `SELECT id, property_id, full_name, id_number, phone, email, nationality, notes, created_at, updated_at FROM guests WHERE id = $1`, id).Scan(
			&g.ID, &g.PropertyID, &g.FullName, &g.IdNumber, &g.Phone, &g.Email, &g.Nationality, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
		)
		return &g, err
	}

	query := "UPDATE guests SET "
	args := []interface{}{id}
	idx := 2

	for k, v := range params {
		query += fmt.Sprintf("%s = $%d, ", k, idx)
		args = append(args, v)
		idx++
	}
	query = strings.TrimSuffix(query, ", ") + " WHERE id = $1 RETURNING id, property_id, full_name, id_number, phone, email, nationality, notes, created_at, updated_at"

	var g models.Guest
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&g.ID, &g.PropertyID, &g.FullName, &g.IdNumber, &g.Phone, &g.Email, &g.Nationality, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
	)
	return &g, err
}

func (r *GuestRepository) List(ctx context.Context, propertyID uuid.UUID, search string, page int, limit int) ([]*models.GuestListDTO, int, error) {
	offset := (page - 1) * limit
	
	whereClause := "WHERE g.property_id = $1 "
	args := []interface{}{propertyID}
	paramIdx := 2
	
	if search != "" {
		whereClause += fmt.Sprintf("AND (g.full_name ILIKE $%d OR g.phone ILIKE $%d OR g.email ILIKE $%d) ", paramIdx, paramIdx, paramIdx)
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern)
		paramIdx++
	}
	
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM guests g %s", whereClause)
	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	
	query := fmt.Sprintf(`
		SELECT 
			g.id, g.full_name, g.phone, g.email, g.nationality,
			COUNT(b.id) as booking_count,
			MAX(b.check_out) as last_visit
		FROM guests g
		LEFT JOIN bookings b ON g.id = b.guest_id AND b.status != 'cancelled'
		%s
		GROUP BY g.id, g.full_name, g.phone, g.email, g.nationality, g.created_at
		ORDER BY g.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, paramIdx, paramIdx+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	list := make([]*models.GuestListDTO, 0)
	for rows.Next() {
		var g models.GuestListDTO
		var lastVisitTime *time.Time
		if err := rows.Scan(&g.ID, &g.FullName, &g.Phone, &g.Email, &g.Nationality, &g.BookingCount, &lastVisitTime); err != nil {
			return nil, 0, err
		}
		if lastVisitTime != nil {
			str := lastVisitTime.Format("2006-01-02")
			g.LastVisit = &str
		}
		list = append(list, &g)
	}
	
	return list, total, rows.Err()
}

func (r *GuestRepository) FindByPhoneOrEmail(ctx context.Context, propertyID uuid.UUID, phone string, email *string) ([]*models.Guest, error) {
	query := `
		SELECT id, property_id, full_name, id_number, phone, email, nationality, notes, created_at, updated_at
		FROM guests
		WHERE property_id = $1 AND (phone = $2 OR (email IS NOT NULL AND $3::text IS NOT NULL AND email = $3::text))
	`
	rows, err := r.db.Query(ctx, query, propertyID, phone, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var list []*models.Guest
	for rows.Next() {
		var g models.Guest
		if err := rows.Scan(&g.ID, &g.PropertyID, &g.FullName, &g.IdNumber, &g.Phone, &g.Email, &g.Nationality, &g.Notes, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &g)
	}
	return list, rows.Err()
}

func (r *GuestRepository) FindExactMatch(ctx context.Context, propertyID uuid.UUID, name string, phone string, email *string) (*models.Guest, error) {
	query := `
		SELECT id, property_id, full_name, id_number, phone, email, nationality, notes, created_at, updated_at
		FROM guests
		WHERE property_id = $1 
		  AND LOWER(full_name) = LOWER($2) 
		  AND (phone = $3 OR (email IS NOT NULL AND $4::text IS NOT NULL AND email = $4::text))
		LIMIT 1
	`
	var g models.Guest
	err := r.db.QueryRow(ctx, query, propertyID, name, phone, email).Scan(
		&g.ID, &g.PropertyID, &g.FullName, &g.IdNumber, &g.Phone, &g.Email, &g.Nationality, &g.Notes, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}
