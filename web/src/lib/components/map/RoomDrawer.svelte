<script lang="ts">
	import type { RoomMap } from '$lib/types';
	import { api } from '$lib/api/client';
	import { _ } from 'svelte-i18n';
	import { addToast } from '$lib/store/toastStore';

	// Subcomponentes refactorizados
	import RoomDetailsCard from './drawer/RoomDetailsCard.svelte';
	import GuestDetailsCard from './drawer/GuestDetailsCard.svelte';
	import BlockDetailsCard from './drawer/BlockDetailsCard.svelte';
	import CheckoutConfirmCard from './drawer/CheckoutConfirmCard.svelte';
	import PendingBookingsList from './drawer/PendingBookingsList.svelte';
	import BlockRoomForm from './drawer/BlockRoomForm.svelte';

	interface Props {
		room: RoomMap | null;
		propertyId: string;
		isOpen: boolean;
		onClose: () => void;
		onAction: (
			action: 'assign' | 'checkin' | 'checkout' | 'block' | 'unblock' | 'update_room' | 'delete_room',
			payload?: any
		) => void;
		mode?: 'setup' | 'ops';
	}

	let { room, propertyId, isOpen, onClose, onAction, mode = 'ops' }: Props = $props();

	// === Estados de Revelación Progresiva ===
	let showBlockForm = $state(false);
	let showAssignList = $state(false);
	let loadingBookings = $state(false);
	let pendingBookings = $state<any[]>([]);

	// === Estados de Setup Mode ===
	let editNumber = $state('');
	let numberError = $state<string | null>(null);
	let showDeleteConfirm = $state(false);

	// === Estados del Formulario de Bloqueo ===
	let blockReason = $state<'maintenance' | 'owner_use' | 'out_of_service'>('maintenance');
	let blockNote = $state('');
	let blockStart = $state(new Date().toISOString().split('T')[0]);
	let blockEnd = $state(new Date(Date.now() + 86400000).toISOString().split('T')[0]);
	let isDateValid = $state(true);
	let showCheckoutConfirm = $state(false);

	// Reiniciar estados al cerrar el Drawer o cambiar de habitación
	$effect(() => {
		if (!isOpen) {
			showBlockForm = false;
			showAssignList = false;
			showCheckoutConfirm = false;
			blockReason = 'maintenance';
			blockNote = '';
		} else if (room) {
			editNumber = room.number;
			numberError = null;
			showDeleteConfirm = false;
		}
		const start = new Date(blockStart);
		const end = new Date(blockEnd);
		isDateValid = end > start;
	});

	// === Acciones de Setup Mode ===
	async function handleNumberBlur() {
		const trimmed = editNumber.trim();
		if (!room || trimmed === '' || trimmed === room.number) {
			numberError = null;
			return;
		}
		
		try {
			await onAction('update_room', { number: trimmed });
			numberError = null;
		} catch (err: any) {
			console.error(err);
			if (err.status === 409 || err.message?.includes('exists')) {
				numberError = 'Room number already exists';
				editNumber = room.number;
			} else {
				numberError = err.message || 'Error updating room';
			}
		}
	}

	async function toggleStatus() {
		if (!room) return;
		const isInactive = room.availability === 'inactive';
		const nextStatus = isInactive ? 'active' : 'inactive';
		const oldAvailability = room.availability;
		
		// Optimistic update
		room.availability = isInactive ? 'available' : 'inactive';
		
		try {
			await onAction('update_room', { status: nextStatus });
		} catch (err: any) {
			console.error(err);
			room.availability = oldAvailability;
		}
	}

	function confirmDelete() {
		showDeleteConfirm = false;
		onAction('delete_room');
	}

	// === Acciones ===
	async function loadPendingBookings() {
		if (!propertyId) return;
		loadingBookings = true;
		showAssignList = true;
		try {
			pendingBookings = (await api.bookings.pending(propertyId)) || [];
		} catch (e) {
			console.error('Failed to load bookings', e);
			pendingBookings = [];
		} finally {
			loadingBookings = false;
		}
	}

	function handleAssign(bookingId: string) {
		if (!room) return;
		onAction('assign', { booking_id: bookingId, room_id: room.id });
		showAssignList = false;
	}

	function handleBlockSubmit() {
		if (!isDateValid) {
			addToast('End date must be after start date', 'error');
			return;
		}

		if (!room) return;
		onAction('block', {
			room_id: room.id,
			reason: blockReason,
			notes: blockNote,
			start_date: blockStart,
			end_date: blockEnd
		});
		showBlockForm = false;
		// Reset
		blockReason = 'maintenance';
		blockNote = '';
		blockStart = new Date().toISOString().split('T')[0];
		blockEnd = new Date(Date.now() + 86400000).toISOString().split('T')[0];
	}

	// === Configuración Derivada de la UI ===
	const primaryAction = $derived.by(() => {
		switch (room?.availability) {
			case 'pending':
				return { labelKey: 'drawer.checkIn', color: 'bg-[#16A34A]', action: 'checkin' as const };
			case 'occupied':
				return { labelKey: 'drawer.checkOut', color: 'bg-[#1C1917]', action: 'checkout' as const };
			case 'blocked':
				return {
					labelKey: 'drawer.removeBlock',
					color: 'bg-[#DC2626]',
					action: 'unblock' as const
				};
			default:
				return {
					labelKey: 'drawer.assignBooking',
					color: 'bg-[#FF8C42]',
					action: 'assign' as const
				};
		}
	});

	const stayNights = $derived.by(() => {
		if (!room?.active_check_in || !room?.active_check_out) return 0;
		const start = new Date(room.active_check_in);
		const end = new Date(room.active_check_out);
		return Math.max(1, Math.round((end.getTime() - start.getTime()) / 86400000));
	});

	function requestCheckout() {
		showCheckoutConfirm = true;
	}

	function confirmCheckout() {
		showCheckoutConfirm = false;
		onAction('checkout');
	}
