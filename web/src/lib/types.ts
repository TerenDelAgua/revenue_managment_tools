// src/lib/types.ts
// Alineado con models/map.go y Spec FMB-001

export type RoomAvailability =
    | 'available'
    | 'occupied'
    | 'pending'
    | 'blocked'
    | 'cleaning'
    | 'inactive';
export type AppMode = 'setup' | 'ops';
export type BlockReason = 'maintenance' | 'owner_use' | 'out_of_service';

export interface RoomType {
    id: string;
    name: string;
    max_occupancy: number;
}

export interface RoomMap {
    active_guest_name?: any;
    pending_guest_name?: any;
    active_guest_phone?: any;
    pending_guest_phone?: any;
    active_guest_nationality?: any;
    pending_guest_nationality?: any;
    active_check_in?: any;
    pending_check_in?: any;
    active_check_out?: any;
    pending_check_out?: any;
    id: string;
    number: string;
    pos_x: number;
    pos_y: number;
    room_type: { id: string; name: string };
    availability: RoomAvailability;
    has_bookings: boolean;
    active_booking: string | null;
    pending_booking: string | null;
    block: string | null;
    block_reason?: string | null;
    block_notes?: string | null;
    block_start_date?: string | null;
    block_end_date?: string | null;
}

export interface FloorMap {
    id: string;
    label: string;
    floor_number: number;
    sort_order: number;
    rooms: RoomMap[];
}

export interface MapResponse {
    property_id: string;
    date_from: string; // YYYY-MM-DD
    date_to: string;   // YYYY-MM-DD
    floors: FloorMap[];
}

// === Actions & Payloads ===
export interface RoomPositionUpdate {
    id: string;
    pos_x: number;
    pos_y: number;
}

export interface CreateRoomBlockPayload {
    room_id: string;
    start_date: string; // YYYY-MM-DD
    end_date: string;   // YYYY-MM-DD
    reason: BlockReason;
    notes?: string;
}

export interface ApiErrorResponse {
    code: string;
    message: string;
}

// === Reports & Revenue Structures ===
export interface ReportResponse {
    property_id: string;
    date_from: string;
    date_to: string;
    total_rooms: number;
    days_in_range: number;
    booked_nights: number;
    total_revenue: number;
    occupancy_rate: number; // 0.00 - 100.00
    adr: number;            // Average Daily Rate
    revpar: number;         // Revenue Per Available Room
}

export interface DailyBreakdown {
    date: string; // YYYY-MM-DD
    occupied_rooms: number;
    available_rooms: number;
    total_rooms: number;
    occupancy_rate: number;
    daily_revenue: number;
    adr: number;
    revpar: number;
}

export interface DailyBreakdownResponse {
    property_id: string;
    date_from: string;
    date_to: string;
    days: DailyBreakdown[];
    summary: ReportResponse;
}

// === Misc / UI ===
export interface BreadcrumbItem {
    label: string;
    href?: string;
    current?: boolean;
}

// === Guests & CRM ===
export interface Guest {
    id: string;
    property_id: string;
    full_name: string;
    id_number: string | null;
    phone: string;
    email: string | null;
    nationality: string | null;
    notes: string | null;
    created_at: string;
    updated_at: string;
}

export interface CreateGuestPayload {
    property_id: string;
    full_name: string;
    id_number: string | null;
    phone: string;
    email: string | null;
    nationality: string | null;
    notes: string | null;
}

export interface GuestBookingDTO {
    id: string;
    check_in: string;
    check_out: string;
    room_number: string | null;
    status: string;
    total_amount: number;
}

export interface GuestDetail extends Guest {
    total_bookings: number;
    total_revenue: number;
    last_visit: string | null;
    bookings: GuestBookingDTO[];
}

export interface GuestListDTO {
    id: string;
    full_name: string;
    phone: string;
    email: string | null;
    nationality: string | null;
    booking_count: number;
    last_visit: string | null;
}

// === Bookings ===
export interface Booking {
    id: string;
    property_id: string;
    room_id: string | null;
    guest_id: string;
    created_by: string;
    check_in: string;
    check_out: string;
    adults: number;
    children: number;
    original_amount: number;
    original_currency: string;
    exchange_rate: number;
    total_amount: number;
    payment_status: 'pending' | 'paid' | 'partial';
    source: 'walk_in' | 'whatsapp' | 'phone' | 'booking_com' | 'airbnb' | 'agoda' | 'traveloka' | 'other';
    status: 'confirmed' | 'checked_in' | 'checked_out' | 'cancelled' | 'no_show';
    notes: string | null;
    created_at: string;
    updated_at: string;
}

export interface CreateBookingPayload {
    property_id: string;
    room_id: string | null;
    guest_id?: string;
    guest?: CreateGuestPayload;
    confirm_guest_reuse?: boolean;
    check_in: string;
    check_out: string;
    adults: number;
    children: number;
    original_amount: number;
    original_currency?: string;
    exchange_rate?: number;
    total_amount?: number;
    source: string;
    notes?: string;
    force_override?: boolean;
}

export interface BookingDetail extends Booking {
    guest_name: string;
    guest_phone: string;
    guest_email: string | null;
    guest_nationality: string | null;
    guest_id_number: string | null;
    guest_notes: string | null;
    room_number: string | null;
    room_type_name: string | null;
    created_by_name: string;
}

