<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Floor, type Property, type Room } from '$lib/api/client';
	import FloorMap from '$lib/components/FloorMap.svelte';

	let properties = $state<Property[]>([]);
	let floors = $state<Floor[]>([]);
	let rooms = $state<Room[]>([]);
	let selectedProperty = $state<Property | null>(null);
	let selectedFloor = $state<Floor | null>(null);
	let selectedRoom = $state<Room | null>(null);
	let sidebarCollapsed = $state(false);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let selectedDate = $state<string>(new Date().toISOString().split('T')[0]);
	let drawerOpen = $state(false);

	async function loadProperties() {
		loading = true;
		error = null;
		try {
			properties = await api.properties.list();
			if (properties.length > 0) {
				selectedProperty = properties[0];
				await loadFloors(selectedProperty.id);
			}
		} catch (e) {
			error = 'Failed to load properties';
		} finally {
			loading = false;
		}
	}

	async function loadFloors(propertyId: string) {
		loading = true;
		error = null;
		try {
			floors = await api.floors.listByProperty(propertyId);
			if (floors.length > 0) {
				selectedFloor = floors[0];
				await loadRooms(selectedFloor.id);
			} else {
				rooms = [];
			}
		} catch (e) {
			error = 'Failed to load floors';
		} finally {
			loading = false;
		}
	}

	async function loadRooms(floorId: string) {
		loading = true;
		error = null;
		try {
			const result = await api.rooms.listByFloor(floorId);
			rooms = result || [];
		} catch (e) {
			error = 'Failed to load rooms';
			rooms = [];
		} finally {
			loading = false;
		}
	}

	async function handleRoomMove(roomId: string, x: number, y: number) {
		try {
			await api.rooms.updatePosition(roomId, { pos_x: x, pos_y: y });
			const index = rooms.findIndex(r => r.id === roomId);
			if (index !== -1) {
				rooms[index] = { ...rooms[index], pos_x: x, pos_y: y };
			}
		} catch (e) {
			console.error('Failed to update room position:', e);
		}
	}

	function selectProperty(property: Property) {
		selectedProperty = property;
		loadFloors(property.id);
	}

	function selectFloor(floor: Floor) {
		selectedFloor = floor;
		loadRooms(floor.id);
	}

	function selectRoom(room: Room) {
		selectedRoom = room;
		drawerOpen = true;
	}

	function getStatusColor(status: string) {
		switch (status?.toLowerCase()) {
			case 'available':
				return 'border-green-600 bg-green-50';
			case 'occupied':
				return 'border-red-600 bg-red-50';
			case 'pending':
			case 'pending check-in':
				return 'border-orange-500 bg-orange-50';
			case 'maintenance':
			case 'blocked':
				return 'border-gray-500 bg-gray-100';
			default:
				return 'border-teren-primary bg-teren-primary-subtle';
		}
	}

	function getStatusText(status: string) {
		switch (status?.toLowerCase()) {
			case 'available':
				return 'Available';
			case 'occupied':
				return 'Occupied';
			case 'pending':
			case 'pending check-in':
				return 'Pending Check-in';
			case 'maintenance':
				return 'Maintenance';
			case 'blocked':
				return 'Blocked';
			default:
				return status;
		}
	}

	onMount(() => {
		loadProperties();
	});
</script>

