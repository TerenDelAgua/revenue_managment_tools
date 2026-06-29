<script lang="ts">
	import { browser } from '$app/environment';
	import RoomGrid from '$lib/components/map/RoomGrid.svelte';
	import FloorTabs from '$lib/components/map/FloorTabs.svelte';
	import RoomPalette from '$lib/components/map/RoomPalette.svelte';
	import RoomDrawer from '$lib/components/map/RoomDrawer.svelte';
	import type { BlockReason, MapResponse, RoomMap, RoomType } from '$lib/types';
	import DateInput from '$lib/components/ui/DateInput.svelte';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import OccupancyBar from '$lib/components/map/OccupancyBar.svelte';
	import { onMount } from 'svelte';
	import { SvelteDate } from 'svelte/reactivity';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // UUID de prueba actual

	let currentUser = $state({ role: 'owner' });
	let mode = $state<'setup' | 'ops'>('ops');
	let dateFrom = $state(new Date().toISOString().split('T')[0]);
	let dateTo = $state(new Date(Date.now() + 86400000).toISOString().split('T')[0]);
	let mapData = $state<MapResponse | null>(null);
	let loading = $state(false);

	let activeFloorId = $state('');
	let drawerOpen = $state(false);
	let selectedRoom = $state<RoomMap | null>(null);
	let roomTypes = $state<RoomType[]>([]);

	type DrawerAction =
		| 'assign'
		| 'checkin'
		| 'checkout'
		| 'block'
		| 'unblock'
		| 'set_cleaning'
		| 'clear_cleaning'
		| 'update_room'
		| 'delete_room';

	type AssignPayload = {
		booking_id: string;
	};

	type BlockPayload = {
		reason: BlockReason;
		notes?: string;
		start_date: string;
		end_date: string;
	};

	type UpdateRoomPayload = Parameters<typeof api.rooms.update>[1];

	function isRecord(value: unknown): value is Record<string, unknown> {
		return typeof value === 'object' && value !== null;
	}

	function isAssignPayload(value: unknown): value is AssignPayload {
		return isRecord(value) && typeof value.booking_id === 'string' && value.booking_id.length > 0;
	}

	function isBlockPayload(value: unknown): value is BlockPayload {
		return (
			isRecord(value) &&
			(value.reason === 'maintenance' ||
				value.reason === 'owner_use' ||
				value.reason === 'out_of_service') &&
			typeof value.start_date === 'string' &&
			typeof value.end_date === 'string' &&
			(value.notes === undefined || typeof value.notes === 'string')
		);
	}

	function isUpdateRoomPayload(value: unknown): value is UpdateRoomPayload {
		return isRecord(value);
	}

	function getErrorMessage(error: unknown, fallback: string): string {
		return error instanceof Error ? error.message : fallback;
	}

	async function loadRoomTypes() {
		try {
			roomTypes = await api.roomTypes.list(propertyId);
		} catch (err) {
			console.error('Failed to load room types:', err);
		}
	}

	onMount(() => {
		loadRoomTypes();
	});

	function findFirstFreePosition(rooms: RoomMap[]): { x: number; y: number } {
		const occupied = new Set(rooms.map((r) => `${r.pos_x},${r.pos_y}`));
		for (let y = 0; y < 20; y++) {
			for (let x = 0; x < 12; x++) {
				if (!occupied.has(`${x},${y}`)) {
					return { x, y };
				}
			}
		}
		return { x: 0, y: 0 };
	}

	async function handleRoomPaletteClick(type: RoomType) {
		if (!activeFloorId) {
			addToast('Por favor, selecciona un piso primero.', 'error');
			return;
		}

		const activeFloor = mapData?.floors.find((f) => f.id === activeFloorId);
		const currentRooms = activeFloor?.rooms || [];
		const { x, y } = findFirstFreePosition(currentRooms);

		try {
			await api.rooms.create({
				floor_id: activeFloorId,
				room_type_id: type.id,
				number: '', // Omit to trigger sequential auto-generation
				status: 'active',
				pos_x: x,
				pos_y: y
			});
			addToast('Habitación añadida al mapa', 'success');
			await loadMap();
		} catch (err: unknown) {
			console.error(err);
			addToast(getErrorMessage(err, 'Error al añadir habitación'), 'error');
		}
	}

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

	// Reactively adjust dates to keep dateTo strictly after dateFrom
	$effect(() => {
		if (dateFrom && dateTo) {
			const from = new Date(dateFrom);
			const to = new Date(dateTo);
			if (from >= to) {
				const nextDay = new SvelteDate(from);
				nextDay.setDate(nextDay.getDate() + 1);
				dateTo = nextDay.toISOString().split('T')[0];
			}
		}
	});

	async function loadMap() {
		if (!browser) return;
		if (new Date(dateFrom) >= new Date(dateTo)) return; // Guard during input transition

		loading = true;
		try {
			mapData = await api.map.get(dateFrom, dateTo, propertyId);

			// Auto-select first floor if none is active
			if (mapData && mapData.floors.length > 0 && !activeFloorId) {
				activeFloorId = mapData.floors[0].id;
			}

			// If selectedRoom is currently active, update its reference to the newly fetched room object
			if (selectedRoom && mapData) {
				const currentId = selectedRoom.id;
				let found = false;
				for (const floor of mapData.floors) {
					const r = floor.rooms.find((r) => r.id === currentId);
					if (r) {
						selectedRoom = r;
						found = true;
						break;
					}
				}
				if (!found) {
					selectedRoom = null;
				}
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

	async function handleDrawerAction(action: DrawerAction | string, payload?: unknown) {
		if (!selectedRoom) return;
		const normalizedAction = action.trim() as DrawerAction;

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
			switch (normalizedAction) {
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

					await api.bookings.checkout(activeBookingId || backup.active_booking || '', propertyId);
					addToast(
						`${guestName || 'Guest'} checked out · Room ${selectedRoom.number} ready for cleaning`,
						'success'
					);
					break;
				}

				case 'set_cleaning': {
					// BT-TEREN-16: housekeeping manual. La habitación pasa a estado
					// operacional `cleaning`. No es vendible hasta que se libere.
					selectedRoom.availability = 'cleaning';
					drawerOpen = false;
					await api.rooms.setCleaning(selectedRoom.id);
					addToast(`Room ${selectedRoom.number} marked as cleaning`, 'success');
					break;
				}

				case 'clear_cleaning': {
					// BT-TEREN-16: housekeeping terminada. Vuelve a `active` (vendible).
					selectedRoom.availability = 'available';
					drawerOpen = false;
					await api.rooms.clearCleaning(selectedRoom.id);
					addToast(`Room ${selectedRoom.number} cleaning done · ready to sell`, 'success');
					break;
				}

				case 'block': {
					if (!isBlockPayload(payload)) {
						throw new Error('Invalid block payload');
					}

					// Optimistic
					selectedRoom.availability = 'blocked';
					drawerOpen = false;

					// Formatear fechas a RFC3339 para deserialización en Go (time.Time)
					const formattedPayload = {
						...payload,
						start_date: `${payload.start_date}T00:00:00Z`,
						end_date: `${payload.end_date}T00:00:00Z`
					};

					await api.roomBlocks.create({
						room_id: selectedRoom.id,
						propertyId,
						...formattedPayload
					});
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
					if (!isAssignPayload(payload)) {
						throw new Error('No booking selected');
					}

					// Optimistic UI
					selectedRoom.availability = 'pending';
					selectedRoom.pending_booking = payload.booking_id;
					drawerOpen = false;

					await api.bookings.assign(payload.booking_id, selectedRoom.id, propertyId);
					addToast(`Room ${selectedRoom.number} assigned`, 'success');
					break;
				}

				case 'update_room': {
					if (!isUpdateRoomPayload(payload)) return;
					await api.rooms.update(selectedRoom.id, payload);
					addToast('Room updated successfully', 'success');
					break;
				}

				case 'delete_room': {
					await api.rooms.delete(selectedRoom.id);
					addToast('Room deleted successfully', 'success');
					drawerOpen = false;
					break;
				}

				default:
					throw new Error(`Unknown action: ${normalizedAction}`);
			}

			// Sync final con datos reales
			await loadMap();
		} catch (error: unknown) {
			console.error(`[Drawer] Action '${normalizedAction}' failed:`, error);

			// Rollback
			Object.assign(selectedRoom, backup);
			drawerOpen = true;

			addToast(
				getErrorMessage(error, `Failed to ${normalizedAction}. Connection lost or conflict detected.`),
				'error'
			);
		}
	}
</script>

<!-- Layout -->
<div class="flex min-h-screen flex-col gap-4 bg-[#F5F4F1] p-4 md:p-6">
	<header class="flex flex-wrap items-center justify-between gap-4">
		<h1 class="text-xl md:text-2xl font-semibold text-[#1C1917]">Hotel Floor Map</h1>
		<div class="flex flex-wrap gap-2 items-end">
			<button
				onclick={() => (mode = mode === 'setup' ? 'ops' : 'setup')}
				class="rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-1.5 md:px-4 md:py-2 text-xs md:text-sm text-[#57534E] transition hover:bg-[#FFF7ED]"
			>
				{mode === 'setup' ? 'Vista Operaciones' : 'Modo Setup'}
			</button>
			<DateInput label="From" value={dateFrom} onChange={(v) => (dateFrom = v)} />
			<DateInput label="To" value={dateTo} onChange={(v) => (dateTo = v)} />
		</div>
	</header>

	{#if currentUser.role === 'owner'}
		<OccupancyBar {propertyId} {dateFrom} {dateTo} />
	{/if}

	<div class="flex flex-1 gap-4">
		{#if mode === 'setup'}
			<aside class="hidden w-64 md:block">
				<RoomPalette roomTypes={roomTypes} onDragStart={() => {}} onClick={handleRoomPaletteClick} />
			</aside>
		{/if}

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
	{mode}
/>
