<script lang="ts">
	import type { RoomMap, RoomAvailability } from '$lib/types';

	interface Props {
		room: RoomMap;
		availState: RoomAvailability;
		mode?: 'setup' | 'ops';
		onSelect: (room: RoomMap) => void;
		onDragStart?: (e: DragEvent, room: RoomMap) => void;
		onUpdateName?: (id: string, newName: string) => void;
	}

	let { room, availState, mode = 'ops', onSelect, onDragStart, onUpdateName }: Props = $props();

	let isEditing = $state(false);
	let localName = $state(room.number);

	$effect(() => {
		localName = room.number;
	});

	// Paleta TEREN · WCAG AA sobre #F5F4F1
	const styles = {
		available: 'bg-[#16A34A] text-white',
		occupied: 'bg-[#DC2626] text-white',
		pending: 'bg-[#D97706] text-white',
		blocked:
			'bg-[#44403C] text-[#FCFBFA] bg-[repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.15)_4px,rgba(255,255,255,0.15)_8px)]',
		inactive: 'bg-[#A8A29E] text-[#1C1917]'
	};

	function handleClick() {
		if (mode === 'setup') {
			isEditing = true;
		} else {
			onSelect(room);
		}
	}

	function commitEdit() {
		isEditing = false;
		if (localName.trim() && localName !== room.number) {
			onUpdateName?.(room.id, localName.trim());
		} else {
			localName = room.number; // Revert
		}
	}

	function handleDragStart(e: DragEvent) {
		if (mode === 'setup' && onDragStart) {
			e.dataTransfer!.setData('text/plain', room.id);
			e.dataTransfer!.effectAllowed = 'move';
			onDragStart(e, room);
		}
	}
</script>

<div
	class="room-token relative flex h-14 w-14 cursor-pointer flex-col items-center justify-center rounded-lg transition-all duration-200 ease-out select-none hover:brightness-110 active:scale-95 {styles[
		availState
	]}"
	style="--pos-x: {room.pos_x}; --pos-y: {room.pos_y}; grid-column: calc(var(--pos-x) + 1); grid-row: calc(var(--pos-y) + 1);"
	role="button"
	tabindex="0"
	draggable={mode === 'setup'}
	ondragstart={handleDragStart}
	onclick={handleClick}
	onkeydown={(e) => e.key === 'Enter' && handleClick()}
>
	{#if isEditing}
		<input
			bind:value={localName}
			onblur={commitEdit}
			onkeydown={(e) => e.key === 'Enter' && commitEdit()}
			class="h-full w-full border-none bg-transparent text-center text-sm font-bold text-white placeholder-white/70 focus:outline-none"
			placeholder="101"
		/>
	{:else}
		<span class="text-sm leading-tight font-semibold tabular-nums">{room.number}</span>
		{#if availState !== 'available'}
			<span
				class="mt-0.5 max-w-[90%] truncate text-[9px] font-medium tracking-wider uppercase opacity-80"
				>{availState}</span
			>
		{/if}
	{/if}
</div>
