<script lang="ts">
	import type { RoomMap } from '$lib/types';
	import { api } from '$lib/api/client';
	import { _ } from 'svelte-i18n';

	interface Props {
		room: RoomMap | null;
		propertyId: string; // ← Se pasa desde el padre, ya que RoomMap no lo contiene
		isOpen: boolean;
		onClose: () => void;
		onAction: (
			action: 'assign' | 'checkin' | 'checkout' | 'block' | 'unblock',
			payload?: any
		) => void;
	}

	let { room, propertyId, isOpen, onClose, onAction }: Props = $props();

	// === Progressive Disclosure States ===
	let showBlockForm = $state(false);
	let showAssignList = $state(false);
	let loadingBookings = $state(false);
	let pendingBookings = $state<any[]>([]);

	// === Block Form State ===
	let blockReason = $state<'maintenance' | 'owner_use' | 'out_of_service'>('maintenance');
	let blockNote = $state('');
	let blockStart = $state(new Date().toISOString().split('T')[0]);
	let blockEnd = $state(new Date(Date.now() + 86400000).toISOString().split('T')[0]);

	// === Actions ===
	async function loadPendingBookings() {
		if (!propertyId) return;
		loadingBookings = true;
		showAssignList = true;
		try {
			pendingBookings = await api.bookings.pending(propertyId);
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
	}

	// === Derived UI Config ===
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

	<!-- Drawer Panel -->
	<div
		class="fixed top-0 right-0 z-50 flex h-full w-full max-w-md transform flex-col bg-[#FCFBFA] shadow-xl transition-transform duration-250 ease-out {isOpen
			? 'translate-x-0'
			: 'translate-x-full'}"
		style="border-left: 1px solid #E7E5E4;"
	>
		<!-- Header -->
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
					{$_(`roomTypes.${room.room_type.name}`, { default: room.room_type.name })} · {$_(
						`status.${room.availability}`
					)}
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
					><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"
					></line></svg
				>
			</button>
		</div>

		<!-- Scrollable Content -->
		<div class="flex-1 space-y-5 overflow-y-auto p-5">
			<!-- Room Details Card -->
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
			{#if room.active_booking || room.pending_booking}
				<div class="space-y-3 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4">
					<h3 class="mb-2 text-[10px] font-bold tracking-widest text-[#57534E] uppercase">
						{room.availability === 'occupied' ? 'Checked In Guest' : 'Incoming Guest'}
					</h3>

					<!-- Guest Name -->
					<div class="flex items-start gap-3">
						<span class="text-lg">👤</span>
						<div class="flex-1">
							<p class="text-sm font-semibold text-[#1C1917]">
								{room.active_guest_name || room.pending_guest_name}
							</p>
							<p class="text-xs text-[#57534E]">Guest</p>
						</div>
					</div>

					<!-- Phone -->
					{#if room.active_guest_phone || room.pending_guest_phone}
						<div class="flex items-center gap-3 pl-11">
							<span class="text-sm font-medium text-[#1C1917]">
								{room.active_guest_phone || room.pending_guest_phone}
							</span>
						</div>
					{/if}

					<!-- Nationality -->
					{#if room.active_guest_nationality || room.pending_guest_nationality}
						<div class="flex items-center gap-3 pl-11">
							<span
								class="rounded-full bg-[#FFF7ED] px-2 py-1 text-xs font-semibold text-[#E06B20]"
							>
								{room.active_guest_nationality || room.pending_guest_nationality}
							</span>
						</div>
					{/if}

					<!-- Dates -->
					<div class="mt-3 grid grid-cols-2 gap-4 border-t border-[#E7E5E4] pt-3">
						<div>
							<p class="text-[10px] text-[#57534E] uppercase">Check-in</p>
							<p class="text-sm font-semibold text-[#1C1917]">
								{room.active_check_in || room.pending_check_in}
							</p>
						</div>
						<div>
							<p class="text-[10px] text-[#57534E] uppercase">Check-out</p>
							<p class="text-sm font-semibold text-[#1C1917]">
								{room.active_check_out || room.pending_check_out}
							</p>
						</div>
					</div>
				</div>
			{/if}

			<!-- INLINE LIST: Pending Bookings -->
			{#if showAssignList}
				<div
					class="animate-in fade-in slide-in-from-top-2 space-y-3 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4 duration-200"
				>
					<div class="flex items-center justify-between border-b border-[#E7E5E4] pb-2">
						<h3 class="text-xs font-bold tracking-wide text-[#1C1917] uppercase">
							{$_('drawer.pendingBookings')}
						</h3>
						<button
							onclick={() => (showAssignList = false)}
							class="rounded px-2 py-1 text-[11px] font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4] hover:text-[#1C1917]"
						>
							{$_('drawer.cancel')}
						</button>
					</div>

					{#if loadingBookings}
						<div class="animate-pulse space-y-2">
							<div class="h-14 rounded-lg bg-[#E7E5E4]"></div>
							<div class="h-14 rounded-lg bg-[#E7E5E4]"></div>
						</div>
					{:else if pendingBookings.length === 0}
						<p class="py-4 text-center text-sm text-[#57534E]">
							{$_('drawer.noPending')}
						</p>
					{:else}
						<div class="scrollbar-thin max-h-56 space-y-2 overflow-y-auto pr-1">
							{#each pendingBookings as booking (booking.id)}
								<button
									onclick={() => handleAssign(booking.id)}
									class="group w-full rounded-lg border border-[#E7E5E4] bg-white p-3.5 text-left transition-all duration-200 hover:-translate-y-0.5 hover:border-[#FF8C42] hover:shadow-md active:scale-[0.98]"
								>
									<div class="flex items-start justify-between gap-3">
										<div class="flex-1">
											<p
												class="font-semibold text-[#1C1917] transition-colors group-hover:text-[#FF8C42]"
											>
												{booking.guest_name}
											</p>
											<p class="mt-0.5 text-[11px] text-[#57534E] tabular-nums">
												{booking.check_in} → {booking.check_out}
											</p>
											<p class="mt-0.5 text-[11px] text-[#57534E] capitalize">
												{booking.source} · {booking.adults}
												{$_('drawer.pax')}
											</p>
										</div>
										<span class="text-sm font-bold whitespace-nowrap text-[#1C1917] tabular-nums">
											IDR {(booking.total_amount / 1000).toFixed(0)}k
										</span>
									</div>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{/if}

			<!-- Block Form (Progressive Disclosure) -->
			{#if showBlockForm}
				<div
					class="animate-in fade-in slide-in-from-top-2 space-y-3 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4 duration-200"
				>
					<h3 class="text-xs font-bold tracking-wide text-[#1C1917] uppercase">
						{$_('drawer.blockRoom')}
					</h3>
					<select
						bind:value={blockReason}
						class="w-full rounded-lg border border-[#E7E5E4] bg-white p-2.5 text-sm text-[#1C1917] outline-none focus:border-[#FF8C42] focus:ring-2 focus:ring-[#FF8C42]/30"
					>
						<option value="maintenance">{$_('drawer.maintenance')}</option>
						<option value="owner_use">{$_('drawer.ownerUse')}</option>
						<option value="out_of_service">{$_('drawer.outOfService')}</option>
					</select>
					<div class="grid grid-cols-2 gap-2">
						<input
							type="date"
							bind:value={blockStart}
							class="w-full rounded-lg border border-[#E7E5E4] bg-white p-2.5 text-sm text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
						/>
						<input
							type="date"
							bind:value={blockEnd}
							class="w-full rounded-lg border border-[#E7E5E4] bg-white p-2.5 text-sm text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
						/>
					</div>
				</div>
			{/if}
		</div>

		<div class="space-y-3 border-t border-[#E7E5E4] bg-[#FCFBFA] p-5">
			{#if showBlockForm}
				<!-- Block confirmation action -->
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
				<!-- Primary Action -->
				<button
					onclick={() =>
						primaryAction.action === 'assign'
							? loadPendingBookings()
							: onAction(primaryAction.action)}
					class="w-full py-3.5 {primaryAction.color} flex items-center justify-center gap-2 rounded-lg font-semibold text-white shadow-sm transition-all duration-200 hover:brightness-110 active:scale-95"
				>
					{$_(primaryAction.labelKey)}
				</button>

				<!-- Block Room option if available -->
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
