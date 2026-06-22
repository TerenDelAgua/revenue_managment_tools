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
		expect(container.querySelector('[data-status="void"]')?.className).toContain('line-through');
	});
});