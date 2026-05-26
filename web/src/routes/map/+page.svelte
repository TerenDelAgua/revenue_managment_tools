<script lang="ts">
	import { browser } from '$app/environment';
	import RoomGrid from '$lib/components/map/RoomGrid.svelte';
	import FloorTabs from '$lib/components/map/FloorTabs.svelte';
	import RoomPalette from '$lib/components/map/RoomPalette.svelte';
	import RoomDrawer from '$lib/components/map/RoomDrawer.svelte';
	import type { MapResponse, RoomMap } from '$lib/types';

	const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

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
			const res = await fetch(`${API_BASE_URL}/map?date_from=${dateFrom}&date_to=${dateTo}`, {
				headers: { 'X-Property-ID': '89ce1655-d0c6-417a-8c69-3ad59241e0d0' } // UUID de prueba actual
			});
			if (!res.ok) throw new Error('Failed to load map');
			mapData = await res.json();

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
			const res = await fetch(`${API_BASE_URL}/rooms/${roomId}/position`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ pos_x: x, pos_y: y })
			});
			if (!res.ok) throw new Error('Failed to update position');
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
		
		try {
			if (action === 'block') {
				// Crear bloqueo
				const res = await fetch(`${API_BASE_URL}/room-blocks`, {
					method: 'POST',
					headers: { 
						'Content-Type': 'application/json',
						'X-Property-ID': '89ce1655-d0c6-417a-8c69-3ad59241e0d0'
					},
					body: JSON.stringify({
						room_id: selectedRoom.id,
						created_by: '80e6c703-9bb6-46c5-a6a3-6eb8b14a23b9', // UUID usuario simulado
						start_date: dateFrom,
						end_date: dateTo, // En este caso bloqueamos por el periodo seleccionado
						reason: payload.reason,
						notes: payload.note || ''
					})
				});
				if (!res.ok) {
					const errData = await res.json();
					throw new Error(errData.message || 'Error al bloquear habitación');
				}
			} else if (action === 'unblock') {
				if (!selectedRoom.block) return;
				const res = await fetch(`${API_BASE_URL}/room-blocks/${selectedRoom.block}`, {
					method: 'DELETE'
				});
				if (!res.ok) throw new Error('Error al eliminar bloqueo');
			}
			// Otras acciones como assign, checkin, checkout son simuladas o se añadirán en Fase 2
			
			drawerOpen = false;
			await loadMap(); // Refrescar estado
		} catch (err: any) {
			alert(err.message || 'Error al realizar acción');
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
			<input
				type="date"
				bind:value={dateFrom}
				onchange={loadMap}
				class="rounded-lg border px-3 py-2"
			/>
			<input
				type="date"
				bind:value={dateTo}
				onchange={loadMap}
				class="rounded-lg border px-3 py-2"
			/>
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
	isOpen={drawerOpen}
	onClose={() => (drawerOpen = false)}
	onAction={handleDrawerAction}
/>
