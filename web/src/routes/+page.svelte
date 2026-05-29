<script lang="ts">
	import { onMount } from 'svelte';
	import { addToast } from '$lib/store/toastStore';
	import type { ReportResponse, DailyBreakdownResponse } from '$lib/types';
	import { api } from '$lib/api/client';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // UUID de prueba actual

	let metrics = $state<ReportResponse | null>(null);
	let dailyData = $state<DailyBreakdownResponse | null>(null);
	let loading = $state(true);

	function triggerFeatureAlert(name: string) {
		addToast(`La funcionalidad '${name}' se encuentra en desarrollo como parte de la Fase 2.`, 'info');
	}

	onMount(async () => {
		const to = new Date();
		const from = new Date();
		from.setDate(from.getDate() - 30);
		
		const dateTo = to.toISOString().split('T')[0];
		const dateFrom = from.toISOString().split('T')[0];
		
		try {
			const [metricsRes, dailyRes] = await Promise.all([
				api.reports.metrics(propertyId, dateFrom, dateTo),
				api.reports.daily(propertyId, dateFrom, dateTo)
			]);

			metrics = metricsRes;
			dailyData = dailyRes;
		} catch (e) {
			console.error('Failed to load dashboard data', e);
		} finally {
			loading = false;
		}
	});

	const formatCurrency = (amount: number) => {
		return `IDR ${amount.toLocaleString('id-ID', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`;
	};
</script>

