package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type PropertyRepository struct {
	db *pgxpool.Pool
}

func NewPropertyRepository(db *pgxpool.Pool) *PropertyRepository {
	return &PropertyRepository{db: db}
}

func (r *PropertyRepository) Create(ctx context.Context, req *models.CreatePropertyRequest) (*models.Property, error) {
	var property models.Property
	settings := req.Settings
	if settings == nil {
		settings = make(map[string]any)
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO properties (name, slug, currency, timezone, settings)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, slug, currency, timezone, settings, created_at, updated_at
	`, req.Name, req.Slug, req.Currency, req.Timezone, settings).Scan(
		&property.ID, &property.Name, &property.Slug, &property.Currency,
		&property.Timezone, &property.Settings, &property.CreatedAt, &property.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}
	return &property, nil
}

func (r *PropertyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Property, error) {
	var property models.Property
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, currency, timezone, settings, created_at, updated_at
		FROM properties WHERE id = $1
	`, id).Scan(
		&property.ID, &property.Name, &property.Slug, &property.Currency,
		&property.Timezone, &property.Settings, &property.CreatedAt, &property.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &property, nil
}

func (r *PropertyRepository) List(ctx context.Context) ([]*models.Property, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, slug, currency, timezone, settings, created_at, updated_at
		FROM properties ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var properties []*models.Property
	for rows.Next() {
		var property models.Property
		err := rows.Scan(
			&property.ID, &property.Name, &property.Slug, &property.Currency,
			&property.Timezone, &property.Settings, &property.CreatedAt, &property.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		properties = append(properties, &property)
	}

	return properties, rows.Err()
}
