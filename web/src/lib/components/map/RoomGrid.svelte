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
		const rawX = Math.floor((e.clientX - rect.left) / STEP);
		const rawY = Math.floor((e.clientY - rect.top) / STEP);

		// BR-06: Clamp to 12x20 grid
		const posX = Math.max(0, Math.min(11, rawX));
		const posY = Math.max(0, Math.min(19, rawY));

		onDrop(roomId, posX, posY);
	}
</script>

<div
	bind:this={gridEl}
	class="room-grid grid overflow-auto rounded-xl bg-[#F5F4F1] p-4"
	style="grid-template-columns: repeat(12, 3.5rem); grid-auto-rows: 3.5rem; gap: 6px;"
	ondragover={handleDragOver}
	ondrop={handleDrop}
>
	{#each rooms as room (room.id)}
		<RoomToken {room} availState={room.availability} {mode} {onSelect} {onUpdateName} />
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
