<!--
	InvoiceWidget.svelte
	TEREN Hotels — Invoicing & Payments (B6)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §6

	Compact invoice summary embedded in the RoomDrawer. Shows status badge,
	breakdown (subtotal · PPN · total), paid vs balance, the list of payments,
	and the primary actions (open PDF, regenerate, void).

	Behaviour
	- Loads the invoice for a given bookingId. Renders an inline skeleton
	  while loading and a calm "no invoice" message if the API returns 404.
	- All copy comes from the `invoiceWidget` namespace — never hardcoded.
	- Status pills use the Design System v1.1 tokens (success / warning /
	  error / info / sidebar). Numbers use tabular-nums so any animation is
	  crisp (per AGENTS.md).
	- Actions emit events via callbacks; the parent (RoomDrawer) decides how
	  to coordinate downstream effects (e.g. reloading the floor map).
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import PaymentForm from './PaymentForm.svelte';
	import type {
		InvoiceDetail,
		Payment,
		PaymentMethod,
		PaymentStatus
	} from '$lib/types';

	// Dev auth fallback — UUID of the seeded "Admin User" (role=admin).
	// In production this will come from a real session; until then, the
	// backend uses this to attribute the payment to a valid user.
	const DEV_USER_ID = 'd3b04521-2cc4-4e6b-b3b2-b6d673d31ca1';

	interface Props {
		bookingId: string | null;
		propertyId: string;
		/** Optional callback fired after a successful write (void, regen). */
		onChange?: () => void;
	}

	let { bookingId, propertyId, onChange }: Props = $props();

	// === Local state ===
	let invoice = $state<InvoiceDetail | null>(null);
	let loading = $state(false);
	let loadError = $state<string | null>(null);
	let showPayments = $state(false);
	let showVoidForm = $state(false);
	let voidReason = $state('');
	let submittingVoid = $state(false);
	let regeneratingPdf = $state(false);
	let showPaymentForm = $state(false);

	// === Derived ===
	const isVoid = $derived(invoice?.effective_status === 'void');
	const status = $derived<PaymentStatus | null>(invoice?.effective_status ?? null);

	// Status pill → DS token. Kept local to the widget so the rest of the
	// app stays free of invoice-specific palette decisions.
	const statusStyle = $derived.by(() => {
		switch (status) {
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
			default:
				return {
					dot: 'bg-teren-text-muted',
					text: 'text-teren-text-muted',
					bg: 'bg-teren-background-base'
				};
		}
	});

	// === Lifecycle ===
	$effect(() => {
		// Re-fetch when the booking changes. Cleared bookingId ⇒ cleared widget.
		if (bookingId) {
			void loadInvoice(bookingId);
		} else {
			invoice = null;
			loadError = null;
			showVoidForm = false;
			showPaymentForm = false;
			voidReason = '';
		}
	});

	async function loadInvoice(id: string) {
		loading = true;
		loadError = null;
		try {
			invoice = await api.invoices.getByBooking(id);
		} catch (e: any) {
			// 404 → no invoice yet, that's a calm state, not an error toast.
			if (e?.status === 404) {
				invoice = null;
			} else {
				loadError = e?.message ?? 'error';
				invoice = null;
			}
		} finally {
			loading = false;
		}
	}

	// === Actions ===
	function openPdf() {
		if (!invoice?.pdf_url) return;
		// Open in a new tab. Backend issues a presigned URL when R2 is enabled.
		window.open(invoice.pdf_url, '_blank', 'noopener,noreferrer');
	}

	async function regeneratePdf() {
		if (!invoice) return;
		regeneratingPdf = true;
		try {
			const res = await api.invoices.regeneratePDF(invoice.id);
			invoice = { ...invoice, pdf_url: res.pdf_url };
			addToast($_('invoiceWidget.toasts.regenerateSuccess'), 'success');
			onChange?.();
		} catch (e: any) {
			addToast(
				e?.message ?? $_('invoiceWidget.toasts.regenerateError'),
				'error'
			);
		} finally {
			regeneratingPdf = false;
		}
	}

	async function confirmVoid() {
		if (!invoice) return;
		const reason = voidReason.trim();
		if (!reason) {
			addToast($_('invoiceWidget.void.missingReason'), 'error');
			return;
		}
		submittingVoid = true;
		try {
			// Dev auth: we don't have a user_id in the widget context, so we
			// pass an empty string. The backend will resolve the default
			// owner in middleware. Owner override is required for refunds
			// (BR-INV-010); voiding without payments is always allowed.
			const updated = await api.invoices.void(invoice.id, reason, '');
			invoice = updated;
			showVoidForm = false;
			voidReason = '';
			addToast($_('invoiceWidget.toasts.voidSuccess'), 'success');
			onChange?.();
		} catch (e: any) {
			addToast(
				e?.message ?? $_('invoiceWidget.toasts.voidError'),
				'error'
			);
		} finally {
			submittingVoid = false;
		}
	}

	function methodLabel(m: PaymentMethod): string {
		return $_(`invoiceWidget.payments.method.${m}`);
	}

	function formatMoney(value: number, currency = 'IDR'): string {
		// Spec §6: IDR grouping by dot. Keeping it dependency-free so the
		// widget works in tests without Intl polyfills.
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `${currency} ${grouped}`;
	}

	function formatDate(iso: string | null): string {
		if (!iso) return '—';
		try {
			const d = new Date(iso);
			return d.toLocaleDateString();
		} catch {
			return iso;
		}
	}

	function paymentSign(p: Payment): string {
		// Refunds render with a leading minus; cobros stay positive.
		if (p.is_reversal || p.amount < 0) {
			return '−';
		}
		return '';
	}

	function paymentIsRefund(p: Payment): boolean {
		return p.is_reversal || p.amount < 0;
	}

	async function handlePaymentSuccess() {
		// B7: refetch the invoice so the breakdown + balance + status pill
		// reflect the new payment. The PaymentForm is dismissed and a calm
		// toast confirms the operation.
		showPaymentForm = false;
		addToast($_('paymentForm.toasts.success'), 'success');
		if (bookingId) await loadInvoice(bookingId);
		onChange?.();
	}

