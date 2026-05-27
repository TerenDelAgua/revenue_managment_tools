<script lang="ts">
	import type { RoomMap } from '$lib/types';

	interface Props {
		room: RoomMap | null;
		isOpen: boolean;
		onClose: () => void;
		onAction: (action: string, payload?: any) => void;
	}

	let { room, isOpen, onClose, onAction }: Props = $props();

	// Estado local para formularios inline (Progressive Disclosure)
	let showBlockForm = $state(false);
	let blockReason = $state('maintenance');
	let blockNote = $state('');
	let blockStart = $state(new Date().toISOString().split('T')[0]);
	let blockEnd = $state(new Date(Date.now() + 86400000).toISOString().split('T')[0]);

	// Mapeo de etiquetas legibles
	const statusLabels = {
		available: 'Disponible',
		occupied: 'Ocupada',
		pending: 'Pendiente',
		blocked: 'Bloqueada',
		inactive: 'Inactiva'
	};

	const statusColors = {
		available: 'bg-[#16A34A]',
		occupied: 'bg-[#DC2626]',
		pending: 'bg-[#D97706]',
		blocked: 'bg-[#44403C]',
		inactive: 'bg-[#A8A29E]'
	};

	function handleBlockSubmit() {
		onAction('block', {
			room_id: room?.id,
			reason: blockReason,
			notes: blockNote,
			start_date: blockStart,
			end_date: blockEnd
		});
		showBlockForm = false;
	}
</script>

