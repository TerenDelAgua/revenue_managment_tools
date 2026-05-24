package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type FloorRepository struct {
	db *pgxpool.Pool
}

func NewFloorRepository(db *pgxpool.Pool) *FloorRepository {
	return &FloorRepository{db: db}
}

func (r *FloorRepository) Create(ctx context.Context, req *models.CreateFloorRequest) (*models.Floor, error) {
	var floor models.Floor
	sortOrder := req.SortOrder
	if sortOrder == 0 {
		sortOrder = req.FloorNumber
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO floors (property_id, floor_number, label, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING id, property_id, floor_number, label, sort_order, created_at, updated_at
	`, req.PropertyID, req.FloorNumber, req.Label, sortOrder).Scan(
		&floor.ID, &floor.PropertyID, &floor.FloorNumber, &floor.Label,
		&floor.SortOrder, &floor.CreatedAt, &floor.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}
	return &floor, nil
}

func (r *FloorRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Floor, error) {
	var floor models.Floor
	err := r.db.QueryRow(ctx, `
		SELECT id, property_id, floor_number, label, sort_order, created_at, updated_at
		FROM floors WHERE id = $1
	`, id).Scan(
		&floor.ID, &floor.PropertyID, &floor.FloorNumber, &floor.Label,
		&floor.SortOrder, &floor.CreatedAt, &floor.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &floor, nil
}

func (r *FloorRepository) ListByProperty(ctx context.Context, propertyID uuid.UUID) ([]*models.Floor, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, property_id, floor_number, label, sort_order, created_at, updated_at
		FROM floors WHERE property_id = $1 ORDER BY sort_order ASC, floor_number ASC
	`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var floors []*models.Floor
	for rows.Next() {
		var floor models.Floor
		err := rows.Scan(
			&floor.ID, &floor.PropertyID, &floor.FloorNumber, &floor.Label,
			&floor.SortOrder, &floor.CreatedAt, &floor.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		floors = append(floors, &floor)
	}

	return floors, rows.Err()
}
