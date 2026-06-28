<!--
	/invoices — invoice list + detail drawer (B9)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.7 / §4.2

	The page orchestrates the list and the detail drawer. Clicking a row
	opens the drawer (no modal — slide-in, preserving context per AGENTS.md).
	The list and drawer are independent: closing the drawer does not
	reset the list's filter state.
-->
<script lang="ts">
	import { _ } from 'svelte-i18n';
	import InvoiceList from '$lib/components/invoice/InvoiceList.svelte';
	import InvoiceDrawer from '$lib/components/invoice/InvoiceDrawer.svelte';
	import type { InvoiceSummary } from '$lib/types';

	const propertyId = '89ce1655-d0c6-417a-8c69-3ad59241e0d0'; // dev seed

	let selectedInvoiceId = $state<string | null>(null);
	let drawerOpen = $state(false);
	// Component-instance handle for the InvoiceList so we can call its
	// public `refresh()` method when the drawer reports a mutation.
	let listRef = $state<{ refresh: () => Promise<void> } | null>(null);

	function handleSelect(inv: InvoiceSummary) {
		selectedInvoiceId = inv.id;
		drawerOpen = true;
	}

	function closeDrawer() {
		drawerOpen = false;
	}

	/**
	 * The drawer's embedded InvoiceWidget fires this after a successful
	 * mutation (register payment, void, refund, regenerate PDF). We
	 * re-fetch the list so the row behind the drawer reflects the new
	 * state — without forcing the user to hit Refresh manually.
	 */
	async function handleInvoiceChanged() {
		await listRef?.refresh();
	}
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-6 py-4">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-teren-text-main">
				{$_('invoicesPage.title')}
			</h1>
			<p class="mt-1 text-sm text-teren-text-muted">
				{$_('invoicesPage.subtitle')}
			</p>
		</div>
	</header>

	<InvoiceList bind:this={listRef} {propertyId} onSelect={handleSelect} />

	<InvoiceDrawer
		invoiceId={selectedInvoiceId}
		{propertyId}
		isOpen={drawerOpen}
		onClose={closeDrawer}
		onChange={handleInvoiceChanged}
	/>
</div>