<script lang="ts">
	import { onMount } from 'svelte';
	import type { ReportResponse } from '$lib/types';
	import { api } from '$lib/api/client';

	interface Props {
		propertyId: string;
		dateFrom: string;
		dateTo: string;
	}

	let { propertyId, dateFrom, dateTo }: Props = $props();

	let metrics = $state<ReportResponse | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showDetails = $state(false);

	async function fetchMetrics() {
		if (!propertyId || !dateFrom || !dateTo) {
			error = 'Missing required parameters';
			loading = false;
			return;
		}

		loading = true;
		error = null;

		try {
			metrics = await api.reports.metrics(propertyId, dateFrom, dateTo);
		} catch (e: any) {
			console.error('[OccupancyBar] Failed to fetch metrics:', e);
			error = e.message || 'Unable to load revenue metrics';
			metrics = null;
		} finally {
			loading = false;
		}
	}

	// Refetch when dates change
	$effect(() => {
		if (propertyId && dateFrom && dateTo) {
			fetchMetrics();
		}
	});

	// Format helpers
	const formatCurrency = (amount: number) => {
		return `IDR ${amount.toLocaleString('id-ID', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`;
	};

	const formatPercent = (value: number) => {
		return `${value.toFixed(1)}%`;
	};
</script>

<div class="border-b border-[#E7E5E4] bg-[#FCFBFA] p-4">
	{#if loading}
		<!-- Skeleton Loading -->
		<div class="flex animate-pulse flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex items-center gap-4">
				<div class="h-10 w-24 rounded bg-[#E7E5E4]"></div>
				<div class="h-10 w-32 rounded bg-[#E7E5E4]"></div>
			</div>
			<div class="flex gap-3">
				<div class="h-16 w-32 rounded-lg bg-[#E7E5E4]"></div>
				<div class="h-16 w-32 rounded-lg bg-[#E7E5E4]"></div>
			</div>
		</div>
	{:else if error}
		<!-- Error State -->
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2 text-[#DC2626]">
				<span class="text-lg">⚠️</span>
				<span class="text-sm font-medium">{error}</span>
			</div>
			<button
				onclick={fetchMetrics}
				class="rounded-lg bg-[#F5F4F1] px-3 py-1.5 text-sm font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4]"
			>
				Retry
			</button>
		</div>
	{:else if metrics}
		<!-- Data Display -->
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<!-- Left: Occupancy Stats -->
			<div class="flex items-center gap-4">
				<div class="flex items-baseline">
					<span class="mr-1 text-4xl font-bold text-[#1C1917] tabular-nums">
						{metrics.occupancy_rate.toFixed(0)}
					</span>
					<span class="text-sm text-[#57534E]">%</span>
				</div>

				<div class="h-10 w-px bg-[#E7E5E4]"></div>

				<div class="flex items-center gap-2">
					<span class="text-lg font-medium text-[#1C1917] tabular-nums">
						{Math.round(metrics.booked_nights / (metrics.days_in_range || 1))}
					</span>
					<span class="text-sm text-[#57534E]">/</span>
					<span class="text-lg font-medium text-[#57534E] tabular-nums">
						{metrics.total_rooms}
					</span>
					<span class="text-sm text-[#57534E]">rooms</span>
				</div>
			</div>

			<!-- Center: RevPAR & ADR -->
			<div class="flex items-center gap-3">
				<div class="min-w-[140px] rounded-lg border border-[#FF8C42]/30 bg-[#FFF7ED] p-2.5">
					<div class="text-xs tracking-wide text-[#57534E] uppercase">RevPAR</div>
					<div class="text-lg font-bold text-[#1C1917] tabular-nums">
						{formatCurrency(metrics.revpar)}
					</div>
				</div>

				<div class="min-w-[140px] rounded-lg border border-[#0EA5E9]/30 bg-[#F0F9FF] p-2.5">
					<div class="text-xs tracking-wide text-[#57534E] uppercase">ADR</div>
					<div class="text-lg font-bold text-[#1C1917] tabular-nums">
						{formatCurrency(metrics.adr)}
					</div>
				</div>
			</div>

			<!-- Right: Toggle Details -->
			<div class="flex items-center gap-2">
				<button
					onclick={() => (showDetails = !showDetails)}
					class="rounded-lg bg-[#F5F4F1] px-3 py-1.5 text-sm font-medium text-[#57534E] transition-colors hover:bg-[#E7E5E4]"
				>
					{showDetails ? 'Hide Details' : 'Show Details'}
				</button>
			</div>
		</div>

		<!-- Detailed View (Progressive Disclosure) -->
		{#if showDetails}
			<div
				class="animate-in fade-in slide-in-from-top-2 mt-4 rounded-xl border border-[#E7E5E4] bg-[#F5F4F1] p-4 duration-200"
			>
				<div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">Occupancy Rate</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">
							{formatPercent(metrics.occupancy_rate)}
						</p>
						<p class="mt-1 text-[11px] text-[#57534E]">
							{metrics.booked_nights} room-nights
						</p>
					</div>

					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">Total Rooms</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">
							{metrics.total_rooms}
						</p>
						<p class="mt-1 text-[11px] text-[#57534E]">
							{metrics.days_in_range} days
						</p>
					</div>

					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">RevPAR</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">
							{formatCurrency(metrics.revpar)}
						</p>
						<p class="mt-1 text-[11px] text-[#57534E]">Revenue per available room</p>
					</div>

					<div class="rounded-lg border border-[#E7E5E4] bg-white p-3 shadow-sm">
						<p class="mb-1 text-xs tracking-wide text-[#57534E] uppercase">ADR</p>
						<p class="text-2xl font-bold text-[#1C1917] tabular-nums">
							{formatCurrency(metrics.adr)}
						</p>
						<p class="mt-1 text-[11px] text-[#57534E]">Average daily rate</p>
					</div>
				</div>

				<!-- Date Range Info -->
				<div
					class="mt-4 border-t border-[#E7E5E4] pt-4 text-xs text-[#57534E]"
				>
					<span>Period: {metrics.date_from} → {metrics.date_to}</span>
				</div>
			</div>
		{/if}
	{/if}
</div>
