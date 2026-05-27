import type { MapResponse, CreateRoomBlockPayload } from '$lib/types';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

async function request<T>(endpoint: string, options?: RequestInit): Promise<T> {
	const response = await fetch(`${API_BASE_URL}${endpoint}`, {
		headers: {
			'Content-Type': 'application/json',
			...options?.headers
		},
		...options
	});

	if (!response.ok) {
		throw new Error(`API Error: ${response.status}`);
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
		updatePosition: (id: string, data: UpdateRoomPositionRequest) =>
			request<Room>(`/rooms/${id}/position`, {
				method: 'PUT',
				body: JSON.stringify(data)
			})
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
		pending: (propertyId: string) =>
			request<any[]>(`/bookings/pending?property_id=${propertyId}`),
		performAction: (action: 'checkin' | 'checkout' | 'unblock', roomId: string) =>
			request<any>(`/bookings/${action}`, {
				method: 'POST',
				body: JSON.stringify({ room_id: roomId })
			})
	},
	roomBlocks: {
		create: (payload: CreateRoomBlockPayload) =>
			request<any>('/room-blocks', {
				method: 'POST',
				body: JSON.stringify(payload)
			})
	}
};
