<script lang="ts">
	import type { RoomMap } from '$lib/types';

	interface Props {
		room: RoomMap | null;
		isOpen: boolean;
		onClose: () => void;
		onAction: (
			action: 'assign' | 'checkin' | 'checkout' | 'block' | 'unblock' | 'view_block' | 'activate',
			payload?: any
		) => void;
	}

	let { room, isOpen, onClose, onAction }: Props = $props();
	let showBlockForm = $state(false);
	let blockReason = $state('maintenance');
	let blockNote = $state('');
	let loading = $state(false);

	$effect(() => {
		if (room) {
			showBlockForm = false;
			blockReason = 'maintenance';
			blockNote = '';
		}
	});

	// Configuración de acciones según estado (Spec §2.2 / §2.3)
	const actions = $derived(room ? {
		available: { primary: 'Asignar a reserva', icon: '📋', callback: () => onAction('assign') },
		pending: { primary: 'Check-in', icon: '🟢', callback: () => onAction('checkin') },
		occupied: {
			primary: 'Check-out',
			icon: '🚪',
			callback: () => onAction('checkout'),
			variant: 'secondary'
		},
		blocked: { primary: 'Ver motivo', icon: '🔧', callback: () => onAction('view_block') },
		inactive: { primary: 'Activar', icon: '⚡', callback: () => onAction('activate') }
	} : {
		available: { primary: '', icon: '', callback: () => {} },
		pending: { primary: '', icon: '', callback: () => {} },
		occupied: { primary: '', icon: '', callback: () => {} },
		blocked: { primary: '', icon: '', callback: () => {} },
		inactive: { primary: '', icon: '', callback: () => {} }
	});

	async function handleBlock() {
		if (!room) return;
		loading = true;
		try {
			await onAction('block', { reason: blockReason, note: blockNote });
			showBlockForm = false;
		} finally {
			loading = false;
		}
	}
</script>

{#if room}
	<div
		class="pointer-events-none fixed inset-0 z-50 flex justify-end {isOpen
			? 'pointer-events-auto'
			: ''}"
	>
		<!-- Backdrop -->
		<div
			class="absolute inset-0 bg-[#1C1917]/20 backdrop-blur-[2px] transition-opacity duration-200 {isOpen
				? 'opacity-100'
				: 'opacity-0'}"
			onclick={onClose}
			role="button"
			tabindex="0"
			onkeydown={(e) => e.key === 'Escape' && onClose()}
			aria-label="Cerrar panel"
		></div>

		<!-- Drawer Panel -->
		<div
			class="relative h-full w-full max-w-md border-l border-[#E7E5E4] bg-[#FCFBFA] shadow-2xl transition-transform duration-300 ease-out {isOpen
				? 'translate-x-0'
				: 'translate-x-full'} flex flex-col"
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b border-[#E7E5E4] p-5">
				<div>
					<h2 class="text-xl font-semibold text-[#1C1917]">{room.number}</h2>
					<p class="mt-0.5 text-sm text-[#57534E]">{room.room_type.name} · {room.availability}</p>
				</div>
				<button
					onclick={onClose}
					class="rounded-lg p-2 text-[#57534E] transition hover:bg-[#F5F4F1]">✕</button
				>
			</div>

			<!-- Body -->
			<div class="flex-1 overflow-y-auto p-5">
				{#if room.availability === 'blocked'}
					<div class="mb-4 rounded-xl border border-[#FF8C42]/30 bg-[#FFF7ED] p-4">
						<p class="text-sm font-medium text-[#1C1917]">Motivo: {blockReason}</p>
						{#if blockNote}<p class="mt-1 text-xs text-[#57534E]">{blockNote}</p>{/if}
					</div>
				{/if}

				<!-- Unified Block Widget (Inline) -->
				{#if showBlockForm}
					<div
						class="mb-4 rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-4 transition-all duration-200"
					>
						<div class="space-y-3">
							<select
								bind:value={blockReason}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#F5F4F1] p-3 text-[#1C1917] outline-none focus:border-[#FF8C42] focus:ring-2 focus:ring-[#FF8C42]/30"
							>
								<option value="maintenance">Mantenimiento</option>
								<option value="owner_use">Uso propietario</option>
								<option value="out_of_service">Fuera de servicio</option>
							</select>
							<input
								type="text"
								bind:value={blockNote}
								placeholder="Notas opcionales..."
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#F5F4F1] p-3 text-[#1C1917] outline-none focus:border-[#FF8C42] focus:ring-2 focus:ring-[#FF8C42]/30"
							/>
						</div>
					</div>
				{/if}
			</div>

			<!-- Footer Actions -->
			<div class="space-y-2 border-t border-[#E7E5E4] bg-[#FCFBFA] p-5">
				{#if showBlockForm}
					<button
						onclick={handleBlock}
						disabled={loading}
						class="w-full rounded-xl py-3 font-medium bg-[#FF8C42] text-white hover:bg-[#E06B20] transition active:scale-[0.98]"
					>
						{loading ? 'Bloqueando...' : 'Confirmar Bloqueo'}
					</button>
				{:else}
					<button
						onclick={actions[room.availability]?.callback}
						disabled={loading}
						class="w-full rounded-xl py-3 font-medium transition active:scale-[0.98] {room.availability ===
						'occupied'
							? 'bg-[#F5F4F1] text-[#1C1917]'
							: 'bg-[#FF8C42] text-white hover:bg-[#E06B20]'}"
					>
						{actions[room.availability]?.icon}
						{actions[room.availability]?.primary}
					</button>
				{/if}

				{#if room.availability !== 'blocked'}
					<button
						onclick={() => (showBlockForm = !showBlockForm)}
						class="w-full rounded-xl py-3 font-medium text-[#57534E] transition hover:bg-[#F5F4F1]"
					>
						{showBlockForm ? 'Cancelar' : 'Bloquear habitación'}
					</button>
				{:else}
					<button
						onclick={() => onAction('unblock')}
						disabled={loading}
						class="w-full rounded-xl py-3 font-medium text-[#DC2626] transition hover:bg-[#FEF2F2]"
					>
						🗑️ Eliminar bloqueo
					</button>
				{/if}
			</div>
		</div>
	</div>
{/if}
