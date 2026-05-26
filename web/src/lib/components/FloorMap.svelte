<script lang="ts">
	import type { Floor, Room } from '$lib/api/client';

	let { floor, rooms = [], onRoomMove, onRoomClick }: {
		floor: Floor;
		rooms?: Room[];
		onRoomMove?: (roomId: string, x: number, y: number) => void;
		onRoomClick?: (room: Room) => void;
	} = $props();

	let draggedRoom = $state<Room | null>(null);
	let dragOffsetX = $state(0);
	let dragOffsetY = $state(0);
	let isDragging = $state(false);

	function getStatusColor(status: string) {
		switch (status?.toLowerCase()) {
			case 'available':
				return 'border-green-600 bg-green-50 hover:bg-green-100';
			case 'occupied':
				return 'border-red-600 bg-red-50 hover:bg-red-100';
			case 'pending':
			case 'pending check-in':
				return 'border-orange-500 bg-orange-50 hover:bg-orange-100';
			case 'maintenance':
			case 'blocked':
				return 'border-gray-500 bg-gray-100 hover:bg-gray-200';
			default:
				return 'border-teren-primary bg-teren-primary-subtle hover:bg-teren-primary-subtle';
		}
	}

	function getStatusText(status: string) {
		switch (status?.toLowerCase()) {
			case 'available':
				return 'Available';
			case 'occupied':
				return 'Occupied';
			case 'pending':
			case 'pending check-in':
				return 'Pending Check-in';
			case 'maintenance':
				return 'Maintenance';
			case 'blocked':
				return 'Blocked';
			default:
				return status;
		}
	}

	function startDrag(room: Room, e: MouseEvent) {
		isDragging = true;
		draggedRoom = room;
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		dragOffsetX = e.clientX - rect.left;
		dragOffsetY = e.clientY - rect.top;
	}

	function handleDrag(e: MouseEvent) {
		if (!draggedRoom) return;
	}

	function handleEndDrag(e: MouseEvent) {
		if (!draggedRoom || !isDragging) {
			draggedRoom = null;
			isDragging = false;
			return;
		}

		const mapElement = document.getElementById('floor-map');
		if (!mapElement) {
			draggedRoom = null;
			isDragging = false;
			return;
		}

		const rect = mapElement.getBoundingClientRect();
		const x = Math.max(0, Math.floor(e.clientX - rect.left - dragOffsetX));
		const y = Math.max(0, Math.floor(e.clientY - rect.top - dragOffsetY));

		onRoomMove?.(draggedRoom.id, x, y);
		draggedRoom = null;
		isDragging = false;
	}

	function handleClick(room: Room, e: MouseEvent) {
		if (isDragging) return;
		onRoomClick?.(room);
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
			class="absolute cursor-pointer select-none border-2 rounded-xl p-4 shadow-md transition-all hover:shadow-xl {getStatusColor(room.status)}"
			style="left: {room.pos_x}px; top: {room.pos_y}px; min-width: 110px;"
			onmousedown={(e) => startDrag(room, e)}
			onclick={(e) => handleClick(room, e)}
			class:dragging={draggedRoom?.id === room.id}>
			<div class="font-bold text-xl text-teren-text-main">{room.number}</div>
			<div class="text-xs font-medium mt-1 text-teren-text-muted">{getStatusText(room.status)}</div>
		</div>
	{/each}
</div>

<style>
	.dragging {
		cursor: grabbing !important;
		opacity: 0.85;
		z-index: 100;
		transform: scale(1.05);
	}
</style>
