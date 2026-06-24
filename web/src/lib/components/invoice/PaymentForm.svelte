<!--
	PaymentForm.svelte
	TEREN Hotels — Invoicing & Payments (B7)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.3

	Inline payment form. Lives inside the InvoiceWidget, replaces the
	"Register payment" button when active. No modal — keeps the user
	context (BR-INV-007, Tone of Voice: "we come to where they are").

	Behaviour
	- Amount pre-fills with the remaining balance (capped to 0 if overpaid).
	- Method is required; reference becomes required for non-cash per BR-INV-005.
	- Idempotency-Key (UUID v4) is generated client-side on every submit.
	  The backend deduplicates replays inside the 24h TTL (R-06).
	- Submits via api.invoices.registerPayment, then bubbles a `success` event
	  so the parent widget can refetch the invoice and close the form.
	- Errors are surfaced inline (no toast) for amount / reference issues —
	  toasts are reserved for transport failures.
-->
<script lang="ts">
	import { untrack } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import type { Payment, PaymentMethod } from '$lib/types';

	interface Props {
		invoiceId: string;
		propertyId: string;
		/** Current remaining balance (positive number). */
		balance: number;
		/** Dev auth — UUID of the user receiving the payment. */
		receivedBy: string;
		onSuccess?: (payment: Payment) => void;
		onCancel?: () => void;
	}

	let { invoiceId, propertyId, balance, receivedBy, onSuccess, onCancel }: Props = $props();

	// === Form state ===
	// The form is mounted/unmounted by the parent on each tap, so capturing
	// the initial balance is intentional — we don't want a stale refetch to
	// silently rewrite the user's input. untrack() silences the Svelte 5
	// "state_referenced_locally" hint.
	let amountStr = $state(untrack(() => String(Math.max(0, Math.round(balance)))));
	let method = $state<PaymentMethod>('cash');
	let reference = $state('');
	let notes = $state('');
	let submitting = $state(false);
	let formError = $state<string | null>(null);

	// === Derived ===
	const amount = $derived.by(() => {
		const n = Number(amountStr.replace(/[^\d.-]/g, ''));
		return Number.isFinite(n) ? n : NaN;
	});

	// BR-INV-005: reference is mandatory for non-cash methods.
	const needsReference = $derived(method !== 'cash');

	const isValid = $derived.by(() => {
		if (!Number.isFinite(amount) || amount <= 0) return false;
		if (amount > balance) return false;
		if (needsReference && reference.trim() === '') return false;
		return true;
	});

	// === Helpers ===
	function formatMoney(value: number): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `IDR ${grouped}`;
	}

	function amountErrorKey(): string | null {
		if (amountStr.trim() === '') return 'paymentForm.errors.amountRequired';
		if (!Number.isFinite(amount) || amount <= 0) return 'paymentForm.errors.amountPositive';
		if (amount > balance) return 'paymentForm.errors.amountExceeds';
		return null;
	}

	function generateIdempotencyKey(): string {
		// crypto.randomUUID is available in modern browsers and Node 19+.
		// In test envs (jsdom) we fall back to a v4-shaped polyfill.
		if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
			return crypto.randomUUID();
		}
		// RFC 4122 v4 fallback (good enough for dedup purposes).
		return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
			const r = (Math.random() * 16) | 0;
			const v = c === 'x' ? r : (r & 0x3) | 0x8;
			return v.toString(16);
		});
	}

	// === Actions ===
	async function handleSubmit(e: Event) {
		e.preventDefault();
		formError = null;
		if (!isValid) return;

		submitting = true;
		try {
			const idempotencyKey = generateIdempotencyKey();
			const payment = await api.invoices.registerPayment(
				invoiceId,
				{
					method,
					amount,
					reference: needsReference ? reference.trim() : undefined,
					notes: notes.trim() || undefined
				},
				propertyId,
				receivedBy,
				idempotencyKey
			);
			onSuccess?.(payment);
		} catch (err: any) {
			// Backend returns structured error codes (PAYMENT_EXCEEDS_BALANCE,
			// REFERENCE_REQUIRED, INVOICE_VOID, …). Surface the human message.
			formError = err?.message ?? $_('paymentForm.errors.generic');
		} finally {
			submitting = false;
		}
	}

	function handleMethodChange(next: PaymentMethod) {
		method = next;
		// Auto-clear reference when switching back to cash.
		if (next === 'cash') reference = '';
	}

	function setFullAmount() {
		amountStr = String(Math.max(0, Math.round(balance)));
	}
</script>

