<!--
	PaymentForm.svelte
	TEREN Hotels — Invoicing & Payments (B7)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.1.md §4.3 + §4.10

	Inline payment form. Lives inside the InvoiceWidget, replaces the
	"Register payment" button when active. No modal — keeps the user
	context (BR-INV-007, Tone of Voice: "we come to where they are").

	Modes
	- 'payment' (default): positive amount up to balance. Reference
	  required for non-cash (BR-INV-005).
	- 'refund' (B11): negative amount up to total_paid. Reference
	  required for ALL methods (R-01 applies — refunds must always be
	  traceable to a transaction). Notes required (reason for refund).
	  Backend emits is_reversal=true so the payment row flips the
	  effective_status back to 'partial' / 'unpaid'.

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

	export type PaymentFormMode = 'payment' | 'refund';

	interface Props {
		invoiceId: string;
		propertyId: string;
		/** Current remaining balance (positive number). Used in payment mode. */
		balance: number;
		/** Total already collected (positive number). Used in refund mode. */
		totalPaid: number;
		/** Dev auth — UUID of the user issuing the payment / refund. */
		receivedBy: string;
		/** 'payment' = register a charge; 'refund' = register a reversal. */
		mode?: PaymentFormMode;
		/**
		 * Existing payments on this invoice. Required in refund mode so
		 * we can pick a reversal target — the backend rejects refunds
		 * without `reversal_of` (BR-INV-010 data integrity).
		 */
		payments?: Payment[];
		onSuccess?: (payment: Payment) => void;
		onCancel?: () => void;
	}

	let {
		invoiceId,
		propertyId,
		balance,
		totalPaid,
		receivedBy,
		mode = 'payment',
		payments = [],
		onSuccess,
		onCancel
	}: Props = $props();

	// === Form state ===
	// The form is mounted/unmounted by the parent on each tap, so capturing
	// the initial value is intentional — we don't want a stale refetch to
	// silently rewrite the user's input. untrack() silences the Svelte 5
	// "state_referenced_locally" hint.
	const initialAmount = mode === 'refund' ? totalPaid : balance;
	let amountStr = $state(untrack(() => String(Math.max(0, Math.round(initialAmount)))));
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

	// Refund mode: capped to total_paid (BR-INV-010 — can't refund more than collected).
	// Payment mode: capped to balance (BR-INV-003 — can't overpay without owner override).
	const cap = $derived(mode === 'refund' ? totalPaid : balance);

	// BR-INV-005: reference is mandatory for non-cash methods.
	// In refund mode, reference is ALWAYS required (refunds must be traceable, R-01).
	const needsReference = $derived(mode === 'refund' || method !== 'cash');

	// Refund mode: notes (the reason for the refund) is required by the
	// spec. We send notes back as the request body's `notes` field; the
	// backend stores it verbatim.
	const needsNotes = $derived(mode === 'refund');

	const isValid = $derived.by(() => {
		if (!Number.isFinite(amount) || amount <= 0) return false;
		if (amount > cap) return false;
		if (needsReference && reference.trim() === '') return false;
		if (needsNotes && notes.trim() === '') return false;
		return true;
	});

	const isRefund = $derived(mode === 'refund');

	// === Helpers ===
	function formatMoney(value: number): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `IDR ${grouped}`;
	}

	function amountErrorKey(): string | null {
		if (amountStr.trim() === '') {
			return isRefund ? 'paymentForm.refund.errors.amountRequired' : 'paymentForm.errors.amountRequired';
		}
		if (!Number.isFinite(amount) || amount <= 0) {
			return isRefund ? 'paymentForm.refund.errors.amountPositive' : 'paymentForm.errors.amountPositive';
		}
		if (amount > cap) {
			return isRefund ? 'paymentForm.refund.errors.exceedsPaid' : 'paymentForm.errors.amountExceeds';
		}
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
			// Refund: send NEGATIVE amount with is_reversal=true. Notes is
			// required server-side (RefundReason) so we always forward it.
			// Reference is required for ALL refund methods (cash refunds
			// included) — we need a traceable ID even if it's a manual slip.
			const signedAmount = isRefund ? -amount : amount;

			// BR-INV-010 data integrity: refunds must point at the original
			// payment they're reversing (`reversal_of`). We pick the most
			// recent positive payment on this invoice. If the user wants
			// to refund a specific older one, that's a follow-up — out of
			// scope for MVP.
			let reversalOf: string | undefined;
			if (isRefund) {
				const candidates = payments.filter((p) => p.amount > 0 && !p.is_reversal);
				if (candidates.length === 0) {
					formError = $_('paymentForm.refund.errors.noChargeToReverse');
					return;
				}
				const latest = candidates[candidates.length - 1];
				reversalOf = latest.id;
			}

			const payment = await api.invoices.registerPayment(
				invoiceId,
				{
					method,
					amount: signedAmount,
					reference: needsReference ? reference.trim() : undefined,
					notes: notes.trim() || undefined,
					is_reversal: isRefund || undefined,
					reversal_of: reversalOf
				},
				propertyId,
				receivedBy,
				idempotencyKey
			);
			onSuccess?.(payment);
		} catch (err: any) {
			// Backend returns structured error codes (PAYMENT_EXCEEDS_BALANCE,
			// REFERENCE_REQUIRED, REFUND_FORBIDDEN, INVOICE_VOID, …).
			// Surface the human message.
			formError = err?.message ?? $_('paymentForm.errors.generic');
		} finally {
			submitting = false;
		}
	}

	function handleMethodChange(next: PaymentMethod) {
		method = next;
		// Auto-clear reference when switching back to cash in payment mode.
		// Refund mode always keeps the reference.
		if (!isRefund && next === 'cash') reference = '';
	}

	function setFullAmount() {
		amountStr = String(Math.max(0, Math.round(cap)));
	}
