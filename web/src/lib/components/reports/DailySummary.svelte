<!--
	DailySummary.svelte
	TEREN Hotels — Daily cash-closing report (B8)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.9

	Cash-closing payload. Shows the day's invoice counts (paid/partial/
	unpaid/void/overpaid), financial totals (collected/refunded/pending
	+ tax collected), breakdown by payment method, and the per-staff
	attribution. CSV export covers the whole report.
-->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import { toCSV, downloadCSV } from '$lib/util/csv';
	import type { DailySummary, PaymentMethod } from '$lib/types';

	interface Props {
		propertyId: string;
	}

	let { propertyId }: Props = $props();

	// === State ===
	function todayISO(): string {
		return new Date().toISOString().split('T')[0];
	}
	let dateStr = $state(todayISO());
	let summary = $state<DailySummary | null>(null);
	let loading = $state(false);
	let loadError = $state<string | null>(null);

	// === Lifecycle ===
	$effect(() => {
		// Refetch on date or propertyId change.
		if (dateStr) void loadSummary(dateStr);
	});

	async function loadSummary(d: string) {
		loading = true;
		loadError = null;
		try {
			summary = await api.invoices.dailySummary(propertyId, d);
		} catch (e: any) {
			loadError = e?.message ?? 'error';
			summary = null;
		} finally {
			loading = false;
		}
	}

	// === Helpers ===
	function formatMoney(value: number, currency = 'IDR'): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `${currency} ${grouped}`;
	}

	function methodLabel(m: PaymentMethod): string {
		return $_(`invoiceWidget.payments.method.${m}`);
	}

	const methodRows = $derived.by(() => {
		if (!summary) return [] as Array<[PaymentMethod, number]>;
		return Object.entries(summary.by_method ?? {}) as Array<[PaymentMethod, number]>;
	});

	function exportCsv() {
		if (!summary) return;
		const s = summary;
		const rows: (string | number)[][] = [];
		// Counts
		rows.push([$_('reports.daily.csv.invoicesIssued'), s.invoices_issued]);
		rows.push([$_('reports.daily.csv.invoicesPaid'), s.invoices_paid]);
		rows.push([$_('reports.daily.csv.invoicesPartial'), s.invoices_partial]);
		rows.push([$_('reports.daily.csv.invoicesUnpaid'), s.invoices_unpaid]);
		rows.push([$_('reports.daily.csv.invoicesOverpaid'), s.invoices_overpaid]);
		rows.push([$_('reports.daily.csv.invoicesVoid'), s.invoices_void]);
		rows.push([]);
		// Totals
		rows.push([$_('reports.daily.csv.totalRevenue'), s.total_revenue]);
		rows.push([$_('reports.daily.csv.totalCollected'), s.total_collected]);
		rows.push([$_('reports.daily.csv.totalRefunded'), s.total_refunded]);
		rows.push([$_('reports.daily.csv.totalPending'), s.total_pending]);
		rows.push([$_('reports.daily.csv.taxCollected'), s.tax_collected]);
		rows.push([]);
		// By method
		rows.push([$_('reports.daily.csv.methodHeader')]);
		rows.push([$_('reports.daily.csv.method'), $_('reports.daily.csv.amount')]);
		for (const [m, amount] of methodRows) {
			rows.push([methodLabel(m), amount]);
		}
		rows.push([]);
		// Staff
		rows.push([$_('reports.daily.csv.staffHeader')]);
		rows.push([$_('reports.daily.csv.staffName'), $_('reports.daily.csv.paymentsCount'), $_('reports.daily.csv.amountCollected')]);
		for (const member of s.staff_breakdown ?? []) {
			rows.push([member.user_name, member.payments_count, member.amount_collected]);
		}

		const csv = toCSV(
			[
				$_('reports.daily.csv.metric'),
				$_('reports.daily.csv.value')
			],
			rows
		);
		downloadCSV(`daily-summary-${s.date.split('T')[0]}.csv`, csv);
		addToast($_('reports.toasts.exported'), 'success');
	}
</script>

