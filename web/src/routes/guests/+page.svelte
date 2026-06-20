<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import type { GuestListDTO, GuestDetail } from '$lib/types';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // UUID de prueba actual

	// --- ESTADOS PRINCIPALES ---
	let guests = $state<GuestListDTO[]>([]);
	let totalGuests = $state(0);
	let loading = $state(true);
	let searchQuery = $state('');
	let currentPage = $state(1);
	const limit = 20;

	// --- FORMULARIO DE EDICIÓN ---
	let isDrawerOpen = $state(false);
	let selectedGuest = $state<GuestDetail | null>(null);
	let drawerLoading = $state(false);
	let saving = $state(false);

	// Campos editables del Huésped
	let editFullName = $state('');
	let editPhone = $state('');
	let editEmail = $state('');
	let editIdNumber = $state('');
	let editNationality = $state('');
	let editNotes = $state('');

	// Timer de búsqueda (Debounce)
	let searchTimeout: ReturnType<typeof setTimeout> | undefined;

	// --- CARGA DE DATOS ---
	async function loadGuests() {
		loading = true;
		try {
			const res = await api.guests.list(propertyId, searchQuery, currentPage, limit);
			guests = res.guests || [];
			totalGuests = res.pagination?.total || guests.length;
		} catch (e) {
			console.error(e);
			addToast('Error al cargar huéspedes.', 'error');
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadGuests();
	});

	function handleSearchInput(val: string) {
		searchQuery = val;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			loadGuests();
		}, 300);
	}

	// --- ABRIR DETALLE HUÉSPED ---
	async function openGuestDetails(guestId: string) {
		isDrawerOpen = true;
		drawerLoading = true;
		selectedGuest = null;
		try {
			const detail = await api.guests.get(guestId);
			selectedGuest = detail;
			
			// Inicializar campos de edición
			editFullName = detail.full_name || '';
			editPhone = detail.phone || '';
			editEmail = detail.email || '';
			editIdNumber = detail.id_number || '';
			editNationality = detail.nationality || '';
			editNotes = detail.notes || '';
		} catch (e) {
			console.error(e);
			addToast('Error al cargar detalles del huésped.', 'error');
			isDrawerOpen = false;
		} finally {
			drawerLoading = false;
		}
	}

	// --- ACTUALIZAR DATOS HUÉSPED ---
	async function handleUpdateGuest(e: Event) {
		e.preventDefault();
		if (!selectedGuest) return;
		saving = true;
		try {
			const payload = {
				full_name: editFullName,
				phone: editPhone,
				email: editEmail || null,
				id_number: editIdNumber || null,
				nationality: editNationality || null,
				notes: editNotes || null
			};
			await api.guests.update(selectedGuest.id, payload);
			addToast('Perfil del huésped actualizado.', 'success');
			loadGuests();
			
			// Actualizar localmente el detalle
			selectedGuest = {
				...selectedGuest,
				...payload
			};
		} catch (e) {
			console.error(e);
			addToast('Error al guardar cambios.', 'error');
		} finally {
			saving = false;
		}
	}

	// --- DERIVAR CATEGORÍA DE FIDELIDAD (LOYALTY LEVEL) ---
	function getLoyaltyTier(visits: number) {
		if (visits >= 10) return { name: 'VIP ELITE', color: 'bg-[#FF8C42] text-white border-[#FF8C42]', text: 'Propietario / Cliente Recurrente' };
		if (visits >= 4) return { name: 'VIP PLATINUM', color: 'bg-[#FFF7ED] text-[#FF8C42] border-[#FF8C42]/20', text: 'Cliente Frecuente' };
		return { name: 'GUEST STANDARD', color: 'bg-[#F5F4F1] text-[#57534E] border-[#E7E5E4]', text: 'Registrado' };
	}

	function formatDateString(isoString: string | null) {
		if (!isoString) return 'Nunca';
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
			<h2 class="text-2xl font-bold text-[#1C1917]">Gestión de Huéspedes (CRM)</h2>
			<p class="text-sm text-[#57534E] mt-1">Directorio de huéspedes, fidelización de clientes e histórico de visitas.</p>
		</div>
		<span class="rounded-full bg-[#FFF7ED] px-3.5 py-1.5 text-xs font-bold text-[#FF8C42] border border-[#FF8C42]/20">
			✨ Módulo Activo
		</span>
	</div>

	<!-- Buscador y Métricas Rápidas -->
	<div class="grid grid-cols-1 md:grid-cols-4 gap-4">
		<!-- Search box -->
		<div class="md:col-span-2 rounded-xl border border-[#E7E5E4] bg-white p-4 shadow-sm flex items-center">
			<div class="relative w-full">
				<input
					type="text"
					placeholder="Buscar huésped por nombre, email, nacionalidad o teléfono..."
					value={searchQuery}
					oninput={(e) => handleSearchInput((e.target as HTMLInputElement).value)}
					class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] pl-9 pr-3 py-2 text-sm text-[#1C1917] focus:outline-none focus:border-[#FF8C42]"
				/>
				<span class="absolute left-3 top-2.5 text-sm text-[#57534E]">🔍</span>
			</div>
		</div>

		<!-- Quick Metric 1: Total Guests -->
		<div class="rounded-xl border border-[#E7E5E4] bg-white p-4 shadow-sm">
			<span class="text-[10px] font-bold text-[#A8A29E] tracking-wider uppercase">Huéspedes Registrados</span>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-1">{totalGuests}</p>
		</div>

		<!-- Quick Metric 2: Frequent customers -->
		<div class="rounded-xl border border-[#E7E5E4] bg-white p-4 shadow-sm">
			<span class="text-[10px] font-bold text-[#FF8C42] tracking-wider uppercase">Fidelización de Clientes</span>
			<p class="text-xs text-[#57534E] mt-2">Visitas registradas históricas vinculadas automáticamente.</p>
		</div>
	</div>

	<!-- Grid/Table List of Guests -->
	<div class="rounded-xl border border-[#E7E5E4] bg-white shadow-sm overflow-hidden">
		{#if loading}
			<div class="p-8 space-y-4">
				{#each Array(4) as _}
					<div class="h-12 w-full bg-[#F5F4F1] animate-pulse rounded-lg"></div>
				{/each}
			</div>
		{:else if guests.length === 0}
			<div class="p-12 text-center">
				<span class="text-4xl block mb-2">👥</span>
				<h4 class="font-bold text-[#1C1917]">No se encontraron perfiles</h4>
				<p class="text-xs text-[#57534E] mt-1">Introduce otros términos de búsqueda para encontrar registros.</p>
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left border-collapse text-xs md:text-sm">
					<thead>
						<tr class="bg-[#FCFBFA] border-b border-[#E7E5E4] text-[#57534E] font-bold">
							<th class="p-4">Huésped</th>
							<th class="p-4">Nacionalidad</th>
							<th class="p-4">Contacto</th>
							<th class="p-4 text-center">Visitas</th>
							<th class="p-4">Última Estancia</th>
							<th class="p-4">Nivel Fidelidad</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[#F5F4F1]">
						{#each guests as guest (guest.id)}
							{@const loyalty = getLoyaltyTier(guest.booking_count)}
							<tr
								onclick={() => openGuestDetails(guest.id)}
								class="hover:bg-[#FCFBFA] cursor-pointer transition"
							>
								<!-- Guest name -->
								<td class="p-4">
									<div class="flex items-center gap-3">
										<div class="h-8 w-8 rounded-full bg-[#1C1917]/10 flex items-center justify-center font-bold text-xs text-[#1C1917]">
											{guest.full_name.slice(0,2).toUpperCase()}
										</div>
										<div>
											<div class="font-semibold text-[#1C1917]">{guest.full_name}</div>
											<div class="text-[10px] text-[#A8A29E]">ID: {guest.id.slice(0,8)}</div>
										</div>
									</div>
								</td>
								<!-- Nationality -->
								<td class="p-4">
									<span class="rounded bg-[#F5F4F1] px-2 py-0.5 font-semibold text-[#57534E]">
										{guest.nationality || 'N/A'}
									</span>
								</td>
								<!-- Contact info -->
								<td class="p-4">
									<div class="text-[#1C1917] font-semibold">{guest.phone}</div>
									{#if guest.email}
										<div class="text-[10px] text-[#57534E]">{guest.email}</div>
									{/if}
								</td>
								<!-- Stays count -->
								<td class="p-4 text-center font-bold text-[#1C1917]">
									{guest.booking_count}
								</td>
								<!-- Last visit date -->
								<td class="p-4 text-[#57534E]">
									{formatDateString(guest.last_visit)}
								</td>
								<!-- Level badge -->
								<td class="p-4">
									<span class="px-2 py-0.5 rounded-full text-[9px] font-bold border {loyalty.color}">
										{loyalty.name}
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

<!-- CRM Guest Detail & Edit Drawer -->
{#if isDrawerOpen}
	<!-- Backdrop -->
	<button
		type="button"
		aria-label="Cerrar detalle del huesped"
		class="fixed inset-0 z-40 bg-[#1C1917]/20 backdrop-blur-[1px] cursor-default w-full text-left"
		onclick={() => isDrawerOpen = false}
	></button>

	<!-- Panel -->
	<div
		transition:fly={{ x: 300, duration: 250 }}
		class="fixed top-0 right-0 z-50 flex h-full w-full max-w-xl flex-col bg-[#FCFBFA] shadow-xl border-l border-[#E7E5E4]"
	>
		<!-- Header -->
		<div class="flex items-start justify-between border-b border-[#E7E5E4] px-6 py-4 bg-[#FCFBFA]">
			<div>
				<h3 class="text-lg font-bold text-[#1C1917]">Detalles del Huésped</h3>
				<p class="text-xs text-[#57534E]">Información histórica y CRM</p>
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
			{#if drawerLoading}
				<div class="space-y-4 animate-pulse">
					<div class="h-12 w-full bg-[#F5F4F1] rounded-xl"></div>
					<div class="h-40 w-full bg-[#F5F4F1] rounded-xl"></div>
				</div>
			{:else if selectedGuest}
				{@const loyalty = getLoyaltyTier(selectedGuest.total_bookings)}
				<!-- Loyalty Banner -->
				<div class="border rounded-xl p-4 flex items-center justify-between {loyalty.color}">
					<div>
						<p class="text-[10px] font-bold uppercase tracking-wider">Rango de Fidelización</p>
						<p class="text-base font-extrabold">{loyalty.name}</p>
						<p class="text-xs mt-1">{loyalty.text}</p>
					</div>
					<div class="text-right">
						<p class="text-[10px] font-semibold">Total Aportado (MVP)</p>
						<p class="text-lg font-bold tabular-nums">{formatCurrency(selectedGuest.total_revenue)}</p>
						<p class="text-xs">{selectedGuest.total_bookings} estancias completadas</p>
					</div>
				</div>

				<!-- Edit profile form -->
				<form onsubmit={handleUpdateGuest} class="space-y-4 bg-white border border-[#E7E5E4] rounded-xl p-4 shadow-sm">
					<h4 class="text-xs font-bold text-[#FF8C42] uppercase tracking-wider border-b border-[#F5F4F1] pb-2">Editar Ficha Demográfica</h4>
					
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="guest-detail-full-name" class="block text-xs font-semibold text-[#57534E] mb-1">Nombre Completo *</label>
							<input
								id="guest-detail-full-name"
								type="text"
								required
								bind:value={editFullName}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-xs text-[#1C1917] focus:outline-none"
							/>
						</div>
						<div>
							<label for="guest-detail-phone" class="block text-xs font-semibold text-[#57534E] mb-1">Teléfono de Contacto *</label>
							<input
								id="guest-detail-phone"
								type="text"
								required
								bind:value={editPhone}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-xs text-[#1C1917] focus:outline-none"
							/>
						</div>
						<div>
							<label for="guest-detail-email" class="block text-xs font-semibold text-[#57534E] mb-1">Email</label>
							<input
								id="guest-detail-email"
								type="email"
								bind:value={editEmail}
								class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-xs text-[#1C1917] focus:outline-none"
							/>
						</div>
						<div class="grid grid-cols-2 gap-2">
							<div>
								<label for="guest-detail-nationality" class="block text-xs font-semibold text-[#57534E] mb-1">Nacionalidad</label>
								<input
									id="guest-detail-nationality"
									type="text"
									placeholder="ESP, IDN..."
									bind:value={editNationality}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-xs text-[#1C1917] focus:outline-none"
								/>
							</div>
							<div>
								<label for="guest-detail-id-number" class="block text-xs font-semibold text-[#57534E] mb-1">Doc. Pasaporte</label>
								<input
									id="guest-detail-id-number"
									type="text"
									placeholder="Passport"
									bind:value={editIdNumber}
									class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-xs text-[#1C1917] focus:outline-none"
								/>
							</div>
						</div>
					</div>

					<div>
						<label for="guest-detail-notes" class="block text-xs font-semibold text-[#57534E] mb-1">Observaciones / Preferencias Generales</label>
						<textarea
							id="guest-detail-notes"
							bind:value={editNotes}
							placeholder="Notas sobre desayuno, almohadas, historial médico..."
							class="w-full rounded-lg border border-[#E7E5E4] bg-[#FCFBFA] px-3 py-2 text-xs text-[#1C1917] h-20 resize-none focus:outline-none"
						></textarea>
					</div>

					<div class="flex justify-end pt-2">
						<button
							type="submit"
							disabled={saving}
							class="rounded-lg bg-[#FF8C42] hover:bg-[#E06B20] text-white px-4 py-2 text-xs font-bold transition disabled:opacity-60"
						>
							{saving ? 'Guardando...' : 'Guardar Cambios'}
						</button>
					</div>
				</form>

				<!-- Stays History -->
				<div class="space-y-3">
					<h4 class="text-xs font-bold text-[#57534E] uppercase tracking-wider">Historial de Estancias</h4>
					{#if !selectedGuest.bookings || selectedGuest.bookings.length === 0}
						<p class="text-xs text-[#57534E] italic">Este huésped no tiene estancias previas completadas.</p>
					{:else}
						<div class="bg-white border border-[#E7E5E4] rounded-xl overflow-hidden shadow-sm">
							<table class="w-full text-left border-collapse text-xs">
								<thead>
									<tr class="bg-[#FCFBFA] border-b border-[#E7E5E4] text-[#57534E] font-bold">
										<th class="p-3">Periodo</th>
										<th class="p-3">Hab.</th>
										<th class="p-3">Estado</th>
										<th class="p-3 text-right">Monto</th>
									</tr>
								</thead>
								<tbody class="divide-y divide-[#F5F4F1]">
									{#each selectedGuest.bookings as visit (visit.id)}
										<tr>
											<td class="p-3 font-medium">
												{formatDateString(visit.check_in)} - {formatDateString(visit.check_out)}
											</td>
											<td class="p-3 font-semibold text-[#1C1917]">
												{visit.room_number ? `Hab. ${visit.room_number}` : 'Unassigned'}
											</td>
											<td class="p-3">
												<span class="capitalize font-semibold">{visit.status}</span>
											</td>
											<td class="p-3 text-right font-bold text-[#1C1917] tabular-nums">
												{formatCurrency(visit.total_amount)}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}
