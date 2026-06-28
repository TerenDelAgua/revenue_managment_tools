<!--
	PaymentForm.svelte
	TEREN Hotels — Invoicing & Payments (B7 / v1.2 Block 8)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.2.md §4.3, §5.2

	Inline payment form. Lives inside the InvoiceWidget, replaces the
	"Register payment" button when active. No modal — keeps the user
	context (BR-INV-007, Tone of Voice: "we come to where they are").

	Modes
	- 'payment' (default): positive amount up to balance. Reference
	  required for non-cash (BR-INV-005).
	- 'refund' (v1.1 B11 / v1.2 Block 8): a 1:1 reversal of a single
	  original payment. The form opens a PICKER of cobrable payments
	  (positive, not invalidated, with remaining_reverseable > 0) and
	  once the user taps one it pre-fills the amount / method / reference
	  / reversal_of fields. Method is locked to match the original (R-07)
	  — the destructive override flow (Block 9) is wired through the
	  ConfirmDestructive modal.

	Behaviour
	- Amount pre-fills with the remaining balance (payment) or
	  remaining_reverseable (refund) — capped to 0 if zero.
	- Idempotency-Key (UUID v4) is generated client-side on every submit.
	  The backend deduplicates replays inside the 24h TTL (R-06).
	- Submits via api.invoices.registerPayment, then bubbles events:
	    * onSuccess  — fired after every successful write (parent refetches).
	    * onComplete — fired when the user is done with the form
	                   (parent closes the form). In 'refund' mode + a
	                   remaining cobrable payment, the form first shows
	                   a "Refund another payment" banner with a "Done"
	                   button that triggers onComplete.
	- Errors are surfaced inline (no toast) for amount / reference issues —
	  toasts are reserved for transport failures.
