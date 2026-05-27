<script lang="ts">
	import { browser } from '$app/environment';
	import RoomGrid from '$lib/components/map/RoomGrid.svelte';
	import FloorTabs from '$lib/components/map/FloorTabs.svelte';
	import RoomPalette from '$lib/components/map/RoomPalette.svelte';
	import RoomDrawer from '$lib/components/map/RoomDrawer.svelte';
	import type { MapResponse, RoomMap } from '$lib/types';
	import DateInput from '$lib/components/ui/DateInput.svelte';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // UUID de prueba actual

	async function parseError(res: Response): Promise<Error> {
		try {
			const data = await res.json();
			return new Error(data.message || `API Error: ${res.status}`);
		} catch {
			return new Error(`API Error: ${res.status}`);
		}
	}

	let mode = $state<'setup' | 'ops'>('ops');
	let dateFrom = $state(new Date().toISOString().split('T')[0]);
	let dateTo = $state(new Date(Date.now() + 86400000).toISOString().split('T')[0]);
	let mapData = $state<MapResponse | null>(null);
	let loading = $state(false);

	let activeFloorId = $state('');
	let drawerOpen = $state(false);
	let selectedRoom = $state<RoomMap | null>(null);

	$effect(() => {
		if (browser) {
			const params = new URLSearchParams(window.location.search);
			const m = params.get('mode');
			if (m === 'setup' || m === 'ops') {
				mode = m;
			}
		}
	});

	// Fetch map data

	async function loadMap() {
		if (!browser) return;
		loading = true;
		try {
			mapData = await api.map.get(dateFrom, dateTo, propertyId);

			// Auto-select first floor if none is active
			if (mapData && mapData.floors.length > 0 && !activeFloorId) {
				activeFloorId = mapData.floors[0].id;
			}
		} catch (err) {
			console.error(err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		loadMap();
	});

	async function handleDrop(roomId: string, x: number, y: number) {
		if (!mapData) return;
		// Optimistic UI: actualizar localmente
		let foundRoom: RoomMap | undefined;
		let oldPos = { x: 0, y: 0 };

		for (const floor of mapData.floors) {
			const r = floor.rooms.find((r) => r.id === roomId);
			if (r) {
				foundRoom = r;
				oldPos = { x: r.pos_x, y: r.pos_y };
				r.pos_x = x;
				r.pos_y = y;
				break;
			}
		}

		if (!foundRoom) return;

		try {
			await api.rooms.updatePosition(roomId, { pos_x: x, pos_y: y });
		} catch {
			// Rollback si falla
			foundRoom.pos_x = oldPos.x;
			foundRoom.pos_y = oldPos.y;
			addToast('Layout no guardado. Verifica conexión.', 'error');
		}
	}

	function openDrawer(room: RoomMap) {
		selectedRoom = room;
		drawerOpen = true;
	}

	async function handleDrawerAction(action: string, payload?: any) {
		if (!selectedRoom) return;

		// 1. Guardar estado original para rollback
		const backup = {
			availability: selectedRoom.availability,
			active_booking: selectedRoom.active_booking,
			pending_booking: selectedRoom.pending_booking,
			block: selectedRoom.block,
			active_guest_name: selectedRoom.active_guest_name,
			pending_guest_name: selectedRoom.pending_guest_name
		};

		try {
			switch (action) {
				case 'checkin': {
					// Validación: necesitamos un booking pending o active
					const bookingId = selectedRoom.pending_booking || selectedRoom.active_booking;
					if (!bookingId) {
						addToast('No booking found for this room.', 'error');
						return;
					}

					// Optimistic UI
					selectedRoom.availability = 'occupied';
					selectedRoom.active_booking = bookingId;
					selectedRoom.pending_booking = null;
					drawerOpen = false;

					await api.bookings.checkin(bookingId, propertyId);
					addToast(`Check-in completed · ${backup.pending_guest_name || 'Guest'}`, 'success');
					break;
				}

				case 'checkout': {
					if (!selectedRoom.active_booking) {
						addToast('No active booking to check out.', 'error');
						return;
					}

					// Optimistic UI
					selectedRoom.availability = 'available';
					const guestName = selectedRoom.active_guest_name;
					const activeBookingId = selectedRoom.active_booking;
					selectedRoom.active_booking = null;
					selectedRoom.active_guest_name = null;
					drawerOpen = false;

					await api.bookings.checkout(activeBookingId || backup.active_booking, propertyId);
					addToast(
						`${guestName || 'Guest'} checked out · Room ${selectedRoom.number} ready`,
						'success'
					);
					break;
				}

				case 'block': {
					// Optimistic
					selectedRoom.availability = 'blocked';
					drawerOpen = false;

					// Formatear fechas a RFC3339 para deserialización en Go (time.Time)
					const formattedPayload = {
						...payload,
						start_date: payload.start_date ? `${payload.start_date}T00:00:00Z` : undefined,
						end_date: payload.end_date ? `${payload.end_date}T00:00:00Z` : undefined
					};

					await api.roomBlocks.create({ room_id: selectedRoom.id, propertyId, ...formattedPayload });
					addToast('Room blocked successfully', 'success');
					break;
				}

				case 'unblock': {
					if (!selectedRoom.block) throw new Error('No block to remove');
					const blockId = selectedRoom.block;
					selectedRoom.availability = 'available';
					selectedRoom.block = null;
					drawerOpen = false;

					await api.roomBlocks.delete(blockId, propertyId);
					addToast('Block removed', 'success');
					break;
				}

				case 'assign': {
					if (!payload?.booking_id) throw new Error('No booking selected');
					// Optimistic UI
					selectedRoom.availability = 'pending';
					selectedRoom.pending_booking = payload.booking_id;
					drawerOpen = false;

					await api.bookings.assign(payload.booking_id, selectedRoom.id, propertyId);
					addToast(`Room ${selectedRoom.number} assigned`, 'success');
					break;
				}

				default:
					throw new Error(`Unknown action: ${action}`);
			}

			// Sync final con datos reales
			await loadMap();
		} catch (error: any) {
			console.error(`[Drawer] Action '${action}' failed:`, error);

			// Rollback
			Object.assign(selectedRoom, backup);
			drawerOpen = true;

			addToast(error?.message || `Failed to ${action}. Connection lost or conflict detected.`, 'error');
		}
	}
</script>

<!-- Layout -->
<div class="flex min-h-screen flex-col gap-4 bg-[#F5F4F1] p-6">
	<header class="flex flex-wrap items-center justify-between gap-4">
		<h1 class="text-2xl font-semibold text-[#1C1917]">Hotel Floor Map</h1>
		<div class="flex gap-2">
			<button
				onclick={() => (mode = mode === 'setup' ? 'ops' : 'setup')}
				class="rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-4 py-2 text-[#57534E] transition hover:bg-[#FFF7ED]"
			>
				{mode === 'setup' ? 'Vista Operaciones' : 'Modo Setup'}
			</button>
			<DateInput label="From" value={dateFrom} onChange={(v) => (dateFrom = v)} />
			<DateInput label="To" value={dateTo} onChange={(v) => (dateTo = v)} />
		</div>
	</header>

	<div class="flex flex-1 gap-4">
		<aside class="hidden w-64 md:block">
			{#if mode === 'setup'}
				<RoomPalette roomTypes={[]} onDragStart={() => {}} />
			{/if}
		</aside>

		<main class="flex-1 space-y-4">
			<FloorTabs
				floors={mapData?.floors ?? []}
				activeId={activeFloorId}
				onSelect={(id) => (activeFloorId = id)}
				{mode}
			/>

			{#if loading}
				<div class="h-64 animate-pulse rounded-xl bg-[#FCFBFA]"></div>
			{:else if activeFloorId}
				<RoomGrid
					rooms={mapData?.floors.find((f) => f.id === activeFloorId)?.rooms ?? []}
					{mode}
					onSelect={openDrawer}
					onDrop={handleDrop}
				/>
			{/if}
		</main>
	</div>
</div>

<RoomDrawer
	room={selectedRoom}
	{propertyId}
	isOpen={drawerOpen}
	onClose={() => (drawerOpen = false)}
	onAction={handleDrawerAction}
/>
