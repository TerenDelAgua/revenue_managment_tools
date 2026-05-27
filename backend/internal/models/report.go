package models

import (
	"time"

	"github.com/google/uuid"
)

type ReportRequest struct {
	PropertyID uuid.UUID `json:"property_id"`
	DateFrom   time.Time `json:"date_from"`
	DateTo     time.Time `json:"date_to"`
}

type ReportResponse struct {
	PropertyID    uuid.UUID `json:"property_id"`
	DateFrom      string    `json:"date_from"` // YYYY-MM-DD
	DateTo        string    `json:"date_to"`   // YYYY-MM-DD
	TotalRooms    int       `json:"total_rooms"`
	DaysInRange   int       `json:"days_in_range"`
	BookedNights  int       `json:"booked_nights"`
	TotalRevenue  float64   `json:"total_revenue"`
	OccupancyRate float64   `json:"occupancy_rate"` // 0.00 - 100.00
	ADR           float64   `json:"adr"`            // Average Daily Rate
	RevPAR        float64   `json:"revpar"`         // Revenue Per Available Room
}

type DailyBreakdown struct {
	Date           string  `json:"date"` // YYYY-MM-DD
	OccupiedRooms  int     `json:"occupied_rooms"`
	AvailableRooms int     `json:"available_rooms"`
	TotalRooms     int     `json:"total_rooms"`
	OccupancyRate  float64 `json:"occupancy_rate"` // 0-100
	DailyRevenue   float64 `json:"daily_revenue"`
	ADR            float64 `json:"adr"`
	RevPAR         float64 `json:"revpar"`
}

type DailyBreakdownResponse struct {
	PropertyID string           `json:"property_id"`
	DateFrom   string           `json:"date_from"`
	DateTo     string           `json:"date_to"`
	Days       []DailyBreakdown `json:"days"`
	Summary    ReportResponse   `json:"summary"`
}
