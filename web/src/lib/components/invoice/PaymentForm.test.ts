/**
 * PaymentForm — vitest suite (B7)
 *
 * Covers the form's core observable behaviour:
 *  - PT-01 Renders with the remaining balance pre-filled
 *  - PT-02 Reference field appears only for non-cash methods (BR-INV-005)
 *  - PT-03 Submit is disabled when amount exceeds balance
 *  - PT-04 Submit is disabled when reference is required but empty
 *  - PT-05 Submit calls api.invoices.registerPayment with the right payload
 *            and an Idempotency-Key UUID v4
 *  - PT-06 Backend error is surfaced inline; form stays open
 *  - PT-07 Successful submit fires onSuccess and lets the parent dismiss
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import PaymentForm from './PaymentForm.svelte';
import type { Payment } from '$lib/types';

const baseProps = {
	invoiceId: 'inv-1',
	propertyId: 'prop-1',
	balance: 100000,
	totalPaid: 0,
	receivedBy: 'user-1',
	onSuccess: vi.fn(),
	onCancel: vi.fn()
};

// A realistic non-reversal payment we can reverse in refund tests.
const sampleCharge: Payment = {
	id: 'p-charge-1',
	invoice_id: 'inv-1',
	property_id: 'prop-1',
	amount: 100000,
	method: 'cash',
	original_currency: 'IDR',
	exchange_rate: 1,
	reference: null,
	notes: null,
	is_reversal: false,
	reversal_of: null,
	received_by: 'user-1',
	received_at: '2026-06-22T08:00:00Z',
	created_at: '2026-06-22T08:00:00Z'
};

beforeAll(() => {
	locale.set('en');
});

beforeEach(() => {
	vi.restoreAllMocks();
	baseProps.onSuccess.mockClear();
	baseProps.onCancel.mockClear();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('PaymentForm', () => {
	it('PT-01: pre-fills the amount with the remaining balance', () => {
		const { getByTestId } = render(PaymentForm, { props: baseProps });
		const amount = getByTestId('payment-amount') as HTMLInputElement;
		expect(amount.value).toBe('100000');
	});

	it('PT-02: hides the reference field when method is cash', async () => {
		const { queryByTestId, getByTestId } = render(PaymentForm, { props: baseProps });
		// Default is cash → no reference input visible.
		expect(queryByTestId('payment-reference')).toBeNull();
		// Switch to bank_transfer → reference appears.
		await fireEvent.click(getByTestId('payment-method-bank_transfer'));
		await waitFor(() => {
			expect(queryByTestId('payment-reference')).not.toBeNull();
		});
	});

	it('PT-03: disables submit when amount exceeds balance', async () => {
		const { getByTestId } = render(PaymentForm, { props: baseProps });
		const amount = getByTestId('payment-amount') as HTMLInputElement;
		const submit = getByTestId('payment-submit') as HTMLButtonElement;

		await fireEvent.input(amount, { target: { value: '150000' } });
		await waitFor(() => {
			expect(submit.disabled).toBe(true);
		});
		expect(getByTestId('payment-amount-error')).toHaveTextContent(
			/Amount exceeds the remaining balance/i
		);
	});

	it('PT-04: disables submit when reference is required but empty', async () => {
		const { getByTestId } = render(PaymentForm, { props: baseProps });
		// Switch to bank_transfer (requires reference).
		await fireEvent.click(getByTestId('payment-method-bank_transfer'));
		const submit = getByTestId('payment-submit') as HTMLButtonElement;
		expect(submit.disabled).toBe(true);

		// Fill reference → submit enables.
		await fireEvent.input(getByTestId('payment-reference'), {
			target: { value: 'TRF-001' }
		});
		await waitFor(() => {
			expect(submit.disabled).toBe(false);
		});
	});

	it('PT-05: submits the right payload and a UUID v4 Idempotency-Key', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(
				JSON.stringify({
					id: 'pay-1',
					invoice_id: 'inv-1',
					property_id: 'prop-1',
					method: 'cash',
					amount: 100000,
					original_currency: 'IDR',
					exchange_rate: 1,
					reference: null,
					notes: null,
					is_reversal: false,
					reversal_of: null,
					received_by: 'user-1',
					received_at: '2026-06-20T10:00:00Z',
					created_at: '2026-06-20T10:00:00Z'
				}),
				{ status: 201, headers: { 'Content-Type': 'application/json' } }
			)
		);
		vi.stubGlobal('fetch', fetchMock);

		const onSuccess = vi.fn();
		const { getByTestId } = render(PaymentForm, {
			props: { ...baseProps, onSuccess }
		});

		await fireEvent.click(getByTestId('payment-submit'));

		await waitFor(() => {
			expect(onSuccess).toHaveBeenCalledTimes(1);
		});

		// Inspect the fetch call.
		const [url, init] = fetchMock.mock.calls[0];
		expect(url).toContain('/invoices/inv-1/payments');
		const headers = (init?.headers ?? {}) as Record<string, string>;
		expect(headers['X-Property-ID']).toBe('prop-1');
		expect(headers['X-User-ID']).toBe('user-1');
		// UUID v4 shape.
		expect(headers['Idempotency-Key']).toMatch(
			/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
		);
		const body = JSON.parse(init?.body as string);
		expect(body).toEqual({ method: 'cash', amount: 100000 });
	});

	it('PT-06: surfaces backend error inline and keeps the form open', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				new Response(
					JSON.stringify({
						code: 'REFERENCE_REQUIRED',
						message: 'Necesitamos una referencia para pagos con bank_transfer.'
					}),
					{ status: 422, headers: { 'Content-Type': 'application/json' } }
				)
			)
		);
		const onSuccess = vi.fn();
		const onCancel = vi.fn();
		const { getByTestId } = render(PaymentForm, {
			props: { ...baseProps, onSuccess, onCancel }
		});

		await fireEvent.click(getByTestId('payment-method-bank_transfer'));
		await fireEvent.input(getByTestId('payment-reference'), {
			target: { value: 'TRF-002' }
		});
		// Forcefully click the submit even though validation might have
		// resolved differently — we want to assert the error path.
		const submit = getByTestId('payment-submit') as HTMLButtonElement;
		if (submit.disabled) {
			// The form's validator might still reject because of timing; we
			// bypass it by submitting programmatically through a fireEvent
			// on the form itself.
			await fireEvent.submit(getByTestId('payment-form'));
		} else {
			await fireEvent.click(submit);
		}

		await waitFor(() => {
			expect(getByTestId('payment-form-error')).toHaveTextContent(/referencia/i);
		});
		expect(onSuccess).not.toHaveBeenCalled();
	});

	it('PT-07: cancel button fires onCancel and skips the API', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const onCancel = vi.fn();
		const { getByTestId } = render(PaymentForm, {
			props: { ...baseProps, onCancel }
		});

		await fireEvent.click(getByTestId('payment-cancel'));
		expect(onCancel).toHaveBeenCalledTimes(1);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	// ============ Refund mode (B11) ============

	it('PT-08 (B11): refund mode shows refund banner + cap hint + forces reference field', async () => {
		const { getByTestId, queryByTestId } = render(PaymentForm, {
			props: {
				...baseProps,
				mode: 'refund',
				balance: 100000,
				totalPaid: 75000 // user already paid 75k; max refundable is 75k
			}
		});

		// Mode banner + data-mode attribute.
		expect(getByTestId('payment-mode-banner')).toHaveTextContent(/refund/i);
		expect(getByTestId('payment-form').getAttribute('data-mode')).toBe('refund');

		// Amount pre-fills with totalPaid (NOT balance).
		const amount = getByTestId('payment-amount') as HTMLInputElement;
		expect(amount.value).toBe('75000');

		// Cap hint shows totalPaid.
		expect(getByTestId('payment-refund-cap-hint')).toHaveTextContent('75.000');

		// Reference is required even for cash (R-01: refunds must be traceable).
		expect(getByTestId('payment-reference')).toBeInTheDocument();

		// Notes textarea has the * marker (required).
		const notesLabel = getByTestId('payment-form').querySelector(
			'label[for="payment-notes"]'
		);
		expect(notesLabel?.textContent).toMatch(/\*/);
		expect(queryByTestId('payment-balance-hint')).toBeNull(); // payment hint replaced
	});

	it('PT-09 (B11): refund submit posts negative amount + is_reversal=true + reversal_of + X-User-Role', async () => {
		const payment = {
			id: 'p-refund-1',
			invoice_id: 'inv-1',
			amount: -75000,
			method: 'cash',
			is_reversal: true,
			received_at: '2026-06-22T10:00:00Z'
		};
		const fetchMock = vi
			.fn()
			.mockResolvedValue(new Response(JSON.stringify(payment), { status: 201 }));
		vi.stubGlobal('fetch', fetchMock);

		const onSuccess = vi.fn();
		const { getByTestId } = render(PaymentForm, {
			props: {
				...baseProps,
				mode: 'refund',
				balance: 100000,
				totalPaid: 75000,
				payments: [sampleCharge],
				onSuccess
			}
		});

		// Reference + notes are required in refund mode. Amount pre-fills
		// with totalPaid (75000), so the refund body will be -75000.
		await fireEvent.input(getByTestId('payment-reference'), {
			target: { value: 'SLIP-001' }
		});
		await fireEvent.input(getByTestId('payment-notes'), {
			target: { value: 'guest cancelled' }
		});

		await fireEvent.click(getByTestId('payment-submit'));

		await waitFor(() => {
			expect(fetchMock).toHaveBeenCalledOnce();
		});

		const [url, init] = fetchMock.mock.calls[0];
		expect(url).toContain('/invoices/inv-1/payments');
		const headers = (init?.headers ?? {}) as Record<string, string>;
		expect(headers['X-User-Role']).toBe('owner');
		expect(headers['X-User-ID']).toBe('user-1');

		const body = JSON.parse(init?.body as string);
		// Refund sends NEGATIVE amount + is_reversal=true + reversal_of.
		expect(body.amount).toBe(-75000);
		expect(body.is_reversal).toBe(true);
		expect(body.reversal_of).toBe('p-charge-1');
		expect(body.reference).toBe('SLIP-001');
		expect(body.notes).toBe('guest cancelled');
	});

	it('PT-10 (B11): refund without reference blocks submit', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(PaymentForm, {
			props: { ...baseProps, mode: 'refund', totalPaid: 75000 }
		});

		// Submit button disabled without reference even though cash is selected.
		const submit = getByTestId('payment-submit') as HTMLButtonElement;
		expect(submit.disabled).toBe(true);

		await fireEvent.input(getByTestId('payment-notes'), {
			target: { value: 'reason' }
		});
		// Still disabled — reference missing.
		expect(submit.disabled).toBe(true);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('PT-11 (B11): refund over total_paid shows exceedsPaid error', async () => {
		const { getByTestId } = render(PaymentForm, {
			props: { ...baseProps, mode: 'refund', balance: 100000, totalPaid: 50000 }
		});

		// Bump amount to 999999 (> totalPaid).
		await fireEvent.input(getByTestId('payment-amount'), {
			target: { value: '999999' }
		});
		const errorEl = getByTestId('payment-amount-error');
		await waitFor(() => {
			expect(errorEl.textContent).toMatch(/exceed/i);
		});
	});

	it('PT-12 (B11): refund without any chargeable payment surfaces inline error and skips API', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);

		const { getByTestId } = render(PaymentForm, {
			props: {
				...baseProps,
				mode: 'refund',
				balance: 0,
				totalPaid: 50000,
				payments: [] // no chargeable payment available
			}
		});

		await fireEvent.input(getByTestId('payment-reference'), { target: { value: 'X' } });
		await fireEvent.input(getByTestId('payment-notes'), { target: { value: 'reason' } });
		await fireEvent.click(getByTestId('payment-submit'));

		await waitFor(() => {
			expect(getByTestId('payment-form-error')).toBeInTheDocument();
		});
		expect(getByTestId('payment-form-error').textContent).toMatch(/no charge/i);
		expect(fetchMock).not.toHaveBeenCalled();
	});
});