{#snippet sparkline(data: number[], color: string)}
	<svg viewBox="0 0 100 30" class="w-full h-8 mt-2" preserveAspectRatio="none">
		{#if data.length > 1}
			{@const max = Math.max(...data, 1)}
			{@const points = data.map((d, i) => `${(i / (data.length - 1)) * 100},${30 - (d / max) * 30}`).join(' ')}
			<polyline points={points} fill="none" stroke={color} stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
		{/if}
	</svg>
{/snippet}

<div class="flex flex-col gap-6 max-w-6xl mx-auto py-4">


	<!-- Header Area -->
	<div class="flex flex-wrap items-center justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold tracking-tight text-[#1C1917]">Executive Dashboard</h2>
			<p class="text-sm text-[#57534E] mt-1">Real-time intelligence and property operations at a glance.</p>
		</div>
		<div class="flex gap-2 bg-[#FCFBFA] border border-[#E7E5E4] rounded-xl p-1.5 shadow-sm">
			<button class="rounded-lg bg-[#F5F4F1] px-3.5 py-1.5 text-xs font-semibold text-[#1C1917] transition hover:brightness-95">
				Last 30 Days
			</button>
			<button class="rounded-lg px-3.5 py-1.5 text-xs font-semibold text-[#57534E] transition hover:bg-[#F5F4F1]">
				This Week
			</button>
		</div>
	</div>

	<!-- Stats Grid (TEREN Design System) -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Occupancy</span>
				<span class="text-lg">🛏️</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">{loading ? '...' : (metrics?.occupancy_rate?.toFixed(1) || '0')}%</p>
			{#if dailyData && dailyData.days.length > 0}
				{@render sparkline(dailyData.days.map(d => d.occupancy_rate), '#16A34A')}
			{:else}
				<div class="h-8 mt-2"></div>
			{/if}
		</div>

		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">RevPAR</span>
				<span class="text-lg">💲</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">{loading ? '...' : formatCurrency(metrics?.revpar || 0)}</p>
			{#if dailyData && dailyData.days.length > 0}
				{@render sparkline(dailyData.days.map(d => d.revpar), '#3B82F6')}
			{:else}
				<div class="h-8 mt-2"></div>
			{/if}
		</div>

		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Average Daily Rate</span>
				<span class="text-lg">📈</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">{loading ? '...' : formatCurrency(metrics?.adr || 0)}</p>
			{#if dailyData && dailyData.days.length > 0}
				{@render sparkline(dailyData.days.map(d => d.adr), '#8B5CF6')}
			{:else}
				<div class="h-8 mt-2"></div>
			{/if}
		</div>

		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Active Blocks</span>
				<span class="text-lg">🔧</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">0 rooms</p>
			<p class="text-xs text-[#D97706] mt-1 font-medium">No blocks found</p>
		</div>
	</div>

	<!-- Operations & Builder Section -->
	<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-2">
		<!-- Core Feature Card: Floor Map Builder -->
		<div class="lg:col-span-2 rounded-xl border border-[#FF8C42]/30 bg-white p-6 shadow-md flex flex-col justify-between">
			<div>
				<div class="flex justify-between items-center">
					<span class="px-2.5 py-1 rounded bg-[#FFF7ED] text-[10px] font-bold text-[#FF8C42] tracking-wider uppercase">
						Active Module
					</span>
					<span class="text-xs text-[#A8A29E]">Fase 1 Release</span>
				</div>
				<h3 class="text-xl font-bold text-[#1C1917] mt-4">Floor Map & Layout Designer</h3>
				<p class="text-sm text-[#57534E] mt-2 leading-relaxed">
					Design, manage, and scale your hotel layout in an advanced grid system. View real-time room availability, toggle between operations and configuration mode, and set room blocks directly.
				</p>
			</div>

			<div class="flex flex-wrap gap-3 mt-6 pt-6 border-t border-[#E7E5E4]">
				<a
					href="/map?mode=setup"
					class="rounded-xl bg-[#FF8C42] hover:bg-[#E06B20] text-white px-5 py-3 text-sm font-semibold transition active:scale-[0.98] shadow-sm"
				>
					📐 Design Floor Layout
				</a>
				<a
					href="/map"
					class="rounded-xl border border-[#E7E5E4] hover:bg-[#F5F4F1] text-[#1C1917] px-5 py-3 text-sm font-semibold transition active:scale-[0.98]"
				>
					🟢 Operations View
				</a>
			</div>
		</div>

		<!-- Upcoming Features (CRM/Pricing Panel) -->
		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-6 shadow-sm flex flex-col justify-between">
			<div>
				<h4 class="font-bold text-[#1C1917] border-b border-[#E7E5E4] pb-3">Planned Features</h4>
				
				<div class="space-y-4 mt-4">
					<button
						onclick={() => triggerFeatureAlert('Dynamic Pricing Engine')}
						class="w-full text-left group flex items-start gap-3 rounded-xl p-3 border border-transparent hover:border-[#E7E5E4] hover:bg-[#FCFBFA] transition"
					>
						<span class="text-xl">📈</span>
						<div>
							<div class="flex items-center gap-1.5">
								<span class="text-sm font-bold text-[#1C1917]">Dynamic Pricing</span>
								<span class="px-1.5 py-0.5 rounded bg-[#F5F4F1] text-[9px] font-bold text-[#57534E]">PHASE 2</span>
							</div>
							<p class="text-xs text-[#57534E] mt-0.5 leading-snug">Algorithmic rate pricing engine.</p>
						</div>
					</button>

					<button
						onclick={() => triggerFeatureAlert('Reservations & Bookings Terminal')}
						class="w-full text-left group flex items-start gap-3 rounded-xl p-3 border border-transparent hover:border-[#E7E5E4] hover:bg-[#FCFBFA] transition"
					>
						<span class="text-xl">📅</span>
						<div>
							<div class="flex items-center gap-1.5">
								<span class="text-sm font-bold text-[#1C1917]">Bookings Terminal</span>
								<span class="px-1.5 py-0.5 rounded bg-[#F5F4F1] text-[9px] font-bold text-[#57534E]">PHASE 2</span>
							</div>
							<p class="text-xs text-[#57534E] mt-0.5 leading-snug">Check-in and front desk operations.</p>
						</div>
					</button>

					<button
						onclick={() => triggerFeatureAlert('Guest Intelligence Hub')}
						class="w-full text-left group flex items-start gap-3 rounded-xl p-3 border border-transparent hover:border-[#E7E5E4] hover:bg-[#FCFBFA] transition"
					>
						<span class="text-xl">👥</span>
						<div>
							<div class="flex items-center gap-1.5">
								<span class="text-sm font-bold text-[#1C1917]">Guest Intelligence</span>
								<span class="px-1.5 py-0.5 rounded bg-[#F5F4F1] text-[9px] font-bold text-[#57534E]">PHASE 2</span>
							</div>
							<p class="text-xs text-[#57534E] mt-0.5 leading-snug">Loyalty directory and preferences database.</p>
						</div>
					</button>
				</div>
			</div>

			<p class="text-[10px] text-[#A8A29E] text-center mt-6">
				TEREN Revenue Suite · Crafting Soulful Hotel Systems.
			</p>
		</div>
	</div>
</div>
