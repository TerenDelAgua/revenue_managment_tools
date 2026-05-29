<script lang="ts">
	interface Floor {
		id: string;
		label: string;
		floor_number: number;
	}
	interface Props {
		floors: Floor[];
		activeId: string;
		onSelect: (id: string) => void;
		mode: 'setup' | 'ops';
	}
	let { floors, activeId, onSelect, mode }: Props = $props();
</script>

<div class="scrollbar-hide flex items-center gap-1 overflow-x-auto pb-2">
	{#each floors as floor (floor.id)}
		<button
			onclick={() => onSelect(floor.id)}
			class="rounded-t-lg px-4 py-2 text-sm font-medium whitespace-nowrap transition duration-200 ease-out
      {floor.id === activeId
				? 'border-b-2 border-[#FF8C42] bg-[#FCFBFA] text-[#FF8C42]'
				: 'text-[#57534E] hover:bg-[#F5F4F1] hover:text-[#1C1917]'}"
		>
			{floor.label}
		</button>
	{/each}

	{#if mode === 'setup'}
		<button class="px-3 py-2 text-sm text-[#57534E] transition hover:text-[#FF8C42]"
			>+ Añadir</button
		>
	{/if}
</div>
