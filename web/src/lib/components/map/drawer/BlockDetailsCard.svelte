<script lang="ts">
	import type { RoomMap } from '$lib/types';

	interface Props {
		room: RoomMap;
	}

	let { room }: Props = $props();
</script>

<div class="space-y-3 rounded-xl border border-[#DC2626]/20 bg-[#FEF2F2] p-4">
	<h3 class="mb-2 text-[10px] font-bold tracking-widest text-[#991B1B] uppercase">
		Room Blocked Details
	</h3>

	<!-- Reason -->
	<div class="flex items-start gap-3">
		<span class="text-lg">🛡️</span>
		<div class="flex-1">
			<p class="text-sm font-semibold text-[#1C1917] capitalize">
				{room.block_reason?.replace('_', ' ') || 'Maintenance'}
			</p>
			<p class="text-xs text-[#57534E]">Reason</p>
		</div>
	</div>

	<!-- Notes -->
	{#if room.block_notes}
		<div class="flex items-start gap-3 pl-11">
			<div class="flex-1">
				<p class="text-sm text-[#1C1917]">
					{room.block_notes}
				</p>
				<p class="text-[10px] text-[#57534E] uppercase mt-1">Notes</p>
			</div>
		</div>
	{/if}

	<!-- Dates -->
	<div class="mt-3 grid grid-cols-2 gap-4 border-t border-[#DC2626]/10 pt-3">
		<div>
			<p class="text-[10px] text-[#57534E] uppercase">Blocked From</p>
			<p class="text-sm font-semibold text-[#1C1917]">
				{room.block_start_date}
			</p>
		</div>
		<div>
			<p class="text-[10px] text-[#57534E] uppercase">Blocked To</p>
			<p class="text-sm font-semibold text-[#1C1917]">
				{room.block_end_date}
			</p>
		</div>
	</div>
</div>
