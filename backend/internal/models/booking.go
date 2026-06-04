package models

import (
	"time"

	"github.com/google/uuid"
)

type Booking struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	PropertyID       uuid.UUID  `json:"property_id" db:"property_id"`
	RoomID           *uuid.UUID `json:"room_id" db:"room_id"`
	GuestID          uuid.UUID  `json:"guest_id" db:"guest_id"`
	CreatedBy        uuid.UUID  `json:"created_by" db:"created_by"`
	CheckIn          time.Time  `json:"check_in" db:"check_in"`
	CheckOut         time.Time  `json:"check_out" db:"check_out"`
	Adults           int        `json:"adults" db:"adults"`
	Children         int        `json:"children" db:"children"`
	OriginalAmount   float64    `json:"original_amount" db:"original_amount"`
	OriginalCurrency string     `json:"original_currency" db:"original_currency"`
	ExchangeRate     float64    `json:"exchange_rate" db:"exchange_rate"`
	TotalAmount      float64    `json:"total_amount" db:"total_amount"`
	PaymentStatus    string     `json:"payment_status" db:"payment_status"`
	Source           string     `json:"source" db:"source"`
	Status           string     `json:"status" db:"status"`
	Notes            *string    `json:"notes" db:"notes"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

type CreateBookingRequest struct {
	PropertyID        uuid.UUID           `json:"property_id" binding:"required"`
	RoomID            *uuid.UUID          `json:"room_id"`
	GuestID           *uuid.UUID          `json:"guest_id"`
	Guest             *CreateGuestRequest `json:"guest"`
	ConfirmGuestReuse *bool               `json:"confirm_guest_reuse"`
	CreatedBy         uuid.UUID           `json:"created_by"`
	CheckIn           time.Time           `json:"check_in" binding:"required"`
	CheckOut          time.Time           `json:"check_out" binding:"required"`
	Adults            int                 `json:"adults"`
	Children          int                 `json:"children"`
	OriginalAmount    float64             `json:"original_amount" binding:"required"`
	OriginalCurrency  string              `json:"original_currency"`
	ExchangeRate      float64             `json:"exchange_rate"`
	TotalAmount       float64             `json:"total_amount"`
	Source            string              `json:"source" binding:"required"`
	Status            string              `json:"status"`
	PaymentStatus     string              `json:"payment_status"`
	Notes             string              `json:"notes"`
	ForceOverride     bool                `json:"force_override"`
}

type BookingDetail struct {
	Booking
	GuestName        string  `json:"guest_name"`
	GuestPhone       string  `json:"guest_phone"`
	GuestEmail       *string `json:"guest_email"`
	GuestNationality *string `json:"guest_nationality"`
	GuestIdNumber    *string `json:"guest_id_number"`
	GuestNotes       *string `json:"guest_notes"`
	RoomNumber       *string `json:"room_number"`
	RoomTypeName     *string `json:"room_type_name"`
	CreatedByName    string  `json:"created_by_name"`
}

type CreateBookingResponse struct {
	Booking
	GuestReused bool `json:"guest_reused"`
}

type UpdateBookingRequest struct {
	RoomID         *uuid.UUID `json:"room_id"`
	CheckIn        time.Time  `json:"check_in"`
	CheckOut       time.Time  `json:"check_out"`
	Adults         int        `json:"adults"`
	Children       int        `json:"children"`
	OriginalAmount float64    `json:"original_amount"`
	Notes          string     `json:"notes"`
	ForceOverride  bool       `json:"force_override"`
}
