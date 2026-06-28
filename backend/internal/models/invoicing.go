package models

import (
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Invoicing & Payments models — TEREN Hotels
// Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md
// =============================================================================

// InvoiceStatus is the LIFECYCLE state of the invoice
// (active/void/refunded). v1.2 (R-08) adds 'refunded' as a terminal
// state reached when total_refunded >= total.
//
// The derived payment status (unpaid/partial/paid/overpaid) is computed
// in SQL CASE statements against the payments table — see EffectiveStatus
// on InvoiceDetail. This split follows the spec: lifecycle is mutable
// (admin can void, trigger can flip to refunded), derived status is
// recomputed on every read.
type InvoiceStatus string

const (
	InvoiceStatusActive   InvoiceStatus = "active"
	InvoiceStatusVoid     InvoiceStatus = "void"
	InvoiceStatusRefunded InvoiceStatus = "refunded" // v1.2 R-08
)

// PaymentStatus is the DERIVED state of the invoice vs its payments.
// Computed in SQL at query time. v1.2 added PaymentStatusRefunded so
// the UI doesn't need a separate field for the refunded lifecycle.
type PaymentStatus string

const (
	PaymentStatusUnpaid   PaymentStatus = "unpaid"
	PaymentStatusPartial  PaymentStatus = "partial"
	PaymentStatusPaid     PaymentStatus = "paid"
	PaymentStatusOverpaid PaymentStatus = "overpaid"
	// PaymentStatusVoid mirrors the invoice lifecycle when the invoice is
	// voided (no payments can occur). Surfaced via the same field so the
	// UI doesn't need a separate status for the void case.
	PaymentStatusVoid PaymentStatus = "void"
	// PaymentStatusRefunded (v1.2 R-08): mirrors invoices.status when
	// the invoice has been fully refunded. Terminal state.
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// PaymentMethod follows the spec §2.1 enumeration.
type PaymentMethod string

const (
	PaymentMethodCash         PaymentMethod = "cash"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
	PaymentMethodQris         PaymentMethod = "qris"
	PaymentMethodCard         PaymentMethod = "card"
)

// =============================================================================
// Core entities
// =============================================================================

// Invoice is the header of a factura. Line items and payments are loaded
// separately by the repository.
type Invoice struct {
	ID               uuid.UUID     `json:"id" db:"id"`
	PropertyID       uuid.UUID     `json:"property_id" db:"property_id"`
	BookingID        uuid.UUID     `json:"booking_id" db:"booking_id"`
	InvoiceNumber    string        `json:"invoice_number" db:"invoice_number"`
	Subtotal         float64       `json:"subtotal" db:"subtotal"`
	TaxAmount        float64       `json:"tax_amount" db:"tax_amount"`
	PPNRateSnapshot  float64       `json:"ppn_rate_snapshot" db:"ppn_rate_snapshot"`
	Total            float64       `json:"total" db:"total"`
	OriginalCurrency string        `json:"original_currency" db:"original_currency"`
	ExchangeRate     float64       `json:"exchange_rate" db:"exchange_rate"`
	Status           InvoiceStatus `json:"status" db:"status"`
	// NeedsReview (v1.2 R-08): TRUE marks invoices with data integrity
	// issues (typically legacy over-refund). Excluded from revenue/tax
	// reports until owner resolves manually.
	NeedsReview      bool          `json:"needs_review" db:"needs_review"`
	IssuedAt         time.Time     `json:"issued_at" db:"issued_at"`
	PaidAt           *time.Time    `json:"paid_at,omitempty" db:"paid_at"`
	VoidedAt         *time.Time    `json:"voided_at,omitempty" db:"voided_at"`
	VoidedBy         *uuid.UUID    `json:"voided_by,omitempty" db:"voided_by"`
	VoidReason       *string       `json:"void_reason,omitempty" db:"void_reason"`
	CreatedBy        uuid.UUID     `json:"created_by" db:"created_by"`
	PDFURL           *string       `json:"pdf_url,omitempty" db:"pdf_url"`
	Notes            *string       `json:"notes,omitempty" db:"notes"`
	CreatedAt        time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at" db:"updated_at"`
}

// InvoiceLineItem is a line on an invoice. One invoice can have many items
// (e.g. room + breakfast + tour).
type InvoiceLineItem struct {
	ID          uuid.UUID `json:"id" db:"id"`
	InvoiceID   uuid.UUID `json:"invoice_id" db:"invoice_id"`
	Description string    `json:"description" db:"description"`
	Quantity    float64   `json:"quantity" db:"quantity"`
	UnitPrice   float64   `json:"unit_price" db:"unit_price"`
	Total       float64   `json:"total" db:"total"`
	SortOrder   int       `json:"sort_order" db:"sort_order"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Payment is a single cobro or refund against an invoice. amount > 0 = cobro,
// amount < 0 = refund (BR-INV-010). is_reversal=true and reversal_of point
// to the original payment being reversed.
//
// v1.2 R-07: invalidation fields let us retire bad rows (e.g. legacy
// -666000 with mixed references) without losing audit trail. Invalidated
// rows are excluded from total_paid / total_refunded / effective_status
// computations but remain visible in the timeline with a strikethrough.
type Payment struct {
	ID               uuid.UUID     `json:"id" db:"id"`
	InvoiceID        uuid.UUID     `json:"invoice_id" db:"invoice_id"`
	PropertyID       uuid.UUID     `json:"property_id" db:"property_id"`
	Method           PaymentMethod `json:"method" db:"method"`
	Amount           float64       `json:"amount" db:"amount"`
	OriginalCurrency string        `json:"original_currency" db:"original_currency"`
	ExchangeRate     float64       `json:"exchange_rate" db:"exchange_rate"`
	Reference        *string       `json:"reference,omitempty" db:"reference"`
	Notes            *string       `json:"notes,omitempty" db:"notes"`
	IsReversal       bool          `json:"is_reversal" db:"is_reversal"`
	ReversalOf       *uuid.UUID    `json:"reversal_of,omitempty" db:"reversal_of"`
	ReceivedBy       uuid.UUID     `json:"received_by" db:"received_by"`
	ReceivedAt       time.Time     `json:"received_at" db:"received_at"`
	CreatedAt        time.Time     `json:"created_at" db:"created_at"`
	// InvalidatedAt (R-07): when non-null, this row is excluded from
	// total_paid / total_refunded / effective_status. UI renders with
	// strikethrough + tooltip "Invalidated: {reason}".
	InvalidatedAt     *time.Time `json:"invalidated_at,omitempty" db:"invalidated_at"`
	InvalidatedBy     *uuid.UUID `json:"invalidated_by,omitempty" db:"invalidated_by"`
	InvalidatedReason *string    `json:"invalidated_reason,omitempty" db:"invalidated_reason"`
}

// =============================================================================
// Composite / response shapes (spec §4.8)
// =============================================================================

// InvoiceDetail is the full payload returned by GetInvoiceByID and
// GetInvoiceByBookingID. It includes the header + line items + payments
// + computed financial fields.
type InvoiceDetail struct {
	Invoice
	LineItems       []InvoiceLineItem `json:"line_items"`
	Payments        []Payment         `json:"payments"`
	TotalPaid       float64           `json:"total_paid"`
	TotalRefunded   float64           `json:"total_refunded"`
	Balance         float64           `json:"balance"`
	EffectiveStatus PaymentStatus     `json:"effective_status"`
}

// InvoiceSummary is the trimmed shape used in the list endpoint (§4.7).
// It omits line items and payments for performance.
type InvoiceSummary struct {
	ID              uuid.UUID     `json:"id" db:"id"`
	InvoiceNumber   string        `json:"invoice_number" db:"invoice_number"`
	BookingID       uuid.UUID     `json:"booking_id" db:"booking_id"`
	Subtotal        float64       `json:"subtotal" db:"subtotal"`
	TaxAmount       float64       `json:"tax_amount" db:"tax_amount"`
	Total           float64       `json:"total" db:"total"`
	TotalPaid       float64       `json:"total_paid" db:"total_paid"`
	Balance         float64       `json:"balance" db:"balance"`
	Status          InvoiceStatus `json:"status" db:"status"`
	EffectiveStatus PaymentStatus `json:"effective_status" db:"effective_status"`
	IssuedAt        time.Time     `json:"issued_at" db:"issued_at"`
	PaidAt          *time.Time    `json:"paid_at,omitempty" db:"paid_at"`
	VoidedAt        *time.Time    `json:"voided_at,omitempty" db:"voided_at"`
	GuestName       *string       `json:"guest_name,omitempty" db:"guest_name"`
	RoomNumber      *string       `json:"room_number,omitempty" db:"room_number"`
}

// =============================================================================
// Request DTOs
// =============================================================================

// CreateInvoiceInput captures the minimum to create an invoice. The repository
// auto-computes invoice_number, ppn_rate_snapshot and tax_amount from the
// property's settings.
type CreateInvoiceInput struct {
	PropertyID uuid.UUID
	BookingID  uuid.UUID
	Subtotal   float64
	PPNRate    float64 // optional override; 0 = use property's ppn_rate
	LineItems  []NewLineItem
	CreatedBy  uuid.UUID
}

// NewLineItem is a line item supplied at invoice creation.
type NewLineItem struct {
	Description string
	Quantity    float64
	UnitPrice   float64
	SortOrder   int
}

// RegisterPaymentInput captures a single payment. amount > 0 = cobro,
// amount < 0 = refund. IsReversal + ReversalOf are required when amount < 0.
//
// v1.2 R-07: ForceOverride lets owner override method-match rule when
// the refund method differs from the original. Requires user.role='owner'.
// Frontend prepends "[OVERRIDE] method changed from {X} to {Y} by {user}"
// to the notes.
type RegisterPaymentInput struct {
	InvoiceID     uuid.UUID
	PropertyID    uuid.UUID
	Method        PaymentMethod
	Amount        float64
	Reference     string // required for non-cash (BR-INV-005)
	Notes         string
	IsReversal    bool
	ReversalOf    *uuid.UUID
	ForceOverride bool // v1.2 R-07
	ReceivedBy    uuid.UUID
}

// RefundAllInput captures the parameters of POST /invoices/:id/refund-all
// (spec §4.12). Reason is required and goes into refund_batches + each
// individual refund's notes.
type RefundAllInput struct {
	InvoiceID     uuid.UUID
	Reason        string
	ForceOverride bool
	InitiatedBy   uuid.UUID
}

// RefundBatch is the audit row written by POST /refund-all (v1.2 R-07).
// One row per batch even if N refunds were generated.
type RefundBatch struct {
	ID            uuid.UUID   `json:"id" db:"id"`
	InvoiceID     uuid.UUID   `json:"invoice_id" db:"invoice_id"`
	PropertyID    uuid.UUID   `json:"property_id" db:"property_id"`
	InitiatedBy   uuid.UUID   `json:"initiated_by" db:"initiated_by"`
	Reason        string      `json:"reason" db:"reason"`
	PaymentIDs    []uuid.UUID `json:"payment_ids" db:"payment_ids"`
	TotalRefunded float64     `json:"total_refunded" db:"total_refunded"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
}

// VoidInvoiceInput is the audit metadata required by trg_invoice_void_audit.
type VoidInvoiceInput struct {
	VoidedBy   uuid.UUID
	VoidReason string
}

// ListInvoicesFilter is the input to the list endpoint (§4.7).
// Empty fields are treated as "no filter".
type ListInvoicesFilter struct {
	PropertyID uuid.UUID
	Status     PaymentStatus // effective status filter (unpaid/partial/paid/overpaid/void)
	DateFrom   *time.Time
	DateTo     *time.Time
	Search     string // matches invoice_number exact, room number, or guest name
	Page       int    // 1-based
	Limit      int    // default 50, max 200
}

// =============================================================================
// Aggregations (spec §4.9, §4.11)
// =============================================================================

// DailySummary is the end-of-day cash-closing payload (spec §4.9).
// v1.2 R-08 adds InvoicesRefunded and NeedsReviewCount; NetRevenue is
// computed as collected - refunded.
type DailySummary struct {
	Date             time.Time                 `json:"date"`
	PropertyID       uuid.UUID                 `json:"property_id"`
	InvoicesIssued   int                       `json:"invoices_issued"`
	InvoicesPaid     int                       `json:"invoices_paid"`
	InvoicesPartial  int                       `json:"invoices_partial"`
	InvoicesUnpaid   int                       `json:"invoices_unpaid"`
	InvoicesVoid     int                       `json:"invoices_void"`
	InvoicesOverpaid int                       `json:"invoices_overpaid"`
	InvoicesRefunded int                       `json:"invoices_refunded"`   // v1.2 R-08
	NeedsReviewCount int                       `json:"needs_review_count"` // v1.2 R-08
	TotalRevenue     float64                   `json:"total_revenue"`
	TotalCollected   float64                   `json:"total_collected"`
	TotalRefunded    float64                   `json:"total_refunded"`
	NetRevenue       float64                   `json:"net_revenue"` // v1.2 R-08 = collected - refunded
	TotalPending     float64                   `json:"total_pending"`
	ByMethod         map[PaymentMethod]float64 `json:"by_method"`
	TaxCollected     float64                   `json:"tax_collected"`
	StaffBreakdown   []StaffPaymentSummary     `json:"staff_breakdown"`
}

// StaffPaymentSummary aggregates payments received by a single user.
type StaffPaymentSummary struct {
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	UserName        string    `json:"user_name" db:"user_name"`
	PaymentsCount   int       `json:"payments_count" db:"payments_count"`
	AmountCollected float64   `json:"amount_collected" db:"amount_collected"`
}

// MonthlyTaxReport is the PPN report for the fiscal period (spec §4.11).
// v1.2 R-08 / R-09 Q3: refunds nets PPN del MES DEL REFUND (cash basis).
// net_tax = tax_amount * (1 - refunded/total).
// Excludes invoices with needs_review=true.
type MonthlyTaxReport struct {
	PropertyID       uuid.UUID `json:"property_id"`
	Year             int       `json:"year"`
	Month            int       `json:"month,omitempty"`
	TotalSubtotal    float64   `json:"total_subtotal"`
	NetSubtotal      float64   `json:"net_subtotal"` // v1.2 R-08
	TotalTax         float64   `json:"total_tax"`
	NetTaxCollected  float64   `json:"net_tax_collected"`
	TotalRefunded    float64   `json:"refunds_total"`
	RefundsCount     int       `json:"refunds_count"` // v1.2 R-08
	InvoicesCount    int       `json:"invoices_count"`
	VoidCount        int       `json:"void_count"`
	RefundedCount    int       `json:"refunded_count"`     // v1.2 R-08
	NeedsReviewCount int       `json:"needs_review_count"` // v1.2 R-08
	Excluded         int       `json:"excluded_needs_review"`
}

// =============================================================================
// Idempotency (R-06, spec §4.3)
// =============================================================================

// IdempotentResponse is the cache entry stored in idempotency_keys.
type IdempotentResponse struct {
	Key          uuid.UUID
	UserID       uuid.UUID
	Endpoint     string
	ResponseBody []byte // raw JSON of the original 2xx response
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