</script>

{#if room}
	<!-- Backdrop -->
	<button
		type="button"
		aria-label="Close drawer"
		class="fixed inset-0 z-40 block w-full bg-[#1C1917]/20 backdrop-blur-[1px] transition-opacity duration-200 {isOpen
			? 'cursor-default opacity-100'
			: 'pointer-events-none opacity-0'}"
		onclick={onClose}
	></button>

	<!-- Panel del Drawer -->
	<div
		class="fixed top-0 right-0 z-50 flex h-full w-full max-w-md transform flex-col bg-[#FCFBFA] shadow-xl transition-transform duration-250 ease-out {isOpen
			? 'translate-x-0'
			: 'translate-x-full'}"
		style="border-left: 1px solid #E7E5E4;"
	>
		<!-- Encabezado -->
		<div class="flex items-start justify-between border-b border-[#E7E5E4] bg-[#FCFBFA] px-6 py-4">
			<div>
				<div class="mb-1 flex items-center gap-2">
					<h2 class="text-xl font-bold tracking-tight text-[#1C1917]">{room.number}</h2>
					<span
						class="h-2.5 w-2.5 rounded-full {room.availability === 'available'
							? 'bg-[#16A34A]'
							: room.availability === 'occupied'
								? 'bg-[#DC2626]'
								: room.availability === 'pending'
									? 'bg-[#D97706]'
									: room.availability === 'blocked'
										? 'bg-[#44403C]'
										: 'bg-[#A8A29E]'}"
					></span>
				</div>
				<p class="text-sm text-[#57534E]">
					{$_(`roomTypes.${room.room_type.name}`, { default: room.room_type.name })} · {$_(`status.${room.availability}`)}
				</p>
			</div>
			<button
				title={$_('drawer.close')}
				onclick={onClose}
				class="rounded-lg p-2 text-[#57534E] transition-colors hover:bg-[#F5F4F1] hover:text-[#1C1917]"
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="20"
					height="20"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<line x1="18" y1="6" x2="6" y2="18"></line>
					<line x1="6" y1="6" x2="18" y2="18"></line>
				</svg>
			</button>
		</div>

		<!-- Contenido Scrollable -->
		<div class="flex-1 space-y-5 overflow-y-auto p-5">
			{#if mode === 'setup'}
				<!-- Tarjeta de Configuración (Setup Mode) -->
				<div class="rounded-xl border border-[#E7E5E4] bg-white p-5 shadow-sm space-y-4">
					<h3 class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Configuración de Habitación</h3>
					
					<!-- Room Number Input -->
					<div class="space-y-1">
						<label class="block text-xs font-semibold text-[#57534E]">Número de Habitación</label>
						<div class="relative">
							<input 
								type="text" 
								bind:value={editNumber}
								onblur={handleNumberBlur}
								class="w-full rounded-lg border px-3 py-2 text-sm font-medium text-stone-800 focus:outline-none 
								{numberError ? 'border-[#DC2626] ring-1 ring-[#DC2626]' : 'border-[#E7E5E4] focus:border-[#FF8C42] focus:ring-1 focus:ring-[#FF8C42]'}"
							/>
							{#if numberError}
								<p class="mt-1 text-xs font-medium text-[#DC2626]">{numberError}</p>
							{/if}
						</div>
					</div>

					<!-- Status Toggle -->
					<div class="flex items-center justify-between py-2 border-t border-b border-[#F5F4F1]">
						<div>
							<span class="block text-sm font-semibold text-stone-800">Estado de Ventas</span>
							<span class="text-xs text-[#57534E]">
								{room.availability === 'inactive' ? 'Inactiva (No se vende)' : 'Activa (Disponible para reservas)'}
							</span>
						</div>
						
						<button
							type="button"
							onclick={toggleStatus}
							class="relative h-6 w-11 rounded-full p-1 transition-colors cursor-pointer {room.availability === 'inactive' ? 'bg-stone-300' : 'bg-[#16A34A]'}"
						>
							<div class="h-4 w-4 rounded-full bg-white transition-transform {room.availability === 'inactive' ? '' : 'translate-x-5'}"></div>
						</button>
					</div>

					<!-- Deletion Section -->
					<div class="pt-2 space-y-2">
						<span class="block text-xs font-semibold text-[#57534E]">Eliminar Habitación</span>
						{#if room.has_bookings}
							<div class="rounded-lg bg-stone-50 border border-stone-200 p-3 text-xs text-[#57534E] leading-relaxed">
								⚠️ Esta habitación tiene historial de reservas y no se puede eliminar permanentemente. 
								Si deseas sacarla de la venta, desactiva el <strong>Estado de Ventas</strong> arriba.
							</div>
							<button 
								disabled 
								type="button"
								class="w-full py-2.5 rounded-lg border border-stone-200 bg-stone-100 text-xs font-semibold text-stone-400 cursor-not-allowed text-center"
							>
								Has booking history
							</button>
						{:else}
							<button 
								type="button"
								onclick={() => showDeleteConfirm = true}
								class="w-full py-2.5 rounded-lg bg-[#DC2626] hover:bg-[#B91C1C] text-xs font-semibold text-white transition-colors cursor-pointer text-center"
							>
								Eliminar Habitación
							</button>
						{/if}
					</div>
				</div>
			{:else}
				<!-- Ficha de Detalles de Habitación -->
				<RoomDetailsCard {room} />

				<!-- Ficha de Huésped (Hospedado o Entrante) -->
				{#if room.active_booking || room.pending_booking}
					<GuestDetailsCard {room} />
				{/if}

				<!-- Ficha de Detalles del Bloqueo -->
				{#if room.availability === 'blocked' && room.block}
					<BlockDetailsCard {room} />
				{/if}
			{/if}

			<!-- Confirmación de Check-out -->
			{#if showCheckoutConfirm && room?.availability === 'occupied'}
				<CheckoutConfirmCard
					{room}
					{stayNights}
					onCancel={() => (showCheckoutConfirm = false)}
					onConfirm={confirmCheckout}
				/>
			{/if}

			<!-- Listado de Reservas Pendientes de Asignar -->
			{#if showAssignList}
				<PendingBookingsList
					{loadingBookings}
					{pendingBookings}
					onCancel={() => (showAssignList = false)}
					onAssign={handleAssign}
				/>
			{/if}

			<!-- Formulario para Bloquear Habitación -->
			{#if showBlockForm}
				<BlockRoomForm
					bind:blockReason
					bind:blockNote
					bind:blockStart
					bind:blockEnd
					{isDateValid}
				/>
			{/if}
		</div>

		<!-- Pie de página / Acciones Principales -->
		<div class="space-y-3 border-t border-[#E7E5E4] bg-[#FCFBFA] p-5">
			{#if mode === 'setup'}
				<button
					onclick={onClose}
					class="w-full py-3 bg-[#1C1917] hover:bg-[#3F3D38] flex items-center justify-center gap-2 rounded-lg font-semibold text-white shadow-sm transition-all duration-200 active:scale-95 cursor-pointer text-sm"
				>
					Listo / Cerrar
				</button>
			{:else if showBlockForm}
				<button
					onclick={handleBlockSubmit}
					class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#FF8C42] py-3.5 font-semibold text-white shadow-sm transition-all duration-200 hover:brightness-110 active:scale-95"
				>
					{$_('drawer.confirmBlock')}
				</button>
				<button
					onclick={() => (showBlockForm = false)}
					class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#F5F4F1] py-2.5 text-xs font-medium text-[#57534E] transition-all duration-200 hover:bg-[#E7E5E4]"
				>
					{$_('drawer.cancel')}
				</button>
			{:else if !showAssignList}
				{#if room.availability === 'occupied' && !showCheckoutConfirm}
					<button
						onclick={requestCheckout}
						class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#1C1917] py-3.5 font-semibold text-white shadow-sm transition-all duration-200 hover:bg-[#3F3D38] active:scale-95"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="20"
							height="20"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
							<polyline points="16 17 21 12 16 7"></polyline>
							<line x1="21" y1="12" x2="9" y2="12"></line>
						</svg>
						Check Out Guest
					</button>
				{:else if room.availability !== 'occupied'}
					<button
						onclick={() =>
							primaryAction.action === 'assign'
								? loadPendingBookings()
								: onAction(primaryAction.action)}
						class="w-full py-3.5 {primaryAction.color} flex items-center justify-center gap-2 rounded-lg font-semibold text-white shadow-sm transition-all duration-200 hover:brightness-110 active:scale-95"
					>
						{$_(primaryAction.labelKey)}
					</button>
				{/if}

				<!-- Opción de Bloqueo si está disponible -->
				{#if room.availability !== 'blocked' && room.availability !== 'inactive'}
					<button
						onclick={() => (showBlockForm = true)}
						class="w-full rounded-lg bg-[#F5F4F1] py-2.5 text-xs font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4]"
					>
						{$_('drawer.blockRoom')}
					</button>
				{/if}
			{/if}
		</div>
	</div>
{/if}

{#if showDeleteConfirm && room}
	<!-- Destructive Deletion Confirmation Modal -->
	<div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/40 backdrop-blur-sm" onclick={() => showDeleteConfirm = false}>
		<div class="w-full max-w-sm rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-6 shadow-xl" onclick={(e) => e.stopPropagation()}>
			<h3 class="text-lg font-bold text-[#1C1917]">¿Eliminar habitación?</h3>
			
			<p class="mt-2 text-sm text-[#57534E] leading-relaxed">
				¿Estás seguro de que deseas eliminar permanentemente la habitación <strong class="text-stone-800">{room.number}</strong>? Esta acción no se puede deshacer.
			</p>
			
			<div class="mt-6 flex flex-wrap gap-2 justify-end">
				<button 
					type="button" 
					class="rounded-lg border border-[#E7E5E4] bg-white px-4 py-2 text-sm font-medium text-[#57534E] hover:bg-[#F5F4F1] cursor-pointer"
					onclick={() => showDeleteConfirm = false}
				>
					Cancelar
				</button>
				<button 
					type="button" 
					class="rounded-lg bg-[#DC2626] hover:bg-[#B91C1C] px-4 py-2 text-sm font-medium text-white transition cursor-pointer"
					onclick={confirmDelete}
				>
					Eliminar
				</button>
			</div>
		</div>
	</div>
{/if}
