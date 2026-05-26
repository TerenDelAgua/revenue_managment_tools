<script lang="ts">
	const states = [
		{ color: 'bg-[#16A34A]', label: 'Available', desc: 'Ready for booking' },
		{ color: 'bg-[#DC2626]', label: 'Occupied', desc: 'Guest checked in' },
		{ color: 'bg-[#D97706]', label: 'Pending', desc: 'Confirmed, arrival soon' },
		{ color: 'bg-[#44403C]', label: 'Blocked', desc: 'Maintenance/Owner use' },
		{ color: 'bg-[#A8A29E]', label: 'Inactive', desc: 'Not for rent' }
	];
</script>

<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-4 shadow-sm">
	<h3 class="mb-3 text-sm font-semibold text-[#1C1917]">Status Legend</h3>
	<div class="space-y-2">
		{#each states as state}
			<div class="flex items-center gap-3">
				<div class="h-4 w-4 rounded {state.color} shadow-sm"></div>
				<div>
					<p class="text-sm font-medium text-[#1C1917]">{state.label}</p>
					<p class="text-xs text-[#57534E]">{state.desc}</p>
				</div>
			</div>
		{/each}
	</div>
</div>
