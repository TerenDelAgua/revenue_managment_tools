import { describe, it, expect } from 'vitest';
import { formatMoney, roundToWhole } from './money';

describe('formatMoney', () => {
	it('renders integer amounts without trailing zeros', () => {
		expect(formatMoney(650)).toBe('IDR 650');
		expect(formatMoney(555000)).toBe('IDR 555.000');
		expect(formatMoney(0)).toBe('IDR 0');
	});

	it('renders fractional amounts with 2 decimals + IDR-style comma', () => {
		// The bug from the smoke session: 650 + 11% tax = 721.50.
		expect(formatMoney(721.5)).toBe('IDR 721,50');
		expect(formatMoney(721.5)).not.toBe('IDR 722'); // regression guard
		expect(formatMoney(71.5)).toBe('IDR 71,50');
		expect(formatMoney(555000.5)).toBe('IDR 555.000,50');
	});

	it('groups thousands with a dot regardless of decimals', () => {
		expect(formatMoney(1234567.89)).toBe('IDR 1.234.567,89');
		expect(formatMoney(1234567)).toBe('IDR 1.234.567');
	});

	it('accepts an explicit currency override', () => {
		expect(formatMoney(100, { currency: 'USD' })).toBe('USD 100');
		expect(formatMoney(100.5, { currency: 'USD' })).toBe('USD 100,50');
	});

	it('accepts a forced-decimals mode for KPIs that always want cents', () => {
		expect(formatMoney(650, { decimals: 2 })).toBe('IDR 650,00');
		expect(formatMoney(721.5, { decimals: 0 })).toBe('IDR 722'); // forced round
	});

	it('handles non-finite inputs gracefully', () => {
		expect(formatMoney(NaN)).toBe('IDR 0');
		expect(formatMoney(Infinity)).toBe('IDR 0');
	});
});

describe('roundToWhole', () => {
	it('rounds half-up at the .5 boundary', () => {
		expect(roundToWhole(721.5)).toBe(722);
		expect(roundToWhole(721.4)).toBe(721);
		expect(roundToWhole(0.5)).toBe(1);
	});
});
