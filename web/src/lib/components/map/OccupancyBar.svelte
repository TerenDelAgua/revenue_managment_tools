<script lang="ts">
	import { page } from '$app/stores';
	import { addToast } from '$lib/store/toastStore';
	import type { MapResponse } from '$lib/types';

	interface Props {
		mapData: MapResponse | null;
	}

	let { mapData }: Props = $props();
	let occupancyRate = $derived.by(() => {
		if (!mapData) return 0;
		const totalRooms = mapData.floors.flatMap((f) => f.rooms).length;
		const occupiedRooms = mapData.floors
			.flatMap((f) => f.rooms)
			.filter((r) => r.availability === 'occupied').length;
		return totalRooms > 0 ? Math.round((occupiedRooms / totalRooms) * 100) : 0;
	});

	let revpar = $derived.by(() => {
		if (!mapData) return 0;
		const totalRooms = mapData.floors.flatMap((f) => f.rooms).length;
		// En un MVP real, esto vendría de un endpoint dedicado
		return totalRooms > 0 ? 500000 : 0; // Ejemplo: IDR 500.000
	});

	let showDetails = $state(false);
</script>

{#if mapData}
	<div class="border-b border-[#E7E5E4] bg-[#FCFBFA] p-4">
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<!-- Left: Occupancy Stats -->
			<div class="flex items-center gap-4">
				<div class="flex items-center">
					<div class="mr-1 text-4xl font-bold text-[#1C1917] tabular-nums">{occupancyRate}</div>
					<span class="text-sm text-[#57534E]">%</span>
				</div>

				<div class="h-10 w-px bg-[#E7E5E4]"></div>

				<div class="flex items-center gap-2">
					<div class="text-lg font-medium text-[#1C1917] tabular-nums">
						{mapData.floors.flatMap((f) => f.rooms).filter((r) => r.availability === 'occupied')
							.length}
					</div>
					<span class="text-sm text-[#57534E]">/</span>
					<div class="text-lg font-medium text-[#57534E] tabular-nums">
						{mapData.floors.flatMap((f) => f.rooms).length}
					</div>
					<span class="text-sm text-[#57534E]">rooms</span>
				</div>
			</div>

			<!-- Center: RevPAR -->
			<div class="flex items-center gap-3">
				<div class="rounded-lg border border-[#FF8C42]/30 bg-[#FFF7ED] p-2.5">
					<div class="text-xs tracking-wide text-[#57534E] uppercase">RevPAR</div>
					<div class="text-lg font-bold text-[#1C1917] tabular-nums">
						IDR {revpar.toLocaleString()}
					</div>
				</div>
			</div>

			<!-- Right: Actions -->
			<div class="flex items-center gap-2">
				<button
					onclick={() => (showDetails = !showDetails)}
					class="rounded-lg bg-[#F5F4F1] px-3 py-1.5 text-sm font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4]"
				>
					Details
				</button>
			</div>
		</div>

		<!-- Detailed View (Progressive Disclosure) -->
		{#if showDetails}
			<div
				class="animate-in fade-in slide-in-from-top-2 mt-4 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4 duration-200"
			>
				<div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">Occupancy</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">{occupancyRate}%</p>
					</div>

					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">Rooms Occupied</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">
							{mapData.floors.flatMap((f) => f.rooms).filter((r) => r.availability === 'occupied')
								.length}
						</p>
					</div>

					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">RevPAR</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">
							IDR {revpar.toLocaleString()}
						</p>
					</div>

					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">ADR</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">IDR 600.000</p>
					</div>
				</div>
			</div>
		{/if}
	</div>
{/if}
