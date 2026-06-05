package models

import (
	"time"

	"github.com/google/uuid"
)

type RoomType struct {
	ID           uuid.UUID `json:"id" db:"id"`
	PropertyID   uuid.UUID `json:"property_id" db:"property_id"`
	Name         string    `json:"name" db:"name"`
	MaxOccupancy int       `json:"max_occupancy" db:"max_occupancy"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Room struct {
	ID          uuid.UUID `json:"id" db:"id"`
	FloorID     uuid.UUID `json:"floor_id" db:"floor_id"`
	PropertyID  uuid.UUID `json:"property_id" db:"property_id"`
	RoomTypeID  uuid.UUID `json:"room_type_id" db:"room_type_id"`
	Number      string    `json:"number" db:"number"`
	Status      string    `json:"status" db:"status"`
	PosX        int       `json:"pos_x" db:"pos_x"`
	PosY        int       `json:"pos_y" db:"pos_y"`
	HasBookings bool      `json:"has_bookings" db:"has_bookings"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateRoomRequest struct {
	FloorID    uuid.UUID `json:"floor_id" validate:"required"`
	RoomTypeID uuid.UUID `json:"room_type_id" validate:"required"`
	Number     string    `json:"number"` // Optional, auto-generated if empty
	Status     string    `json:"status" validate:"required"`
	PosX       int       `json:"pos_x"`
	PosY       int       `json:"pos_y"`
}

type UpdateRoomRequest struct {
	RoomTypeID *uuid.UUID `json:"room_type_id"`
	Number     *string    `json:"number"`
	Status     *string    `json:"status"`
	PosX       *int       `json:"pos_x"`
	PosY       *int       `json:"pos_y"`
}

type UpdateRoomPositionRequest struct {
	PosX int `json:"pos_x" validate:"required"`
	PosY int `json:"pos_y" validate:"required"`
}