<form
	class="space-y-3 border-t border-teren-background-base bg-teren-info-subtle/30 px-5 py-4"
	data-testid="payment-form"
	onsubmit={handleSubmit}
	novalidate
>
	<!-- Amount -->
	<div>
		<label
			for="payment-amount"
			class="block text-xs font-semibold text-teren-text-main"
		>
			{$_('paymentForm.amountLabel')}
		</label>
		<div class="relative mt-1">
			<input
				id="payment-amount"
				type="text"
				inputmode="decimal"
				bind:value={amountStr}
				class="w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 pr-16 text-sm tabular-nums text-teren-text-main focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
				placeholder="0"
				data-testid="payment-amount"
				aria-invalid={amountErrorKey() !== null}
			/>
			<button
				type="button"
				onclick={setFullAmount}
				class="absolute top-1/2 right-2 -translate-y-1/2 rounded-md bg-teren-primary/10 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-teren-primary transition-colors hover:bg-teren-primary/20 cursor-pointer"
				data-testid="payment-set-full"
			>
				{$_('paymentForm.maxButton')}
			</button>
		</div>
		{#if amountErrorKey()}
			<p class="mt-1 text-xs text-teren-error-base" data-testid="payment-amount-error">
				{$_(amountErrorKey()!)}
			</p>
		{:else}
			<p class="mt-1 text-xs text-teren-text-muted">
				{$_('paymentForm.balanceHint', { values: { balance: formatMoney(balance) } })}
			</p>
		{/if}
	</div>

	<!-- Method -->
	<div>
		<span class="block text-xs font-semibold text-teren-text-main">
			{$_('paymentForm.methodLabel')}
		</span>
		<div class="mt-1 grid grid-cols-2 gap-1.5" role="radiogroup">
			{#each ['cash', 'bank_transfer', 'qris', 'card'] as m (m)}
				<button
					type="button"
					role="radio"
					aria-checked={method === m}
					onclick={() => handleMethodChange(m as PaymentMethod)}
					class="rounded-lg border px-3 py-2 text-xs font-semibold transition-all cursor-pointer
						{method === m
							? 'border-teren-primary bg-teren-primary text-white shadow-sm'
							: 'border-teren-border-subtle bg-white text-teren-text-muted hover:border-teren-primary/40 hover:text-teren-text-main'}"
					data-testid="payment-method-{m}"
				>
					{$_(`invoiceWidget.payments.method.${m}`)}
				</button>
			{/each}
		</div>
	</div>

	<!-- Reference (BR-INV-005) -->
	{#if needsReference}
		<div>
			<label
				for="payment-reference"
				class="block text-xs font-semibold text-teren-text-main"
			>
				{$_('paymentForm.referenceLabel')}
				<span class="text-teren-error-base">*</span>
			</label>
			<input
				id="payment-reference"
				type="text"
				bind:value={reference}
				placeholder={$_('paymentForm.referencePlaceholder')}
				class="mt-1 w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main placeholder:text-teren-text-muted focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
				data-testid="payment-reference"
			/>
			<p class="mt-1 text-xs text-teren-text-muted">
				{$_('paymentForm.referenceHint')}
			</p>
		</div>
	{/if}

	<!-- Notes (optional) -->
	<div>
		<label
			for="payment-notes"
			class="block text-xs font-semibold text-teren-text-main"
		>
			{$_('paymentForm.notesLabel')}
		</label>
		<textarea
			id="payment-notes"
			bind:value={notes}
			rows="2"
			placeholder={$_('paymentForm.notesPlaceholder')}
			class="mt-1 w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main placeholder:text-teren-text-muted focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
			data-testid="payment-notes"
		></textarea>
	</div>

	{#if formError}
		<p class="text-xs text-teren-error-base" data-testid="payment-form-error">
			{formError}
		</p>
	{/if}

	<!-- Actions -->
	<div class="flex flex-wrap gap-2 pt-1">
		<button
			type="submit"
			disabled={!isValid || submitting}
			class="flex-1 rounded-lg bg-teren-primary px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
			data-testid="payment-submit"
		>
			{submitting ? $_('paymentForm.submitting') : $_('paymentForm.submit')}
		</button>
		<button
			type="button"
			disabled={submitting}
			onclick={() => onCancel?.()}
			class="rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-xs font-medium text-teren-text-muted transition-colors hover:bg-teren-background-base disabled:opacity-50 cursor-pointer"
			data-testid="payment-cancel"
		>
			{$_('paymentForm.cancel')}
		</button>
	</div>
</form>