</script>

<form
	class="space-y-3 border-t border-teren-background-base px-5 py-4 {isRefund
		? 'bg-teren-warning-subtle/40'
		: 'bg-teren-info-subtle/30'}"
	data-testid="payment-form"
	data-mode={mode}
	onsubmit={handleSubmit}
	novalidate
>
	<!-- Mode banner -->
	{#if isRefund}
		<p
			class="text-[10px] font-bold uppercase tracking-wider text-teren-warning-hover dark:text-teren-warning-base"
			data-testid="payment-mode-banner"
		>
			{$_('paymentForm.refund.banner')}
		</p>
	{/if}

	<!-- Amount -->
	<div>
		<label
			for="payment-amount"
			class="block text-xs font-semibold text-teren-text-main"
		>
			{isRefund ? $_('paymentForm.refund.amountLabel') : $_('paymentForm.amountLabel')}
		</label>
		<div class="relative mt-1">
			<input
				id="payment-amount"
				type="text"
				inputmode="decimal"
				bind:value={amountStr}
				class="w-full rounded-lg border bg-white px-3 py-2 pr-16 text-sm tabular-nums text-teren-text-main focus:outline-none focus:ring-1 {amountErrorKey()
					? 'border-teren-error-base ring-1 ring-teren-error-base'
					: 'border-teren-border-subtle focus:border-teren-primary focus:ring-teren-primary'}"
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
				{isRefund ? $_('paymentForm.refund.maxButton') : $_('paymentForm.maxButton')}
			</button>
		</div>
		{#if amountErrorKey()}
			<p class="mt-1 text-xs text-teren-error-base" data-testid="payment-amount-error">
				{$_(amountErrorKey()!)}
			</p>
		{:else if isRefund}
			<p class="mt-1 text-xs text-teren-text-muted" data-testid="payment-refund-cap-hint">
				{$_('paymentForm.refund.capHint', { values: { cap: formatMoney(cap) } })}
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

	<!-- Reference (BR-INV-005 / refund R-01) -->
	{#if needsReference}
		<div>
			<label
				for="payment-reference"
				class="block text-xs font-semibold text-teren-text-main"
			>
				{isRefund
					? $_('paymentForm.refund.referenceLabel')
					: $_('paymentForm.referenceLabel')}
				<span class="text-teren-error-base">*</span>
			</label>
			<input
				id="payment-reference"
				type="text"
				bind:value={reference}
				placeholder={isRefund
					? $_('paymentForm.refund.referencePlaceholder')
					: $_('paymentForm.referencePlaceholder')}
				class="mt-1 w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-sm text-teren-text-main placeholder:text-teren-text-muted focus:border-teren-primary focus:outline-none focus:ring-1 focus:ring-teren-primary"
				data-testid="payment-reference"
			/>
			<p class="mt-1 text-xs text-teren-text-muted">
				{isRefund
					? $_('paymentForm.refund.referenceHint')
					: $_('paymentForm.referenceHint')}
			</p>
		</div>
	{/if}

	<!-- Notes (refund: required / payment: optional) -->
	<div>
		<label
			for="payment-notes"
			class="block text-xs font-semibold text-teren-text-main"
		>
			{isRefund
				? $_('paymentForm.refund.notesLabel')
				: $_('paymentForm.notesLabel')}
			{#if needsNotes}
				<span class="text-teren-error-base">*</span>
			{/if}
		</label>
		<textarea
			id="payment-notes"
			bind:value={notes}
			rows="2"
			placeholder={isRefund
				? $_('paymentForm.refund.notesPlaceholder')
				: $_('paymentForm.notesPlaceholder')}
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
			class="flex-1 rounded-lg px-3 py-2 text-xs font-semibold text-white transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer {isRefund
				? 'bg-teren-warning-hover hover:brightness-110'
				: 'bg-teren-primary hover:brightness-110'}"
			data-testid="payment-submit"
		>
			{submitting
				? $_('paymentForm.submitting')
				: isRefund
					? $_('paymentForm.refund.submit')
					: $_('paymentForm.submit')}
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