{#if room}
	<!-- Backdrop -->
	<button
		type="button"
		aria-label="Close drawer"
		class="fixed inset-0 z-40 w-full border-none bg-[#1C1917]/30 p-0 backdrop-blur-sm transition-opacity duration-300 {isOpen
			? 'opacity-100'
			: 'pointer-events-none opacity-0'}"
		onclick={onClose}
	></button>

	<!-- Drawer Panel -->
	<div
		class="fixed top-0 right-0 z-50 flex h-full w-full max-w-md transform flex-col bg-[#FCFBFA] shadow-2xl transition-transform duration-300 ease-out {isOpen
			? 'translate-x-0'
			: 'translate-x-full'}"
		style="border-left: 1px solid #E7E5E4;"
	>
		<!-- Header -->
		<div class="flex items-start justify-between border-b border-[#E7E5E4] bg-[#FCFBFA] px-6 py-5">
			<div>
				<div class="mb-1 flex items-center gap-2">
					<h2 class="text-2xl font-bold tracking-tight text-[#1C1917]">{room.number}</h2>
					<span class="h-2.5 w-2.5 rounded-full {statusColors[room.availability]}"></span>
				</div>
				<div class="flex items-center gap-2">
					<span class="text-sm font-medium text-[#57534E]">{room.room_type.name}</span>
					<span class="h-1.5 w-1.5 rounded-full bg-[#D6D3D1]"></span>
					<span class="text-sm font-medium tracking-wide text-[#57534E] uppercase">
						{statusLabels[room.availability]}
					</span>
				</div>
			</div>
			<button
				title="Close"
				onclick={onClose}
				class="rounded-lg p-2 text-[#57534E] transition-colors hover:bg-[#F5F4F1] hover:text-[#1C1917]"
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="24"
					height="24"
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

		<!-- Content Scrollable -->
		<div class="flex-1 space-y-6 overflow-y-auto p-6">
			<!-- Info Card (Datos reales disponibles) -->
			<div class="rounded-xl border border-[#E7E5E4] bg-white p-4 shadow-sm">
				<h3 class="mb-3 text-xs font-bold tracking-wider text-[#57534E] uppercase">Room Details</h3>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<p class="text-xs text-[#57534E]">Type</p>
						<p class="text-sm font-semibold text-[#1C1917]">{room.room_type.name}</p>
					</div>
					<div>
						<p class="text-xs text-[#57534E]">Reference</p>
						<p class="font-mono text-sm font-semibold text-[#1C1917]">{room.id.slice(0, 8)}...</p>
					</div>
				</div>

				<!-- Estado de Booking (Si existe) -->
				{#if room.active_booking}
					<div
						class="mt-4 rounded-lg border border-t border-[#DC2626]/20 border-[#E7E5E4] bg-[#DC2626]/5 p-3 pt-4"
					>
						<p class="text-xs font-bold text-[#DC2626] uppercase">Checked In</p>
						<p class="mt-1 text-xs text-[#57534E]">
							Booking ID: {room.active_booking.slice(0, 8)}...
						</p>
					</div>
				{/if}

				{#if room.block}
					<div
						class="mt-4 rounded-lg border border-t border-[#44403C]/20 border-[#E7E5E4] bg-[#44403C]/5 p-3 pt-4"
					>
						<p class="text-xs font-bold text-[#44403C] uppercase">Blocked</p>
						<p class="mt-1 text-xs text-[#57534E]">Block ID: {room.block.slice(0, 8)}...</p>
					</div>
				{/if}
			</div>

			<!-- Block Inline Form (Progressive Disclosure) -->
			{#if showBlockForm}
				<div
					class="animate-in fade-in slide-in-from-top-2 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-5 duration-200"
				>
					<h3 class="mb-3 text-sm font-bold text-[#1C1917]">Block Room</h3>
					<div class="space-y-4">
						<div>
							<label for="blockReason" class="mb-1 block text-xs font-medium text-[#57534E]"
								>Reason</label
							>
							<select
								id="blockReason"
								bind:value={blockReason}
								class="w-full rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] transition-all outline-none focus:border-[#FF8C42] focus:ring-2 focus:ring-[#FF8C42]/30"
							>
								<option value="maintenance">Maintenance</option>
								<option value="owner_use">Owner Use</option>
								<option value="out_of_service">Out of Service</option>
							</select>
						</div>
						<div class="grid grid-cols-2 gap-3">
							<div>
								<label for="blockStart" class="mb-1 block text-xs font-medium text-[#57534E]"
									>Start Date</label
								>
								<input
									id="blockStart"
									type="date"
									bind:value={blockStart}
									class="w-full rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
								/>
							</div>
							<div>
								<label for="blockEnd" class="mb-1 block text-xs font-medium text-[#57534E]"
									>End Date</label
								>
								<input
									id="blockEnd"
									type="date"
									bind:value={blockEnd}
									class="w-full rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
								/>
							</div>
						</div>
						<div>
							<label for="blockNote" class="mb-1 block text-xs font-medium text-[#57534E]"
								>Notes (Optional)</label
							>
							<textarea
								id="blockNote"
								bind:value={blockNote}
								rows="2"
								placeholder="Why is this room blocked?"
								class="w-full resize-none rounded-lg border border-[#E7E5E4] bg-white p-3 text-[#1C1917] outline-none focus:ring-2 focus:ring-[#FF8C42]/30"
							></textarea>
						</div>
					</div>
				</div>
			{/if}
		</div>

		<!-- Footer Actions -->
		<div class="space-y-3 border-t border-[#E7E5E4] bg-[#FCFBFA] p-6">
			<!-- Primary Action Contextual -->
			{#if !showBlockForm}
				{#if room.availability === 'available'}
					<button
						onclick={() => onAction('assign')}
						class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#FF8C42] py-3.5 font-semibold text-white shadow-sm transition-all duration-200 hover:bg-[#E06B20] active:scale-95"
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
							><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"
							></path><rect x="8" y="2" width="8" height="4" rx="1" ry="1"></rect></svg
						>
						Assign Booking
					</button>
				{:else if room.availability === 'pending'}
					<button
						onclick={() => onAction('checkin')}
						class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#16A34A] py-3.5 font-semibold text-white shadow-sm transition-all duration-200 hover:bg-[#15803D] active:scale-95"
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
							><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline
								points="22 4 12 14.01 9 11.01"
							></polyline></svg
						>
						Check In
					</button>
				{:else if room.availability === 'occupied'}
					<button
						onclick={() => onAction('checkout')}
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
							><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path><polyline
								points="16 17 21 12 16 7"
							></polyline><line x1="21" y1="12" x2="9" y2="12"></line></svg
						>
						Check Out
					</button>
				{:else if room.availability === 'blocked'}
					<button
						onclick={() => onAction('unblock')}
						class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#DC2626] py-3.5 font-semibold text-white shadow-sm transition-all duration-200 hover:bg-[#B91C1C] active:scale-95"
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
							><circle cx="12" cy="12" r="10"></circle><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"
							></line></svg
						>
						Remove Block
					</button>
				{:else if room.availability === 'inactive'}
					<button
						onclick={() => onAction('activate')}
						class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#A8A29E] py-3.5 font-semibold text-white shadow-sm transition-all duration-200 hover:bg-[#78716C] active:scale-95"
					>
						Activate Room
					</button>
				{/if}
			{/if}

			<!-- Secondary Actions -->
			<div class="grid grid-cols-2 gap-3 pt-2">
				{#if room.availability !== 'blocked' && room.availability !== 'inactive' && !showBlockForm}
					<button
						onclick={() => (showBlockForm = true)}
						class="col-span-2 rounded-lg bg-[#F5F4F1] py-3 text-sm font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4]"
					>
						Block Room
					</button>
				{/if}
				{#if showBlockForm}
					<button
						onclick={() => (showBlockForm = false)}
						class="rounded-lg bg-[#F5F4F1] py-3 text-sm font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4]"
					>
						Cancel
					</button>
					<button
						onclick={handleBlockSubmit}
						class="rounded-lg bg-[#FF8C42] py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-[#E06B20] active:scale-95"
					>
						Confirm Block
					</button>
				{/if}
			</div>
		</div>
	</div>
{/if}
