<script lang="ts">
	const states = [
		{
			color: 'bg-[#16A34A] text-white',
			label: 'Available',
			icon: '',
			desc: 'Disponible (Limpio)',
			pattern: ''
		},
		{
			color: 'bg-[#DC2626] text-white',
			label: 'Occupied',
			icon: '🛏️',
			desc: 'Ocupada + Icono Cama',
			pattern: ''
		},
		{
			color: 'bg-[#D97706] text-white',
			label: 'Pending',
			icon: '⏳',
			desc: 'Pendiente + Icono Reloj',
			pattern: ''
		},
		{
			color: 'bg-[#44403C] text-[#FCFBFA]',
			label: 'Blocked',
			icon: '🔧',
			desc: 'Bloqueada + Raya + Icono Llave',
			pattern:
				'bg-[repeating-linear-gradient(45deg,transparent,transparent_2px,rgba(255,255,255,0.1)_2px,rgba(255,255,255,0.1)_4px)]'
		},
		{
			color: 'bg-[#A8A29E] text-[#1C1917] opacity-60',
			label: 'Inactive',
			icon: '',
			desc: 'Inactiva (Opacidad reducida)',
			pattern: ''
		}
	];
</script>

<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-4 shadow-sm">
	<h3 class="mb-3 text-sm font-semibold text-[#1C1917]">Status Legend</h3>
	<div class="space-y-3">
		{#each states as state}
			<div class="flex items-center gap-3">
				<!-- Miniature RoomToken for Legend -->
				<div
					class="relative flex h-8 w-8 flex-shrink-0 items-center justify-center rounded border-b border-black/10 shadow-sm {state.color} {state.pattern}"
				>
					{#if state.icon}
						<span class="text-[10px]">{state.icon}</span>
					{/if}
				</div>
				<div>
					<p class="text-sm font-medium text-[#1C1917]">{state.label}</p>
					<p class="text-xs text-[#57534E]">{state.desc}</p>
				</div>
			</div>
		{/each}
	</div>
</div>
