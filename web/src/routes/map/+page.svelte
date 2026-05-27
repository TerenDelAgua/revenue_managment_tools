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
			block: selectedRoom.block
		};

		// 2. Optimistic UI: Actualizar inmediatamente
		switch (action) {
			case 'checkin':
				selectedRoom.availability = 'occupied';
				selectedRoom.active_booking = selectedRoom.pending_booking;
				selectedRoom.pending_booking = null;
				break;
			case 'checkout':
				selectedRoom.availability = 'available';
				selectedRoom.active_booking = null;
				break;
			case 'block':
				selectedRoom.availability = 'blocked';
				selectedRoom.block = 'temp_block_id';
				break;
			case 'unblock':
				selectedRoom.availability = 'available';
				selectedRoom.block = null;
				break;
			case 'assign':
				selectedRoom.availability = 'pending';
				selectedRoom.pending_booking = payload?.booking_id || 'temp_booking';
				break;
		}

		drawerOpen = false; // Cerrar drawer tras acción

		try {
			// 3. Llamadas API reales usando el cliente (sin `fetch` directo)
			switch (action) {
				case 'checkin':
					if (!backup.active_booking) throw new Error('No active booking');
					await api.bookings.checkin(backup.active_booking, propertyId);
					break;
				case 'checkout':
					if (!backup.active_booking) throw new Error('No active booking');
					await api.bookings.checkout(backup.active_booking, propertyId);
					break;
				case 'block':
					await api.roomBlocks.create({ room_id: selectedRoom.id, propertyId, ...payload });
					break;
				case 'unblock':
					if (!backup.block) throw new Error('No block to remove');
					await api.roomBlocks.delete(backup.block, propertyId);
					break;
				case 'assign':
					if (!payload?.booking_id) throw new Error('No booking selected');
					await api.bookings.assign(payload.booking_id, selectedRoom.id, propertyId);
					break;
				default:
					throw new Error('Unknown action');
			}

			// 4. Sync final: Refrescar mapa con datos reales del servidor
			await loadMap();
		} catch (error) {
			console.error(`[Drawer] Action '${action}' failed:`, error);

			// Rollback silencioso
			Object.assign(selectedRoom, backup);
			drawerOpen = true; // Reabrir para reintentar

			// Feedback no bloqueante (TEREN Style)
			addToast(`Failed to ${action}. Connection lost or conflict detected.`, 'error');
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
