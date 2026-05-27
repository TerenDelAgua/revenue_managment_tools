<script lang="ts">
	import { browser } from '$app/environment';
	import RoomGrid from '$lib/components/map/RoomGrid.svelte';
	import FloorTabs from '$lib/components/map/FloorTabs.svelte';
	import RoomPalette from '$lib/components/map/RoomPalette.svelte';
	import RoomDrawer from '$lib/components/map/RoomDrawer.svelte';
	import type { MapResponse, RoomMap } from '$lib/types';
	import DateInput from '$lib/components/ui/DateInput.svelte';
	import { api } from '$lib/api/client';

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
			alert('Layout no guardado. Verifica conexión.');
		}
	}

	function openDrawer(room: RoomMap) {
		selectedRoom = room;
		drawerOpen = true;
	}

	async function handleDrawerAction(action: string, payload?: any) {
		if (!selectedRoom) return;

		// 1. Optimistic UI: Actualizar el estado local inmediatamente para feedback visual
		const originalAvailability = selectedRoom.availability;
		let newState = selectedRoom.availability;

		if (action === 'checkin') newState = 'occupied';
		else if (action === 'checkout') newState = 'available';
		else if (action === 'block') newState = 'blocked';
		else if (action === 'unblock') newState = 'available';

		// Aplicar cambio local
		selectedRoom.availability = newState;
		drawerOpen = false; // Cerrar drawer

		try {
			// 2. Llamada al Backend usando el API client unificado
			if (action === 'block') {
				await api.roomBlocks.create(payload);
			} else if (action === 'checkin' || action === 'checkout' || action === 'unblock') {
				await api.bookings.performAction(action, selectedRoom.id);
			}

			// 3. Refrescar datos reales
			await loadMap();
		} catch (error) {
			// 4. Rollback si falla
			console.error('Action failed', error);
			alert('Action failed. Reverting...');
			selectedRoom.availability = originalAvailability;
			drawerOpen = true; // Re-abrir drawer para que intente de nuevo
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
