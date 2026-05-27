package repository

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type ReportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{db: db}
}

// GetMetrics calcula KPIs hoteleros para un rango de fechas.
// Usa una única consulta agregada con solapamiento exacto de rangos.
func (r *ReportRepository) GetMetrics(ctx context.Context, req models.ReportRequest) (*models.ReportResponse, error) {
	var res models.ReportResponse

	err := r.db.QueryRow(ctx, `
		WITH params AS (
			SELECT $1::uuid AS property_id, $2::date AS date_from, $3::date AS date_to
		),
		stats AS (
			SELECT
				COUNT(DISTINCT r.id) FILTER (WHERE r.status != 'inactive') AS total_rooms,
				($3::date - $2::date) AS days_in_range,
				COALESCE(SUM(
					LEAST(b.check_out, $3::date) - GREATEST(b.check_in, $2::date)
				), 0) AS booked_nights,
				COALESCE(SUM(
					b.total_amount::numeric * 
					(LEAST(b.check_out, $3::date) - GREATEST(b.check_in, $2::date))::numeric / 
					(b.check_out - b.check_in)::numeric
				), 0) AS revenue
			FROM rooms r
			LEFT JOIN bookings b ON b.room_id = r.id
				AND b.status NOT IN ('cancelled', 'no_show')
				AND b.check_in < $3::date
				AND b.check_out > $2::date
			WHERE r.property_id = $1
		)
		SELECT
			total_rooms,
			days_in_range,
			booked_nights,
			revenue,
			CASE WHEN days_in_range > 0 AND total_rooms > 0 
				THEN ROUND((booked_nights::numeric / (total_rooms::numeric * days_in_range::numeric)) * 100, 2) 
				ELSE 0 
			END AS occupancy_rate,
			CASE WHEN booked_nights > 0 
				THEN ROUND(revenue / booked_nights::numeric, 2) 
				ELSE 0 
			END AS adr,
			CASE WHEN days_in_range > 0 AND total_rooms > 0 
				THEN ROUND(revenue / (total_rooms::numeric * days_in_range::numeric), 2) 
				ELSE 0 
			END AS revpar
		FROM stats
	`, req.PropertyID, req.DateFrom, req.DateTo).Scan(
		&res.TotalRooms, &res.DaysInRange, &res.BookedNights, &res.TotalRevenue,
		&res.OccupancyRate, &res.ADR, &res.RevPAR,
	)
	if err != nil {
		return nil, err
	}

	res.PropertyID = req.PropertyID
	res.DateFrom = req.DateFrom.Format("2006-01-02")
	res.DateTo = req.DateTo.Format("2006-01-02")

	// Asegurar que no haya -0.00 por redondeo flotante
	if math.Abs(res.RevPAR) < 0.005 {
		res.RevPAR = 0
	}
	if math.Abs(res.ADR) < 0.005 {
		res.ADR = 0
	}

	return &res, nil
}

func (r *ReportRepository) GetDailyBreakdown(ctx context.Context, req models.ReportRequest) (*models.DailyBreakdownResponse, error) {
	// Primero obtenemos el total de rooms (constante)
	var totalRooms int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms 
		WHERE property_id = $1 AND status != 'inactive'
	`, req.PropertyID).Scan(&totalRooms)
	if err != nil {
		return nil, err
	}

	// Generamos series temporal día a día
	rows, err := r.db.Query(ctx, `
		WITH date_series AS (
			SELECT generate_series($2::date, $3::date, '1 day'::interval)::date AS date
		),
		daily_stats AS (
			SELECT 
				ds.date,
				COUNT(DISTINCT b.room_id) AS occupied_rooms,
				COALESCE(SUM(
					CASE 
						WHEN b.status NOT IN ('cancelled', 'no_show') 
						THEN b.total_amount / NULLIF(b.check_out - b.check_in, 0)
						ELSE 0
					END
				), 0) AS daily_revenue
			FROM date_series ds
			LEFT JOIN bookings b ON b.room_id IN (
				SELECT id FROM rooms WHERE property_id = $1
			)
			AND b.status NOT IN ('cancelled', 'no_show')
			AND b.check_in <= ds.date AND b.check_out > ds.date
			GROUP BY ds.date
		)
		SELECT 
			ds.date,
			COALESCE(ds.occupied_rooms, 0) AS occupied_rooms,
			$4 - COALESCE(ds.occupied_rooms, 0) AS available_rooms,
			$4 AS total_rooms,
			CASE 
				WHEN $4 > 0 
				THEN ROUND((COALESCE(ds.occupied_rooms, 0)::numeric / $4::numeric) * 100, 2)
				ELSE 0 
			END AS occupancy_rate,
			COALESCE(ds.daily_revenue, 0) AS daily_revenue,
			CASE 
				WHEN COALESCE(ds.occupied_rooms, 0) > 0 
				THEN ROUND(ds.daily_revenue / ds.occupied_rooms::numeric, 2)
				ELSE 0 
			END AS adr,
			CASE 
				WHEN $4 > 0 
				THEN ROUND(ds.daily_revenue / $4::numeric, 2)
				ELSE 0 
			END AS revpar
		FROM date_series ds
		LEFT JOIN daily_stats ds2 ON ds.date = ds2.date
		ORDER BY ds.date ASC
	`, req.PropertyID, req.DateFrom, req.DateTo, totalRooms)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []models.DailyBreakdown
	for rows.Next() {
		var day models.DailyBreakdown
		var date time.Time
		if err := rows.Scan(
			&date,
			&day.OccupiedRooms,
			&day.AvailableRooms,
			&day.TotalRooms,
			&day.OccupancyRate,
			&day.DailyRevenue,
			&day.ADR,
			&day.RevPAR,
		); err != nil {
			return nil, err
		}
		day.Date = date.Format("2006-01-02")
		days = append(days, day)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Calculamos el resumen general
	summary, err := r.GetMetrics(ctx, req)
	if err != nil {
		return nil, err
	}

	return &models.DailyBreakdownResponse{
		PropertyID: req.PropertyID.String(),
		DateFrom:   req.DateFrom.Format("2006-01-02"),
		DateTo:     req.DateTo.Format("2006-01-02"),
		Days:       days,
		Summary:    *summary,
	}, nil
}
