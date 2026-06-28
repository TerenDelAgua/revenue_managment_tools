<!--
	/reports — Daily cash-closing & Monthly tax report (B8)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.9 / §4.11

	Two tabs in a single page. No modal, no slide-in: the user picks
	the report type and the right component renders below. Property id
	is the same UUID used across the app (dev seed) — production will
	source it from a session/store.
-->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import DailySummary from '$lib/components/reports/DailySummary.svelte';
	import TaxReport from '$lib/components/reports/TaxReport.svelte';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // dev seed

	type Tab = 'daily' | 'tax';
	let activeTab = $state<Tab>('daily');
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6 py-4">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-teren-text-main">
				{$_('reports.title')}
			</h1>
			<p class="mt-1 text-sm text-teren-text-muted">
				{$_('reports.subtitle')}
			</p>
		</div>
	</header>

	<!-- Tabs -->
	<div class="flex gap-1 rounded-xl border border-teren-border-subtle bg-teren-background-base p-1 shadow-sm" role="tablist">
		<button
			role="tab"
			aria-selected={activeTab === 'daily'}
			onclick={() => (activeTab = 'daily')}
			class="flex-1 rounded-lg px-3 py-2 text-sm font-semibold transition-all cursor-pointer
				{activeTab === 'daily'
					? 'bg-white text-teren-primary shadow-sm'
					: 'text-teren-text-muted hover:text-teren-text-main'}"
			data-testid="tab-daily"
		>
			{$_('reports.tabs.daily')}
		</button>
		<button
			role="tab"
			aria-selected={activeTab === 'tax'}
			onclick={() => (activeTab = 'tax')}
			class="flex-1 rounded-lg px-3 py-2 text-sm font-semibold transition-all cursor-pointer
				{activeTab === 'tax'
					? 'bg-white text-teren-primary shadow-sm'
					: 'text-teren-text-muted hover:text-teren-text-main'}"
			data-testid="tab-tax"
		>
			{$_('reports.tabs.tax')}
		</button>
	</div>

	{#if activeTab === 'daily'}
		<DailySummary {propertyId} />
	{:else}
		<TaxReport {propertyId} />
	{/if}
</div>