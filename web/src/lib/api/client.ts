import type {
	MapResponse,
	CreateRoomBlockPayload,
	ReportResponse,
	DailyBreakdownResponse,
	Booking,
	BookingDetail,
	CreateBookingPayload,
	GuestDetail,
	GuestListDTO,
	Guest,
	InvoiceDetail,
	InvoiceListResponse,
	ListInvoicesFilter,
	RegisterPaymentPayload,
	Payment,
	DailySummary,
	MonthlyTaxReport
} from '$lib/types';
import { env } from '$env/dynamic/public';

const API_BASE_URL = env.PUBLIC_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

async function request<T>(endpoint: string, options?: RequestInit): Promise<T> {
	const response = await fetch(`${API_BASE_URL}${endpoint}`, {
		headers: {
			'Content-Type': 'application/json',
			...options?.headers
		},
		...options
	});

	if (!response.ok) {
		let message = `API Error: ${response.status}`;
		try {
			const errBody = await response.json();
			if (errBody && errBody.message) {
				message = errBody.message;
			} else if (errBody && errBody.error) {
				if (typeof errBody.error === 'string') {
					message = errBody.error;
				} else if (errBody.error.message) {
					message = errBody.error.message;
				}
			} else if (errBody && errBody.code) {
				message = `${errBody.code}: ${errBody.message || ''}`;
			}
		} catch (_) {
			// ignore JSON parse error, use fallback status message
		}
		const error = new Error(message) as any;
		error.status = response.status;
		throw error;
	}

	if (response.status === 204) {
		return {} as T;
	}

	return response.json();
}

export interface Property {
	id: string;
	name: string;
	slug: string;
	currency: string;
	timezone: string;
	settings: Record<string, any>;
	created_at: string;
	updated_at: string;
}

export interface Floor {
	id: string;
	property_id: string;
	floor_number: number;
	label?: string;
	sort_order: number;
	created_at: string;
	updated_at: string;
}

export interface RoomType {
	id: string;
	property_id: string;
	name: string;
	max_occupancy: number;
	created_at: string;
	updated_at: string;
}

export interface Room {
	id: string;
	floor_id: string;
	room_type_id: string;
	number: string;
	status: string;
	pos_x: number;
	pos_y: number;
	created_at: string;
	updated_at: string;
	room_type?: RoomType;
}

export interface CreatePropertyRequest {
	name: string;
	slug: string;
	currency: string;
	timezone: string;
	settings?: Record<string, any>;
}

export interface CreateFloorRequest {
	property_id: string;
	floor_number: number;
	label?: string;
	sort_order?: number;
}

export interface CreateRoomRequest {
	floor_id: string;
	room_type_id: string;
	number: string;
	status: string;
	pos_x?: number;
	pos_y?: number;
}

export interface UpdateRoomPositionRequest {
	pos_x: number;
	pos_y: number;
}