-->
<script lang="ts">
	import { untrack } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import type { Payment, PaymentMethod } from '$lib/types';
	import ConfirmDestructive from '$lib/components/common/ConfirmDestructive.svelte';

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
		 * the picker can list cobrable targets. The backend rejects
		 * refunds without `reversal_of` (BR-INV-010 data integrity).
		 */
		payments?: Payment[];
		/**
		 * v1.2 (Block 8/9): when the picker has selected the specific
		 * payment being reversed, the refund form locks the method to
		 * match `targetPayment.method` (R-07). The user can override this
		 * only after passing through ConfirmDestructive — the resulting
		 * request carries `force_override=true`.
		 *
		 * If null/undefined in 'refund' mode, the form opens with the
		 * PICKER step instead. If non-null, the form opens directly on
		 * the pre-filled form (used by tests + by callers that want to
		 * skip the picker).
		 */
		targetPayment?: Payment | null;
		/**
		 * v1.2 (Block 8): when false and mode='refund', the form does NOT
		 * close itself after a successful refund — it shows a "Refund
		 * another payment" banner so the user can chain multiple refunds
		 * in a single session. Default: true (close on success).
		 */
		closeOnSuccess?: boolean;
		/** Fires after a successful submit. Parent refetches the invoice. */
		onSuccess?: (payment: Payment) => void;
		/** Fires when the user dismisses the form (Done, Cancel, close). */
		onComplete?: () => void;
		/** Fires on the Cancel button. Defaults to onComplete if unset. */
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
		targetPayment = null,
		closeOnSuccess = true,
		onSuccess,
		onComplete,
		onCancel
	}: Props = $props();

	// === Form state ===
	// The form is mounted/unmounted by the parent on each tap, so capturing
	// the initial value is intentional — we don't want a stale refetch to
	// silently rewrite the user's input. untrack() silences the Svelte 5
	// "state_referenced_locally" hint.
	const initialAmount = untrack(() => (mode === 'refund' ? totalPaid : balance));
	// Pre-fill with the exact balance/totalPaid from the backend (no
	// rounding). The DB stores totals with 2-decimal precision (e.g.
	// 721.50 for 650 + 11% tax), but the UI rounds display to integers.
	// If we pre-fill the rounded integer (722) the backend's strict
	// amount check 422s because 722 > 721.50. Sending the exact 721.50
	// makes the round-trip work.
	let amountStr = $state(String(Math.max(0, initialAmount)));
	let method = $state<PaymentMethod>('cash');
	let reference = $state('');
	let notes = $state('');
	let submitting = $state(false);
	let formError = $state<string | null>(null);

	// === v1.2 — picker / target state (R-07) ===
	// `selectedTarget` mirrors the target we're about to refund. In
	// 'refund' mode without an external `targetPayment` prop, the form
	// starts with selectedTarget=null and shows the picker. Clicking an
	// item assigns the target + pre-fills the form fields.
	let selectedTarget = $state<Payment | null>(untrack(() => targetPayment));
	// After a successful refund we keep the form mounted (if
	// closeOnSuccess=false) and surface a "Refund another" banner. The
	// value is the refund row that was just created, so the banner can
	// show "−Rp X refunded" context.
	let lastRefundResult = $state<Payment | null>(null);

	// === v1.2 — force_override flow (R-07 + R-09 Q1) ===
	// When a target payment is provided (refund picker, Block 8), the
	// method is locked to match the original. The user can opt into a
	// destructive override via the ConfirmDestructive modal.
	// `methodLocked` controls the UI (disabled radios + hint); the
	// actual wire flag is `forceOverride`.
	// We capture the initial value from props via untrack() so Svelte 5
	// does not warn about referencing reactive props in $state() initializers.
	let methodLocked = $state(untrack(() => mode === 'refund' && targetPayment !== null));
	let forceOverride = $state(false);
	let showChangeMethodModal = $state(false);

	// === Derived ===
	const amount = $derived.by(() => {
		const n = Number(amountStr.replace(/[^\d.-]/g, ''));
		return Number.isFinite(n) ? n : NaN;
	});

	// Refund mode: capped to remaining_reverseable of the selected target
	// (BR-INV-010 / R-07). When no target is picked yet we use total_paid
	// as a soft hint but the form is not submittable without one.
	// Payment mode: capped to balance (BR-INV-003).
	const cap = $derived.by(() => {
		if (mode !== 'refund') return balance;
		if (selectedTarget) {
			return selectedTarget.remaining_reverseable ?? selectedTarget.amount;
		}
		return totalPaid;
	});

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
		if (mode === 'refund' && !selectedTarget) return false;
		if (needsReference && reference.trim() === '') return false;
		if (needsNotes && notes.trim() === '') return false;
		return true;
	});

	const isRefund = $derived(mode === 'refund');

	// True when the picker has handed us a target payment — that's the
	// case where method locking + override modal applies. We accept both
	// the external prop and the internal selectedTarget.
	const hasTarget = $derived(isRefund && (selectedTarget ?? targetPayment) !== null);

	// The effective target is the internal one (set by the picker) or
	// the external prop (caller-controlled).
	const effectiveTarget = $derived<Payment | null>(selectedTarget ?? targetPayment);

	// Cobrable payments for the picker: positive, not a reversal, not
	// invalidated, with something left to refund. Sort by date desc so
	// the most recent charge is on top — that's what the user usually
	// wants to refund first.
	const cobrablePayments = $derived.by(() => {
		return payments
			.filter(
				(p) =>
					p.amount > 0 &&
					!p.is_reversal &&
					!p.invalidated_at &&
					(p.remaining_reverseable ?? p.amount) > 0
			)
			.slice()
			.sort((a, b) => b.received_at.localeCompare(a.received_at));
	});

	const isPickerOpen = $derived(isRefund && !selectedTarget && !lastRefundResult);

	// === Helpers ===
	function formatMoney(value: number): string {
		const fixed = Math.round(value).toString();
		const grouped = fixed.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
		return `IDR ${grouped}`;
	}

	function formatDate(iso: string | null | undefined): string {
		if (!iso) return '—';
		try {
			return new Date(iso).toLocaleDateString();
		} catch {
			return iso;
		}
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

	function defaultRefundReference(p: Payment): string {
		// R-07: pre-fill as REFUND-{original.reference} or REFUND-{id[:8]}.
		return p.reference ? `REFUND-${p.reference}` : `REFUND-${p.id.slice(0, 8)}`;
	}

	// === Picker actions ===
	function selectPayment(p: Payment) {
		// Reset method-lock + override state for the new target.
		selectedTarget = p;
		methodLocked = true;
		forceOverride = false;
		method = p.method;
		// Pre-fill amount / reference to match the spec.
		const capValue = p.remaining_reverseable ?? p.amount;
		amountStr = String(Math.max(0, capValue));
		reference = defaultRefundReference(p);
		notes = '';
		formError = null;
	}

	function backToPicker() {
		selectedTarget = null;
		lastRefundResult = null;
		amountStr = String(Math.max(0, totalPaid));
		method = 'cash';
		reference = '';
		notes = '';
		methodLocked = false;
		forceOverride = false;
		formError = null;
	}

	function refundAnother() {
		// After a successful refund, swap the banner for a fresh picker so
		// the user can keep going. We also drop the last refund result so
		// the next round starts from a clean slate.
		lastRefundResult = null;
		selectedTarget = null;
		amountStr = String(Math.max(0, totalPaid));
		method = 'cash';
		reference = '';
		notes = '';
		methodLocked = false;
		forceOverride = false;
		formError = null;
	}

	// === Submit ===
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
			// payment they're reversing (`reversal_of`). The picker hands us
			// the target via `effectiveTarget`; if no target is set we
			// fall back to the most recent positive payment (legacy v1.1
			// behaviour, kept for callers that pre-date the picker).
			let reversalOf: string | undefined;
			if (isRefund) {
				if (effectiveTarget) {
					reversalOf = effectiveTarget.id;
				} else {
					const candidates = payments.filter((p) => p.amount > 0 && !p.is_reversal);
					if (candidates.length === 0) {
						formError = $_('paymentForm.refund.errors.noChargeToReverse');
						return;
					}
					const latest = candidates[candidates.length - 1];
					reversalOf = latest.id;
				}
			}

			const payment = await api.invoices.registerPayment(
				invoiceId,
				{
					method,
					amount: signedAmount,
					reference: needsReference ? reference.trim() : undefined,
					notes: notes.trim() || undefined,
					is_reversal: isRefund || undefined,
					reversal_of: reversalOf,
					// R-07: only forward force_override when we actually
					// flipped out of the locked method (avoids sending a
					// stray flag in the legacy / no-target case).
					force_override: forceOverride || undefined
				},
				propertyId,
				receivedBy,
				idempotencyKey
			);

			// Always fire onSuccess so the parent can refetch.
			onSuccess?.(payment);

			if (isRefund) {
				// Refund flow: keep the form mounted so the user can issue
				// more refunds in this session. The banner is the new
				// "Done / Refund another" hub. The parent only closes
				// when we fire onComplete (the user clicks Done).
				lastRefundResult = payment;
				submitting = false;
				if (closeOnSuccess) {
					onComplete?.();
				}
			} else {
				// Payment flow: same as before — close on success.
				if (closeOnSuccess) {
					onComplete?.();
				}
			}
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
		// Locked state: the radios fire onchange but we want the modal
		// to gate the switch. We honour the user's selection only after
		// they confirm — until then `method` stays at the locked value.
		if (hasTarget && methodLocked && next !== effectiveTarget?.method) {
			showChangeMethodModal = true;
			return;
		}
		method = next;
		// Auto-clear reference when switching back to cash in payment mode.
		// Refund mode always keeps the reference.
		if (!isRefund && next === 'cash') reference = '';
	}

	function setFullAmount() {
		amountStr = String(Math.max(0, cap));
	}

	function openChangeMethodModal() {
		showChangeMethodModal = true;
	}

	function confirmChangeMethod() {
		methodLocked = false;
		forceOverride = true;
		showChangeMethodModal = false;
	}

	function cancelChangeMethod() {
		showChangeMethodModal = false;
	}

	function dismiss() {
		// Cancel button: prefer the dedicated handler, fall back to onComplete.
		if (onCancel) onCancel();
		else onComplete?.();
	}

	function doneWithRefunds() {
		lastRefundResult = null;
		selectedTarget = null;
		onComplete?.();
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
	hidden={isPickerOpen || (isRefund && lastRefundResult !== null && !closeOnSuccess)}
>
	<!-- Mode banner -->
	{#if isRefund}
		<p
			class="text-[10px] font-bold uppercase tracking-wider text-teren-warning-hover dark:text-teren-warning-base"
			data-testid="payment-mode-banner"
		>
			{$_('paymentForm.refund.banner')}
			{#if effectiveTarget}
				· {$_('paymentForm.refund.reverting', {
					values: {
						method: $_(`invoiceWidget.payments.method.${effectiveTarget.method}`),
						amount: formatMoney(effectiveTarget.amount),
						date: formatDate(effectiveTarget.received_at)
					}
				})}
			{/if}
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
							: 'border-teren-border-subtle bg-white text-teren-text-muted hover:border-teren-primary/40 hover:text-teren-text-main'}
						{hasTarget && methodLocked ? 'cursor-not-allowed opacity-70' : ''}"
					disabled={hasTarget && methodLocked && m !== effectiveTarget?.method}
					data-testid="payment-method-{m}"
				>
					{$_(`invoiceWidget.payments.method.${m}`)}
				</button>
			{/each}
		</div>
		<!-- v1.2 — refund method locking hint (R-07). The radios are disabled
		     for non-matching methods; a small link below opens the
		     ConfirmDestructive modal that gates the override. -->
		{#if hasTarget && methodLocked && effectiveTarget}
			<p
				class="mt-1 text-[11px] text-teren-text-muted"
				data-testid="payment-method-locked-hint"
			>
				{$_('paymentForm.refund.lockedMethodHint', {
					values: {
						method: $_(`invoiceWidget.payments.method.${effectiveTarget.method}`)
					}
				})}
				·
				<button
					type="button"
					class="font-semibold text-teren-warning-hover underline-offset-2 hover:underline cursor-pointer"
					onclick={openChangeMethodModal}
					data-testid="payment-method-change-link"
				>
					{$_('paymentForm.refund.changeMethodToggle')}
				</button>
			</p>
		{:else if hasTarget && !methodLocked && effectiveTarget}
			<p
				class="mt-1 text-[11px] font-semibold text-teren-warning-hover"
				data-testid="payment-method-override-active"
			>
				{$_('paymentForm.refund.changeMethodActive')}
			</p>
		{/if}
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
			onclick={dismiss}
			class="rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-xs font-medium text-teren-text-muted transition-colors hover:bg-teren-background-base disabled:opacity-50 cursor-pointer"
			data-testid="payment-cancel"
		>
			{isRefund && selectedTarget && !targetPayment
				? $_('paymentForm.refund.backToPicker')
				: $_('paymentForm.cancel')}
		</button>
	</div>
</form>

<!-- v1.2 (Block 8) — refund picker (R-07). Renders BEFORE the form
     when the user is in 'refund' mode and hasn't picked a target yet.
     Each item is a cobrable payment; clicking it sets `selectedTarget`
     and pre-fills the form above (this is also the entry point for
     `force_override` via the modal). -->
{#if isPickerOpen}
	<div
		class="space-y-2 border-t border-teren-background-base bg-teren-warning-subtle/40 px-5 py-4"
		data-testid="refund-picker"
	>
		<p
			class="text-[10px] font-bold uppercase tracking-wider text-teren-warning-hover dark:text-teren-warning-base"
			data-testid="refund-picker-heading"
		>
			{$_('paymentForm.refund.pickerHeading')}
		</p>
		{#if cobrablePayments.length === 0}
			<p
				class="text-xs text-teren-text-muted"
				data-testid="refund-picker-empty"
			>
				{$_('paymentForm.refund.pickerEmpty')}
			</p>
		{:else}
			<ul class="space-y-1.5" role="list" data-testid="refund-picker-list">
				{#each cobrablePayments as p (p.id)}
					<li>
						<button
							type="button"
							class="flex w-full items-center justify-between gap-3 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-left transition-colors hover:border-teren-primary/40 cursor-pointer"
							onclick={() => selectPayment(p)}
							data-testid="refund-picker-item"
							data-payment-id={p.id}
						>
							<span class="flex flex-col gap-0.5">
								<span class="flex items-center gap-1.5">
									<span
										class="inline-flex h-4 w-4 items-center justify-center rounded-full bg-teren-primary/10 text-[10px] font-bold text-teren-primary"
										aria-hidden="true"
									>●</span>
									<span class="text-xs font-semibold text-teren-text-main">
										{$_(`invoiceWidget.payments.method.${p.method}`)}
									</span>
									<span class="text-[11px] text-teren-text-muted">
										· {formatDate(p.received_at)}
									</span>
								</span>
								<span class="text-[11px] text-teren-text-muted">
									{$_('paymentForm.refund.pickerItemRef', {
										values: {
											ref: p.reference ?? $_('paymentForm.refund.pickerNoRef')
										}
									})}
								</span>
							</span>
							<span class="flex flex-col items-end gap-0.5">
								<span class="text-xs font-bold text-teren-text-main tabular-nums">
									{formatMoney(p.amount)}
								</span>
								<span class="text-[10px] text-teren-text-muted">
									{$_('paymentForm.refund.pickerItemAvailable', {
										values: {
											amount: formatMoney(p.remaining_reverseable ?? p.amount)
										}
									})}
								</span>
							</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
		<button
			type="button"
			onclick={dismiss}
			class="mt-2 w-full rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-xs font-medium text-teren-text-muted transition-colors hover:bg-teren-background-base cursor-pointer"
			data-testid="refund-picker-cancel"
		>
			{$_('paymentForm.cancel')}
		</button>
	</div>
{/if}

<!-- v1.2 (Block 8) — "Refund another payment" banner. Shown after a
     successful refund when closeOnSuccess=false. The form remains
     mounted so the user can chain refunds in a single session. -->
{#if isRefund && lastRefundResult && !closeOnSuccess}
	<div
		class="space-y-2 border-t border-teren-background-base bg-teren-success-subtle/30 px-5 py-3"
		data-testid="refund-success-banner"
	>
		<p
			class="text-[11px] font-semibold text-teren-success-hover"
			data-testid="refund-success-message"
		>
			{$_('paymentForm.refund.successMessage', {
				values: { amount: formatMoney(Math.abs(lastRefundResult.amount)) }
			})}
		</p>
		<div class="flex flex-wrap gap-2">
			<button
				type="button"
				onclick={refundAnother}
				class="flex-1 rounded-lg border border-teren-primary bg-white px-3 py-2 text-xs font-semibold text-teren-primary transition-colors hover:bg-teren-primary/10 cursor-pointer"
				data-testid="refund-another-button"
			>
				{$_('paymentForm.refund.refundAnother')}
			</button>
			<button
				type="button"
				onclick={doneWithRefunds}
				class="flex-1 rounded-lg bg-teren-text-main px-3 py-2 text-xs font-semibold text-white transition-colors hover:brightness-110 cursor-pointer"
				data-testid="refund-done-button"
			>
				{$_('paymentForm.refund.done')}
			</button>
		</div>
	</div>
{/if}

<!-- v1.2 (Block 9) — destructive confirm for refund method override.
     Renders only when the user explicitly asks to change the locked
     method. The modal flips `forceOverride` to true so the request
     body carries the audit-trail flag. -->
<ConfirmDestructive
	open={showChangeMethodModal}
	title={$_('confirmDestructive.changeRefundMethod.title')}
	description={$_('confirmDestructive.changeRefundMethod.description')}
	checkboxLabel={$_('confirmDestructive.changeRefundMethod.checkbox')}
	confirmLabel={$_('confirmDestructive.changeRefundMethod.confirm')}
	cancelLabel={$_('confirmDestructive.changeRefundMethod.cancel')}
	onConfirm={confirmChangeMethod}
	onCancel={cancelChangeMethod}
/>