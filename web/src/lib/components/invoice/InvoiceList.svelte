<!--
	InvoiceList.svelte
	TEREN Hotels — Invoice list with filters + pagination (B9)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.7

	Table view of all invoices for a property with:
	- status filter (effective_status, all)
	- date range (date_from / date_to)
	- search by invoice number / guest / room
	- 50-row pagination
	- row click → opens a detail drawer via the `onSelect` callback

	The component is dumb: it only fetches and renders. All write actions
	(register payment, void, regenerate PDF) live in the InvoiceWidget
	embedded in the drawer. The list never blocks on row updates — the
	drawer handles its own refetch and the list reloads on user demand
	via the refresh button.
-->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import { toCSV, downloadCSV } from '$lib/util/csv';
	import type {
		InvoiceSummary,
		ListInvoicesFilter,
		PaymentStatus
	} from '$lib/types';

	interface Props {
		propertyId: string;
		/** Click handler — receives the selected summary, parent decides UI. */
		onSelect?: (invoice: InvoiceSummary) => void;
	}

	let { propertyId, onSelect }: Props = $props();

	// === Filter state ===
	const STATUSES: Array<{ value: 'all' | PaymentStatus; labelKey: string }> = [
		{ value: 'all', labelKey: 'invoicesList.statusAll' },
		{ value: 'paid', labelKey: 'invoicesList.statusPaid' },
		{ value: 'partial', labelKey: 'invoicesList.statusPartial' },
		{ value: 'unpaid', labelKey: 'invoicesList.statusUnpaid' },
		{ value: 'overpaid', labelKey: 'invoicesList.statusOverpaid' },
		{ value: 'void', labelKey: 'invoicesList.statusVoid' },
		// v1.2 (R-08) — refunded joins the filterable list per spec §5.4.
		{ value: 'refunded', labelKey: 'invoicesList.statusRefunded' }
	];

	let statusFilter = $state<'all' | PaymentStatus>('all');
	let dateFrom = $state('');
	let dateTo = $state('');
	let search = $state('');
	let page = $state(1);
	const limit = 50;

	// === Data state ===
	let invoices = $state<InvoiceSummary[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let loadError = $state<string | null>(null);

	const totalPages = $derived(Math.max(1, Math.ceil(total / limit)));

	// === Refetch on any filter change (debounced via $effect dependencies) ===
	$effect(() => {
		if (!propertyId) return;
		void loadInvoices();
	});

	async function loadInvoices() {
		loading = true;
		loadError = null;
		try {
			const filter: ListInvoicesFilter = {
				property_id: propertyId,
				page,
				limit
			};
			if (statusFilter !== 'all') filter.status = statusFilter;
			if (dateFrom) filter.date_from = dateFrom;
			if (dateTo) filter.date_to = dateTo;
			if (search.trim()) filter.search = search.trim();

			const res = await api.invoices.list(filter);
			invoices = res.invoices;
			total = res.pagination.total;
		} catch (e: any) {
			loadError = e?.message ?? 'error';
			invoices = [];
			total = 0;
		} finally {
			loading = false;
		}
	}

	function clearFilters() {
		statusFilter = 'all';
		dateFrom = '';
		dateTo = '';
		search = '';
		page = 1;
	}

	function goToPage(p: number) {
		if (p < 1 || p > totalPages) return;
		page = p;
	}

	/** True while a paginated export is in flight — disables the button. */
	let exporting = $state(false);

	/**
	 * Builds the current filter object from local state. Pulled out so
	 * the table fetch and the export fetch share one source of truth.
	 */
	function currentFilter(): ListInvoicesFilter {
		const filter: ListInvoicesFilter = { property_id: propertyId, page: 1, limit: 50 };
		if (statusFilter !== 'all') filter.status = statusFilter;
		if (dateFrom) filter.date_from = dateFrom;
		if (dateTo) filter.date_to = dateTo;
		if (search.trim()) filter.search = search.trim();
		return filter;
	}

	/**
	 * Export the full result set matching the active filters (not just
	 * the current page) to a CSV file. Paginates internally up to a hard
	 * cap of 1000 invoices so a runaway query can't OOM the browser.
	 * The CSV mirrors the on-screen columns plus a status label.
	 */
	async function exportCsv() {
		if (exporting) return;
		exporting = true;
		try {
			const baseFilter = currentFilter();
			const allRows: InvoiceSummary[] = [];
			const MAX_PAGES = 20; // 20 × 50 = 1000 invoices cap
			for (let p = 1; p <= MAX_PAGES; p++) {
				const res = await api.invoices.list({ ...baseFilter, page: p });
				allRows.push(...res.invoices);
				if (allRows.length >= res.pagination.total) break;
				if (res.invoices.length === 0) break;
			}
			if (allRows.length === 0) {
				addToast($_('invoicesList.toasts.exportEmpty'), 'info');
				return;
			}
			const headers = [
				$_('invoicesList.columns.number'),
				$_('invoicesList.columns.guest'),
				$_('invoicesList.columns.room'),
				$_('invoicesList.columns.total'),
				$_('invoicesList.columns.balance'),
				$_('invoicesList.columns.status'),
				$_('invoicesList.columns.issued')
			];
			const rows = allRows.map((inv) => [
				inv.invoice_number,
				inv.guest_name ?? '',
				inv.room_number ?? '',
				String(inv.total),
				String(inv.balance),
				$_(`invoiceWidget.status.${inv.effective_status}`),
				formatDate(inv.issued_at)
			]);
			const csv = toCSV(headers, rows);
			const stamp = new Date().toISOString().split('T')[0];
			downloadCSV(`invoices-${stamp}.csv`, csv);
			addToast(
				$_('invoicesList.toasts.exported', { values: { count: allRows.length } }),
				'success'
			);
		} catch (e: any) {
			addToast(
				e?.message ?? $_('invoicesList.errors.exportFailed'),
				'error'
			);
		} finally {
			exporting = false;
		}
	}

	function formatMoney(value: number, currency = 'IDR'): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `${currency} ${grouped}`;
	}

	function formatDate(iso: string): string {
		try {
			return new Date(iso).toLocaleDateString();
		} catch {
			return iso;
		}
	}

	// Status pill style — kept local; mirrors the InvoiceWidget pill palette.
	function statusTone(s: PaymentStatus | 'active'): { dot: string; text: string; bg: string } {
		switch (s) {
			case 'paid':
				return {
					dot: 'bg-teren-success-base',
					text: 'text-teren-success-hover dark:text-teren-success-base',
					bg: 'bg-teren-success-subtle'
				};
			case 'partial':
				return {
					dot: 'bg-teren-warning-base',
					text: 'text-teren-warning-hover dark:text-teren-warning-base',
					bg: 'bg-teren-warning-subtle'
				};
			case 'unpaid':
				return {
					dot: 'bg-teren-error-base',
					text: 'text-teren-error-hover dark:text-teren-error-base',
					bg: 'bg-teren-error-subtle'
				};
			case 'overpaid':
				return {
					dot: 'bg-teren-warning-base',
					text: 'text-teren-warning-hover dark:text-teren-warning-base',
					bg: 'bg-teren-warning-subtle'
				};
			case 'void':
				return {
					dot: 'bg-teren-text-muted',
					text: 'text-teren-text-muted line-through',
					bg: 'bg-teren-background-base'
				};
			// v1.2 (R-08, R-09 Q4) — same error palette as the widget.
			case 'refunded':
				return {
					dot: 'bg-teren-error-base',
					text: 'text-teren-error-hover dark:text-teren-error-base',
					bg: 'bg-teren-error-subtle'
				};
			default:
				return {
					dot: 'bg-teren-text-muted',
					text: 'text-teren-text-muted',
					bg: 'bg-teren-background-base'
				};
		}
	}
