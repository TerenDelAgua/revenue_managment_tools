<script lang="ts">
	import type { RoomMap } from '$lib/types';

	interface Props {
		room: RoomMap;
		stayNights: number;
		onCancel: () => void;
		onConfirm: () => void;
	}

	let { room, stayNights, onCancel, onConfirm }: Props = $props();
</script>

<div
	class="animate-in fade-in slide-in-from-top-2 space-y-3 rounded-xl border border-[#FF8C42]/40 bg-[#FFF7ED] p-4 duration-200"
>
	<div class="flex items-center gap-2">
		<span class="text-lg">🧾</span>
		<h3 class="text-sm font-bold tracking-wide text-[#1C1917] uppercase">
			Confirm Check-out
		</h3>
	</div>

	<div class="space-y-2 rounded-lg border border-[#E7E5E4] bg-white p-3">
		<div class="flex justify-between text-sm">
			<span class="text-[#57534E]">Guest</span>
			<span class="font-semibold text-[#1C1917]">{room.active_guest_name}</span>
		</div>
		<div class="flex justify-between text-sm">
			<span class="text-[#57534E]">Stay</span>
			<span class="font-semibold text-[#1C1917] tabular-nums">
				{room.active_check_in} → {room.active_check_out}
			</span>
		</div>
		<div class="flex justify-between text-sm">
			<span class="text-[#57534E]">Nights</span>
			<span class="font-semibold text-[#1C1917] tabular-nums">{stayNights}</span>
		</div>
		<div class="flex items-center justify-between border-t border-[#E7E5E4] pt-2">
			<span class="text-sm font-medium text-[#1C1917]">Total</span>
			<span class="text-lg font-bold text-[#FF8C42] tabular-nums">IDR 500.000</span>
		</div>
	</div>

	<div class="grid grid-cols-2 gap-2">
		<button
			onclick={onCancel}
			class="rounded-lg border border-[#E7E5E4] bg-white py-2.5 text-sm font-medium text-[#57534E] transition-colors hover:bg-[#F5F4F1]"
		>
			Cancel
		</button>
		<button
			onclick={onConfirm}
			class="rounded-lg bg-[#1C1917] py-2.5 text-sm font-semibold text-white shadow-sm transition-all hover:bg-[#3F3D38] active:scale-95"
		>
			Confirm Check-out
		</button>
	</div>
</div>
