<!--
	TaxReport.svelte
	TEREN Hotels — Monthly PPN tax report (B8)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.11

	Aggregated tax report for a given month (or whole year). Shows the
	subtotal, tax collected, invoice count, refunds, and net tax
	collected. CSV export covers the same payload.
-->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import { toCSV, downloadCSV } from '$lib/util/csv';
	import type { MonthlyTaxReport } from '$lib/types';

	interface Props {
		propertyId: string;
	}

	let { propertyId }: Props = $props();

	// === State ===
	const now = new Date();
	let year = $state(now.getFullYear());
	let month = $state<number | 'all'>(now.getMonth() + 1);
	let report = $state<MonthlyTaxReport | null>(null);
	let loading = $state(false);
	let loadError = $state<string | null>(null);

	$effect(() => {
		// Refetch on year/month/property change.
		if (year) void loadReport(year, month === 'all' ? undefined : (month as number));
	});

	async function loadReport(y: number, m?: number) {
		loading = true;
		loadError = null;
		try {
			report = await api.invoices.taxReport(propertyId, y, m);
		} catch (e: any) {
			loadError = e?.message ?? 'error';
			report = null;
		} finally {
			loading = false;
		}
	}

	function formatMoney(value: number, currency = 'IDR'): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `${currency} ${grouped}`;
	}

	const monthLabel = $derived.by(() => {
		if (month === 'all') return $_('reports.tax.wholeYear');
		// 1-12 → use Date to format a localised month name. We rely on
		// the user's current locale for the name itself.
		const d = new Date(year, (month as number) - 1, 1);
		return d.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
	});

	function exportCsv() {
		if (!report) return;
		const r = report;
		const rows: (string | number)[][] = [
			[$_('reports.tax.csv.period'), monthLabel],
			[$_('reports.tax.csv.invoicesCount'), r.invoices_count],
			[$_('reports.tax.csv.totalSubtotal'), r.total_subtotal],
			[$_('reports.tax.csv.totalTax'), r.total_tax],
			[$_('reports.tax.csv.refundsTotal'), r.refunds_total],
			[$_('reports.tax.csv.voidCount'), r.void_count],
			[$_('reports.tax.csv.netTaxCollected'), r.net_tax_collected]
		];

		const csv = toCSV([$_('reports.tax.csv.metric'), $_('reports.tax.csv.value')], rows);
		const slug = month === 'all' ? `${r.year}` : `${r.year}-${String(r.month).padStart(2, '0')}`;
		downloadCSV(`tax-report-${slug}.csv`, csv);
		addToast($_('reports.toasts.exported'), 'success');
	}

	function yearOptions(): number[] {
		const out: number[] = [];
		for (let y = now.getFullYear(); y >= now.getFullYear() - 4; y--) out.push(y);
		return out;
	}
</script>

<section class="rounded-xl border border-teren-border-subtle bg-white p-5 shadow-sm" data-testid="tax-report">
	<header class="mb-5 flex flex-wrap items-end justify-between gap-3">
		<div>
			<h2 class="text-base font-bold text-teren-text-main">{$_('reports.tax.title')}</h2>
			<p class="text-xs text-teren-text-muted">{$_('reports.tax.subtitle')}</p>
		</div>
		<div class="flex flex-wrap items-end gap-2">
			<label class="block">
				<span class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">{$_('reports.tax.yearLabel')}</span>
				<select
					bind:value={year}
					class="mt-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary tabular-nums"
					data-testid="tax-year-select"
				>
					{#each yearOptions() as y (y)}
						<option value={y}>{y}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">{$_('reports.tax.monthLabel')}</span>
				<select
					bind:value={month}
					class="mt-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
					data-testid="tax-month-select"
				>
					<option value="all">{$_('reports.tax.wholeYear')}</option>
					{#each Array.from({ length: 12 }, (_, i) => i + 1) as m (m)}
						<option value={m}>{new Date(2000, m - 1, 1).toLocaleDateString(undefined, { month: 'long' })}</option>
					{/each}
				</select>
			</label>
			<button
				type="button"
				disabled={!report || loading}
				onclick={exportCsv}
				class="rounded-lg bg-teren-primary px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				data-testid="tax-export-csv"
			>
				⬇ {$_('reports.tax.export')}
			</button>
		</div>
	</header>

	{#if loadError}
		<p class="text-sm text-teren-error-base">{$_('reports.errors.loadFailed')}</p>
	{:else if loading && !report}
		<p class="text-sm text-teren-text-muted">{$_('reports.loading')}</p>
	{:else if report}
		<!-- v1.2 R-09 Q2: same ⚠ banner as the daily summary when
		     the period contains any needs_review invoices. The tax
		     totals below deliberately exclude those rows from
		     aggregations (BR-INV-011). -->
		{#if report.needs_review_count > 0}
			<div
				class="mb-3 flex items-start gap-2 rounded-lg border border-teren-warning-base/30 bg-teren-warning-subtle px-3 py-2 text-xs text-teren-text-main"
				data-testid="tax-needs-review-banner"
				role="alert"
			>
				<span class="text-sm leading-none" aria-hidden="true">⚠</span>
				<span>
					{$_('reports.daily.needsReviewBanner', {
						values: { count: report.needs_review_count }
					})}
				</span>
			</div>
		{/if}
		<p class="mb-3 text-sm text-teren-text-muted" data-testid="tax-period-label">
			{monthLabel}
		</p>

		<dl class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.tax.metrics.invoicesCount')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-text-main" data-testid="tax-invoices-count">
					{report.invoices_count}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.tax.metrics.totalSubtotal')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-text-main" data-testid="tax-total-subtotal">
					{formatMoney(report.total_subtotal)}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.tax.metrics.totalTax')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-success-hover dark:text-teren-success-base" data-testid="tax-total-tax">
					{formatMoney(report.total_tax)}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.tax.metrics.refundsTotal')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-error-base" data-testid="tax-refunds">
					−{formatMoney(report.refunds_total)}
				</dd>
			</div>
			<!-- v1.2 R-08: count of fully-refunded invoices in the period. -->
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.tax.metrics.refundedCount')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-error-base" data-testid="tax-refunded-count">
					{report.refunded_count}
				</dd>
			</div>
			<div class="rounded-lg border border-teren-border-subtle bg-white p-4">
				<dt class="text-xs text-teren-text-muted">{$_('reports.tax.metrics.voidCount')}</dt>
				<dd class="mt-1 text-lg font-bold tabular-nums text-teren-text-muted" data-testid="tax-void-count">
					{report.void_count}
				</dd>
			</div>
			<div class="rounded-lg border-2 border-teren-primary/30 bg-teren-primary/5 p-4">
				<dt class="text-xs font-semibold text-teren-primary">{$_('reports.tax.metrics.netTax')}</dt>
				<dd class="mt-1 text-xl font-extrabold tabular-nums text-teren-primary" data-testid="tax-net">
					{formatMoney(report.net_tax_collected)}
				</dd>
			</div>
		</dl>
	{/if}
</section>