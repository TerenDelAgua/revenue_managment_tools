/**
 * csv.ts — minimal CSV builder + downloader
 *
 * Zero-dependency, works in any modern browser. Used by the reports
 * module (B8) to export daily summaries and monthly tax reports.
 *
 * Quirks handled
 *  - Always CRLF line endings (RFC 4180).
 *  - Cells are escaped if they contain `"`, `,`, `\n` or `\r`.
 *  - Empty / null / undefined become empty cells (not "null" / "undefined").
 *  - Numbers stay raw (no quoting) — most spreadsheet apps parse them.
 *  - `downloadCSV` triggers a browser download via an in-memory <a>.
 */

export type CSVCell = string | number | boolean | null | undefined;

export function escapeCell(value: CSVCell): string {
	if (value === null || value === undefined) return '';
	const s = String(value);
	// RFC 4180: quote when the cell contains a comma, quote, CR or LF.
	if (/[",\r\n]/.test(s)) {
		return `"${s.replace(/"/g, '""')}"`;
	}
	return s;
}

export function toCSV(headers: string[], rows: CSVCell[][]): string {
	const lines: string[] = [];
	lines.push(headers.map(escapeCell).join(','));
	for (const row of rows) {
		lines.push(row.map(escapeCell).join(','));
	}
	return lines.join('\r\n') + '\r\n';
}

/**
 * Triggers a browser download for the given CSV content. Adds a UTF-8 BOM
 * so Excel opens it with the right encoding by default.
 */
export function downloadCSV(filename: string, content: string): void {
	if (typeof document === 'undefined' || typeof URL === 'undefined') return;
	const blob = new Blob(['\uFEFF' + content], { type: 'text/csv;charset=utf-8' });
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	// Firefox requires the element to be in the DOM to honour `download`.
	document.body.appendChild(a);
	a.click();
	document.body.removeChild(a);
	// Defer revoke so the click handler can finish.
	setTimeout(() => URL.revokeObjectURL(url), 0);
}
