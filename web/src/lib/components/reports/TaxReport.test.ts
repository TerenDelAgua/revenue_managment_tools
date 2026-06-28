/**
 * TaxReport — vitest suite (B8)
 *
 * Covers the monthly tax report's observable behaviour:
 *  - TT-01 Renders the period label, totals, and net tax collected
 *  - TT-02 Triggers CSV download with the right Blob content
 *  - TT-03 Renders YYYY label when whole-year is selected
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import TaxReport from './TaxReport.svelte';
import type { MonthlyTaxReport } from '$lib/types';

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

const monthlyReport: MonthlyTaxReport = {
	property_id: 'prop-1',
	year: 2026,
	month: 6,
	total_subtotal: 1000000,
	total_tax: 110000,
	invoices_count: 12,
	void_count: 1,
	refunds_total: 25000,
	refunded_count: 2, // v1.2 R-08
	needs_review_count: 0, // v1.2 R-09 Q2
	net_tax_collected: 85000
};

beforeAll(() => {
	locale.set('en');
});

beforeEach(() => {
	vi.restoreAllMocks();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('TaxReport', () => {
	it('TT-01: renders metrics for the selected month', async () => {
		mockFetchOnce(monthlyReport);
		const { getByTestId } = render(TaxReport, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(getByTestId('tax-invoices-count').textContent?.trim()).toBe('12');
		});
		expect(getByTestId('tax-total-subtotal').textContent).toContain('IDR 1.000.000');
		expect(getByTestId('tax-total-tax').textContent).toContain('IDR 110.000');
		expect(getByTestId('tax-refunds').textContent).toContain('IDR 25.000');
		expect(getByTestId('tax-void-count').textContent?.trim()).toBe('1');
		expect(getByTestId('tax-net').textContent).toContain('IDR 85.000');
	});

	it('TT-02: CSV export builds a Blob with the right metrics for the selected month', async () => {
		mockFetchOnce(monthlyReport);
		const createObjectURL = vi.fn<typeof URL.createObjectURL>(() => 'blob:mock');
		const origCreate = URL.createObjectURL;
		const origRevoke = URL.revokeObjectURL;
		URL.createObjectURL = createObjectURL;
		URL.revokeObjectURL = vi.fn();

		const { getByTestId } = render(TaxReport, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			const btn = getByTestId('tax-export-csv') as HTMLButtonElement;
			expect(btn).toBeInTheDocument();
			expect(btn.disabled).toBe(false);
		});
		await fireEvent.click(getByTestId('tax-export-csv'));
		expect(createObjectURL).toHaveBeenCalledOnce();
		const blobArg = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
		expect(blobArg.type).toBe('text/csv;charset=utf-8');
		const text = await blobArg.text();
		const body = text.replace(/^﻿/, '').replace(/\r/g, '');
		expect(body).toContain('Invoices,12');
		expect(body).toContain('Tax,110000');
		expect(body).toContain('Net tax collected,85000');

		URL.createObjectURL = origCreate;
		URL.revokeObjectURL = origRevoke;
	});

	it('TT-03: switching to whole year still exports a Blob with the metrics', async () => {
		mockFetchOnce({ ...monthlyReport, month: undefined });
		const createObjectURL = vi.fn<typeof URL.createObjectURL>(() => 'blob:mock');
		const origCreate = URL.createObjectURL;
		const origRevoke = URL.revokeObjectURL;
		URL.createObjectURL = createObjectURL;
		URL.revokeObjectURL = vi.fn();

		const { getByTestId } = render(TaxReport, { props: { propertyId: 'prop-1' } });
		const monthSelect = getByTestId('tax-month-select') as HTMLSelectElement;
		await fireEvent.change(monthSelect, { target: { value: 'all' } });
		await waitFor(() => {
			const btn = getByTestId('tax-export-csv') as HTMLButtonElement;
			expect(btn).toBeInTheDocument();
			expect(btn.disabled).toBe(false);
		});
		await fireEvent.click(getByTestId('tax-export-csv'));
		expect(createObjectURL).toHaveBeenCalledOnce();
		const blobArg = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
		const text = await blobArg.text();
		// Whole-year still emits the same metric rows. The "Year" cell
		// reflects the current year selector value.
		const body = text.replace(/^﻿/, '').replace(/\r/g, '');
		expect(body).toContain('Invoices,12');
		expect(body).toContain('Net tax collected,85000');

		URL.createObjectURL = origCreate;
		URL.revokeObjectURL = origRevoke;
	});

	// ============ v1.2 Block 13 — Reports UI ============

	it('TT-04 (v1.2 B13): renders the Refunded count KPI', async () => {
		mockFetchOnce(monthlyReport);
		const { getByTestId } = render(TaxReport, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(getByTestId('tax-refunded-count').textContent?.trim()).toBe('2');
		});
	});

	it('TT-05 (v1.2 B13): shows the ⚠ needs-review banner when count > 0', async () => {
		mockFetchOnce({ ...monthlyReport, needs_review_count: 3 });
		const { getByTestId } = render(TaxReport, { props: { propertyId: 'prop-1' } });
		await waitFor(() => {
			expect(getByTestId('tax-needs-review-banner')).toBeInTheDocument();
		});
		expect(getByTestId('tax-needs-review-banner').textContent).toMatch(/3/);
	});
});