<section class="rounded-xl border border-teren-border-subtle bg-white p-5 shadow-sm" data-testid="daily-summary">
	<!-- Date selector -->
	<header class="mb-5 flex flex-wrap items-end justify-between gap-3">
		<div>
			<h2 class="text-base font-bold text-teren-text-main">{$_('reports.daily.title')}</h2>
			<p class="text-xs text-teren-text-muted">{$_('reports.daily.subtitle')}</p>
		</div>
		<div class="flex flex-wrap items-end gap-2">
			<label class="block">
				<span class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">{$_('reports.daily.dateLabel')}</span>
				<input
					type="date"
					bind:value={dateStr}
					class="mt-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary tabular-nums"
					data-testid="daily-date-input"
				/>
			</label>
			<button
				type="button"
				disabled={!summary || loading}
				onclick={exportCsv}
				class="rounded-lg bg-teren-primary px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				data-testid="daily-export-csv"
			>
				⬇ {$_('reports.daily.export')}
			</button>
		</div>
	</header>

	{#if loadError}
		<p class="text-sm text-teren-error-base">{$_('reports.errors.loadFailed')}</p>
	{:else if loading && !summary}
		<p class="text-sm text-teren-text-muted">{$_('reports.loading')}</p>
	{:else if summary}
		<!-- Counts grid -->
		<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
			{#each [
				{ key: 'issued', labelKey: 'reports.daily.counts.issued', value: summary.invoices_issued, tone: 'neutral' },
				{ key: 'paid', labelKey: 'reports.daily.counts.paid', value: summary.invoices_paid, tone: 'success' },
				{ key: 'partial', labelKey: 'reports.daily.counts.partial', value: summary.invoices_partial, tone: 'warning' },
				{ key: 'unpaid', labelKey: 'reports.daily.counts.unpaid', value: summary.invoices_unpaid, tone: 'error' },
				{ key: 'overpaid', labelKey: 'reports.daily.counts.overpaid', value: summary.invoices_overpaid, tone: 'warning' },
				{ key: 'void', labelKey: 'reports.daily.counts.void', value: summary.invoices_void, tone: 'muted' }
			] as cell (cell.key)}
				<div class="rounded-lg border border-teren-border-subtle bg-teren-background-base p-3 text-center">
					<p class="text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">
						{$_(cell.labelKey)}
					</p>
					<p
						class="mt-1 text-2xl font-bold tabular-nums
						{cell.tone === 'success' ? 'text-teren-success-hover dark:text-teren-success-base' : ''}
						{cell.tone === 'warning' ? 'text-teren-warning-hover dark:text-teren-warning-base' : ''}
						{cell.tone === 'error' ? 'text-teren-error-hover dark:text-teren-error-base' : ''}
						{cell.tone === 'muted' ? 'text-teren-text-muted line-through' : ''}
						{cell.tone === 'neutral' ? 'text-teren-text-main' : ''}"
						data-testid="daily-count-{cell.key}"
					>
						{cell.value}
					</p>
				</div>
			{/each}
		</div>

		<!-- Financials -->
		<dl class="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.daily.financials.collected')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-success-hover dark:text-teren-success-base" data-testid="daily-collected">
					{formatMoney(summary.total_collected)}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.daily.financials.refunded')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-error-base" data-testid="daily-refunded">
					−{formatMoney(summary.total_refunded)}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.daily.financials.pending')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-text-main" data-testid="daily-pending">
					{formatMoney(summary.total_pending)}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.daily.financials.tax')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-text-main" data-testid="daily-tax">
					{formatMoney(summary.tax_collected)}
				</dd>
			</div>
		</dl>

		<!-- Method breakdown -->
		<div class="mt-5">
			<h3 class="text-xs font-bold uppercase tracking-wider text-teren-text-muted">
				{$_('reports.daily.byMethod')}
			</h3>
			{#if methodRows.length === 0}
				<p class="mt-2 text-sm text-teren-text-muted">{$_('reports.daily.noPayments')}</p>
			{:else}
				<ul class="mt-2 space-y-1.5" data-testid="daily-by-method">
					{#each methodRows as [m, amount] (m)}
						<li class="flex items-baseline justify-between rounded-md bg-teren-background-base px-3 py-1.5">
							<span class="text-sm font-medium text-teren-text-main">{methodLabel(m)}</span>
							<span class="tabular-nums text-sm font-semibold text-teren-text-main">
								{formatMoney(amount)}
							</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<!-- Staff breakdown -->
		<div class="mt-5">
			<h3 class="text-xs font-bold uppercase tracking-wider text-teren-text-muted">
				{$_('reports.daily.byStaff')}
			</h3>
			{#if (summary.staff_breakdown ?? []).length === 0}
				<p class="mt-2 text-sm text-teren-text-muted">{$_('reports.daily.noStaff')}</p>
			{:else}
				<table class="mt-2 w-full text-sm" data-testid="daily-by-staff">
					<thead>
						<tr class="text-left text-xs uppercase tracking-wider text-teren-text-muted">
							<th class="pb-1">{$_('reports.daily.csv.staffName')}</th>
							<th class="pb-1 text-right">{$_('reports.daily.csv.paymentsCount')}</th>
							<th class="pb-1 text-right">{$_('reports.daily.csv.amountCollected')}</th>
						</tr>
					</thead>
					<tbody>
						{#each summary.staff_breakdown as member (member.user_id)}
							<tr class="border-t border-teren-background-base">
								<td class="py-1.5 text-teren-text-main">{member.user_name}</td>
								<td class="py-1.5 text-right tabular-nums text-teren-text-muted">{member.payments_count}</td>
								<td class="py-1.5 text-right tabular-nums font-semibold text-teren-text-main">
									{formatMoney(member.amount_collected)}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>