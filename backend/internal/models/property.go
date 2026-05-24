package models

import (
	"time"

	"github.com/google/uuid"
)

type Property struct {
	ID        uuid.UUID      `json:"id" db:"id"`
	Name      string         `json:"name" db:"name"`
	Slug      string         `json:"slug" db:"slug"`
	Currency  string         `json:"currency" db:"currency"`
	Timezone  string         `json:"timezone" db:"timezone"`
	Settings  map[string]any `json:"settings" db:"settings"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
}

type CreatePropertyRequest struct {
	Name     string         `json:"name" validate:"required"`
	Slug     string         `json:"slug" validate:"required"`
	Currency string         `json:"currency" validate:"required"`
	Timezone string         `json:"timezone" validate:"required"`
	Settings map[string]any `json:"settings"`
}

type UpdatePropertyRequest struct {
	Name     *string        `json:"name"`
	Slug     *string        `json:"slug"`
	Currency *string        `json:"currency"`
	Timezone *string        `json:"timezone"`
	Settings map[string]any `json:"settings"`
}
