<script lang="ts">
	import { _ } from 'svelte-i18n';
	interface RoomType {
		id: string;
		name: string;
		max_occupancy: number;
	}
	interface Props {
		roomTypes: RoomType[];
		onDragStart: (e: DragEvent, type: RoomType) => void;
	}
	let { roomTypes, onDragStart }: Props = $props();

	function handleDragStart(e: DragEvent, type: RoomType) {
		e.dataTransfer!.setData('text/plain', type.id);
		e.dataTransfer!.setData('application/json', JSON.stringify(type));
		e.dataTransfer!.effectAllowed = 'copy';
		onDragStart(e, type);
	}
</script>

<div class="space-y-3 rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-4">
	<h3 class="text-sm font-semibold tracking-wide text-[#1C1917] uppercase">{$_('drawer.availableTypes')}</h3>
	{#each roomTypes as type (type.id)}
		<div
			draggable
			ondragstart={(e) => handleDragStart(e, type)}
			class="flex cursor-grab items-center justify-between rounded-lg bg-[#F5F4F1] p-3 transition select-none hover:bg-[#FFF7ED] active:cursor-grabbing"
		>
			<span class="font-medium text-[#1C1917]">{$_(`roomTypes.${type.name}`, { default: type.name })}</span>
			<span class="rounded-full bg-[#FFF7ED] px-2 py-1 text-xs font-semibold text-[#E06B20]"
				>{type.max_occupancy} {$_('drawer.pax')}</span
			>
		</div>
	{/each}
</div>
