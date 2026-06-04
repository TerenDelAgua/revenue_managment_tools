<script lang="ts">
	import { onMount } from 'svelte';
	import { fade, slide, fly } from 'svelte/transition';
	import { api } from '$lib/api/client';
	import { _ } from 'svelte-i18n';
	import { addToast } from '$lib/store/toastStore';
	import type { BookingDetail, RoomMap, GuestListDTO } from '$lib/types';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // UUID de prueba actual

	// --- ESTADOS PRINCIPALES ---
	let bookings = $state<BookingDetail[]>([]);
	let totalBookings = $state(0);
	let loading = $state(true);
	let activeTab = $state<'arrivals' | 'all' | 'pending'>('arrivals');
	let searchQuery = $state('');
	let currentPage = $state(1);
	const limit = 20;

	// --- ESTADO DEL WIDGET DE CREACIÓN ---
	let isFormOpen = $state(false);
	let formLoading = $state(false);
	let formError = $state<string | null>(null);

	// Campos de Reserva
	let checkIn = $state(new Date().toISOString().split('T')[0]);
	let checkOut = $state(new Date(Date.now() + 86400000).toISOString().split('T')[0]);
	let adults = $state(1);
	let children = $state(0);
	let originalAmount = $state<number>(0);
	let source = $state<'walk_in' | 'whatsapp' | 'phone' | 'booking_com' | 'airbnb' | 'agoda' | 'traveloka' | 'other'>('walk_in');
	let notes = $state('');
	let forceOverride = $state(false);

	// Campos de Huésped
	let guestName = $state('');
	let guestPhone = $state('');
	let guestEmail = $state('');
	let guestIdNumber = $state('');
	let guestNationality = $state('');
	let guestNotes = $state('');
	let selectedGuestId = $state<string | null>(null);

	// Búsqueda inteligente de Huésped
	let guestSearchQuery = $state('');
	let guestSearchResults = $state<GuestListDTO[]>([]);
	let showGuestSuggestions = $state(false);
	let searchTimeout: any;

	// Coincidencia parcial (Huésped existente detectado)
	let detectedGuest = $state<GuestListDTO | null>(null);

	// --- LISTA DE HABITACIONES PARA ASIGNAR ---
	let allRooms = $state<RoomMap[]>([]);
	let selectedRoomId = $state<string | null>(null);

	// --- ESTADO DEL DRAWER DE DETALLE ---
	let isDrawerOpen = $state(false);
	let selectedBooking = $state<BookingDetail | null>(null);
	let drawerActionLoading = $state(false);

	// Estado de cancelación
	let showCancelConfirm = $state(false);
	let cancelReason = $state('');

	// Re-asignación de habitación desde el Drawer
	let showRoomAssign = $state(false);
	let newRoomId = $state<string | null>(null);

	// --- DERIVADOS (SVELTE 5 RUNES) ---
	const filteredBookings = $derived.by(() => {
		let list = bookings;
		if (activeTab === 'arrivals') {
			list = list.filter(b => b.status === 'confirmed');
		} else if (activeTab === 'pending') {
			list = list.filter(b => b.room_id === null && b.status === 'confirmed');
		}
		return list;
	});

	const stayNights = $derived.by(() => {
		const start = new Date(checkIn);
		const end = new Date(checkOut);
		if (isNaN(start.getTime()) || isNaN(end.getTime())) return 0;
		return Math.max(1, Math.round((end.getTime() - start.getTime()) / 86400000));
	});

	// --- CARGA DE DATOS ---
	async function loadBookings() {
		loading = true;
		try {
			// El backend acepta filter status. Filtraremos en local o backend según sea necesario
			const statusFilter = ''; // Cargamos todos para poder alternar pestañas fluidamente
			const res = await api.bookings.list(propertyId, statusFilter, searchQuery, currentPage, limit);
			bookings = res.bookings || [];
			totalBookings = res.pagination?.total || bookings.length;
		} catch (e: any) {
			console.error(e);
			addToast($_('bookingsForm.toasts.loadError'), 'error');
		} finally {
			loading = false;
		}
	}

	async function loadRooms() {
		try {
			// Cargamos el mapa de hoy para obtener las habitaciones físicas
			const today = new Date().toISOString().split('T')[0];
			const tomorrow = new Date(Date.now() + 86400000).toISOString().split('T')[0];
			const mapData = await api.map.get(today, tomorrow, propertyId);
			if (mapData && mapData.floors) {
				const rooms: RoomMap[] = [];
				for (const floor of mapData.floors) {
					rooms.push(...floor.rooms);
				}
				allRooms = rooms;
			}
		} catch (e) {
			console.error('Error al cargar habitaciones', e);
		}
	}

	onMount(() => {
		loadBookings();
		loadRooms();
	});

	// --- BUSCADOR DE HUÉSPED CON DEBOUNCE ---
	function handleGuestContactInput(val: string, type: 'phone' | 'email') {
		if (type === 'phone') guestPhone = val;
		if (type === 'email') guestEmail = val;

		const query = val.trim();
		if (query.length < 3) {
			guestSearchResults = [];
			showGuestSuggestions = false;
			detectedGuest = null;
			return;
		}

		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(async () => {
			try {
				const res = await api.guests.list(propertyId, query, 1, 5);
				guestSearchResults = res.guests || [];
				showGuestSuggestions = guestSearchResults.length > 0;

				// Buscar coincidencia exacta para alerta visual sutil
				const match = guestSearchResults.find(g => 
					(g.phone && g.phone === guestPhone) || 
					(g.email && g.email.toLowerCase() === guestEmail.toLowerCase())
				);
				if (match && match.id !== selectedGuestId) {
					detectedGuest = match;
				} else {
					detectedGuest = null;
				}
			} catch (e) {
				console.error(e);
			}
		}, 300);
	}

	function linkGuest(guest: GuestListDTO) {
		selectedGuestId = guest.id;
		guestName = guest.full_name;
		guestPhone = guest.phone;
		guestEmail = guest.email || '';
		guestNationality = guest.nationality || '';
		
		guestSearchResults = [];
		showGuestSuggestions = false;
		detectedGuest = null;
		
		addToast($_('bookingsForm.toasts.guestLinked', { values: { name: guest.full_name } }), 'success');
	}

	function unlinkGuest() {
		selectedGuestId = null;
		guestName = '';
		guestPhone = '';
		guestEmail = '';
		guestIdNumber = '';
		guestNationality = '';
		guestNotes = '';
		detectedGuest = null;
		addToast($_('bookingsForm.toasts.guestUnlinked'), 'info');
	}

	// --- CREAR RESERVA ---
	async function handleCreateBooking(e: Event) {
		e.preventDefault();
		if (!guestName || !guestPhone) {
			addToast($_('bookingsForm.toasts.missingNamePhone'), 'error');
			return;
		}

		formLoading = true;
		try {
			const payload: any = {
				property_id: propertyId,
				room_id: selectedRoomId || null,
				check_in: `${checkIn}T14:00:00Z`,
				check_out: `${checkOut}T12:00:00Z`,
				adults,
				children,
				original_amount: Number(originalAmount),
				original_currency: 'IDR',
				exchange_rate: 1.0,
				total_amount: Number(originalAmount),
				source,
				notes,
				force_override: forceOverride
			};

			if (selectedGuestId) {
				payload.guest_id = selectedGuestId;
			} else {
				payload.guest = {
					property_id: propertyId,
					full_name: guestName,
					phone: guestPhone,
					email: guestEmail || null,
					id_number: guestIdNumber || null,
					nationality: guestNationality || null,
					notes: guestNotes || null
				};
			}

			const res = await api.bookings.create(payload);
			addToast(res.guest_reused ? $_('bookingsForm.toasts.bookingCreatedReused') : $_('bookingsForm.toasts.bookingCreatedSuccess'), 'success');
			
			// Reset form
			isFormOpen = false;
			resetForm();
			loadBookings();
		} catch (err: any) {
			console.error(err);
			const isConflict = err.status === 409 || err.message?.includes('ROOM_NOT_AVAILABLE') || err.message?.includes('BLOCK_CONFLICT') || err.message?.includes('ACTIVE_BOOKING_CONFLICT');
			if (isConflict) {
				formError = $_('bookingsForm.toasts.conflictError');
				addToast('⚠️ Conflicto de disponibilidad detectado.', 'error');
			} else {
				formError = err.status === 500 ? $_('bookingsForm.toasts.unexpectedError') : (err.message || $_('bookingsForm.toasts.unexpectedError'));
				addToast($_('bookingsForm.toasts.createError'), 'error');
			}
			
			// Scroll suave al banner de error (especificación TEREN 3.9)
			setTimeout(() => {
				const banner = document.getElementById('form-error-banner');
				if (banner) {
					banner.scrollIntoView({ behavior: 'smooth', block: 'center' });
				}
			}, 50);
		} finally {
			formLoading = false;
		}
	}

	function resetForm() {
		formError = null;
		selectedGuestId = null;
		guestName = '';
		guestPhone = '';
		guestEmail = '';
		guestIdNumber = '';
		guestNationality = '';
		guestNotes = '';
		originalAmount = 0;
		notes = '';
		selectedRoomId = null;
		forceOverride = false;
		detectedGuest = null;
	}

	// --- ACCIONES DRAWER ---
	function openBookingDetails(booking: BookingDetail) {
		selectedBooking = booking;
		showCancelConfirm = false;
		showRoomAssign = false;
		newRoomId = booking.room_id;
		isDrawerOpen = true;
	}

	async function handleCheckin() {
		if (!selectedBooking) return;
		drawerActionLoading = true;
		try {
			await api.bookings.checkin(selectedBooking.id, propertyId);
			addToast($_('bookingsForm.toasts.checkinSuccess'), 'success');
			refreshSelectedBooking();
			loadBookings();
		} catch (e: any) {
			addToast($_('bookingsForm.toasts.checkinError'), 'error');
		} finally {
			drawerActionLoading = false;
		}
	}

	async function handleCheckout() {
		if (!selectedBooking) return;
		drawerActionLoading = true;
		try {
			await api.bookings.checkout(selectedBooking.id, propertyId);
			addToast($_('bookingsForm.toasts.checkoutSuccess'), 'success');
			refreshSelectedBooking();
			loadBookings();
		} catch (e: any) {
			addToast($_('bookingsForm.toasts.checkoutError'), 'error');
		} finally {
			drawerActionLoading = false;
		}
	}

	async function handleAssignRoom() {
		if (!selectedBooking || !newRoomId) return;
		drawerActionLoading = true;
		try {
			await api.bookings.assign(selectedBooking.id, newRoomId, propertyId);
			addToast($_('bookingsForm.toasts.assignSuccess'), 'success');
			showRoomAssign = false;
			refreshSelectedBooking();
			loadBookings();
		} catch (e: any) {
			addToast($_('bookingsForm.toasts.assignError'), 'error');
		} finally {
			drawerActionLoading = false;
		}
	}

	async function handleCancelBooking() {
		if (!selectedBooking) return;
		if (!cancelReason.trim()) {
			addToast($_('bookingsForm.toasts.cancelMissingReason'), 'error');
			return;
		}
		drawerActionLoading = true;
		try {
			await api.bookings.cancel(selectedBooking.id, cancelReason);
			addToast($_('bookingsForm.toasts.cancelSuccess'), 'success');
			showCancelConfirm = false;
			cancelReason = '';
			refreshSelectedBooking();
			loadBookings();
		} catch (e: any) {
			addToast($_('bookingsForm.toasts.cancelError'), 'error');
		} finally {
			drawerActionLoading = false;
		}
	}

	async function refreshSelectedBooking() {
		if (!selectedBooking) return;
		try {
			const updated = await api.bookings.get(selectedBooking.id);
			selectedBooking = updated;
		} catch (e) {
			isDrawerOpen = false;
		}
	}

	function formatDateString(isoString: string) {
		if (!isoString) return '';
		return new Date(isoString).toLocaleDateString('es-ES', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	const formatCurrency = (amount: number) => {
		return `IDR ${amount.toLocaleString('id-ID', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`;
	};
</script>

<div class="flex flex-col gap-6 max-w-6xl mx-auto py-4">
	<!-- Page Header -->
	<div class="flex justify-between items-center">
		<div>
			<h2 class="text-2xl font-bold text-[#1C1917]">{$_('bookingsForm.title')}</h2>
			<p class="text-sm text-[#57534E] mt-1">{$_('bookingsForm.subtitle')}</p>
		</div>
		<button
			onclick={() => isFormOpen = !isFormOpen}
			class="rounded-xl bg-[#FF8C42] hover:bg-[#E06B20] text-white px-5 py-2.5 text-sm font-semibold transition active:scale-95 shadow-sm"
		>
			{isFormOpen ? $_('bookingsForm.cancelRegistration') : $_('bookingsForm.newBooking')}
		</button>
	</div>

	<!-- Inline Creation Form (Svelte 5 slide & fly) -->
	{#if isFormOpen}
		<div
			transition:slide={{ duration: 250 }}
			class="rounded-xl border border-[#FF8C42]/20 bg-white p-6 shadow-md"
		>
			<form onsubmit={handleCreateBooking} class="space-y-6">
				<h3 class="font-bold text-[#1C1917] text-lg border-b border-[#E7E5E4] pb-2">{$_('bookingsForm.registerNewBooking')}</h3>
				
				{#if formError}
					<div
						id="form-error-banner"
						transition:slide={{ duration: 200 }}
						class="bg-[#FEF2F2] border border-[#DC2626]/20 rounded-lg p-3 flex items-start gap-2.5"
					>
						<span class="text-[#DC2626] text-base shrink-0">⚠️</span>
						<div class="space-y-1">
							<p class="text-sm font-semibold text-[#DC2626]">{$_('bookingsForm.registrationFailed')}</p>
							<p class="text-xs text-[#DC2626]/90">{formError}</p>
						</div>
						<button
							type="button"
							onclick={() => formError = null}
							class="text-xs font-bold text-[#DC2626]/70 hover:text-[#DC2626] ml-auto"
						>
							{$_('bookingsForm.dismiss')}
						</button>
					</div>
				{/if}
				
				<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
					<!-- Guest CRM linking section -->
					<div class="space-y-4">
						<h4 class="text-xs font-bold text-[#FF8C42] uppercase tracking-wider">{$_('bookingsForm.guestInfo')}</h4>
						
						<!-- Phone search first -->
						<div>
							<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.mobilePhone')}</label>
							<div class="relative">
								<input
									type="tel"
									required
									pattern={"^\\+?[0-9\\s\\-]{6,20}$"}
									placeholder="{$_('bookingsForm.phonePlaceholder')}"
									value={guestPhone}
									oninput={(e) => handleGuestContactInput((e.target as HTMLInputElement).value, 'phone')}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								/>
								{#if selectedGuestId}
									<button
										type="button"
										onclick={unlinkGuest}
										class="absolute right-2.5 top-2 text-xs text-[#DC2626] font-semibold hover:underline"
									>
										{$_('bookingsForm.unlink')}
									</button>
								{/if}
							</div>
						</div>

						<!-- Guest suggestions popup -->
						{#if showGuestSuggestions && guestSearchResults.length > 0}
							<div class="bg-white border border-[#E7E5E4] rounded-xl shadow-lg p-2 max-h-40 overflow-y-auto space-y-1">
								<p class="text-[10px] font-bold text-[#A8A29E] px-2 py-1">{$_('bookingsForm.historicalGuests')}</p>
								{#each guestSearchResults as guest}
									<button
										type="button"
										onclick={() => linkGuest(guest)}
										class="w-full text-left px-3 py-1.5 text-xs rounded-lg hover:bg-[#FFF7ED] flex justify-between items-center transition"
									>
										<div>
											<span class="font-semibold text-[#1C1917]">{guest.full_name}</span>
											<span class="text-[#57534E] ml-2">({guest.phone})</span>
										</div>
										<span class="text-[10px] text-[#FF8C42] font-semibold">{$_('bookingsForm.link')}</span>
									</button>
								{/each}
							</div>
						{/if}

						<!-- Name -->
						<div>
							<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.fullName')}</label>
							<input
								type="text"
								required
								disabled={selectedGuestId !== null}
								placeholder="{$_('bookingsForm.fullNamePlaceholder')}"
								bind:value={guestName}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42] disabled:opacity-60"
							/>
						</div>

						<!-- Email -->
						<div>
							<label class="block text-xs font-semibold text-[#57534E] mb-1">Email</label>
							<input
								type="email"
								disabled={selectedGuestId !== null}
								placeholder="{$_('bookingsForm.emailPlaceholder')}"
								value={guestEmail}
								oninput={(e) => handleGuestContactInput((e.target as HTMLInputElement).value, 'email')}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42] disabled:opacity-60"
							/>
						</div>

						<div class="grid grid-cols-2 gap-4">
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.documentPassport')}</label>
								<input
									type="text"
									disabled={selectedGuestId !== null}
									placeholder="{$_('bookingsForm.idPlaceholder')}"
									bind:value={guestIdNumber}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42] disabled:opacity-60"
								/>
							</div>
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.nationality')}</label>
								<input
									type="text"
									disabled={selectedGuestId !== null}
									placeholder="{$_('bookingsForm.nationalityPlaceholder')}"
									bind:value={guestNationality}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42] disabled:opacity-60"
								/>
							</div>
						</div>

						<!-- CRM suggested link banner -->
						{#if detectedGuest}
							<div transition:fade class="bg-[#FFF7ED] border border-[#FF8C42]/20 p-3 rounded-lg flex items-center justify-between">
								<div class="text-xs text-[#57534E]">
									{$_('bookingsForm.existingProfilePrefix')} <strong>{detectedGuest.full_name}</strong> {$_('bookingsForm.existingProfileSuffix')}
								</div>
								<button
									type="button"
									onclick={() => linkGuest(detectedGuest!)}
									class="text-xs font-bold text-[#FF8C42] hover:underline shrink-0 ml-2"
								>
									{$_('bookingsForm.useProfile')}
								</button>
							</div>
						{/if}
					</div>

					<!-- Booking details -->
					<div class="space-y-4">
						<h4 class="text-xs font-bold text-[#FF8C42] uppercase tracking-wider">{$_('bookingsForm.stayDetails')}</h4>
						
						<div class="grid grid-cols-2 gap-4">
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">Check-in</label>
								<input
									type="date"
									required
									bind:value={checkIn}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								/>
							</div>
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">Check-out</label>
								<input
									type="date"
									required
									bind:value={checkOut}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								/>
							</div>
						</div>

						<div class="grid grid-cols-3 gap-2">
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.adults')}</label>
								<input
									type="number"
									min="1"
									required
									bind:value={adults}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								/>
							</div>
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.children')}</label>
								<input
									type="number"
									min="0"
									required
									bind:value={children}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								/>
							</div>
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.nights')}</label>
								<div class="w-full bg-[#F5F4F1] border border-[#E7E5E4] rounded-lg px-3 py-2 text-sm text-[#57534E] text-center font-semibold">
									{$_('bookingsForm.nightsCount', { values: { count: stayNights } })}
								</div>
							</div>
						</div>

						<div class="grid grid-cols-2 gap-4">
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.physicalRoom')}</label>
								<select
									bind:value={selectedRoomId}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2.5 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								>
									<option value={null}>{$_('bookingsForm.unassigned')}</option>
									{#each allRooms as room}
										<option value={room.id}>{$_('bookingsForm.roomNumber', { values: { number: room.number } })} ({room.room_type.name})</option>
									{/each}
								</select>
							</div>
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.bookingSource')}</label>
								<select
									bind:value={source}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2.5 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
								>
									<option value="walk_in">{$_('bookingsForm.sources.walk_in')}</option>
									<option value="whatsapp">{$_('bookingsForm.sources.whatsapp')}</option>
									<option value="phone">{$_('bookingsForm.sources.phone')}</option>
									<option value="booking_com">{$_('bookingsForm.sources.booking_com')}</option>
									<option value="airbnb">{$_('bookingsForm.sources.airbnb')}</option>
									<option value="agoda">{$_('bookingsForm.sources.agoda')}</option>
									<option value="traveloka">{$_('bookingsForm.sources.traveloka')}</option>
									<option value="other">{$_('bookingsForm.sources.other')}</option>
								</select>
							</div>
						</div>

						<div class="grid grid-cols-1 gap-2">
							<div>
								<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.originalAmount', { values: { count: stayNights } })}</label>
								<div class="relative">
									<span class="absolute left-3 top-2 text-sm text-[#57534E] font-bold">IDR</span>
									<input
										type="number"
										required
										min="0"
										bind:value={originalAmount}
										class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] pl-12 pr-3 py-2 text-sm text-[#1C1917] font-semibold tabular-nums focus:outline-none focus:border-[#FF8C42]"
									/>
								</div>
							</div>
						</div>

						<div>
							<label class="block text-xs font-semibold text-[#57534E] mb-1">{$_('bookingsForm.observations')}</label>
							<textarea
								placeholder="{$_('bookingsForm.observationsPlaceholder')}"
								bind:value={notes}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42] h-16 resize-none"
							></textarea>
						</div>

						<!-- Force override option -->
						<div class="flex items-center gap-2 pt-2">
							<input type="checkbox" id="force" bind:checked={forceOverride} class="rounded border-[#E7E5E4] text-[#FF8C42] focus:ring-[#FF8C42]" />
							<label for="force" class="text-xs text-[#57534E] font-medium cursor-pointer selection:bg-transparent">
								{$_('bookingsForm.forceOverride')}
							</label>
						</div>
					</div>
				</div>

				<div class="flex justify-end gap-3 pt-4 border-t border-[#E7E5E4]">
					<button
						type="button"
						onclick={() => isFormOpen = false}
						class="rounded-xl border border-[#E7E5E4] hover:bg-[#F5F4F1] text-[#1C1917] px-5 py-3 text-sm font-semibold transition active:scale-95"
					>
						Cancelar
					</button>
					<button
						type="submit"
						disabled={formLoading}
						class="rounded-xl bg-[#FF8C42] hover:bg-[#E06B20] text-white px-6 py-3 text-sm font-semibold transition active:scale-95 shadow-sm disabled:opacity-60"
					>
						{formLoading ? $_('bookingsForm.registering') : $_('bookingsForm.confirmBooking')}
					</button>
				</div>
			</form>
		</div>
	{/if}

	<!-- Pestañas de Filtro -->
	<div class="flex border-b border-[#E7E5E4]">
		<button
			onclick={() => { activeTab = 'arrivals'; currentPage = 1; }}
			class="px-6 py-3 text-sm font-bold border-b-2 transition-all {activeTab === 'arrivals' ? 'border-[#FF8C42] text-[#FF8C42]' : 'border-transparent text-[#57534E] hover:text-[#1C1917]'}"
		>
			{$_('bookingsForm.tabs.arrivals')}
		</button>
		<button
			onclick={() => { activeTab = 'all'; currentPage = 1; }}
			class="px-6 py-3 text-sm font-bold border-b-2 transition-all {activeTab === 'all' ? 'border-[#FF8C42] text-[#FF8C42]' : 'border-transparent text-[#57534E] hover:text-[#1C1917]'}"
		>
			{$_('bookingsForm.tabs.all')}
		</button>
		<button
			onclick={() => { activeTab = 'pending'; currentPage = 1; }}
			class="px-6 py-3 text-sm font-bold border-b-2 transition-all {activeTab === 'pending' ? 'border-[#FF8C42] text-[#FF8C42]' : 'border-transparent text-[#57534E] hover:text-[#1C1917]'}"
		>
			{$_('bookingsForm.tabs.pending')}
		</button>
	</div>

	<!-- Listado de Reservas -->
	<div class="rounded-xl border border-[#E7E5E4] bg-white shadow-sm overflow-hidden">
		<!-- Buscador e Info -->
		<div class="p-4 border-b border-[#E7E5E4] flex flex-col sm:flex-row items-center justify-between gap-4 bg-[#FCFBFA]">
			<div class="relative w-full sm:max-w-xs">
				<input
					type="text"
					placeholder="{$_('bookingsForm.searchPlaceholder')}"
					value={searchQuery}
					oninput={(e) => { searchQuery = (e.target as HTMLInputElement).value; currentPage = 1; loadBookings(); }}
					class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] pl-9 pr-3 py-1.5 text-xs text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
				/>
				<span class="absolute left-3 top-2.5 text-xs text-[#57534E]">🔍</span>
			</div>
			<div class="text-xs text-[#57534E] font-medium">
				{$_('bookingsForm.showingBookings')} <span class="text-[#1C1917] font-bold">{filteredBookings.length}</span> {$_('bookingsForm.bookingsCount')}
			</div>
		</div>

		{#if loading}
			<div class="p-8 space-y-4">
				{#each Array(4) as _}
					<div class="h-12 w-full bg-[#F5F4F1] animate-pulse rounded-lg"></div>
				{/each}
			</div>
		{:else if filteredBookings.length === 0}
			<div class="p-12 text-center">
				<span class="text-4xl block mb-2">📭</span>
				<h4 class="font-bold text-[#1C1917]">{$_('bookingsForm.noBookingsFound')}</h4>
				<p class="text-xs text-[#57534E] mt-1">{$_('bookingsForm.noBookingsHint')}</p>
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left border-collapse text-xs md:text-sm">
					<thead>
						<tr class="bg-[#FCFBFA] border-b border-[#E7E5E4] text-[#57534E] font-bold">
							<th class="p-4">{$_('bookingsForm.columns.mainGuest')}</th>
							<th class="p-4">{$_('bookingsForm.columns.room')}</th>
							<th class="p-4">{$_('bookingsForm.columns.dates')}</th>
							<th class="p-4 text-right">{$_('bookingsForm.columns.amount')}</th>
							<th class="p-4">{$_('bookingsForm.columns.source')}</th>
							<th class="p-4">{$_('bookingsForm.columns.status')}</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[#F5F4F1]">
						{#each filteredBookings as booking}
							<tr
								onclick={() => openBookingDetails(booking)}
								class="hover:bg-[#FCFBFA] cursor-pointer transition"
							>
								<!-- Guest name -->
								<td class="p-4">
									<div class="flex items-center gap-3">
										<div class="h-8 w-8 rounded-full bg-[#FF8C42]/10 flex items-center justify-center font-bold text-xs text-[#FF8C42]">
											{booking.guest_name.slice(0,2).toUpperCase()}
										</div>
										<div>
											<div class="font-semibold text-[#1C1917]">{booking.guest_name}</div>
											<div class="text-[10px] text-[#57534E]">{booking.guest_phone}</div>
										</div>
									</div>
								</td>
								<!-- Room number -->
								<td class="p-4 font-semibold text-[#1C1917]">
									{#if booking.room_id}
										<span>Hab. {booking.room_number}</span>
										<div class="text-[10px] text-[#57534E] font-normal">{booking.room_type_name}</div>
									{:else}
										<span class="text-[#DC2626] font-semibold bg-[#DC2626]/10 px-2 py-0.5 rounded">{$_('bookingsForm.badges.unassigned')}</span>
									{/if}
								</td>
								<!-- Dates -->
								<td class="p-4">
									<div class="font-semibold text-[#1C1917]">
										{formatDateString(booking.check_in)} - {formatDateString(booking.check_out)}
									</div>
									<div class="text-[10px] text-[#57534E]">
										{Math.max(1, Math.round((new Date(booking.check_out).getTime() - new Date(booking.check_in).getTime()) / 86400000))} noches
									</div>
								</td>
								<!-- Amount -->
								<td class="p-4 text-right font-bold text-[#1C1917] tabular-nums">
									{formatCurrency(booking.total_amount)}
								</td>
								<!-- Source -->
								<td class="p-4 text-[#57534E]">
									<span class="capitalize">{booking.source.replace('_', ' ')}</span>
								</td>
								<!-- Status badge -->
								<td class="p-4">
									<span
										class="px-2.5 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider
										{booking.status === 'confirmed' ? 'bg-[#FFF7ED] text-[#FF8C42] border border-[#FF8C42]/20' : ''}
										{booking.status === 'checked_in' ? 'bg-[#DCFCE7] text-[#16A34A] border border-[#16A34A]/20' : ''}
										{booking.status === 'checked_out' ? 'bg-[#F5F4F1] text-[#57534E] border border-[#E7E5E4]' : ''}
										{booking.status === 'cancelled' || booking.status === 'no_show' ? 'bg-red-50 text-red-600 border border-red-200' : ''}"
									>
										{booking.status === 'confirmed' ? 'Confirmada' : ''}
										{booking.status === 'checked_in' ? 'Check In' : ''}
										{booking.status === 'checked_out' ? 'Check Out' : ''}
										{booking.status === 'cancelled' ? 'Cancelada' : ''}
										{booking.status === 'no_show' ? 'No Show' : ''}
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>

<!-- Drawer de Detalle Lateral (Svelte 5 Drawer) -->
{#if isDrawerOpen && selectedBooking}
	<!-- Backdrop -->
	<button
		type="button"
		class="fixed inset-0 z-40 bg-[#1C1917]/20 backdrop-blur-[1px] cursor-default w-full text-left"
		onclick={() => isDrawerOpen = false}
	></button>

	<!-- Panel -->
	<div
		transition:fly={{ x: 300, duration: 250 }}
		class="fixed top-0 right-0 z-50 flex h-full w-full max-w-lg flex-col bg-[#FCFBFA] shadow-xl border-l border-[#E7E5E4]"
	>
		<!-- Header -->
		<div class="flex items-start justify-between border-b border-[#E7E5E4] px-6 py-4 bg-[#FCFBFA]">
			<div>
				<div class="flex items-center gap-2">
					<h3 class="text-lg font-bold text-[#1C1917]">Reserva #{selectedBooking.id.slice(0, 8).toUpperCase()}</h3>
					<span
						class="px-2 py-0.5 rounded text-[9px] font-bold uppercase
						{selectedBooking.status === 'confirmed' ? 'bg-[#FFF7ED] text-[#FF8C42]' : ''}
						{selectedBooking.status === 'checked_in' ? 'bg-[#DCFCE7] text-[#16A34A]' : ''}
						{selectedBooking.status === 'checked_out' ? 'bg-[#F5F4F1] text-[#57534E]' : ''}
						{selectedBooking.status === 'cancelled' ? 'bg-red-50 text-red-600' : ''}"
					>
						{selectedBooking.status === 'confirmed' ? 'Confirmada' : ''}
						{selectedBooking.status === 'checked_in' ? 'Huésped Hospedado' : ''}
						{selectedBooking.status === 'checked_out' ? 'Finalizada (Check-out)' : ''}
						{selectedBooking.status === 'cancelled' ? 'Cancelada' : ''}
					</span>
				</div>
				<p class="text-xs text-[#57534E]">Registrado por {selectedBooking.created_by_name}</p>
			</div>
			<button
				onclick={() => isDrawerOpen = false}
				class="rounded-lg p-2 text-[#57534E] hover:bg-[#F5F4F1] hover:text-[#1C1917] transition"
			>
				✕
			</button>
		</div>

		<!-- Scrollable content -->
		<div class="flex-1 overflow-y-auto p-6 space-y-6">
			<!-- Dates & Room Card -->
			<div class="bg-white border border-[#E7E5E4] rounded-xl p-4 shadow-sm grid grid-cols-2 gap-4">
				<div>
					<p class="text-[10px] font-bold text-[#FF8C42] uppercase">Fecha de Entrada</p>
					<p class="text-sm font-bold text-[#1C1917]">{formatDateString(selectedBooking.check_in)}</p>
					<p class="text-[10px] text-[#57534E]">14:00 Check-in</p>
				</div>
				<div>
					<p class="text-[10px] font-bold text-[#FF8C42] uppercase">Fecha de Salida</p>
					<p class="text-sm font-bold text-[#1C1917]">{formatDateString(selectedBooking.check_out)}</p>
					<p class="text-[10px] text-[#57534E]">12:00 Check-out</p>
				</div>
				<div class="col-span-2 border-t border-[#F5F4F1] pt-3 flex justify-between items-center">
					<div>
						<p class="text-[10px] font-bold text-[#FF8C42] uppercase">Habitación Asignada</p>
						<p class="text-sm font-bold text-[#1C1917]">
							{selectedBooking.room_id ? `Habitación ${selectedBooking.room_number}` : 'Sin Asignar'}
						</p>
						<p class="text-[10px] text-[#57534E]">{selectedBooking.room_type_name || '-'}</p>
					</div>
					{#if selectedBooking.status === 'confirmed'}
						<button
							onclick={() => showRoomAssign = !showRoomAssign}
							class="text-xs font-semibold bg-[#FFF7ED] text-[#FF8C42] border border-[#FF8C42]/20 px-3 py-1.5 rounded-lg hover:bg-[#FF8C42]/10"
						>
							{showRoomAssign ? 'Cerrar' : 'Cambiar/Asignar'}
						</button>
					{/if}
				</div>
			</div>

			<!-- Room Assignment sub-form inside Drawer -->
			{#if showRoomAssign}
				<div transition:slide class="bg-[#FCFBFA] border border-[#E7E5E4] rounded-xl p-4 shadow-inner space-y-3">
					<p class="text-xs font-bold text-[#1C1917]">Seleccionar Habitación</p>
					<div class="flex gap-2">
						<select
							bind:value={newRoomId}
							class="flex-1 rounded-lg border border-[#E7E5E4] bg-white px-3 py-2 text-xs text-[#1C1917] focus:outline-none"
						>
							<option value={null}>[Dejar Sin Asignar]</option>
							{#each allRooms as room}
								<option value={room.id}>Hab. {room.number} - {room.room_type.name}</option>
							{/each}
						</select>
						<button
							onclick={handleAssignRoom}
							disabled={drawerActionLoading}
							class="rounded-lg bg-[#FF8C42] hover:bg-[#E06B20] text-white px-4 py-2 text-xs font-bold transition disabled:opacity-60"
						>
							Guardar
						</button>
					</div>
				</div>
			{/if}

			<!-- Guest CRM Profile details -->
			<div class="bg-[#F5F4F1] border border-[#E7E5E4] rounded-xl p-4 space-y-3">
				<h4 class="text-[10px] font-bold text-[#57534E] uppercase tracking-wider">Perfil del Huésped</h4>
				<div class="flex items-center gap-3">
					<div class="h-10 w-10 rounded-full bg-[#1C1917] text-white flex items-center justify-center font-bold">
						{selectedBooking.guest_name.slice(0,2).toUpperCase()}
					</div>
					<div>
						<h5 class="text-sm font-bold text-[#1C1917]">{selectedBooking.guest_name}</h5>
						<p class="text-xs text-[#57534E]">{selectedBooking.guest_phone} · {selectedBooking.guest_email || 'Sin Email'}</p>
					</div>
				</div>
				<div class="grid grid-cols-2 gap-2 text-xs pt-2 border-t border-[#E7E5E4]">
					<div>
						<span class="text-[#57534E]">Nacionalidad:</span>
						<span class="font-semibold ml-1">{selectedBooking.guest_nationality || 'No especificada'}</span>
					</div>
					<div>
						<span class="text-[#57534E]">Identificación:</span>
						<span class="font-semibold ml-1">{selectedBooking.guest_id_number || 'No especificada'}</span>
					</div>
				</div>
			</div>

			<!-- Financial details -->
			<div class="bg-white border border-[#E7E5E4] rounded-xl p-4 shadow-sm space-y-3">
				<h4 class="text-[10px] font-bold text-[#FF8C42] uppercase tracking-wider">Detalle Financiero (MVP)</h4>
				<div class="flex justify-between items-center text-sm border-b border-[#F5F4F1] pb-2">
					<span class="text-[#57534E]">Importe Alojamiento</span>
					<span class="font-bold text-[#1C1917] tabular-nums">{formatCurrency(selectedBooking.total_amount)}</span>
				</div>
				<div class="flex justify-between items-center text-sm pt-1">
					<span class="text-[#57534E]">Estado de Pago:</span>
					<span
						class="px-2 py-0.5 rounded text-[10px] font-bold uppercase
						{selectedBooking.payment_status === 'paid' ? 'bg-[#DCFCE7] text-[#16A34A]' : ''}
						{selectedBooking.payment_status === 'pending' ? 'bg-red-50 text-red-600' : ''}
						{selectedBooking.payment_status === 'partial' ? 'bg-amber-50 text-amber-600' : ''}"
					>
						{selectedBooking.payment_status === 'paid' ? 'Pagado' : ''}
						{selectedBooking.payment_status === 'pending' ? 'Pendiente' : ''}
						{selectedBooking.payment_status === 'partial' ? 'Pago Parcial' : ''}
					</span>
				</div>
			</div>

			<!-- Notes & Observations -->
			{#if selectedBooking.notes}
				<div class="bg-white border border-[#E7E5E4] rounded-xl p-4 shadow-sm">
					<h4 class="text-[10px] font-bold text-[#57534E] uppercase tracking-wider mb-2">Notas Especiales</h4>
					<p class="text-xs text-[#1C1917] whitespace-pre-wrap">{selectedBooking.notes}</p>
				</div>
			{/if}

			<!-- Action Confirmations (Cancel booking dialog) -->
			{#if showCancelConfirm}
				<div transition:slide class="bg-red-50 border border-red-200 rounded-xl p-4 space-y-3">
					<p class="text-xs font-bold text-red-800">Motivo de Cancelación</p>
					<textarea
						required
						placeholder="Escribe la razón de la cancelación..."
						bind:value={cancelReason}
						class="w-full bg-white border border-red-200 rounded-lg p-2 text-xs text-[#1C1917] h-16 resize-none focus:outline-none focus:border-red-500"
					></textarea>
					<div class="flex justify-end gap-2">
						<button
							type="button"
							onclick={() => showCancelConfirm = false}
							class="px-3 py-1.5 rounded-lg border border-red-200 hover:bg-red-100 text-red-800 text-xs font-semibold"
						>
							Volver
						</button>
						<button
							type="button"
							onclick={handleCancelBooking}
							disabled={drawerActionLoading}
							class="px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-700 text-white text-xs font-bold transition disabled:opacity-60"
						>
							Confirmar Cancelación
						</button>
					</div>
				</div>
			{/if}
		</div>

		<!-- Action Footer -->
		<div class="border-t border-[#E7E5E4] p-6 bg-[#FCFBFA] space-y-3">
			{#if !showCancelConfirm}
				{#if selectedBooking.status === 'confirmed'}
					<button
						onclick={handleCheckin}
						disabled={drawerActionLoading || !selectedBooking.room_id}
						class="w-full bg-[#16A34A] hover:bg-[#15803D] text-white py-3.5 font-bold rounded-xl transition shadow-sm active:scale-95 disabled:opacity-60 flex justify-center items-center gap-2"
					>
						🔔 Realizar Check-In
					</button>
					{#if !selectedBooking.room_id}
						<p class="text-[10px] text-red-600 text-center font-semibold">Debes asignar una habitación antes de realizar el Check-in</p>
					{/if}
				{/if}

				{#if selectedBooking.status === 'checked_in'}
					<button
						onclick={handleCheckout}
						disabled={drawerActionLoading}
						class="w-full bg-[#1C1917] hover:bg-[#3F3D38] text-white py-3.5 font-bold rounded-xl transition shadow-sm active:scale-95 disabled:opacity-60"
					>
						🔑 Realizar Check-Out (Físico)
					</button>
				{/if}

				{#if selectedBooking.status !== 'cancelled' && selectedBooking.status !== 'checked_out'}
					<button
						onclick={() => showCancelConfirm = true}
						class="w-full bg-red-50 hover:bg-red-100 text-red-600 py-2.5 text-xs font-bold rounded-xl border border-red-200 transition"
					>
						✕ Cancelar Reserva
					</button>
				{/if}
			{/if}
		</div>
	</div>
{/if}
