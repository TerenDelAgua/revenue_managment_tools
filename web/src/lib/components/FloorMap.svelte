<script lang="ts">
	import type { Floor, Room } from '$lib/api/client';
	import { api } from '$lib/api/client';

	let { floor, rooms = [], onRoomMove }: {
		floor: Floor;
		rooms?: Room[];
		onRoomMove?: (roomId: string, x: number, y: number) => void;
	} = $props();

	let draggedRoom = $state<Room | null>(null);
	let dragOffsetX = $state(0);
	let dragOffsetY = $state(0);

	function startDrag(room: Room, e: MouseEvent) {
		draggedRoom = room;
		const rect = (e.target as HTMLElement).getBoundingClientRect();
		dragOffsetX = e.clientX - rect.left;
		dragOffsetY = e.clientY - rect.top;
	}

	function handleDrag(e: MouseEvent) {
		if (!draggedRoom) return;
	}

	function handleEndDrag(e: MouseEvent) {
		if (!draggedRoom) return;

		const mapElement = document.getElementById('floor-map');
		if (!mapElement) {
			draggedRoom = null;
			return;
		}

		const rect = mapElement.getBoundingClientRect();
		const x = Math.max(0, Math.floor(e.clientX - rect.left - dragOffsetX));
		const y = Math.max(0, Math.floor(e.clientY - rect.top - dragOffsetY));

		onRoomMove?.(draggedRoom.id, x, y);
		draggedRoom = null;
	}
</script>

<div id="floor-map" 
	class="relative w-full h-[600px] bg-teren-surface-base border border-teren-border-subtle rounded-xl overflow-hidden"
	onmousemove={handleDrag}
	onmouseup={handleEndDrag}
	onmouseleave={handleEndDrag}>
	<div class="absolute inset-0 opacity-20" style="
		background-image: linear-gradient(#E7E5E4 1px, transparent 1px),
		                  linear-gradient(90deg, #E7E5E4 1px, transparent 1px);
		background-size: 50px 50px;
	"></div>

	{#each rooms as room (room.id)}
		<div 
			role="button"
			class="absolute cursor-grab select-none bg-teren-primary-subtle border-2 border-teren-primary rounded-lg p-3 shadow-md transition-shadow hover:shadow-lg"
			style="left: {room.pos_x}px; top: {room.pos_y}px; min-width: 100px;"
			onmousedown={(e) => startDrag(room, e)}
			class:dragging={draggedRoom?.id === room.id}>
			<div class="font-semibold text-teren-text-main">{room.number}</div>
			<div class="text-xs text-teren-text-muted mt-1">{room.status}</div>
		</div>
	{/each}
</div>

<style>
	.dragging {
		cursor: grabbing !important;
		opacity: 0.8;
		z-index: 100;
	}
</style>
