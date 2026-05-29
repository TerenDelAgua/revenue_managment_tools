<script lang="ts">
	import { _ } from 'svelte-i18n';

	interface Props {
		loadingBookings: boolean;
		pendingBookings: any[];
		onCancel: () => void;
		onAssign: (bookingId: string) => void;
	}

	let { loadingBookings, pendingBookings, onCancel, onAssign }: Props = $props();
</script>

<div
	class="animate-in fade-in slide-in-from-top-2 space-y-3 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4 duration-200"
>
	<div class="flex items-center justify-between border-b border-[#E7E5E4] pb-2">
		<h3 class="text-xs font-bold tracking-wide text-[#1C1917] uppercase">
			{$_('drawer.pendingBookings')}
		</h3>
		<button
			onclick={onCancel}
			class="rounded px-2 py-1 text-[11px] font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4] hover:text-[#1C1917]"
		>
			{$_('drawer.cancel')}
		</button>
	</div>

	{#if loadingBookings}
		<div class="animate-pulse space-y-2">
			<div class="h-14 rounded-lg bg-[#E7E5E4]"></div>
			<div class="h-14 rounded-lg bg-[#E7E5E4]"></div>
		</div>
	{:else if !pendingBookings || pendingBookings.length === 0}
		<p class="py-4 text-center text-sm text-[#57534E]">
			{$_('drawer.noPending')}
		</p>
	{:else}
		<div class="scrollbar-thin max-h-56 space-y-2 overflow-y-auto pr-1">
			{#each pendingBookings as booking (booking.id)}
				<button
					onclick={() => onAssign(booking.id)}
					class="group w-full rounded-lg border border-[#E7E5E4] bg-white p-3.5 text-left transition-all duration-200 hover:-translate-y-0.5 hover:border-[#FF8C42] hover:shadow-md active:scale-[0.98]"
				>
					<div class="flex items-start justify-between gap-3">
						<div class="flex-1">
							<p
								class="font-semibold text-[#1C1917] transition-colors group-hover:text-[#FF8C42]"
							>
								{booking.guest_name}
							</p>
							<p class="mt-0.5 text-[11px] text-[#57534E] tabular-nums">
								{booking.check_in} → {booking.check_out}
							</p>
							<p class="mt-0.5 text-[11px] text-[#57534E] capitalize">
								{booking.source} · {booking.adults}
								{$_('drawer.pax')}
							</p>
						</div>
					</div>
				</button>
			{/each}
		</div>
	{/if}
</div>
