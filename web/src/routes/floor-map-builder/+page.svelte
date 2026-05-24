<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api, type Floor, type Property, type Room } from '$lib/api/client';
	import FloorMap from '$lib/components/FloorMap.svelte';

	let properties: Property[] = [];
	let floors: Floor[] = [];
	let rooms: Room[] = [];
	let selectedProperty: Property | null = null;
	let selectedFloor: Floor | null = null;
	let loading = false;
	let error: string | null = null;

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
			rooms = await api.rooms.listByFloor(floorId);
		} catch (e) {
			error = 'Failed to load rooms';
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

	onMount(() => {
		loadProperties();
	});

	function selectProperty(property: Property) {
		selectedProperty = property;
		loadFloors(property.id);
	}

	function selectFloor(floor: Floor) {
		selectedFloor = floor;
		loadRooms(floor.id);
	}
</script>

<div class="max-w-7xl mx-auto">
	<div class="mb-8">
		<h1 class="text-3xl font-bold text-teren-text-main mb-2">Floor Map Builder</h1>
		<p class="text-teren-text-muted">Design and manage your hotel floor layouts</p>
	</div>

	{#if loading}
		<div class="text-center py-12 text-teren-text-muted">Loading...</div>
	{:else if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-6">
			{error}
		</div>
	{:else}
		<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
			<div class="lg:col-span-1 space-y-6">
				<div class="bg-teren-surface-base border border-teren-border-subtle rounded-xl p-6">
					<h2 class="text-lg font-semibold text-teren-text-main mb-4">Properties</h2>
					<div class="space-y-2">
						{#each properties as property (property.id)}
							<button
								on:click={() => selectProperty(property)}
								class="w-full text-left px-4 py-3 rounded-lg transition-colors {selectedProperty?.id === property.id ? 'bg-teren-primary-subtle border border-teren-primary' : 'hover:bg-teren-background-base border border-transparent'}">
								<div class="font-medium text-teren-text-main">{property.name}</div>
								<div class="text-xs text-teren-text-muted mt-1">{property.currency}</div>
							</button>
						{/each}
					</div>
				</div>

				{#if selectedProperty}
					<div class="bg-teren-surface-base border border-teren-border-subtle rounded-xl p-6">
						<h2 class="text-lg font-semibold text-teren-text-main mb-4">Floors</h2>
						<div class="space-y-2">
							{#each floors as floor (floor.id)}
								<button
									on:click={() => selectFloor(floor)}
									class="w-full text-left px-4 py-3 rounded-lg transition-colors {selectedFloor?.id === floor.id ? 'bg-teren-primary-subtle border border-teren-primary' : 'hover:bg-teren-background-base border border-transparent'}">
									<div class="font-medium text-teren-text-main">
										Floor {floor.floor_number}
									</div>
									{#if floor.label}
										<div class="text-xs text-teren-text-muted mt-1">{floor.label}</div>
									{/if}
								</button>
							{/each}
						</div>
					</div>
				{/if}
			</div>

			<div class="lg:col-span-3">
				{#if selectedFloor}
					<div class="bg-teren-surface-base border border-teren-border-subtle rounded-xl p-6">
						<div class="flex items-center justify-between mb-4">
							<h2 class="text-xl font-semibold text-teren-text-main">
								Floor {selectedFloor.floor_number}
								{#if selectedFloor.label} - {selectedFloor.label}{/if}
							</h2>
							<span class="text-sm text-teren-text-muted">{rooms.length} rooms</span>
						</div>

						<FloorMap 
							floor={selectedFloor} 
							rooms={rooms}
							onRoomMove={handleRoomMove} />
					</div>
				{:else}
					<div class="bg-teren-surface-base border border-teren-border-subtle rounded-xl p-12 text-center">
						<div class="text-teren-text-muted">Select a floor to view the map</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