export const api = {
	properties: {
		list: () => request<Property[]>('/properties'),
		get: (id: string) => request<Property>(`/properties/${id}`),
		create: (data: CreatePropertyRequest) =>
			request<Property>('/properties', {
				method: 'POST',
				body: JSON.stringify(data)
			})
	},
	floors: {
		listByProperty: (propertyId: string) => request<Floor[]>(`/properties/${propertyId}/floors`),
		get: (id: string) => request<Floor>(`/floors/${id}`),
		create: (data: CreateFloorRequest) =>
			request<Floor>('/floors', {
				method: 'POST',
				body: JSON.stringify(data)
			})
	},
	rooms: {
		listByFloor: (floorId: string) => request<Room[]>(`/floors/${floorId}/rooms`),
		get: (id: string) => request<Room>(`/rooms/${id}`),
		create: (data: CreateRoomRequest) =>
			request<Room>('/rooms', {
				method: 'POST',
				body: JSON.stringify(data)
			}),
		update: (id: string, data: Partial<CreateRoomRequest>) =>
			request<Room>(`/rooms/${id}`, {
				method: 'PATCH',
				body: JSON.stringify(data)
			}),
		delete: (id: string) =>
			request<any>(`/rooms/${id}`, {
				method: 'DELETE'
			}),
		updatePosition: (id: string, data: UpdateRoomPositionRequest) =>
			request<Room>(`/rooms/${id}/position`, {
				method: 'PUT',
				body: JSON.stringify(data)
			}),
		// Housekeeping: marca la habitación como "cleaning" (POST) o la libera (DELETE).
		// El backend maneja idempotencia y rechaza con 409 si la room es inactive.
		setCleaning: (id: string) =>
			request<Room>(`/rooms/${id}/cleaning`, { method: 'POST' }),
		clearCleaning: (id: string) =>
			request<Room>(`/rooms/${id}/cleaning`, { method: 'DELETE' })
	},
	roomTypes: {
		list: (propertyId: string) => request<RoomType[]>(`/properties/${propertyId}/room-types`)
	},
	map: {
		get: (dateFrom: string, dateTo: string, propertyId: string) =>
			request<MapResponse>(`/map?date_from=${dateFrom}&date_to=${dateTo}`, {
				headers: {
					'X-Property-ID': propertyId
				}
			})
	},
	bookings: {
		list: (propertyId: string, status?: string, search?: string, page?: number, limit?: number) => {
			const url = `/bookings?property_id=${propertyId}&status=${status || ''}&search=${encodeURIComponent(search || '')}&page=${page || 1}&limit=${limit || 50}`;
			return request<{ bookings: BookingDetail[]; pagination: { page: number; limit: number; total: number } }>(url);
		},
		get: (id: string) => request<BookingDetail>(`/bookings/${id}`),
		create: (data: CreateBookingPayload) =>
			request<any>('/bookings', {
				method: 'POST',
				body: JSON.stringify(data)
			}),
		update: (id: string, data: any) =>
			request<BookingDetail>(`/bookings/${id}`, {
				method: 'PATCH',
				body: JSON.stringify(data)
			}),
		pending: (propertyId: string) =>
			request<any[]>(`/bookings/pending?property_id=${propertyId}`),
		checkin: (bookingId: string, propertyId: string) =>
			request<any>(`/bookings/${bookingId}/checkin`, {
				method: 'POST',
				headers: { 'X-Property-ID': propertyId }
			}),
		checkout: (bookingId: string, propertyId: string) =>
			request<any>(`/bookings/${bookingId}/checkout`, {
				method: 'POST',
				headers: { 'X-Property-ID': propertyId }
			}),
		cancel: (bookingId: string, reason: string) =>
			request<any>(`/bookings/${bookingId}/cancel`, {
				method: 'POST',
				body: JSON.stringify({ reason })
			}),
		assign: (bookingId: string, roomId: string, propertyId: string) =>
			request<any>(`/bookings/${bookingId}/assign`, {
				method: 'PATCH',
				headers: { 'X-Property-ID': propertyId },
				body: JSON.stringify({ room_id: roomId })
			})
	},
	guests: {
		list: (propertyId: string, search?: string, page?: number, limit?: number) => {
			const url = `/guests?property_id=${propertyId}&search=${encodeURIComponent(search || '')}&page=${page || 1}&limit=${limit || 50}`;
			return request<{ guests: GuestListDTO[]; pagination: { page: number; limit: number; total: number } }>(url);
		},
		get: (id: string) => request<GuestDetail>(`/guests/${id}`),
		create: (data: any) =>
			request<Guest>('/guests', {
				method: 'POST',
				body: JSON.stringify(data)
			}),
		update: (id: string, data: any) =>
			request<Guest>(`/guests/${id}`, {
				method: 'PATCH',
				body: JSON.stringify(data)
			})
	},
	roomBlocks: {
		create: (payload: CreateRoomBlockPayload & { propertyId: string }) => {
			const { propertyId, ...rest } = payload;
			return request<any>('/room-blocks', {
				method: 'POST',
				headers: { 'X-Property-ID': propertyId },
				body: JSON.stringify(rest)
			});
		},
		delete: (blockId: string, propertyId: string) =>
			request<any>(`/room-blocks/${blockId}`, {
				method: 'DELETE',
				headers: { 'X-Property-ID': propertyId }
			})
	},
	reports: {
		metrics: (propertyId: string, dateFrom: string, dateTo: string) =>
			request<ReportResponse>(`/reports/metrics?property_id=${propertyId}&date_from=${dateFrom}&date_to=${dateTo}`),
		daily: (propertyId: string, dateFrom: string, dateTo: string) =>
			request<DailyBreakdownResponse>(`/reports/daily?property_id=${propertyId}&date_from=${dateFrom}&date_to=${dateTo}`)
	},
	invoices: {
		// Read — spec §4.1, §4.2
		getByID: (id: string) => request<InvoiceDetail>(`/invoices/${id}`),
		getByBooking: (bookingId: string) => request<InvoiceDetail>(`/invoices/by-booking/${bookingId}`),

		// List — spec §4.7. Filters are optional; backend applies them as AND.
		list: (filter: ListInvoicesFilter) => {
			const qs = new URLSearchParams();
			qs.set('property_id', filter.property_id);
			if (filter.status) qs.set('status', filter.status);
			if (filter.date_from) qs.set('date_from', filter.date_from);
			if (filter.date_to) qs.set('date_to', filter.date_to);
			if (filter.search) qs.set('search', filter.search);
			if (filter.page) qs.set('page', String(filter.page));
			if (filter.limit) qs.set('limit', String(filter.limit));
			return request<InvoiceListResponse>(`/invoices?${qs.toString()}`);
		},

		// Aggregations — spec §4.9, §4.11
		dailySummary: (propertyId: string, date?: string, tz?: string) => {
			const qs = new URLSearchParams({ property_id: propertyId });
			if (date) qs.set('date', date);
			if (tz) qs.set('tz', tz);
			return request<DailySummary>(`/invoices/daily-summary?${qs.toString()}`);
		},
		taxReport: (propertyId: string, year: number, month?: number) => {
			const qs = new URLSearchParams({ property_id: propertyId, year: String(year) });
			if (month) qs.set('month', String(month));
			return request<MonthlyTaxReport>(`/invoices/tax-report?${qs.toString()}`);
		},

		// Write — spec §4.3 (R-01 reference required, R-06 Idempotency-Key)
		// Pass an idempotencyKey (UUID v4) to opt-in to dedup.
		registerPayment: (
			invoiceId: string,
			payload: RegisterPaymentPayload,
			propertyId: string,
			receivedBy: string,
			idempotencyKey?: string
		) => {
			const headers: Record<string, string> = {
				'X-Property-ID': propertyId,
				'X-User-ID': receivedBy
			};
			if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
			return request<Payment>(`/invoices/${invoiceId}/payments`, {
				method: 'POST',
				headers,
				body: JSON.stringify(payload)
			});
		},

		// Write — spec §4.4
		void: (invoiceId: string, reason: string, voidedBy: string) =>
			request<InvoiceDetail>(`/invoices/${invoiceId}/void`, {
				method: 'POST',
				headers: { 'X-User-ID': voidedBy },
				body: JSON.stringify({ reason })
			}),

		// Write — spec §4.5
		updateNotes: (invoiceId: string, notes: string) =>
			request<InvoiceDetail>(`/invoices/${invoiceId}/notes`, {
				method: 'PATCH',
				body: JSON.stringify({ notes })
			}),

		// Write — spec §4.6
		regeneratePDF: (invoiceId: string) =>
			request<{ pdf_url: string }>(`/invoices/${invoiceId}/regenerate-pdf`, {
				method: 'POST'
			})
	}
};
