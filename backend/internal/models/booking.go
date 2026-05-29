package models

import (
	"time"

	"github.com/google/uuid"
)

type Booking struct {
	ID          uuid.UUID `json:"id" db:"id"`
	PropertyID  uuid.UUID `json:"property_id" db:"property_id"`
	RoomID      uuid.UUID `json:"room_id" db:"room_id"`
	GuestID     uuid.UUID `json:"guest_id" db:"guest_id"`
	CreatedBy   uuid.UUID `json:"created_by" db:"created_by"`
	CheckIn     time.Time `json:"check_in" db:"check_in"`
	CheckOut    time.Time `json:"check_out" db:"check_out"`
	TotalAmount float64   `json:"total_amount" db:"total_amount"`
	Source      string    `json:"source" db:"source"`
	Status      string    `json:"status" db:"status"`
	Notes       string    `json:"notes" db:"notes"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateBookingRequest struct {
	PropertyID  uuid.UUID `json:"property_id" binding:"required"`
	RoomID      uuid.UUID `json:"room_id" binding:"required"`
	GuestID     uuid.UUID `json:"guest_id" binding:"required"`
	CreatedBy   uuid.UUID `json:"created_by" binding:"required"`
	CheckIn     time.Time `json:"check_in" binding:"required"`
	CheckOut    time.Time `json:"check_out" binding:"required"`
	Source      string    `json:"source" binding:"required"`
	Status      string    `json:"status" binding:"required"`
	TotalAmount float64   `json:"total_amount" binding:"required"`
	Notes       string    `json:"notes"`
}
