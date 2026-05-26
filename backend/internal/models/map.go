package models

import (
	"time"

	"github.com/google/uuid"
)

type MapAvailabilityRequest struct {
	PropertyID uuid.UUID `json:"property_id"`
	DateFrom   time.Time `json:"date_from"`
	DateTo     time.Time `json:"date_to"`
}

type MapResponse struct {
	PropertyID uuid.UUID  `json:"property_id"`
	DateFrom   string     `json:"date_from"`
	DateTo     string     `json:"date_to"`
	Floors     []FloorMap `json:"floors"`
}

type FloorMap struct {
	ID          uuid.UUID `json:"id"`
	Label       string    `json:"label"`
	FloorNumber int       `json:"floor_number"`
	SortOrder   int       `json:"sort_order"`
	Rooms       []RoomMap `json:"rooms"`
}

type RoomMap struct {
	ID               uuid.UUID   `json:"id"`
	Number           string      `json:"number"`
	PosX             int         `json:"pos_x"`
	PosY             int         `json:"pos_y"`
	RoomType         RoomTypeRef `json:"room_type"`
	Availability     string      `json:"availability"`
	ActiveBookingID  *uuid.UUID  `json:"active_booking"`
	PendingBookingID *uuid.UUID  `json:"pending_booking"`
	BlockID          *uuid.UUID  `json:"block"`
}

type RoomTypeRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type RoomPositionUpdate struct {
	ID   uuid.UUID `json:"id"`
	PosX int       `json:"pos_x"`
	PosY int       `json:"pos_y"`
}
