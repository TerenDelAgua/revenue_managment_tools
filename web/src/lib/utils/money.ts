/**
 * Money formatting helpers (IDR by default).
 *
 * Why this exists
 * ---------------
 * The DB stores amounts with 2-decimal precision (NUMERIC(14,2)) but
 * the previous in-component `formatMoney` implementations rounded to
 * the nearest integer with `Math.round(value).toString()`, hiding the
 * cents from the user. That mismatch is the cause of the v1.2
 * "balance shows IDR 722 but the form prefills 721.5" bug — the
 * invoice really is 721.50 (subtotal 650 + 11% tax) and the user has
 * no way to know because the display rounds.
 *
 * The fix: show 2 decimals whenever the value has a non-zero
 * fractional part (i.e. when there are cents to display). Integer
 * amounts stay clean ("650" not "650.00").
 *
 * Format
 * ------
 * IDR convention: "." thousand separator, "," decimal separator.
 *   650    → "IDR 650"
 *   721.5  → "IDR 721,50"
 *   555000 → "IDR 555.000"
 */

export interface FormatMoneyOptions {
	currency?: string;
	/** Force a fixed number of decimals (overrides the auto-detect). */
	decimals?: number;
}

export function formatMoney(value: number, options: FormatMoneyOptions = {}): string {
	const { currency = 'IDR', decimals } = options;

	const safe = Number.isFinite(value) ? value : 0;
	const useDecimals =
		typeof decimals === 'number'
			? decimals
			: Math.round(safe * 100) % 100 !== 0
				? 2
				: 0;

	const fixed = safe.toFixed(useDecimals);
	const [intPart, decPart] = fixed.split('.');
	const grouped = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
	return decPart ? `${currency} ${grouped},${decPart}` : `${currency} ${grouped}`;
}

/**
 * Round a money value to whole units (drop the cents). Used for
 * estimates / KPIs where cents are noise. NOT for invoice totals —
 * those should keep their decimals.
 */
export function roundToWhole(value: number): number {
	return Math.round(value);
}
