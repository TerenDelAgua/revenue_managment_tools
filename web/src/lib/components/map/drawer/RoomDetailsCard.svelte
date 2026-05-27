<script lang="ts">
	import type { RoomMap } from '$lib/types';
	import { _ } from 'svelte-i18n';

	interface Props {
		room: RoomMap;
	}

	let { room }: Props = $props();
</script>

<div class="rounded-xl border border-[#E7E5E4] bg-white p-4 shadow-sm">
	<h3 class="mb-3 text-[10px] font-bold tracking-widest text-[#57534E] uppercase">
		{$_('drawer.details')}
	</h3>
	<div class="grid grid-cols-2 gap-4 text-sm">
		<div>
			<p class="text-xs text-[#57534E]">{$_('drawer.type')}</p>
			<p class="font-medium text-[#1C1917]">
				{$_(`roomTypes.${room.room_type.name}`, { default: room.room_type.name })}
			</p>
		</div>
		<div>
			<p class="text-xs text-[#57534E]">{$_('drawer.ref')}</p>
			<p class="font-mono text-[#1C1917]">{room.id.slice(0, 8)}</p>
		</div>
	</div>
</div>
