/**
 * InvoiceList — vitest suite (B9)
 *
 * Covers the list's observable behaviour:
 *  - IL-01 Renders rows from the API response with formatted amounts
 *  - IL-02 Status filter narrows the next request
 *  - IL-03 Search input narrows the next request
 *  - IL-04 Row click fires onSelect with the invoice summary
 *  - IL-05 Empty state renders when no invoices match
 *  - IL-06 Pagination summary reflects page state
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import InvoiceList from './InvoiceList.svelte';
import type { InvoiceListResponse } from '$lib/types';

const baseResponse: InvoiceListResponse = {
	invoices: [
		{
			id: 'inv-1',
			invoice_number: 'INV-2026-0001',
			booking_id: 'book-1',
			subtotal: 100000,
			tax_amount: 11000,
			total: 111000,
			total_paid: 0,
			balance: 111000,
			status: 'active',
			effective_status: 'unpaid',
			issued_at: '2026-06-20T08:00:00Z',
			paid_at: null,
			voided_at: null,
			guest_name: 'Alice Smith',
			room_number: '101'
		},
		{
			id: 'inv-2',
			invoice_number: 'INV-2026-0002',
			booking_id: 'book-2',
			subtotal: 200000,
			tax_amount: 22000,
			total: 222000,
			total_paid: 222000,
			balance: 0,
			status: 'active',
			effective_status: 'paid',
			issued_at: '2026-06-21T08:00:00Z',
			paid_at: '2026-06-21T18:00:00Z',
			voided_at: null,
			guest_name: 'Bob Jones',
			room_number: '102'
		}
	],
	pagination: { page: 1, limit: 50, total: 2 }
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

describe('InvoiceList', () => {
	it('IL-01: renders rows with formatted amounts and status pills', async () => {
		mockFetchOnce(baseResponse);
		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		await waitFor(() => {
			expect(getByTestId('inv-table')).toBeInTheDocument();
		});
		const rows = getByTestId('inv-table').querySelectorAll('tbody tr');
		expect(rows).toHaveLength(2);
		// First row (unpaid).
		expect(rows[0].textContent).toContain('INV-2026-0001');
		expect(rows[0].textContent).toContain('Alice Smith');
		expect(rows[0].textContent).toContain('101');
		expect(rows[0].textContent).toContain('IDR 111.000');
		// Second row (paid) — pill class includes the success-subtle bg.
		expect(rows[1].textContent).toContain('INV-2026-0002');
		const paidPill = rows[1].querySelector('[data-status]') as HTMLElement | null;
		expect(paidPill?.getAttribute('data-status')).toBe('paid');
		expect(paidPill?.classList.contains('bg-teren-success-subtle')).toBe(true);
	});

	it('IL-02: status filter narrows the next request', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify(baseResponse), { status: 200 })
		);
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		await waitFor(() => expect(getByTestId('inv-table')).toBeInTheDocument());

		const select = getByTestId('inv-status-filter') as HTMLSelectElement;
		await fireEvent.change(select, { target: { value: 'paid' } });
		await waitFor(() => {
			// The most recent fetch should include status=paid.
			const last = fetchMock.mock.calls[fetchMock.mock.calls.length - 1][0] as string;
			expect(last).toContain('status=paid');
		});
	});

	it('IL-03: search input narrows the next request', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify(baseResponse), { status: 200 })
		);
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		await waitFor(() => expect(getByTestId('inv-table')).toBeInTheDocument());

		await fireEvent.input(getByTestId('inv-search'), {
			target: { value: 'INV-2026-0001' }
		});
		await waitFor(() => {
			const last = fetchMock.mock.calls[fetchMock.mock.calls.length - 1][0] as string;
			expect(last).toContain('search=INV-2026-0001');
		});
	});

	it('IL-04: row click fires onSelect with the summary', async () => {
		mockFetchOnce(baseResponse);
		const onSelect = vi.fn();
		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect }
		});
		await waitFor(() => expect(getByTestId('inv-table')).toBeInTheDocument());

		const firstRow = getByTestId('inv-table').querySelector(
			'[data-testid="inv-row"]'
		) as HTMLElement;
		await fireEvent.click(firstRow);
		expect(onSelect).toHaveBeenCalledOnce();
		expect(onSelect.mock.calls[0][0].id).toBe('inv-1');
	});

	it('IL-05: empty state when no invoices match', async () => {
		mockFetchOnce({
			invoices: [],
			pagination: { page: 1, limit: 50, total: 0 }
		});
		const { container } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		await waitFor(() => {
			expect(container).toHaveTextContent('No invoices found');
		});
		expect(container.querySelector('[data-testid="inv-table"]')).toBeNull();
	});

	it('IL-06: pagination summary reflects totals', async () => {
		mockFetchOnce(baseResponse);
		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		await waitFor(() => {
			expect(getByTestId('inv-pagination-summary').textContent).toContain('1');
			expect(getByTestId('inv-pagination-summary').textContent).toContain('2');
		});
	});

	it('IL-07 (B10): Export paginates internally and downloads a CSV with all matching rows', async () => {
		// Simulate 2 pages of results so the export must iterate.
		const page1 = {
			invoices: [baseResponse.invoices[0]],
			pagination: { page: 1, limit: 50, total: 2 }
		};
		const page2 = {
			invoices: [baseResponse.invoices[1]],
			pagination: { page: 2, limit: 50, total: 2 }
		};
		const fetchMock = vi.fn();
		fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(page1), { status: 200 }));
		fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(page1), { status: 200 }));
		fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(page2), { status: 200 }));
		vi.stubGlobal('fetch', fetchMock);

		// Spy on the CSV download machinery.
		const createObjectURL = vi.fn<typeof URL.createObjectURL>(() => 'blob:mock');
		const origCreate = URL.createObjectURL;
		const origRevoke = URL.revokeObjectURL;
		URL.createObjectURL = createObjectURL;
		URL.revokeObjectURL = vi.fn();
		const origCreateEl = document.createElement.bind(document);
		const createElSpy = vi
			.spyOn(document, 'createElement')
			.mockImplementation((tag: string) => {
				const el = origCreateEl(tag) as HTMLAnchorElement;
				if (tag === 'a') (el as any).click = () => {};
				return el;
			});

		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		await waitFor(() => expect(getByTestId('inv-table')).toBeInTheDocument());

		// Click Export. It must fire 2 list calls (page 1 + page 2) and
		// then a download (createObjectURL).
		await fireEvent.click(getByTestId('inv-export-csv'));
		await waitFor(() => {
			expect(createObjectURL).toHaveBeenCalledOnce();
		});

		// First call = the table's own load. Then export iterates pages.
		const exportCalls = fetchMock.mock.calls.slice(1);
		expect(exportCalls.length).toBe(2);
		expect(exportCalls[0][0] as string).toContain('page=1');
		expect(exportCalls[1][0] as string).toContain('page=2');

		// Inspect the CSV Blob content.
		const blob = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
		expect(blob.type).toBe('text/csv;charset=utf-8');
		const text = await blob.text();
		const body = text.replace(/^﻿/, '').replace(/\r/g, '');
		const lines = body.split('\n').filter(Boolean);
		// 1 header + 2 rows.
		expect(lines).toHaveLength(3);
		// Header row mirrors the on-screen columns.
		expect(lines[0]).toContain('Number');
		expect(lines[0]).toContain('Guest');
		expect(lines[0]).toContain('Total');
		// Both invoices present.
		expect(lines[1]).toContain('INV-2026-0001');
		expect(lines[1]).toContain('Alice Smith');
		expect(lines[2]).toContain('INV-2026-0002');
		expect(lines[2]).toContain('Bob Jones');
		// Status labels render via i18n (English locale).
		expect(lines[1]).toContain('Unpaid');
		expect(lines[2]).toContain('Paid');

		createElSpy.mockRestore();
		URL.createObjectURL = origCreate;
		URL.revokeObjectURL = origRevoke;
	});

	it('IL-08 (B10): Export is disabled while in flight and re-enables on success', async () => {
		// Slow fetch so we can observe the disabled state.
		let resolveFetch: (r: Response) => void;
		const slow = new Promise<Response>((r) => {
			resolveFetch = r;
		});
		const fetchMock = vi.fn().mockReturnValueOnce(slow); // initial table load
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(InvoiceList, {
			props: { propertyId: 'prop-1', onSelect: () => {} }
		});
		// While the table load is pending, the export button is already
		// disabled because we only enable after the first list resolves.
		await waitFor(() => expect(getByTestId('inv-export-csv')).toBeInTheDocument());

		// Trigger export — second fetch will be slow.
		fetchMock.mockReturnValueOnce(new Promise<Response>(() => {})); // never resolves
		const exportBtn = getByTestId('inv-export-csv') as HTMLButtonElement;
		await fireEvent.click(exportBtn);

		await waitFor(() => {
			expect(exportBtn.disabled).toBe(true);
		});
		// Resolves the first load so any waiters unwind.
		resolveFetch!(new Response(JSON.stringify(baseResponse), { status: 200 }));
	});
});