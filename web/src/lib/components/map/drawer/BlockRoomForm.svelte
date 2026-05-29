<script lang="ts">
	interface Props {
		blockReason: 'maintenance' | 'owner_use' | 'out_of_service';
		blockNote: string;
		blockStart: string;
		blockEnd: string;
		isDateValid: boolean;
	}

	let {
		blockReason = $bindable(),
		blockNote = $bindable(),
		blockStart = $bindable(),
		blockEnd = $bindable(),
		isDateValid
	}: Props = $props();
</script>

<div
	class="animate-in fade-in slide-in-from-top-2 space-y-3 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4 duration-200"
>
	<h3 class="text-xs font-bold tracking-wide text-[#1C1917] uppercase">Block Room</h3>

	<!-- Reason Selection -->
	<select
		bind:value={blockReason}
		class="w-full rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] transition-all outline-none focus:border-[#FF8C42] focus:ring-2 focus:ring-[#FF8C42]/30"
	>
		<option value="maintenance">Maintenance</option>
		<option value="owner_use">Owner Use</option>
		<option value="out_of_service">Out of Service</option>
	</select>

	<!-- Date Range -->
	<div class="grid grid-cols-2 gap-3">
		<div class="relative">
			<label class="mb-1 block text-xs font-medium text-[#57534E]">Start Date</label>
			<input
				type="date"
				bind:value={blockStart}
				class="w-full rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
			/>
			{#if !isDateValid && blockStart}
				<div class="absolute -bottom-5 left-0 text-xs text-[#DC2626]">Invalid date</div>
			{/if}
		</div>
		<div class="relative">
			<label class="mb-1 block text-xs font-medium text-[#57534E]">End Date</label>
			<input
				type="date"
				bind:value={blockEnd}
				class="w-full rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
			/>
			{#if !isDateValid && blockEnd}
				<div class="absolute -bottom-5 left-0 text-xs text-[#DC2626]">
					End must be after start
				</div>
			{/if}
		</div>
	</div>

	<!-- Notes -->
	<div>
		<label class="mb-1 block text-xs font-medium text-[#57534E]">Notes (Optional)</label>
		<textarea
			bind:value={blockNote}
			rows="2"
			placeholder="Why is this room blocked?"
			class="w-full resize-none rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
		></textarea>
	</div>
</div>
