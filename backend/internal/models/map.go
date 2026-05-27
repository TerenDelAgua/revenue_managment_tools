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
	ID          uuid.UUID  `json:"id"`
	Label       string     `json:"label"`
	FloorNumber int        `json:"floor_number"`
	SortOrder   int        `json:"sort_order"`
	Rooms       []*RoomMap `json:"rooms"`
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

	// Guest information
	ActiveGuestName        *string `json:"active_guest_name"`
	ActiveGuestPhone       *string `json:"active_guest_phone"`
	ActiveGuestNationality *string `json:"active_guest_nationality"`
	ActiveCheckIn          *string `json:"active_check_in"` // YYYY-MM-DD
	ActiveCheckOut         *string `json:"active_check_out"`

	PendingGuestName        *string `json:"pending_guest_name"`
	PendingGuestPhone       *string `json:"pending_guest_phone"`
	PendingGuestNationality *string `json:"pending_guest_nationality"`
	PendingCheckIn          *string `json:"pending_check_in"`
	PendingCheckOut         *string `json:"pending_check_out"`
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
