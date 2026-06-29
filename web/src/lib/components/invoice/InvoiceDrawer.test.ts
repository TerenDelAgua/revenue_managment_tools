/**
 * InvoiceDrawer — vitest suite (B9)
 *
 * Covers the drawer's observable behaviour:
 *  - ID-01 Loads the invoice via getByID and shows metadata + widget
 *  - ID-02 Closes on backdrop click
 *  - ID-03 Closes on Escape key
 *  - ID-04 Empty/error state when the invoice is missing
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import InvoiceDrawer from './InvoiceDrawer.svelte';
import type { InvoiceDetail } from '$lib/types';

const baseInvoice: InvoiceDetail = {
	id: 'inv-1234-5678',
	property_id: 'prop-1',
	booking_id: 'book-9',
	invoice_number: 'INV-2026-0009',
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
	pdf_url: 'https://example.com/inv-9.pdf',
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

beforeAll(() => {
	locale.set('en');
});

beforeEach(() => {
	vi.restoreAllMocks();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('InvoiceDrawer', () => {
	it('ID-01: loads the invoice and shows metadata + the embedded widget', async () => {
		mockFetchOnce(baseInvoice);
		const { container, getByTestId } = render(InvoiceDrawer, {
			props: {
				invoiceId: 'inv-1234-5678',
				propertyId: 'prop-1',
				isOpen: true,
				onClose: () => {}
			}
		});

		await waitFor(() => {
			expect(container).toHaveTextContent('INV-2026-0009');
		});

		// The widget is rendered too (with its own data-testid).
		expect(getByTestId('invoice-widget')).toBeInTheDocument();
	});

	it('ID-02: backdrop click fires onClose', async () => {
		mockFetchOnce(baseInvoice);
		const onClose = vi.fn();
		const { container } = render(InvoiceDrawer, {
			props: {
				invoiceId: 'inv-1234-5678',
				propertyId: 'prop-1',
				isOpen: true,
				onClose
			}
		});
		await waitFor(() => {
			expect(container).toHaveTextContent('INV-2026-0009');
		});
		// The backdrop is a <button aria-label="Close drawer">.
		const backdrop = container.querySelector('button[aria-label="Close drawer"]') as HTMLButtonElement;
		expect(backdrop).toBeTruthy();
		await fireEvent.click(backdrop);
		expect(onClose).toHaveBeenCalledOnce();
	});

	it('ID-03: Escape key fires onClose when open', async () => {
		mockFetchOnce(baseInvoice);
		const onClose = vi.fn();
		render(InvoiceDrawer, {
			props: {
				invoiceId: 'inv-1234-5678',
				propertyId: 'prop-1',
				isOpen: true,
				onClose
			}
		});
		await waitFor(() => {
			// Wait for the drawer to be mounted (escape listener attached).
		});
		await fireEvent.keyDown(window, { key: 'Escape' });
		expect(onClose).toHaveBeenCalledOnce();
	});

	it('ID-04: shows load error when the API fails', async () => {
		mockFetchOnce({ message: 'not found' }, 500);
		const { container } = render(InvoiceDrawer, {
			props: {
				invoiceId: 'inv-missing',
				propertyId: 'prop-1',
				isOpen: true,
				onClose: () => {}
			}
		});
		await waitFor(() => {
			expect(container).toHaveTextContent('Could not load this invoice.');
		});
		// Widget is NOT rendered when there's no invoice.
		expect(container.querySelector('[data-testid="invoice-widget"]')).toBeNull();
	});
});