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