import { describe, it, expect, vi } from 'vitest';
import { toCSV, escapeCell, downloadCSV } from './csv';

describe('csv util', () => {
	describe('escapeCell', () => {
		it('returns empty string for null/undefined', () => {
			expect(escapeCell(null)).toBe('');
			expect(escapeCell(undefined)).toBe('');
		});

		it('passes through simple strings', () => {
			expect(escapeCell('hello')).toBe('hello');
		});

		it('quotes cells with commas', () => {
			expect(escapeCell('a,b')).toBe('"a,b"');
		});

		it('escapes embedded quotes and wraps in quotes', () => {
			expect(escapeCell('she said "hi"')).toBe('"she said ""hi"""');
		});

		it('quotes cells with newlines', () => {
			expect(escapeCell('line1\nline2')).toBe('"line1\nline2"');
		});

		it('numbers stay raw', () => {
			expect(escapeCell(42)).toBe('42');
			expect(escapeCell(3.14)).toBe('3.14');
		});
	});

	describe('toCSV', () => {
		it('builds a header + rows CSV with CRLF', () => {
			const csv = toCSV(['name', 'age'], [['alice', 30], ['bob', 25]]);
			expect(csv).toBe('name,age\r\nalice,30\r\nbob,25\r\n');
		});

		it('escapes problematic cells but keeps numbers raw', () => {
			const csv = toCSV(
				['desc', 'amount', 'method'],
				[['parking, extra', 100, 'cash']]
			);
			expect(csv).toBe('desc,amount,method\r\n"parking, extra",100,cash\r\n');
		});

		it('handles empty rows', () => {
			const csv = toCSV(['a', 'b'], []);
			expect(csv).toBe('a,b\r\n');
		});

		it('renders null/undefined cells as empty', () => {
			const csv = toCSV(['a', 'b', 'c'], [['x', null, undefined]]);
			expect(csv).toBe('a,b,c\r\nx,,\r\n');
		});
	});

	describe('downloadCSV', () => {
		it('creates a Blob and triggers a click on an anchor element', () => {
			const createObjectURL = vi.fn<typeof URL.createObjectURL>(() => 'blob:mock-url');
			const revokeObjectURL = vi.fn<typeof URL.revokeObjectURL>();
			const clickMock = vi.fn();
			const originalCreate = URL.createObjectURL;
			const originalRevoke = URL.revokeObjectURL;
			URL.createObjectURL = createObjectURL;
			URL.revokeObjectURL = revokeObjectURL;

			// Spy on document.createElement to capture the <a> element.
			const origCreate = document.createElement.bind(document);
			const createElSpy = vi
				.spyOn(document, 'createElement')
				.mockImplementation((tag: string) => {
					const el = origCreate(tag);
					if (tag === 'a') el.click = clickMock;
					return el;
				});

			downloadCSV('test.csv', 'a,b\r\n1,2\r\n');

			expect(createObjectURL).toHaveBeenCalledOnce();
			expect(clickMock).toHaveBeenCalledOnce();
			const blobArg = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
			expect(blobArg.type).toBe('text/csv;charset=utf-8');
			// BOM is prepended for Excel UTF-8 friendliness.
			blobArg.text().then((_text) => {
				// jsdom strips a leading U+FEFF from Blob.text(), so we read
				// the raw bytes instead and inspect the first one.
				return blobArg.arrayBuffer().then((buf) => {
					const bytes = new Uint8Array(buf);
					expect(bytes[0]).toBe(0xef);
					expect(bytes[1]).toBe(0xbb);
					expect(bytes[2]).toBe(0xbf);
				});
			});

			createElSpy.mockRestore();
			URL.createObjectURL = originalCreate;
			URL.revokeObjectURL = originalRevoke;
		});
	});
});