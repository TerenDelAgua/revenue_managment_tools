package models

import (
	"time"

	"github.com/google/uuid"
)

type RoomBlock struct {
	ID        uuid.UUID `json:"id" db:"id"`
	RoomID    uuid.UUID `json:"room_id" db:"room_id"`
	CreatedBy uuid.UUID `json:"created_by" db:"created_by"`
	StartDate time.Time `json:"start_date" db:"start_date"`
	EndDate   time.Time `json:"end_date" db:"end_date"`
	Reason    string    `json:"reason" db:"reason"`
	Notes     *string   `json:"notes" db:"notes"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateRoomBlockRequest struct {
	RoomID    uuid.UUID `json:"room_id" validate:"required"`
	CreatedBy uuid.UUID `json:"created_by" validate:"required"`
	StartDate time.Time `json:"start_date" validate:"required"`
	EndDate   time.Time `json:"end_date" validate:"required"`
	Reason    string    `json:"reason" validate:"required"`
	Notes     *string   `json:"notes"`
}
