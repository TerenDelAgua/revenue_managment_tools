<script lang="ts">
	import type { RoomMap } from '$lib/types';
	import RoomToken from './RoomToken.svelte';

	interface Props {
		rooms: RoomMap[];
		mode?: 'setup' | 'ops';
		onSelect: (room: RoomMap) => void;
		onDrop: (roomId: string, x: number, y: number) => void;
		onUpdateName?: (id: string, name: string) => void;
	}

	let { rooms, mode = 'ops', onSelect, onDrop, onUpdateName }: Props = $props();
	let gridEl: HTMLDivElement;

	const CELL_SIZE = 56; // px
	const GAP = 6; // px
	const STEP = CELL_SIZE + GAP;

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		e.dataTransfer!.dropEffect = 'move';
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		const roomId = e.dataTransfer!.getData('text/plain');
		if (!roomId || mode !== 'setup') return;

		const rect = gridEl.getBoundingClientRect();
		// Account for 24px padding (p-6) and scroll positions of the grid element
		const rawX = Math.floor((e.clientX - rect.left - 24 + gridEl.scrollLeft) / STEP);
		const rawY = Math.floor((e.clientY - rect.top - 24 + gridEl.scrollTop) / STEP);

		// BR-06: Clamp to 12x20 grid
		const posX = Math.max(0, Math.min(11, rawX));
		const posY = Math.max(0, Math.min(19, rawY));

		onDrop(roomId, posX, posY);
	}

	// Calculate the lowest block's Y position to dynamically expand the grid downwards
	let maxPosY = $derived(rooms.length > 0 ? Math.max(...rooms.map((r) => r.pos_y)) : 0);
	// Always provide at least 6 rows, and ensure there's at least 1 empty row below the lowest block (clamped to 20 max)
	let rowCount = $derived(Math.min(20, Math.max(6, maxPosY + 2)));
</script>

<div
	bind:this={gridEl}
	class="room-grid overflow-auto rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-6 shadow-sm"
	style="display: grid; grid-template-columns: repeat(12, 56px); grid-template-rows: repeat({rowCount}, 56px); gap: 6px; background-image: radial-gradient(#E7E5E4 1px, transparent 1px); background-size: 20px 20px;"
	ondragover={handleDragOver}
	ondrop={handleDrop}
	role="grid"
	tabindex="0"
>
	{#each rooms as room (room.id)}
		<RoomToken {room} {mode} {onSelect} />
	{/each}

	{#if mode === 'setup' && rooms.length === 0}
		<div
			class="col-span-12 flex h-40 flex-col items-center justify-center rounded-lg border-2 border-dashed border-[#D6D3D1] text-sm text-[#57534E]"
		>
			<span class="mb-2 text-3xl opacity-40">📐</span>
			Arrastra una habitación aquí para empezar
		</div>
	{/if}
</div>