</script>

<section
	class="rounded-xl border border-teren-border-subtle bg-white shadow-sm"
	data-testid="invoice-widget"
	data-booking-id={bookingId ?? ''}
>
	<!-- Header -->
	<header class="flex items-start justify-between gap-3 border-b border-teren-background-base px-5 py-4">
		<div class="min-w-0 flex-1">
			<h3 class="text-xs font-bold uppercase tracking-wider text-teren-primary">
				{$_('invoiceWidget.title')}
			</h3>
			{#if invoice}
				<p class="mt-1 truncate text-sm font-semibold text-teren-text-main">
					{$_('invoiceWidget.invoiceNumber', { values: { number: invoice.invoice_number } })}
				</p>
				<p class="text-xs text-teren-text-muted">
					{$_('invoiceWidget.issuedAt', { values: { date: formatDate(invoice.issued_at) } })}
				</p>
			{:else if loading}
				<p class="mt-1 text-xs text-teren-text-muted">
					{$_('invoiceWidget.loading')}
				</p>
			{:else}
				<p class="mt-1 text-xs text-teren-text-muted">
					{$_('invoiceWidget.noInvoice')}
				</p>
			{/if}
		</div>

		{#if status}
			<span
				class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold {statusStyle.bg} {statusStyle.text}"
				data-testid="invoice-status-pill"
				data-status={status}
			>
				<span class="h-1.5 w-1.5 rounded-full {statusStyle.dot}"></span>
				{$_(`invoiceWidget.status.${status}`)}
			</span>
		{/if}
	</header>

	{#if loadError}
		<div class="px-5 py-3 text-xs text-teren-error-base">
			{$_('invoiceWidget.loadError')}
		</div>
	{:else if invoice}
		<!-- Breakdown -->
		<dl class="space-y-1.5 px-5 py-4 text-sm">
			<div class="flex items-baseline justify-between">
				<dt class="text-teren-text-muted">{$_('invoiceWidget.breakdown.subtotal')}</dt>
				<dd class="tabular-nums text-teren-text-main" data-testid="invoice-subtotal">
					{formatMoney(invoice.subtotal)}
				</dd>
			</div>
			<div class="flex items-baseline justify-between">
				<dt class="text-teren-text-muted">
					{$_('invoiceWidget.breakdown.tax', {
						values: { rate: (invoice.ppn_rate_snapshot * 100).toFixed(0) }
					})}
				</dt>
				<dd class="tabular-nums text-teren-text-main" data-testid="invoice-tax">
					{formatMoney(invoice.tax_amount)}
				</dd>
			</div>
			<div class="mt-2 flex items-baseline justify-between border-t border-teren-background-base pt-2">
				<dt class="text-sm font-semibold text-teren-text-main">
					{$_('invoiceWidget.breakdown.total')}
				</dt>
				<dd
					class="tabular-nums text-base font-bold text-teren-text-main"
					data-testid="invoice-total"
				>
					{formatMoney(invoice.total)}
				</dd>
			</div>
			<div class="flex items-baseline justify-between pt-2">
				<dt class="text-teren-text-muted">{$_('invoiceWidget.breakdown.paid')}</dt>
				<dd class="tabular-nums text-teren-success-hover dark:text-teren-success-base" data-testid="invoice-paid">
					{formatMoney(invoice.total_paid)}
				</dd>
			</div>
			{#if invoice.total_refunded > 0}
				<div class="flex items-baseline justify-between">
					<dt class="text-teren-text-muted">{$_('invoiceWidget.breakdown.refunded')}</dt>
					<dd class="tabular-nums text-teren-error-base">
						−{formatMoney(invoice.total_refunded)}
					</dd>
				</div>
			{/if}
			<div class="flex items-baseline justify-between">
				<dt class="font-semibold text-teren-text-main">{$_('invoiceWidget.breakdown.balance')}</dt>
				<dd
					class="tabular-nums font-semibold {invoice.balance === 0
						? 'text-teren-success-hover dark:text-teren-success-base'
						: 'text-teren-text-main'}"
					data-testid="invoice-balance"
				>
					{formatMoney(invoice.balance)}
				</dd>
			</div>
		</dl>

		<!-- Payments list (collapsible) -->
		<div class="border-t border-teren-background-base px-5 py-3">
			<button
				type="button"
				class="flex w-full items-center justify-between text-xs font-semibold uppercase tracking-wide text-teren-text-muted transition-colors hover:text-teren-text-main cursor-pointer"
				onclick={() => (showPayments = !showPayments)}
				aria-expanded={showPayments}
			>
				<span>
					{$_('invoiceWidget.payments.title')}
					<span class="ml-1 normal-case tracking-normal text-teren-text-muted">
						· {$_('invoiceWidget.payments.count', { values: { count: invoice.payments.length } })}
					</span>
				</span>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="14"
					height="14"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="transition-transform duration-200 {showPayments ? 'rotate-180' : ''}"
				>
					<polyline points="6 9 12 15 18 9"></polyline>
				</svg>
			</button>

			{#if showPayments}
				<ul class="mt-3 space-y-1.5 text-xs" data-testid="invoice-payments">
					{#if invoice.payments.length === 0}
						<li class="text-teren-text-muted">{$_('invoiceWidget.payments.none')}</li>
					{:else}
						{#each invoice.payments as p (p.id)}
							<li class="flex items-baseline justify-between gap-2">
								<span class="flex items-center gap-1.5">
									{#if paymentIsRefund(p)}
										<span class="rounded bg-teren-error-subtle px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-teren-error-base">
											{$_('invoiceWidget.payments.refund')}
										</span>
									{/if}
									<span class="text-teren-text-main">{methodLabel(p.method)}</span>
									{#if p.reference}
										<span class="text-teren-text-muted">· {p.reference}</span>
									{/if}
								</span>
								<span
									class="tabular-nums font-medium {paymentIsRefund(p)
										? 'text-teren-error-base'
										: 'text-teren-text-main'}"
								>
									{paymentSign(p)}{formatMoney(Math.abs(p.amount))}
								</span>
							</li>
						{/each}
					{/if}
				</ul>
			{/if}
		</div>

		<!-- Void form (inline progressive disclosure, no modal) -->
		{#if showVoidForm && !isVoid}
			<div class="border-t border-teren-background-base bg-teren-error-subtle/30 px-5 py-4">
				<label
					for="invoice-void-reason"
					class="block text-xs font-semibold text-teren-text-main"
				>
					{$_('invoiceWidget.void.reasonLabel')}
				</label>
				<textarea
					id="invoice-void-reason"
					bind:value={voidReason}
					placeholder={$_('invoiceWidget.void.reasonPlaceholder')}
					rows="2"
					class="mt-1 w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main placeholder:text-teren-text-muted focus:border-teren-error-base focus:outline-none focus:ring-1 focus:ring-teren-error-base"
				></textarea>
				<div class="mt-3 flex gap-2">
					<button
						type="button"
						disabled={submittingVoid}
						onclick={confirmVoid}
						class="flex-1 rounded-lg bg-teren-error-base px-3 py-2 text-xs font-semibold text-white transition-all hover:bg-teren-error-hover active:scale-95 disabled:opacity-60 cursor-pointer"
					>
						{submittingVoid ? '…' : $_('invoiceWidget.void.confirm')}
					</button>
					<button
						type="button"
						disabled={submittingVoid}
						onclick={() => {
							showVoidForm = false;
							voidReason = '';
						}}
						class="rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-xs font-medium text-teren-text-muted transition-colors hover:bg-teren-background-base cursor-pointer"
					>
						{$_('invoiceWidget.void.cancel')}
					</button>
				</div>
			</div>
		{/if}

		<!-- Payment form (inline progressive disclosure, no modal) -->
		{#if showPaymentForm && invoice}
			<PaymentForm
				invoiceId={invoice.id}
				{propertyId}
				balance={invoice.balance}
				receivedBy={DEV_USER_ID}
				onSuccess={handlePaymentSuccess}
				onCancel={() => (showPaymentForm = false)}
			/>
		{/if}

		<!-- Actions -->
		{#if !isVoid}
			<footer class="flex flex-wrap gap-2 border-t border-teren-background-base px-5 py-4">
				{#if invoice.pdf_url}
					<button
						type="button"
						onclick={openPdf}
						class="flex-1 rounded-lg bg-teren-primary px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 cursor-pointer"
						data-testid="invoice-open-pdf"
					>
						{$_('invoiceWidget.actions.openPdf')}
					</button>
				{:else}
					<button
						type="button"
						disabled={regeneratingPdf}
						onclick={regeneratePdf}
						class="flex-1 rounded-lg bg-teren-primary px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 disabled:opacity-60 cursor-pointer"
						data-testid="invoice-regenerate-pdf"
					>
						{regeneratingPdf ? '…' : $_('invoiceWidget.actions.regeneratePdf')}
					</button>
				{/if}
				{#if invoice.balance > 0 && !showPaymentForm}
					<button
						type="button"
						onclick={() => (showPaymentForm = true)}
						class="rounded-lg border border-teren-success-base/40 bg-teren-success-subtle px-3 py-2 text-xs font-semibold text-teren-success-hover transition-colors hover:bg-teren-success-subtle/70 dark:text-teren-success-base cursor-pointer"
						data-testid="invoice-payment-toggle"
					>
						{$_('invoiceWidget.actions.registerPayment')}
					</button>
				{/if}
				{#if !showVoidForm}
					<button
						type="button"
						onclick={() => (showVoidForm = true)}
						class="rounded-lg border border-teren-error-base/40 bg-teren-error-subtle px-3 py-2 text-xs font-semibold text-teren-error-hover transition-colors hover:bg-teren-error-subtle/70 dark:text-teren-error-base cursor-pointer"
						data-testid="invoice-void-toggle"
					>
						{$_('invoiceWidget.actions.void')}
					</button>
				{/if}
			</footer>
		{/if}
	{/if}
</section>