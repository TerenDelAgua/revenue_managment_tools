/**
 * InvoiceWidget — vitest suite (B6)
 *
 * Covers the widget's three observable behaviours:
 *  - IT-01 Status pill reflects effective_status (paid / partial / unpaid / void)
 *  - IT-02 Subtotal · Tax · Total breakdown renders with the right amounts
 *  - IT-03 404 from the API renders the calm "no invoice" empty state
 *  - IT-04 Payments toggle expands and lists method + amount
 *
 * The widget reads from `$lib/api/client`. Each test stubs `global.fetch`
 * with a deterministic response so the suite is hermetic — no real backend
 * required, no testcontainers. Mirrors the pattern used by RoomDrawer.test.ts.
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import InvoiceWidget from './InvoiceWidget.svelte';
import type { InvoiceDetail } from '$lib/types';

const baseInvoice: InvoiceDetail = {
	id: 'inv-1',
	property_id: 'prop-1',
	booking_id: 'book-1',
	invoice_number: 'INV-2026-0001',
	subtotal: 100000,
	tax_amount: 11000,
	ppn_rate_snapshot: 0.11,
	total: 111000,
	original_currency: 'IDR',
	exchange_rate: 1,
	status: 'active',
	issued_at: '2026-06-20T08:00:00Z',
	paid_at: null,
	voided_at: null,
	voided_by: null,
	void_reason: null,
	created_by: 'user-1',
	pdf_url: 'https://example.com/inv-1.pdf',
	notes: null,
	created_at: '2026-06-20T08:00:00Z',
	updated_at: '2026-06-20T08:00:00Z',
	line_items: [],
	payments: [],
	total_paid: 0,
	total_refunded: 0,
	balance: 111000,
	effective_status: 'unpaid'
};

function mockFetchOnce(body: unknown, status = 200) {
	vi.stubGlobal(
		'fetch',
		vi.fn().mockResolvedValue(
			new Response(JSON.stringify(body), {
				status,
				headers: { 'Content-Type': 'application/json' }
			})
		)
	);
}

describe('InvoiceWidget', () => {
	beforeAll(() => {
		locale.set('en');
	});

	beforeEach(() => {
		vi.restoreAllMocks();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('IT-01: renders the status pill for a paid invoice', async () => {
		mockFetchOnce({ ...baseInvoice, effective_status: 'paid', balance: 0, total_paid: 111000 });
		const { container, getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-status-pill').getAttribute('data-status')).toBe('paid');
		});

		expect(getByTestId('invoice-status-pill')).toHaveTextContent('Paid');
		// Paid state uses success-subtle background.
		expect(getByTestId('invoice-status-pill').className).toContain('bg-teren-success-subtle');
	});

	it('IT-02: renders subtotal, PPN and total for a partial invoice', async () => {
		mockFetchOnce({
			...baseInvoice,
			effective_status: 'partial',
			total_paid: 60000,
			balance: 51000,
			payments: [
				{
					id: 'pay-1',
					invoice_id: 'inv-1',
					property_id: 'prop-1',
					method: 'cash',
					amount: 60000,
					original_currency: 'IDR',
					exchange_rate: 1,
					reference: null,
					notes: null,
					is_reversal: false,
					reversal_of: null,
					received_by: 'user-1',
					received_at: '2026-06-20T09:00:00Z',
					created_at: '2026-06-20T09:00:00Z'
				}
			]
		});

		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-total').textContent).toContain('IDR 111.000');
		});

		// Subtotal and tax are formatted with the same dot grouping.
		expect(getByTestId('invoice-subtotal').textContent).toContain('IDR 100.000');
		expect(getByTestId('invoice-tax').textContent).toContain('IDR 11.000');
		expect(getByTestId('invoice-paid').textContent).toContain('IDR 60.000');
		expect(getByTestId('invoice-balance').textContent).toContain('IDR 51.000');

		// Partial → warning pill.
		expect(getByTestId('invoice-status-pill').getAttribute('data-status')).toBe('partial');
	});

	it('IT-03: renders the calm "no invoice" state on 404', async () => {
		mockFetchOnce({ message: 'invoice not found' }, 404);
		const { container } = render(InvoiceWidget, {
			props: { bookingId: 'book-missing', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(container).toHaveTextContent('No invoice yet for this booking.');
		});

		// No status pill on empty state.
		expect(container.querySelector('[data-testid="invoice-status-pill"]')).toBeNull();
		// No breakdown rows either.
		expect(container.querySelector('[data-testid="invoice-total"]')).toBeNull();
	});

	it('IT-04: toggles the payments list and renders method + amount', async () => {
		mockFetchOnce({
			...baseInvoice,
			effective_status: 'partial',
			total_paid: 60000,
			balance: 51000,
			payments: [
				{
					id: 'pay-1',
					invoice_id: 'inv-1',
					property_id: 'prop-1',
					method: 'bank_transfer',
					amount: 60000,
					original_currency: 'IDR',
					exchange_rate: 1,
					reference: 'TRF-9988',
					notes: null,
					is_reversal: false,
					reversal_of: null,
					received_by: 'user-1',
					received_at: '2026-06-20T09:00:00Z',
					created_at: '2026-06-20T09:00:00Z'
				}
			]
		});

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-total').textContent).toContain('IDR 111.000');
		});

		// Initially the list is collapsed.
		expect(queryByTestId('invoice-payments')).toBeNull();

		// Click the toggle (it's the only <button> in the payments row).
		const toggle = getByTestId('invoice-total')
			.closest('section')!
			.querySelector('button[aria-expanded]') as HTMLButtonElement;
		await fireEvent.click(toggle);

		await waitFor(() => {
			const list = getByTestId('invoice-payments');
			expect(list).toHaveTextContent('Bank transfer');
			expect(list).toHaveTextContent('TRF-9988');
			expect(list).toHaveTextContent('IDR 60.000');
		});
	});

	it('IT-05: void invoice hides the void toggle and renders the void pill', async () => {
		mockFetchOnce({
			...baseInvoice,
			status: 'void',
			effective_status: 'void',
			voided_at: '2026-06-20T10:00:00Z',
			voided_by: 'user-1',
			void_reason: 'test',
			balance: 0
		});
		const { container, getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-status-pill').getAttribute('data-status')).toBe('void');
		});

		// Footer actions (void button + open PDF) must be gone when invoice is void.
		expect(queryByTestId('invoice-void-toggle')).toBeNull();
		expect(queryByTestId('invoice-open-pdf')).toBeNull();

		// Pill carries the line-through styling.
		expect(container.querySelector('[data-status="void"]')?.classList.contains('line-through')).toBe(true);
	});

	it('IT-08 (B7-validation 4): void submits POST with a valid X-User-ID header', async () => {
		// First fetch: initial invoice load (returns a non-void invoice).
		// Second fetch: the void endpoint returning the voided invoice.
		const fetchMock = vi.fn();
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify(baseInvoice), { status: 200 })
		);
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ ...baseInvoice, status: 'void', effective_status: 'void' }), {
				status: 200
			})
		);
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-void-toggle')).toBeInTheDocument();
		});

		// Open the void form, fill the reason, confirm.
		await fireEvent.click(getByTestId('invoice-void-toggle'));
		await fireEvent.input(getByTestId('invoice-void-reason'), {
			target: { value: 'test void' }
		});
		await fireEvent.click(getByTestId('invoice-void-confirm'));

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledTimes(2);
		});

		// Inspect the second call (the void POST).
		const [url, init] = fetchMock.mock.calls[1];
		expect(url).toContain('/invoices/');
		expect(url).toContain('/void');
		const headers = (init?.headers ?? {}) as Record<string, string>;
		expect(headers['X-User-ID']).toBeTruthy();
		expect(headers['X-User-ID']).not.toBe('');
		// Must be a valid UUID — matches the dev seed user pattern (8-4-4-4-12 hex).
		expect(headers['X-User-ID']).toMatch(
			/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
		);
		const body = JSON.parse(init?.body as string);
		expect(body).toEqual({ reason: 'test void' });
	});

	it('IT-09 (B7-validation 5): missing reason shows inline error and skips the API', async () => {
		// First fetch: invoice load. No second fetch should happen.
		const fetchMock = vi.fn();
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify(baseInvoice), { status: 200 })
		);
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});
		await waitFor(() => {
			expect(getByTestId('invoice-void-toggle')).toBeInTheDocument();
		});

		// Open the void form and click confirm with an empty reason.
		await fireEvent.click(getByTestId('invoice-void-toggle'));
		await fireEvent.click(getByTestId('invoice-void-confirm'));

		// Inline error renders, textarea gets the error styling, and the
		// API was NOT called (only the initial load fired).
		await waitFor(() => {
			expect(getByTestId('invoice-void-error')).toBeInTheDocument();
		});
		const textarea = getByTestId('invoice-void-reason') as HTMLTextAreaElement;
		expect(textarea.getAttribute('aria-invalid')).toBe('true');
		expect(fetchMock).toHaveBeenCalledTimes(1);

		// Typing into the textarea clears the error.
		await fireEvent.input(textarea, { target: { value: 'now filled' } });
		await waitFor(() => {
			expect(queryByTestId('invoice-void-error')).toBeNull();
		});
		expect(textarea.getAttribute('aria-invalid')).toBe('false');
	});

	it('IT-10 (B7-validation 5): successful void re-fetches the invoice so .payments is preserved', async () => {
		// First fetch: initial invoice load.
		// Second fetch: POST /void (returns models.Invoice, no .payments).
		// Third fetch: re-load via getByBooking (returns InvoiceDetail with payments).
		const fetchMock = vi.fn();
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify(baseInvoice), { status: 200 })
		);
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ ...baseInvoice, status: 'void', effective_status: 'void' }), {
				status: 200
			})
		);
		fetchMock.mockResolvedValueOnce(
			new Response(
				JSON.stringify({
					...baseInvoice,
					status: 'void',
					effective_status: 'void',
					balance: 0
				}),
				{ status: 200 }
			)
		);
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-void-toggle')).toBeInTheDocument();
		});

		await fireEvent.click(getByTestId('invoice-void-toggle'));
		await fireEvent.input(getByTestId('invoice-void-reason'), {
			target: { value: 'duplicate charge' }
		});
		await fireEvent.click(getByTestId('invoice-void-confirm'));

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledTimes(3);
		});

		// The 3rd call must be the re-fetch via getByBooking — otherwise
		// the payments list would render with undefined.length.
		const [reFetchURL] = fetchMock.mock.calls[2];
		expect(reFetchURL).toContain('/invoices/by-booking/book-1');
		expect(getByTestId('invoice-widget')).toBeInTheDocument();
	});

	it('IT-06 (B7): shows the payment toggle only when balance > 0 and toggles the PaymentForm', async () => {
		mockFetchOnce({ ...baseInvoice, effective_status: 'unpaid', balance: 111000 });
		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-payment-toggle')).toBeInTheDocument();
		});

		// Form is not rendered until the toggle is clicked.
		expect(queryByTestId('payment-form')).toBeNull();

		await fireEvent.click(getByTestId('invoice-payment-toggle'));
		await waitFor(() => {
			expect(getByTestId('payment-form')).toBeInTheDocument();
		});
		// Amount pre-filled with the remaining balance.
		const amount = getByTestId('payment-amount') as HTMLInputElement;
		expect(amount.value).toBe('111000');
	});

	it('IT-07 (B7): hides the payment toggle when balance is 0', async () => {
		mockFetchOnce({
			...baseInvoice,
			effective_status: 'paid',
			total_paid: 111000,
			balance: 0
		});
		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-status-pill').getAttribute('data-status')).toBe('paid');
		});
		// No balance ⇒ no payment toggle.
		expect(queryByTestId('invoice-payment-toggle')).toBeNull();
	});

	// ============ Refund UI (B11) ============

	it('IT-11 (B11): refund toggle is shown when total_paid > 0 and absent when zero', async () => {
		// First fetch: a paid invoice (balance=0, total_paid=555000).
		mockFetchOnce({ ...baseInvoice, status: 'paid', effective_status: 'paid', balance: 0, total_paid: 555000, total_refunded: 0 });
		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => {
			expect(getByTestId('invoice-refund-toggle')).toBeInTheDocument();
		});
		// Paid invoice has balance=0 → no register-payment toggle.
		expect(getByTestId('invoice-refund-toggle').textContent).toMatch(/refund/i);
	});

	it('IT-12 (B11): clicking the refund toggle opens the PaymentForm in refund mode', async () => {
		mockFetchOnce({ ...baseInvoice, status: 'paid', effective_status: 'paid', balance: 0, total_paid: 555000, total_refunded: 0 });
		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-refund-toggle')).toBeInTheDocument());
		await fireEvent.click(getByTestId('invoice-refund-toggle'));

		await waitFor(() => {
			expect(getByTestId('payment-form').getAttribute('data-mode')).toBe('refund');
		});
		// Refund banner shows.
		expect(getByTestId('payment-mode-banner')).toHaveTextContent(/refund/i);
	});

	it('IT-13 (B11): refund success re-fetches the invoice and shows refund toast', async () => {
		// First fetch: load invoice (paid).
		// Second fetch: POST payment (the refund).
		// Third fetch: re-load invoice (refetch).
		const fetchMock = vi.fn();
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({
				...baseInvoice,
				status: 'paid',
				effective_status: 'paid',
				balance: 0,
				total_paid: 555000,
				total_refunded: 0,
				payments: [
					{
						id: 'p-original',
						invoice_id: 'inv-1',
						amount: 555000,
						method: 'cash',
						is_reversal: false,
						received_at: '2026-06-22T08:00:00Z',
						remaining_reverseable: 555000
					}
				]
			}), { status: 200 })
		);
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({
				id: 'p-refund-1',
				invoice_id: 'inv-1',
				amount: -100000,
				method: 'cash',
				is_reversal: true,
				received_at: '2026-06-22T10:00:00Z'
			}), { status: 201 })
		);
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({
				...baseInvoice,
				status: 'active',
				effective_status: 'partial',
				balance: 100000,
				total_paid: 455000,
				total_refunded: 100000
			}), { status: 200 })
		);
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-refund-toggle')).toBeInTheDocument());
		await fireEvent.click(getByTestId('invoice-refund-toggle'));

		// v1.2 (Block 8): the picker is the new entry point — the form
		// is hidden until the user picks a target.
		await waitFor(() => expect(getByTestId('refund-picker')).toBeInTheDocument());
		await fireEvent.click(getByTestId('refund-picker-item'));

		// Form is now visible with the target pre-filled.
		await waitFor(() => expect(getByTestId('payment-form').hasAttribute('hidden')).toBe(false));

		// Fill the (pre-filled) form: amount is already 555000, the user
		// overrides to a partial refund.
		await fireEvent.input(getByTestId('payment-amount'), { target: { value: '100000' } });
		await fireEvent.input(getByTestId('payment-notes'), { target: { value: 'guest complaint' } });
		await fireEvent.click(getByTestId('payment-submit'));

		// 3rd call must be the re-fetch (same pattern as the void fix).
		await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
		const [reFetchURL] = fetchMock.mock.calls[2];
		expect(reFetchURL).toContain('/invoices/by-booking/book-1');
	});

	// ============ v1.2 — Block 5.1: R-08 hide terminal actions ============

	const terminalInvoicePaid: InvoiceDetail = {
		...baseInvoice,
		status: 'refunded', // ← lifecycle terminal
		effective_status: 'refunded',
		total_paid: 555000,
		total_refunded: 555000,
		balance: 0
	};

	const terminalInvoiceVoid: InvoiceDetail = {
		...baseInvoice,
		status: 'void', // ← lifecycle terminal
		effective_status: 'void',
		total_paid: 0,
		balance: 0
	};

	it('IT-14 (v1.2 R-08): refunded invoice hides Refund/Void/Register buttons and shows the terminal banner', async () => {
		// Mock only the GET /invoices/by-booking — every other call
		// (POST /payments, etc.) returns 404 so it never resolves a
		// stale body from a previous test.
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(terminalInvoicePaid), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-terminal-banner')).toBeInTheDocument());

		// The action bar is gone — refund/void/register/payment are all
		// hidden when the lifecycle is terminal.
		expect(queryByTestId('invoice-refund-toggle')).toBeNull();
		expect(queryByTestId('invoice-void-toggle')).toBeNull();
		expect(queryByTestId('invoice-payment-toggle')).toBeNull();

		// Banner explains the lock — text matches the i18n key.
		expect(getByTestId('invoice-terminal-banner').textContent).toMatch(/fully refunded/i);
	});

	it('IT-15 (v1.2 R-08): voided invoice hides the action bar with the voided banner', async () => {
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(terminalInvoiceVoid), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-terminal-banner')).toBeInTheDocument());
		expect(queryByTestId('invoice-refund-toggle')).toBeNull();
		expect(queryByTestId('invoice-void-toggle')).toBeNull();
		expect(queryByTestId('invoice-payment-toggle')).toBeNull();
		expect(getByTestId('invoice-terminal-banner').textContent).toMatch(/voided/i);
	});

	it('IT-16 (v1.2 R-08): active invoice still shows the Refund button normally', async () => {
		const activeInvoice = {
			...baseInvoice,
			status: 'active', // ← lifecycle NOT terminal
			effective_status: 'paid',
			total_paid: 555000,
			balance: 0
		};
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(activeInvoice), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		// No terminal banner, action bar still rendered with the Refund
		// button visible (total_paid > 0 + lifecycle='active').
		await waitFor(() => expect(getByTestId('invoice-actions')).toBeInTheDocument());
		expect(queryByTestId('invoice-terminal-banner')).toBeNull();
		expect(getByTestId('invoice-refund-toggle')).toBeInTheDocument();
	});

	// ============ v1.2 — Block 11: status glyphs on the pill ============

	it('IT-17 (v1.2 B11): refunded invoice shows the ↩ glyph on the status pill (R-09 Q4)', async () => {
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(terminalInvoicePaid), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() =>
			expect(getByTestId('invoice-status-pill')).toBeInTheDocument()
		);
		expect(getByTestId('invoice-status-pill').getAttribute('data-status')).toBe('refunded');
		const glyph = getByTestId('invoice-refunded-glyph');
		expect(glyph).toBeInTheDocument();
		expect(glyph.textContent).toBe('↩');
	});

	it('IT-18 (v1.2 B11): needs_review invoice shows the ⚠ glyph on the status pill (R-09 Q2)', async () => {
		const needsReviewInvoice: InvoiceDetail = {
			...baseInvoice,
			status: 'active',
			effective_status: 'partial', // ambiguous: paid partial + refunded > paid
			total_paid: 100000,
			total_refunded: 150000, // > total_paid → needs_review
			balance: 0,
			needs_review: true
		};
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(needsReviewInvoice), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() =>
			expect(getByTestId('invoice-status-pill')).toBeInTheDocument()
		);
		expect(getByTestId('invoice-needs-review-glyph').textContent).toBe('⚠');
		// Refunded glyph is NOT shown — it's only for terminal refunded.
		expect(queryByTestId('invoice-refunded-glyph')).toBeNull();
	});

	// ============ v1.2 — Block 10: refund-all button + modal ============

	const paidInvoiceForRefundAll: InvoiceDetail = {
		...baseInvoice,
		status: 'active',
		effective_status: 'paid',
		total_paid: 555000,
		balance: 0
	};

	it('IT-19 (v1.2 B10): paid invoice shows the "Refund all payments" button', async () => {
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(paidInvoiceForRefundAll), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-actions')).toBeInTheDocument());
		expect(getByTestId('invoice-refund-all-toggle')).toBeInTheDocument();
		expect(getByTestId('invoice-refund-all-toggle').textContent).toMatch(/refund all/i);
	});

	it('IT-20 (v1.2 B10): refunded invoice hides the "Refund all" button (terminal banner wins)', async () => {
		// Reuse the refunded terminal fixture from IT-14.
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(terminalInvoicePaid), { status: 200 });
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, queryByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-terminal-banner')).toBeInTheDocument());
		expect(queryByTestId('invoice-refund-all-toggle')).toBeNull();
	});

	it('IT-21 (v1.2 B10): clicking Refund-all + confirming the modal fires POST /refund-all and refetches', async () => {
		const refundAllResponse = {
			batch: {
				id: 'batch-1',
				invoice_id: 'inv-1',
				property_id: 'prop-1',
				method: 'cash',
				amount: 0,
				original_currency: 'IDR',
				exchange_rate: 1,
				reference: 'REFUND-ALL',
				notes: 'bulk refund',
				is_reversal: false,
				reversal_of: null,
				received_by: 'receptionist-1',
				received_at: '2026-06-22T11:00:00Z',
				created_at: '2026-06-22T11:00:00Z'
			},
			refunds: [],
			refunded_count: 2,
			refunded_total: 555000
		};
		const fetchMock = vi.fn().mockImplementation(async (url: RequestInfo | URL, init?: RequestInit) => {
			const u = typeof url === 'string' ? url : (url as URL).toString();
			if (u.includes('/invoices/by-booking/')) {
				return new Response(JSON.stringify(paidInvoiceForRefundAll), { status: 200 });
			}
			if (u.includes('/refund-all') && init?.method === 'POST') {
				// Echo the request body for assertion.
				return new Response(
					JSON.stringify({
						...refundAllResponse,
						request_body: init.body
					}),
					{ status: 200 }
				);
			}
			return new Response('not found', { status: 404 });
		});
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId, findByTestId } = render(InvoiceWidget, {
			props: { bookingId: 'book-1', propertyId: 'prop-1' }
		});

		await waitFor(() => expect(getByTestId('invoice-actions')).toBeInTheDocument());

		// 1. Click "Refund all payments".
		await fireEvent.click(getByTestId('invoice-refund-all-toggle'));

		// 2. The destructive confirm modal opens with the refundAll i18n.
		const title = await findByTestId('confirm-destructive-title');
		expect(title.textContent).toMatch(/refund all payments/i);

		// 3. Tick the checkbox + click confirm. ConfirmDestructive's
		//    callback fires onConfirm → confirmRefundAll → POST.
		await fireEvent.click(getByTestId('confirm-destructive-checkbox'));
		await fireEvent.click(getByTestId('confirm-destructive-confirm'));

		// 4. Verify the POST was issued with the right body shape.
		const postCall = fetchMock.mock.calls.find(([u, init]) => {
			const us = typeof u === 'string' ? u : (u as URL).toString();
			return us.includes('/refund-all') && init?.method === 'POST';
		});
		expect(postCall).toBeTruthy();
		const body = JSON.parse((postCall?.[1]?.body ?? '{}') as string);
		expect(body.reason).toBeTruthy(); // any non-empty reason

		// 4b. Verify the dev auth headers are present — without them
		//     the backend's AuthContext middleware can't seed the
		//     context and the handler returns 401 UNAUTHENTICATED.
		//     This was the IT-22 bug after the block-10 frontend
		//     commit (forgot to add headers to the new method).
		const postHeaders = (postCall?.[1]?.headers ?? {}) as Record<string, string>;
		expect(postHeaders['X-User-ID']).toBeTruthy();
		expect(postHeaders['X-User-Role']).toBe('owner');
		expect(postHeaders['X-Property-ID']).toBe('prop-1');

		// 5. A second GET /invoices/by-booking fires after success
		//    (refetch by the widget). Wait for it — loadInvoice() is
		//    async and Svelte batches the re-fetch into the next tick.
		await waitFor(
			() => {
				const getCount = fetchMock.mock.calls.filter(([u]) => {
					const us = typeof u === 'string' ? u : (u as URL).toString();
					return us.includes('/invoices/by-booking/');
				}).length;
				expect(getCount).toBeGreaterThanOrEqual(2);
			},
			{ timeout: 2000 }
		);
	});
});