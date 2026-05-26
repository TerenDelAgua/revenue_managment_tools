<script lang="ts">
	import FloorTabs from './FloorTabs.svelte';
	import RoomGrid from './RoomGrid.svelte';
	import RoomPalette from './RoomPalette.svelte';
	import StatusLegend from './StatusLegend.svelte'; // Nuevo componente
	import type { MapResponse } from '$lib/types';

	interface Props {
		mapData: MapResponse;
		mode: 'setup' | 'ops';
		dateFrom: string;
		dateTo: string;
		onDateChange: (from: string, to: string) => void;
		onModeToggle: () => void;
		onDrop: (id: string, x: number, y: number) => void;
		onSelect: (room: any) => void;
	}

	let { mapData, mode, dateFrom, dateTo, onDateChange, onModeToggle, onDrop, onSelect }: Props =
		$props();
	let activeFloorId = $state('');

	$effect(() => {
		if (mapData && mapData.floors.length > 0 && !activeFloorId) {
			activeFloorId = mapData.floors[0].id;
		}
	});
</script>

<div class="flex h-full flex-col gap-4">
	<!-- Header del Módulo -->
	<div class="flex flex-wrap items-center justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold text-[#1C1917]">Floor Map</h2>
			<p class="mt-1 text-sm text-[#57534E]">Manage layout and view real-time availability.</p>
		</div>

		<div
			class="flex items-center gap-3 rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-1.5 shadow-sm"
		>
			<button
				onclick={onModeToggle}
				class="rounded-lg px-4 py-2 text-sm font-medium transition-all duration-200
        {mode === 'ops'
					? 'bg-[#FF8C42] text-white shadow-md'
					: 'text-[#57534E] hover:bg-[#F5F4F1]'}"
			>
				Operations
			</button>
			<button
				onclick={onModeToggle}
				class="rounded-lg px-4 py-2 text-sm font-medium transition-all duration-200
        {mode === 'setup'
					? 'bg-[#FF8C42] text-white shadow-md'
					: 'text-[#57534E] hover:bg-[#F5F4F1]'}"
			>
				Setup
			</button>
			<div class="mx-1 h-6 w-px bg-[#E7E5E4]"></div>
			<input
				type="date"
				value={dateFrom}
				onchange={(e) => onDateChange((e.target as HTMLInputElement).value, dateTo)}
				class="border-none bg-transparent px-3 py-1.5 text-sm font-medium text-[#1C1917] focus:ring-0"
			/>
			<span class="text-xs text-[#57534E]">→</span>
			<input
				type="date"
				value={dateTo}
				onchange={(e) => onDateChange(dateFrom, (e.target as HTMLInputElement).value)}
				class="border-none bg-transparent px-3 py-1.5 text-sm font-medium text-[#1C1917] focus:ring-0"
			/>
		</div>
	</div>

	<!-- Área Principal: Sidebar + Grid -->
	<div class="flex min-h-0 flex-1 gap-4">
		<!-- Sidebar de Herramientas -->
		<div class="flex w-72 flex-col gap-4">
			{#if mode === 'setup'}
				<RoomPalette
					roomTypes={[
						{ id: '1', name: 'Standard', max_occupancy: 2 },
						{ id: '2', name: 'Deluxe', max_occupancy: 3 },
						{ id: '3', name: 'Suite', max_occupancy: 4 }
					]}
					onDragStart={() => {}}
				/>
			{:else}
				<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-4 shadow-sm">
					<h3 class="mb-3 text-sm font-semibold text-[#1C1917]">Quick Stats</h3>
					<div class="grid grid-cols-2 gap-3">
						<div class="rounded-lg bg-[#F5F4F1] p-3">
							<p class="text-xs text-[#57534E]">Occupancy</p>
							<p class="text-lg font-bold text-[#1C1917] tabular-nums">78%</p>
						</div>
						<div class="rounded-lg bg-[#F5F4F1] p-3">
							<p class="text-xs text-[#57534E]">RevPAR</p>
							<p class="text-lg font-bold text-[#1C1917] tabular-nums">$84</p>
						</div>
					</div>
				</div>
			{/if}

			<StatusLegend />
		</div>

		<!-- Canvas del Mapa -->
		<div
			class="flex flex-1 flex-col overflow-hidden rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] shadow-sm"
		>
			<!-- Tabs de Pisos -->
			<div class="border-b border-[#E7E5E4] bg-[#FCFBFA] px-4 pt-4">
				<FloorTabs
					floors={mapData?.floors || []}
					activeId={activeFloorId}
					onSelect={(id) => (activeFloorId = id)}
					{mode}
				/>
			</div>

			<!-- Grid Area -->
			<div class="relative flex-1 overflow-auto bg-[#F5F4F1]/50 p-4">
				<!-- Patrón de fondo sutil para guiar el ojo -->
				<div
					class="pointer-events-none absolute inset-0 opacity-[0.03]"
					style="background-image: radial-gradient(#1C1917 1px, transparent 1px); background-size: 20px 20px;"
				></div>

				<RoomGrid
					rooms={mapData?.floors.find((f) => f.id === activeFloorId)?.rooms || []}
					{mode}
					{onDrop}
					{onSelect}
				/>
			</div>
		</div>
	</div>
</div>
