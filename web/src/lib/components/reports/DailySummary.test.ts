/**
 * DailySummary — vitest suite (B8)
 *
 * Covers the report's observable behaviour:
 *  - DT-01 Renders counts + totals from the API response
 *  - DT-02 Renders by-method breakdown from by_method map
 *  - DT-03 Renders staff breakdown table
 *  - DT-04 Triggers CSV download with right filename and content
 *  - DT-05 Empty by_method / staff render the calm empty states
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import DailySummary from './DailySummary.svelte';
import type { DailySummary as DailySummaryType } from '$lib/types';

const baseSummary: DailySummaryType = {
	date: '2026-06-22T00:00:00Z',
	property_id: 'prop-1',
	invoices_issued: 10,
	invoices_paid: 5,
	invoices_partial: 2,
	invoices_unpaid: 1,
	invoices_void: 1,
	invoices_overpaid: 1,
	total_revenue: 1200000,
	total_collected: 800000,
	total_refunded: 50000,
	total_pending: 350000,
	by_method: {
		cash: 500000,
		bank_transfer: 200000,
		qris: 100000
	},
	tax_collected: 80000,
	staff_breakdown: [
		{ user_id: 'u-1', user_name: 'Alice', payments_count: 4, amount_collected: 500000 },
		{ user_id: 'u-2', user_name: 'Bob', payments_count: 3, amount_collected: 300000 }
	]
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

describe('DailySummary', () => {
	it('DT-01: renders counts and totals from the API response', async () => {
		mockFetchOnce(baseSummary);
		const { getByTestId } = render(DailySummary, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(getByTestId('daily-count-paid').textContent?.trim()).toBe('5');
		});
		expect(getByTestId('daily-count-issued').textContent?.trim()).toBe('10');
		expect(getByTestId('daily-count-partial').textContent?.trim()).toBe('2');
		expect(getByTestId('daily-count-unpaid').textContent?.trim()).toBe('1');
		expect(getByTestId('daily-count-void').textContent?.trim()).toBe('1');
		expect(getByTestId('daily-count-overpaid').textContent?.trim()).toBe('1');
		expect(getByTestId('daily-collected').textContent).toContain('IDR 800.000');
		expect(getByTestId('daily-refunded').textContent).toContain('IDR 50.000');
		expect(getByTestId('daily-pending').textContent).toContain('IDR 350.000');
		expect(getByTestId('daily-tax').textContent).toContain('IDR 80.000');
	});

	it('DT-02: renders by-method breakdown', async () => {
		mockFetchOnce(baseSummary);
		const { getByTestId } = render(DailySummary, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(getByTestId('daily-by-method')).toBeInTheDocument();
		});
		const list = getByTestId('daily-by-method');
		expect(list).toHaveTextContent('Cash');
		expect(list).toHaveTextContent('Bank transfer');
		expect(list).toHaveTextContent('QRIS');
		expect(list).toHaveTextContent('IDR 500.000');
	});

	it('DT-03: renders staff breakdown table', async () => {
		mockFetchOnce(baseSummary);
		const { getByTestId } = render(DailySummary, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(getByTestId('daily-by-staff')).toBeInTheDocument();
		});
		const table = getByTestId('daily-by-staff');
		expect(table).toHaveTextContent('Alice');
		expect(table).toHaveTextContent('Bob');
		expect(table).toHaveTextContent('4');
		expect(table).toHaveTextContent('IDR 500.000');
	});

	it('DT-04: triggers CSV download with expected Blob content', async () => {
		mockFetchOnce(baseSummary);
		const createObjectURL = vi.fn<typeof URL.createObjectURL>(() => 'blob:mock');
		const origCreate = URL.createObjectURL;
		const origRevoke = URL.revokeObjectURL;
		URL.createObjectURL = createObjectURL;
		URL.revokeObjectURL = vi.fn();

		const { getByTestId } = render(DailySummary, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			const btn = getByTestId('daily-export-csv') as HTMLButtonElement;
			expect(btn).toBeInTheDocument();
			expect(btn.disabled).toBe(false);
		});

		await fireEvent.click(getByTestId('daily-export-csv'));
		expect(createObjectURL).toHaveBeenCalledOnce();
		const blobArg = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
		expect(blobArg.type).toBe('text/csv;charset=utf-8');
		const text = await blobArg.text();
		// The CSV body carries the data we expect — counts, totals, by-method
		// and staff. We strip the leading BOM and CRLF for the assertion.
		const body = text.replace(/^﻿/, '').replace(/\r/g, '');
		expect(body).toContain('Invoices issued,10');
		expect(body).toContain('Total collected,800000');
		expect(body).toContain('Cash,500000');
		expect(body).toContain('Alice,4,500000');

		URL.createObjectURL = origCreate;
		URL.revokeObjectURL = origRevoke;
	});

	it('DT-05: shows empty states when by_method and staff_breakdown are empty', async () => {
		mockFetchOnce({ ...baseSummary, by_method: {}, staff_breakdown: [] });
		const { container } = render(DailySummary, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(container).toHaveTextContent('No payments recorded for this day.');
		});
		expect(container).toHaveTextContent('No staff payments recorded.');
		expect(container.querySelector('[data-testid="daily-by-method"]')).toBeNull();
		expect(container.querySelector('[data-testid="daily-by-staff"]')).toBeNull();
	});
});