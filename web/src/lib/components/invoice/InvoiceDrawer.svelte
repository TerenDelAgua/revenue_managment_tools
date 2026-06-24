<!--
	InvoiceDrawer.svelte
	TEREN Hotels — Side drawer with full invoice detail (B9)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.2 + §6

	Loads an invoice by ID, embeds the InvoiceWidget (which handles
	its own refetch / payment form / void form / PDF regen), and shows
	some metadata that only the list view cares about (booking link,
	audit timestamps).

	The drawer is dismissable: backdrop click + X button + Escape key.
	Closing it doesn't destroy the cached invoice — the widget stays
	mounted as long as the drawer is open, so writes are fast.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { addToast } from '$lib/store/toastStore';
	import InvoiceWidget from './InvoiceWidget.svelte';
	import type { InvoiceDetail } from '$lib/types';

	interface Props {
		invoiceId: string | null;
		propertyId: string;
		isOpen: boolean;
		onClose: () => void;
	}

	let { invoiceId, propertyId, isOpen, onClose }: Props = $props();

	// === Local state ===
	let invoice = $state<InvoiceDetail | null>(null);
	let loading = $state(false);
	let loadError = $state<string | null>(null);

	// === Refetch on invoiceId change ===
	$effect(() => {
		if (isOpen && invoiceId) {
			void loadInvoice(invoiceId);
		} else if (!isOpen) {
			// Reset on close so a stale invoice doesn't bleed into the next open.
			invoice = null;
			loadError = null;
		}
	});

	async function loadInvoice(id: string) {
		loading = true;
		loadError = null;
		try {
			invoice = await api.invoices.getByID(id);
		} catch (e: any) {
			loadError = e?.message ?? 'error';
			invoice = null;
		} finally {
			loading = false;
		}
	}

	// === Escape to close ===
	onMount(() => {
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape' && isOpen) onClose();
		}
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	function formatMoney(value: number, currency = 'IDR'): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `${currency} ${grouped}`;
	}

	function formatDateTime(iso: string | null | undefined): string {
		if (!iso) return '—';
		try {
			return new Date(iso).toLocaleString();
		} catch {
			return iso;
		}
	}

	async function copyId() {
		if (!invoiceId) return;
		try {
			await navigator.clipboard.writeText(invoiceId);
			addToast($_('invoiceDrawer.toasts.copied'), 'success');
		} catch {
			addToast($_('invoiceDrawer.toasts.copyFailed'), 'error');
		}
	}
</script>

{#if invoiceId}
	<!-- Backdrop -->
	<button
		type="button"
		aria-label={$_('invoiceDrawer.close')}
		class="fixed inset-0 z-40 block w-full bg-teren-text-main/20 backdrop-blur-[1px] transition-opacity duration-200 {isOpen
			? 'cursor-default opacity-100'
			: 'pointer-events-none opacity-0'}"
		onclick={onClose}
	></button>

	<!-- Panel -->
	<aside
		class="fixed top-0 right-0 z-50 flex h-full w-full max-w-md transform flex-col bg-teren-surface-base shadow-xl transition-transform duration-250 ease-out border-l border-teren-border-subtle {isOpen
			? 'translate-x-0'
			: 'translate-x-full'}"
		data-testid="invoice-drawer"
	>
		<!-- Header -->
		<header class="flex items-start justify-between gap-3 border-b border-teren-border-subtle bg-teren-surface-base px-5 py-4">
			<div class="min-w-0 flex-1">
				<h2 class="text-xs font-bold uppercase tracking-wider text-teren-primary">
					{$_('invoiceDrawer.title')}
				</h2>
				{#if invoice}
					<p class="mt-1 truncate text-sm font-semibold text-teren-text-main">
						{invoice.invoice_number}
					</p>
				{:else}
					<p class="mt-1 text-sm text-teren-text-muted">—</p>
				{/if}
			</div>
			<button
				title={$_('invoiceDrawer.close')}
				onclick={onClose}
				class="rounded-lg p-2 text-teren-text-muted transition-colors hover:bg-teren-background-base hover:text-teren-text-main cursor-pointer"
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
				>
					<line x1="18" y1="6" x2="6" y2="18"></line>
					<line x1="6" y1="6" x2="18" y2="18"></line>
				</svg>
			</button>
		</header>

		<!-- Body -->
		<div class="flex-1 space-y-4 overflow-y-auto p-5">
			{#if loadError}
				<p class="text-sm text-teren-error-base">{$_('invoiceDrawer.loadError')}</p>
			{:else if loading && !invoice}
				<p class="text-sm text-teren-text-muted">{$_('invoiceDrawer.loading')}</p>
			{:else if invoice}
				<!-- Audit metadata (only here, the widget handles finance) -->
				<dl class="rounded-xl border border-teren-border-subtle bg-white p-4 text-xs space-y-1.5">
					<div class="flex items-center justify-between gap-2">
						<dt class="text-teren-text-muted">{$_('invoiceDrawer.fields.id')}</dt>
						<dd class="flex items-center gap-1">
							<span class="font-mono text-teren-text-main">{invoice.id.slice(0, 8)}…</span>
							<button
								type="button"
								onclick={copyId}
								class="rounded px-1 text-[10px] font-semibold uppercase tracking-wide text-teren-primary hover:bg-teren-primary/10 cursor-pointer"
							>
								{$_('invoiceDrawer.fields.copy')}
							</button>
						</dd>
					</div>
					<div class="flex items-center justify-between gap-2">
						<dt class="text-teren-text-muted">{$_('invoiceDrawer.fields.booking')}</dt>
						<dd class="font-mono text-teren-text-main">{invoice.booking_id.slice(0, 8)}…</dd>
					</div>
					<div class="flex items-center justify-between gap-2">
						<dt class="text-teren-text-muted">{$_('invoiceDrawer.fields.created')}</dt>
						<dd class="text-teren-text-main">{formatDateTime(invoice.created_at)}</dd>
					</div>
					{#if invoice.paid_at}
						<div class="flex items-center justify-between gap-2">
							<dt class="text-teren-text-muted">{$_('invoiceDrawer.fields.paidAt')}</dt>
							<dd class="text-teren-text-main">{formatDateTime(invoice.paid_at)}</dd>
						</div>
					{/if}
					{#if invoice.voided_at}
						<div class="flex items-center justify-between gap-2">
							<dt class="text-teren-text-muted">{$_('invoiceDrawer.fields.voidedAt')}</dt>
							<dd class="text-teren-error-base">{formatDateTime(invoice.voided_at)}</dd>
						</div>
					{/if}
					<div class="flex items-center justify-between gap-2 border-t border-teren-background-base pt-1.5">
						<dt class="text-teren-text-muted">{$_('invoiceDrawer.fields.total')}</dt>
						<dd class="tabular-nums font-semibold text-teren-text-main">{formatMoney(invoice.total)}</dd>
					</div>
				</dl>

				<!-- Reused widget (finance + actions) -->
				<InvoiceWidget
					bookingId={invoice.booking_id}
					{propertyId}
				/>
			{/if}
		</div>
	</aside>
{/if}