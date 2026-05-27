<script lang="ts">
	import type { RoomMap } from '$lib/types';

	interface Props {
		room: RoomMap;
	}

	let { room }: Props = $props();
</script>

<div class="space-y-3 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4">
	<h3 class="mb-2 text-[10px] font-bold tracking-widest text-[#57534E] uppercase">
		{room.availability === 'occupied' ? 'Checked In Guest' : 'Incoming Guest'}
	</h3>

	<!-- Guest Name -->
	<div class="flex items-start gap-3">
		<span class="text-lg">👤</span>
		<div class="flex-1">
			<p class="text-sm font-semibold text-[#1C1917]">
				{room.active_guest_name || room.pending_guest_name}
			</p>
			<p class="text-xs text-[#57534E]">Guest</p>
		</div>
	</div>

	<!-- Phone -->
	{#if room.active_guest_phone || room.pending_guest_phone}
		<div class="flex items-center gap-3 pl-11">
			<span class="text-sm font-medium text-[#1C1917]">
				{room.active_guest_phone || room.pending_guest_phone}
			</span>
		</div>
	{/if}

	<!-- Nationality -->
	{#if room.active_guest_nationality || room.pending_guest_nationality}
		<div class="flex items-center gap-3 pl-11">
			<span
				class="rounded-full bg-[#FFF7ED] px-2 py-1 text-xs font-semibold text-[#E06B20]"
			>
				{room.active_guest_nationality || room.pending_guest_nationality}
			</span>
		</div>
	{/if}

	<!-- Dates -->
	<div class="mt-3 grid grid-cols-2 gap-4 border-t border-[#E7E5E4] pt-3">
		<div>
			<p class="text-[10px] text-[#57534E] uppercase">Check-in</p>
			<p class="text-sm font-semibold text-[#1C1917]">
				{room.active_check_in || room.pending_check_in}
			</p>
		</div>
		<div>
			<p class="text-[10px] text-[#57534E] uppercase">Check-out</p>
			<p class="text-sm font-semibold text-[#1C1917]">
				{room.active_check_out || room.pending_check_out}
			</p>
		</div>
	</div>
</div>
