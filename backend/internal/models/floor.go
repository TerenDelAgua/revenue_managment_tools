package models

import (
	"time"

	"github.com/google/uuid"
)

type Floor struct {
	ID          uuid.UUID `json:"id" db:"id"`
	PropertyID  uuid.UUID `json:"property_id" db:"property_id"`
	FloorNumber int       `json:"floor_number" db:"floor_number"`
	Label       *string   `json:"label" db:"label"`
	SortOrder   int       `json:"sort_order" db:"sort_order"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateFloorRequest struct {
	PropertyID  uuid.UUID `json:"property_id" validate:"required"`
	FloorNumber int       `json:"floor_number" validate:"required"`
	Label       *string   `json:"label"`
	SortOrder   int       `json:"sort_order"`
}

type UpdateFloorRequest struct {
	FloorNumber *int     `json:"floor_number"`
	Label       *string  `json:"label"`
	SortOrder   *int     `json:"sort_order"`
}