export interface CreateBookingResponse {
    booking: Booking;
    guest_reused: boolean;
}

// === Invoicing & Payments ===
// Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4

export type InvoiceStatus = 'active' | 'void' | 'refunded';
export type PaymentStatus = 'unpaid' | 'partial' | 'paid' | 'overpaid' | 'void' | 'refunded';
export type PaymentMethod = 'cash' | 'bank_transfer' | 'qris' | 'card';

export interface InvoiceLineItem {
    id: string;
    invoice_id: string;
    description: string;
    quantity: number;
    unit_price: number;
    total: number;
    sort_order: number;
    created_at: string;
}

export interface Payment {
    id: string;
    invoice_id: string;
    property_id: string;
    method: PaymentMethod;
    amount: number; // > 0 = cobro, < 0 = refund
    original_currency: string;
    exchange_rate: number;
    reference: string | null;
    notes: string | null;
    is_reversal: boolean;
    reversal_of: string | null;
    received_by: string;
    received_at: string;
    created_at: string;
    /**
     * v1.2 (R-07): amount still available to refund on this payment row.
     * Present only for positive (non-reversal) payments in the invoice
     * detail response; null for reversal rows. Computed server-side as
     * `target.amount - SUM(refund rows WHERE reversal_of = target AND
     * invalidated_at IS NULL)`.
     */
    remaining_reverseable?: number | null;
    /**
     * v1.2 (R-09 Q2): when set, the row is excluded from total_paid /
     * total_refunded / effective_status. Used to retire legacy bad data
     * without losing audit trail.
     */
    invalidated_at?: string | null;
    invalidated_by?: string | null;
    invalidated_reason?: string | null;
}

export interface Invoice {
    id: string;
    property_id: string;
    booking_id: string;
    invoice_number: string;
    subtotal: number;
    tax_amount: number;
    ppn_rate_snapshot: number; // e.g. 0.11 for 11% PPN
    total: number;
    original_currency: string;
    exchange_rate: number;
    status: InvoiceStatus;
    issued_at: string;
    paid_at: string | null;
    voided_at: string | null;
    voided_by: string | null;
    void_reason: string | null;
    created_by: string;
    pdf_url: string | null;
    notes: string | null;
    created_at: string;
    updated_at: string;
}

export interface InvoiceDetail extends Invoice {
    line_items: InvoiceLineItem[];
    payments: Payment[];
    total_paid: number;
    total_refunded: number;
    balance: number;
    /**
     * v1.2 (R-08, R-09 Q2): server-side flag set when valid refunds
     * exceed the original charge (legacy drift). Surfaced as a ⚠
     * glyph on the status pill so the owner can review.
     */
    needs_review?: boolean;
    effective_status: PaymentStatus;
}

export interface InvoiceSummary {
    id: string;
    invoice_number: string;
    booking_id: string;
    subtotal: number;
    tax_amount: number;
    total: number;
    total_paid: number;
    /**
     * v1.2 (R-08): positive sum of non-invalidated refund rows on this
     * invoice. Frontend uses it for KPI cards + the "Refunded" column.
     */
    total_refunded?: number;
    balance: number;
    status: InvoiceStatus;
    effective_status: PaymentStatus;
    /**
     * v1.2 (R-08, R-09 Q2): server-side flag for invoices where the
     * sum of valid refunds exceeds the original charge. Frontend shows
     * a ⚠ glyph next to the status pill.
     */
    needs_review?: boolean;
    issued_at: string;
    paid_at: string | null;
    voided_at: string | null;
    guest_name: string | null;
    room_number: string | null;
}

export interface InvoiceListResponse {
    invoices: InvoiceSummary[];
    pagination: {
        page: number;
        limit: number;
        total: number;
    };
}

export interface ListInvoicesFilter {
    property_id: string;
    status?: PaymentStatus;
    date_from?: string;
    date_to?: string;
    search?: string;
    page?: number;
    limit?: number;
}

export interface RegisterPaymentPayload {
    method: PaymentMethod;
    amount: number;
    reference?: string;
    notes?: string;
    is_reversal?: boolean;
    reversal_of?: string;
    /**
     * R-07 / R-02: when refunding, the user may change the refund method
     * away from the original payment method. This requires a destructive
     * confirmation (ConfirmDestructive) and the backend prepends an
     * "[OVERRIDE] method changed from {X} to {Y} by {user}" note to the
     * audit trail. Forced to owner-only on the server side.
     */
    force_override?: boolean;
}

export interface DailySummary {
    date: string;
    property_id: string;
    invoices_issued: number;
    invoices_paid: number;
    invoices_partial: number;
    invoices_unpaid: number;
    invoices_void: number;
    invoices_overpaid: number;
    total_revenue: number;
    total_collected: number;
    total_refunded: number;
    total_pending: number;
    by_method: Partial<Record<PaymentMethod, number>>;
    tax_collected: number;
    staff_breakdown: Array<{
        user_id: string;
        user_name: string;
        payments_count: number;
        amount_collected: number;
    }>;
}

export interface MonthlyTaxReport {
    property_id: string;
    year: number;
    month?: number;
    total_subtotal: number;
    total_tax: number;
    invoices_count: number;
    void_count: number;
    refunds_total: number;
    net_tax_collected: number;
}