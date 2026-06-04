package models

import (
	"time"

	"github.com/google/uuid"
)

type Guest struct {
	ID          uuid.UUID `json:"id" db:"id"`
	PropertyID  uuid.UUID `json:"property_id" db:"property_id"`
	FullName    string    `json:"full_name" db:"full_name"`
	IdNumber    *string   `json:"id_number" db:"id_number"`
	Phone       string    `json:"phone" db:"phone"`
	Email       *string   `json:"email" db:"email"`
	Nationality *string   `json:"nationality" db:"nationality"`
	Notes       *string   `json:"notes" db:"notes"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateGuestRequest struct {
	PropertyID  uuid.UUID `json:"property_id" binding:"required"`
	FullName    string    `json:"full_name" binding:"required"`
	IdNumber    *string   `json:"id_number"`
	Phone       string    `json:"phone" binding:"required"`
	Email       *string   `json:"email"`
	Nationality *string   `json:"nationality"`
	Notes       *string   `json:"notes"`
}

type UpdateGuestRequest struct {
	FullName    *string `json:"full_name"`
	IdNumber    *string `json:"id_number"`
	Phone       *string `json:"phone"`
	Email       *string `json:"email"`
	Nationality *string `json:"nationality"`
	Notes       *string `json:"notes"`
}

type GuestBookingDTO struct {
	ID          uuid.UUID `json:"id"`
	CheckIn     string    `json:"check_in"`
	CheckOut    string    `json:"check_out"`
	RoomNumber  *string   `json:"room_number"`
	Status      string    `json:"status"`
	TotalAmount float64   `json:"total_amount"`
}

type GuestDetail struct {
	Guest
	TotalBookings int               `json:"total_bookings"`
	TotalRevenue  float64           `json:"total_revenue"`
	LastVisit     *string           `json:"last_visit"`
	Bookings      []GuestBookingDTO `json:"bookings"`
}

type GuestListDTO struct {
	ID           uuid.UUID `json:"id"`
	FullName     string    `json:"full_name"`
	Phone        string    `json:"phone"`
	Email        *string   `json:"email"`
	Nationality  *string   `json:"nationality"`
	BookingCount int       `json:"booking_count"`
	LastVisit    *string   `json:"last_visit"`
}