<div class="flex h-screen bg-teren-background-base">
	<!-- Sidebar -->
	<aside 
		class="bg-teren-surface-base border-r border-teren-border-subtle transition-all duration-300 flex flex-col {sidebarCollapsed ? 'w-20' : 'w-72'}">
		
		<!-- Logo y botón de colapso -->
		<div class="p-4 border-b border-teren-border-subtle flex items-center justify-between">
			{#if !sidebarCollapsed}
				<div class="font-bold text-xl text-teren-text-main">TEREN Hotels</div>
			{/if}
			<button 
				onclick={() => sidebarCollapsed = !sidebarCollapsed}
				class="p-2 rounded-lg hover:bg-teren-background-base transition-colors">
				{#if sidebarCollapsed}
					<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>
				{:else}
					<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
				{/if}
			</button>
		</div>

		<!-- Navigation -->
		<nav class="flex-1 p-4 space-y-2">
			<a href="/floor-map-builder" class="flex items-center gap-3 px-4 py-3 rounded-lg bg-teren-primary-subtle border border-teren-primary text-teren-text-main">
				<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/></svg>
				{#if !sidebarCollapsed}<span>Floor Map</span>{/if}
			</a>
		</nav>

		{#if selectedProperty && !sidebarCollapsed}
			<div class="p-4 border-t border-teren-border-subtle">
				<div class="text-xs font-semibold text-teren-text-muted mb-3">PROPERTIES</div>
				<div class="space-y-2">
					{#each properties as property (property.id)}
						<button
							onclick={() => selectProperty(property)}
							class="w-full text-left px-3 py-2 rounded-lg transition-colors text-sm {selectedProperty?.id === property.id ? 'bg-teren-primary-subtle border border-teren-primary' : 'hover:bg-teren-background-base border border-transparent'}">
							<div class="font-medium text-teren-text-main">{property.name}</div>
						</button>
					{/each}
				</div>
			</div>
		{/if}
	</aside>

	<!-- Main Content -->
	<main class="flex-1 flex flex-col overflow-hidden">
		<!-- Top Bar -->
		<header class="bg-teren-surface-base border-b border-teren-border-subtle px-6 py-4">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-4">
					<h1 class="text-2xl font-bold text-teren-text-main">
						Floor Map
						{#if selectedProperty}
							<span class="text-lg font-normal text-teren-text-muted ml-2">· {selectedProperty.name}</span>
						{/if}
					</h1>
				</div>

				<div class="flex items-center gap-4">
					<!-- Date Picker -->
					<div class="flex items-center gap-2 px-4 py-2 bg-teren-background-base rounded-full border border-teren-border-subtle">
						<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="4" rx="2" ry="2"/><line x1="16" x2="16" y1="2" y2="6"/><line x1="8" x2="8" y1="2" y2="6"/><line x1="3" x2="21" y1="10" y2="10"/></svg>
						<input 
							type="date" 
							bind:value={selectedDate}
							class="bg-transparent border-none text-teren-text-main focus:outline-none text-sm" />
					</div>

					<!-- View Toggle -->
					<div class="flex items-center gap-1 p-1 bg-teren-background-base rounded-lg border border-teren-border-subtle">
						<button class="px-3 py-1.5 rounded-md bg-teren-surface-base text-teren-text-main text-sm font-medium shadow-sm">Grid</button>
						<button class="px-3 py-1.5 rounded-md text-teren-text-muted text-sm">List</button>
					</div>
				</div>
			</div>

			<!-- Stats Bar -->
			{#if selectedProperty}
				<div class="flex items-center gap-8 mt-4 pt-4 border-t border-teren-border-subtle">
					<div class="flex items-center gap-2">
						<span class="text-2xl font-bold text-teren-text-main">{rooms.filter(r => r.status === 'occupied').length}</span>
						<span class="text-teren-text-muted">/ {rooms.length} occupied</span>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-teren-text-muted">Occupancy</span>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-2xl font-bold text-teren-primary">{rooms.filter(r => r.status === 'pending').length}</span>
						<span class="text-teren-text-muted">pending check-in</span>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-2xl font-bold text-teren-text-main">{rooms.filter(r => r.status === 'maintenance' || r.status === 'blocked').length}</span>
						<span class="text-teren-text-muted">blocked</span>
					</div>
					<div class="flex items-center gap-2">
						<span class="text-2xl font-bold text-green-700">IDR 842K</span>
						<span class="text-teren-text-muted">RevPAR today</span>
					</div>
				</div>
			{/if}

			<!-- Floor Selector -->
			{#if selectedProperty && floors.length > 0}
				<div class="flex items-center gap-2 mt-4">
					<button class="px-4 py-2 rounded-lg bg-teren-surface-base border border-teren-primary text-teren-text-main font-medium">All floors</button>
					{#each floors as floor (floor.id)}
						<button
							onclick={() => selectFloor(floor)}
							class="px-4 py-2 rounded-lg transition-colors {selectedFloor?.id === floor.id ? 'bg-teren-surface-base border border-teren-primary text-teren-text-main font-medium' : 'text-teren-text-muted hover:text-teren-text-main'}">
							{floor.label || `Floor ${floor.floor_number}`}
						</button>
					{/each}
				</div>
			{/if}
		</header>

		<!-- Content Area -->
		<div class="flex-1 overflow-auto p-6">
			{#if loading}
				<div class="text-center py-12 text-teren-text-muted">Loading...</div>
			{:else if error}
				<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-6">
					{error}
				</div>
			{:else if selectedFloor}
				<div class="space-y-8">
					<!-- Floor Map -->
					<section>
						<h2 class="text-lg font-semibold text-teren-text-main mb-4">
							{selectedFloor.label?.toUpperCase() || `FLOOR ${selectedFloor.floor_number}`}
						</h2>
						<FloorMap 
							floor={selectedFloor} 
							rooms={rooms}
							onRoomMove={handleRoomMove}
							onRoomClick={selectRoom} />
					</section>

					<!-- Status Legend -->
					<div class="flex items-center gap-6 pt-4">
						<div class="flex items-center gap-2">
							<div class="w-3 h-3 rounded-full bg-green-600"></div>
							<span class="text-sm text-teren-text-muted">Available</span>
						</div>
						<div class="flex items-center gap-2">
							<div class="w-3 h-3 rounded-full bg-red-600"></div>
							<span class="text-sm text-teren-text-muted">Occupied</span>
						</div>
						<div class="flex items-center gap-2">
							<div class="w-3 h-3 rounded-full bg-orange-500"></div>
							<span class="text-sm text-teren-text-muted">Pending check-in</span>
						</div>
						<div class="flex items-center gap-2">
							<div class="w-3 h-3 rounded-full bg-gray-500"></div>
							<span class="text-sm text-teren-text-muted">Blocked / Maintenance</span>
						</div>
					</div>
				</div>
			{:else}
				<div class="bg-teren-surface-base border border-teren-border-subtle rounded-xl p-12 text-center">
					<div class="text-teren-text-muted">Select a property and floor to view the map</div>
				</div>
			{/if}
		</div>
	</main>

	<!-- Room Detail Drawer -->
	{#if drawerOpen && selectedRoom}
		<div class="fixed inset-0 z-50">
			<!-- Backdrop -->
			<div 
				class="absolute inset-0 bg-black/30"
				onclick={() => drawerOpen = false} />
			
			<!-- Drawer -->
			<div class="absolute right-0 top-0 h-full w-96 bg-teren-surface-base border-l border-teren-border-subtle shadow-xl flex flex-col">
				<!-- Drawer Header -->
				<div class="p-6 border-b border-teren-border-subtle flex items-center justify-between">
					<h2 class="text-xl font-bold text-teren-text-main">Room {selectedRoom.number}</h2>
					<button 
						onclick={() => drawerOpen = false}
						class="p-2 rounded-lg hover:bg-teren-background-base transition-colors">
						<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
					</button>
				</div>

				<!-- Drawer Content -->
				<div class="flex-1 overflow-auto p-6 space-y-6">
					<!-- Status Badge -->
					<div class="flex items-center gap-3">
						<span class={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium border {getStatusColor(selectedRoom.status)}`}>
							{getStatusText(selectedRoom.status)}
						</span>
					</div>

					<!-- Room Info -->
					<div class="space-y-3">
						<div class="flex justify-between">
							<span class="text-teren-text-muted">Room Number</span>
							<span class="font-medium text-teren-text-main">{selectedRoom.number}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-teren-text-muted">Status</span>
							<span class="font-medium text-teren-text-main">{getStatusText(selectedRoom.status)}</span>
						</div>
					</div>

					<hr class="border-teren-border-subtle" />

					<!-- Rate (example) -->
					<div class="space-y-3">
						<h3 class="font-semibold text-teren-text-main">Rate</h3>
						<div class="bg-teren-background-base rounded-lg p-4">
							<div class="text-2xl font-bold text-teren-text-main">IDR 850K</div>
							<div class="text-sm text-teren-text-muted">per night</div>
						</div>
					</div>

					<!-- Guest (if occupied) -->
					{#if selectedRoom.status === 'occupied'}
						<hr class="border-teren-border-subtle" />
						<div class="space-y-3">
							<h3 class="font-semibold text-teren-text-main">Guest</h3>
							<div class="bg-teren-background-base rounded-lg p-4 space-y-2">
								<div class="font-medium text-teren-text-main">John Doe</div>
								<div class="text-sm text-teren-text-muted">Check-in: May 20, 2026</div>
								<div class="text-sm text-teren-text-muted">Check-out: May 27, 2026</div>
							</div>
						</div>
					{/if}

					<!-- Block Reason (if blocked/maintenance) -->
					{#if selectedRoom.status === 'maintenance' || selectedRoom.status === 'blocked'}
						<hr class="border-teren-border-subtle" />
						<div class="space-y-3">
							<h3 class="font-semibold text-teren-text-main">Block Reason</h3>
							<div class="bg-teren-background-base rounded-lg p-4">
								<p class="text-teren-text-main">Planned maintenance for air conditioning system.</p>
								<p class="text-sm text-teren-text-muted mt-2">Until: May 30, 2026</p>
							</div>
						</div>
					{/if}
				</div>

				<!-- Drawer Footer -->
				<div class="p-6 border-t border-teren-border-subtle space-y-3">
					<button class="w-full py-3 px-4 rounded-lg bg-teren-primary text-white font-medium hover:bg-teren-primary-hover transition-colors">
						Edit Room
					</button>
					<button class="w-full py-3 px-4 rounded-lg border border-teren-border-subtle text-teren-text-main font-medium hover:bg-teren-background-base transition-colors">
						Change Status
					</button>
					{#if selectedRoom.status === 'available'}
						<button class="w-full py-3 px-4 rounded-lg border border-green-600 text-green-700 font-medium hover:bg-green-50 transition-colors">
							Create Booking
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>
