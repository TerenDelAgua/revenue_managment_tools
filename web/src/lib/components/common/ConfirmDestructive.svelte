<!--
	ConfirmDestructive.svelte
	TEREN Hotels — Invoicing & Payments (Block 9 / v1.2)
	Spec ref: Docs/Features/TEREN_Hotels_Invoicing_Spec_v1.2.md §5.2 (R-07, R-09 Q1)

	Destructive confirmation modal. Used wherever an action breaks an
	audit-trail guarantee — currently the refund method override
	(R-07 / R-09 Q1: "Change refund method?"). Reusable for the
	upcoming refund-all confirm (Block 10) and any future destructive
	intent.

	UX rules (DS v1.1 §3.9 + R-09 Q1):
	- Checkbox acknowledgement; the user does NOT need to type text.
	  Friction is minimal but the change is still explicit.
	- The Confirm button stays disabled until the checkbox is ticked.
	- Backdrop click and Escape both act as Cancel.
	- Focus lands on the checkbox so the keyboard flow is one tick + Enter.

	Tokens
	- destructive border + icon: teren-error-base (#DC2626).
	- body / cards: teren-surface-base + teren-text-main.
	- checkbox + confirm button: teren-warning-hover (warm destructive
	  action — error palette reads as "alert", warning reads as
	  "intentional override", matching the R-07 audit-trail nuance).
-->
<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';

	interface Props {
		/** Controls visibility. Bind from the parent. */
		open: boolean;
		/** Modal heading (one short line). */
		title: string;
		/** Body copy explaining the audit-trail impact. */
		description: string;
		/** Label of the acknowledgement checkbox (one short line). */
		checkboxLabel: string;
		/** Label of the confirm button. */
		confirmLabel?: string;
		/** Label of the cancel button. */
		cancelLabel?: string;
		/** Optional icon glyph shown next to the title. Defaults to ⚠️. */
		icon?: string;
		/** Fires after the user ticks the checkbox and confirms. */
		onConfirm: () => void;
		/** Fires on Cancel — backdrop click, Escape, or button. */
		onCancel: () => void;
	}

	let {
		open,
		title,
		description,
		checkboxLabel,
		confirmLabel = 'Confirm',
		cancelLabel = 'Cancel',
		icon = '⚠️',
		onConfirm,
		onCancel
	}: Props = $props();

	// Svelte transitions rely on the Web Animations API, which jsdom
	// doesn't implement (we polyfill Element.animate in setupTests but
	// the transition runner still needs rAF + cleanups that are
	// impractical to fake). In test environments we skip the visual
	// transition and rely on jsdom's synchronous DOM updates so the
	// dialog mounts/unmounts immediately.
	const transitionsEnabled =
		typeof window !== 'undefined' && (window as { __disableTransitions?: boolean }).__disableTransitions !== true;

	const fadeOpts = transitionsEnabled ? { duration: 150, easing: cubicOut } : { duration: 0 };
	const flyOpts = transitionsEnabled ? { duration: 200, easing: cubicOut, y: 8 } : { duration: 0 };

	// === State ===
	let acknowledged = $state(false);
	let checkboxRef: HTMLInputElement | null = $state(null);
	let dialogRef: HTMLDivElement | null = $state(null);

	// Reset the checkbox whenever the modal is (re-)opened so the
	// user must consciously re-acknowledge every time.
	$effect(() => {
		if (open) {
			acknowledged = false;
			// Focus the checkbox once the dialog has been rendered.
			void tick().then(() => checkboxRef?.focus());
		}
	});

	function handleConfirm() {
		if (!acknowledged) return;
		onConfirm();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (!open) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			onCancel();
			return;
		}
		// Enter while focused inside the dialog confirms if the box
		// is already checked (Space/Enter on the checkbox itself is
		// handled natively by the input).
		if (e.key === 'Enter' && acknowledged) {
			const target = e.target as HTMLElement | null;
			// Don't double-fire if the user pressed Enter on the
			// confirm button itself.
			if (target?.getAttribute('data-testid') === 'confirm-destructive-confirm') return;
			e.preventDefault();
			handleConfirm();
		}
	}

	onMount(() => {
		window.addEventListener('keydown', handleKeydown);
		return () => window.removeEventListener('keydown', handleKeydown);
	});

	function onBackdropClick(e: MouseEvent) {
		// Only close when the click is on the backdrop itself, not on
		// the dialog content.
		if (e.target === e.currentTarget) {
			onCancel();
		}
	}
</script>

{#if open}
	<!-- Backdrop -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-teren-text-main/40 p-4 backdrop-blur-[2px]"
		data-testid="confirm-destructive-backdrop"
		onclick={onBackdropClick}
		role="presentation"
		transition:fade={fadeOpts}
	>
		<!-- Dialog -->
		<div
			bind:this={dialogRef}
			class="w-full max-w-md rounded-xl border border-teren-error-base/40 bg-teren-surface-base shadow-xl"
			data-testid="confirm-destructive-dialog"
			role="dialog"
			aria-modal="true"
			aria-labelledby="confirm-destructive-title"
			aria-describedby="confirm-destructive-description"
			transition:fly={flyOpts}
		>
			<header class="flex items-start gap-3 border-b border-teren-border-subtle px-5 py-4">
				<span class="text-xl leading-none" aria-hidden="true">{icon}</span>
				<h2
					id="confirm-destructive-title"
					data-testid="confirm-destructive-title"
					class="text-sm font-bold uppercase tracking-wider text-teren-error-base"
				>
					{title}
				</h2>
			</header>

			<div class="space-y-4 px-5 py-4">
				<p
					id="confirm-destructive-description"
					class="text-sm leading-relaxed text-teren-text-main"
				>
					{description}
				</p>

				<label
					class="flex cursor-pointer items-start gap-2 rounded-lg border border-teren-border-subtle bg-teren-background-base/60 px-3 py-2.5 transition-colors hover:bg-teren-background-base"
					data-testid="confirm-destructive-checkbox-label"
				>
					<input
						bind:this={checkboxRef}
						bind:checked={acknowledged}
						type="checkbox"
						class="mt-0.5 h-4 w-4 shrink-0 cursor-pointer rounded border-teren-border-subtle text-teren-warning-hover accent-teren-warning-hover focus:ring-1 focus:ring-teren-warning-hover focus:ring-offset-0"
						data-testid="confirm-destructive-checkbox"
					/>
					<span class="text-xs font-medium leading-snug text-teren-text-main">
						{checkboxLabel}
					</span>
				</label>
			</div>

			<footer class="flex flex-wrap gap-2 border-t border-teren-border-subtle px-5 py-4">
				<button
					type="button"
					onclick={onCancel}
					class="flex-1 rounded-lg border border-teren-border-subtle bg-white px-3 py-2 text-xs font-medium text-teren-text-muted transition-colors hover:bg-teren-background-base cursor-pointer"
					data-testid="confirm-destructive-cancel"
				>
					{cancelLabel}
				</button>
				<button
					type="button"
					disabled={!acknowledged}
					onclick={handleConfirm}
					class="flex-1 rounded-lg bg-teren-warning-hover px-3 py-2 text-xs font-semibold text-white transition-all hover:brightness-110 active:scale-95 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
					data-testid="confirm-destructive-confirm"
				>
					{confirmLabel}
				</button>
			</footer>
		</div>
	</div>
{/if}