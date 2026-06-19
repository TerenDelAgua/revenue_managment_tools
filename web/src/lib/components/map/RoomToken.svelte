<script lang="ts">
	import type { RoomMap } from '$lib/types';

	interface Props {
		room: RoomMap;
		mode?: 'setup' | 'ops';
		onSelect: (room: RoomMap) => void;
	}

	let { room, mode = 'ops', onSelect }: Props = $props();

	// Configuración visual estricta del Spec §3
	const config = {
		available: { bg: 'bg-[#16A34A]', text: 'text-white', icon: '', pattern: '' },
		occupied: { bg: 'bg-[#DC2626]', text: 'text-white', icon: '🛏️', pattern: '' },
		pending: { bg: 'bg-[#D97706]', text: 'text-white', icon: '⏳', pattern: '' },
		blocked: {
			bg: 'bg-[#44403C]',
			text: 'text-[#FCFBFA]',
			icon: '🔧',
			// Patrón de rayas diagonal Spec §3
			pattern:
				'bg-[repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.1)_4px,rgba(255,255,255,0.1)_8px)]'
		},
		// Estado operacional: housekeeping en curso. Sky-600 contrasta con el verde
		// de "available" y se lee como "en proceso" (frío/agua vs caliente/ocupado).
		cleaning: { bg: 'bg-[#0284C7]', text: 'text-white', icon: '🧹', pattern: '' },
		inactive: { bg: 'bg-[#A8A29E]', text: 'text-[#1C1917]', icon: '', pattern: 'opacity-60' }
	};

	const style = $derived(config[room.availability]);

	function handleDragStart(e: DragEvent) {
		if (mode !== 'setup') {
			e.preventDefault();
			return;
		}
		e.dataTransfer!.setData('text/plain', room.id);
		e.dataTransfer!.effectAllowed = 'move';
	}
</script>

<div
	class="room-token relative flex h-14 w-14 cursor-pointer flex-col items-center justify-center rounded-lg border-b-2 border-black/10 shadow-sm transition-all duration-200 ease-out select-none hover:-translate-y-1 hover:shadow-md hover:brightness-110 active:scale-95
  {style.bg} {style.text} {style.pattern}"
	style="grid-column: calc({room.pos_x} + 1); grid-row: calc({room.pos_y} + 1);"
	role="button"
	tabindex="0"
	draggable={mode === 'setup'}
	ondragstart={handleDragStart}
	onclick={() => onSelect(room)}
	onkeydown={(e) => {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			onSelect(room);
		}
	}}
>
	<span class="text-sm leading-tight font-bold tabular-nums drop-shadow-sm">{room.number}</span>

	{#if style.icon}
		<span class="mt-0.5 text-[10px] opacity-90">{style.icon}</span>
	{/if}
</div>
