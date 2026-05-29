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
			action: 'assign' | 'checkin' | 'checkout' | 'block' | 'unblock',
			payload?: any
		) => void;
	}

	let { room, propertyId, isOpen, onClose, onAction }: Props = $props();

	// === Estados de Revelación Progresiva ===
	let showBlockForm = $state(false);
	let showAssignList = $state(false);
	let loadingBookings = $state(false);
	let pendingBookings = $state<any[]>([]);

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
		}
		const start = new Date(blockStart);
		const end = new Date(blockEnd);
		isDateValid = end > start;
	});

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
			{#if showBlockForm}
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
