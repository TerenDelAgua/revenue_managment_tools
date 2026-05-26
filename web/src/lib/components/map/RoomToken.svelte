<script lang="ts">
	import type { RoomMap, RoomAvailability } from '$lib/types';

	interface Props {
		room: RoomMap;
		state: RoomAvailability;
		mode?: 'setup' | 'ops';
		onSelect: (room: RoomMap) => void;
		onDragStart?: (e: DragEvent, room: RoomMap) => void;
	}

	let { room, state, mode = 'ops', onSelect, onDragStart }: Props = $props();

	const styles: Record<RoomAvailability, string> = {
		available: 'bg-[#16A34A] border-[#059669] text-white',
		occupied: 'bg-[#DC2626] border-[#B91C1C] text-white',
		pending: 'bg-[#D97706] border-[#B45309] text-white',
		blocked: 'bg-[#44403C] border-[#292524] text-[#FCFBFA]',
		inactive: 'bg-[#A8A29E] border-[#78716C] text-[#1C1917] opacity-60'
	};

	const icons: Record<string, string> = {
		occupied: '🛏️',
		pending: '⏳',
		blocked: '',
		inactive: ''
	};
</script>

<div
	class="room-token group relative flex h-full w-full cursor-pointer flex-col items-center justify-center rounded-lg transition-all duration-200 ease-out select-none
  {styles[state] as string}
  border-t-0 border-r-0 border-b-2 border-l-0 hover:-translate-y-1
  hover:shadow-lg hover:brightness-110 active:scale-95 active:shadow-md"
	style="grid-column: calc(var(--pos-x) + 1); grid-row: calc(var(--pos-y) + 1);"
	role="button"
	tabindex="0"
	draggable={mode === 'setup'}
	ondragstart={(e) => onDragStart?.(e, room)}
	onclick={() => onSelect(room)}
>
	<span class="text-sm leading-tight font-bold tabular-nums drop-shadow-sm">{room.number}</span>

	{#if state !== 'available' && state !== 'inactive'}
		<span class="mt-1 text-[10px] opacity-90">{icons[state]}</span>
	{/if}

	<!-- Tooltip en hover (solo desktop) -->
	<div
		class="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 w-32 -translate-x-1/2 rounded-lg bg-[#1C1917] px-3 py-2 text-center text-xs text-white opacity-0 shadow-xl transition-opacity duration-200 group-hover:opacity-100"
	>
		<p class="font-semibold">{room.number}</p>
		<p class="text-[#A8A29E]">{room.room_type.name}</p>
		<p class="mt-1 text-[#FF8C42] capitalize">{state}</p>
	</div>
</div>