</script>

<section class="rounded-xl border border-teren-border-subtle bg-white shadow-sm" data-testid="invoice-list">
	<!-- Filters bar -->
	<header class="flex flex-wrap items-end gap-3 border-b border-teren-background-base p-4">
		<div class="flex-1 min-w-[200px]">
			<label for="inv-search" class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">
				{$_('invoicesList.searchLabel')}
			</label>
			<input
				id="inv-search"
				type="search"
				bind:value={search}
				placeholder={$_('invoicesList.searchPlaceholder')}
				class="mt-1 w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main placeholder:text-teren-text-muted focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
				data-testid="inv-search"
			/>
		</div>
		<label class="block">
			<span class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">{$_('invoicesList.statusLabel')}</span>
			<select
				bind:value={statusFilter}
				class="mt-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
				data-testid="inv-status-filter"
			>
				{#each STATUSES as opt (opt.value)}
					<option value={opt.value}>{$_(opt.labelKey)}</option>
				{/each}
			</select>
		</label>
		<label class="block">
			<span class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">{$_('invoicesList.fromLabel')}</span>
			<input
				type="date"
				bind:value={dateFrom}
				class="mt-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary tabular-nums"
				data-testid="inv-date-from"
			/>
		</label>
		<label class="block">
			<span class="block text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">{$_('invoicesList.toLabel')}</span>
			<input
				type="date"
				bind:value={dateTo}
				class="mt-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary tabular-nums"
				data-testid="inv-date-to"
			/>
		</label>
		<button
			type="button"
			onclick={clearFilters}
			class="rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-xs font-semibold text-teren-text-muted transition-colors hover:bg-teren-background-base cursor-pointer"
			data-testid="inv-clear-filters"
		>
			{$_('invoicesList.clearFilters')}
		</button>
		<button
			type="button"
			disabled={exporting}
			onclick={exportCsv}
			class="rounded-lg bg-teren-primary px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 disabled:opacity-60 disabled:cursor-not-allowed cursor-pointer"
			data-testid="inv-export-csv"
		>
			{exporting ? '…' : `⬇ ${$_('invoicesList.export')}`}
		</button>
	</header>

	<!-- Body -->
	{#if loadError}
		<p class="p-6 text-sm text-teren-error-base">{$_('invoicesList.errors.loadFailed')}</p>
	{:else if loading && invoices.length === 0}
		<p class="p-6 text-sm text-teren-text-muted">{$_('invoicesList.loading')}</p>
	{:else if invoices.length === 0}
		<div class="flex flex-col items-center gap-2 p-12 text-center">
			<p class="text-base font-semibold text-teren-text-main">{$_('invoicesList.empty.title')}</p>
			<p class="text-sm text-teren-text-muted">{$_('invoicesList.empty.hint')}</p>
		</div>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full text-sm" data-testid="inv-table">
				<thead>
					<tr class="border-b border-teren-background-base text-left text-[10px] font-bold uppercase tracking-wider text-teren-text-muted">
						<th class="px-4 py-2">{$_('invoicesList.columns.number')}</th>
						<th class="px-4 py-2">{$_('invoicesList.columns.guest')}</th>
						<th class="px-4 py-2">{$_('invoicesList.columns.room')}</th>
						<th class="px-4 py-2 text-right">{$_('invoicesList.columns.total')}</th>
						<th class="px-4 py-2 text-right">{$_('invoicesList.columns.balance')}</th>
						<th class="px-4 py-2">{$_('invoicesList.columns.status')}</th>
						<th class="px-4 py-2">{$_('invoicesList.columns.issued')}</th>
					</tr>
				</thead>
				<tbody>
					{#each invoices as inv (inv.id)}
						{@const tone = statusTone(inv.effective_status)}
						<tr
							class="cursor-pointer border-b border-teren-background-base transition-colors hover:bg-teren-background-base/60"
							onclick={() => onSelect?.(inv)}
							data-testid="inv-row"
							data-invoice-id={inv.id}
						>
							<td class="px-4 py-2 font-semibold text-teren-text-main">{inv.invoice_number}</td>
							<td class="px-4 py-2 text-teren-text-main">{inv.guest_name ?? '—'}</td>
							<td class="px-4 py-2 text-teren-text-muted">{inv.room_number ?? '—'}</td>
							<td class="px-4 py-2 text-right tabular-nums text-teren-text-main">{formatMoney(inv.total)}</td>
							<td class="px-4 py-2 text-right tabular-nums text-teren-text-main">{formatMoney(inv.balance)}</td>
							<td class="px-4 py-2">
							<span
								class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10px] font-semibold {tone.bg} {tone.text}"
								data-status={inv.effective_status}
							>
								<span class="h-1.5 w-1.5 rounded-full {tone.dot}"></span>
								{$_(`invoiceWidget.status.${inv.effective_status}`)}
							</span>
						</td>
							<td class="px-4 py-2 text-teren-text-muted">{formatDate(inv.issued_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		<footer class="flex flex-wrap items-center justify-between gap-2 border-t border-teren-background-base px-4 py-3 text-xs text-teren-text-muted">
			<span data-testid="inv-pagination-summary">
				{$_('invoicesList.pagination.summary', {
					values: { from: (page - 1) * limit + 1, to: Math.min(page * limit, total), total }
				})}
			</span>
			<div class="flex items-center gap-1">
				<button
					type="button"
					disabled={page <= 1}
					onclick={() => goToPage(page - 1)}
					class="rounded-md border border-teren-border-subtle bg-white px-2 py-1 text-xs font-medium transition-colors hover:bg-teren-background-base disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
					data-testid="inv-prev-page"
				>
					‹ {$_('invoicesList.pagination.prev')}
				</button>
				<span class="px-2 tabular-nums">{page} / {totalPages}</span>
				<button
					type="button"
					disabled={page >= totalPages}
					onclick={() => goToPage(page + 1)}
					class="rounded-md border border-teren-border-subtle bg-white px-2 py-1 text-xs font-medium transition-colors hover:bg-teren-background-base disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
					data-testid="inv-next-page"
				>
					{$_('invoicesList.pagination.next')} ›
				</button>
			</div>
		</footer>
	{/if}
</section>