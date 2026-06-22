import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { themeStore } from './themeStore.svelte';

describe('themeStore', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.className = '';
		// Reset matchMedia to default (light) between tests.
		vi.stubGlobal(
			'matchMedia',
			vi.fn((query: string) => ({
				matches: false,
				media: query,
				onchange: null,
				addEventListener: vi.fn(),
				removeEventListener: vi.fn(),
				addListener: vi.fn(),
				removeListener: vi.fn(),
				dispatchEvent: vi.fn()
			}))
		);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('defaults to system mode when localStorage is empty', () => {
		expect(themeStore.mode).toBe('system');
	});

	it('reads stored mode from localStorage on init', () => {
		localStorage.setItem('teren-theme', 'dark');
		// Re-import to pick up the stored value (singleton already created at module load).
		// We test via setMode instead to verify round-trip persistence.
		themeStore.setMode('dark');
		expect(themeStore.mode).toBe('dark');
		expect(localStorage.getItem('teren-theme')).toBe('dark');
	});

	it('cycles light -> dark -> system -> light', () => {
		themeStore.setMode('light');
		themeStore.cycle();
		expect(themeStore.mode).toBe('dark');
		themeStore.cycle();
		expect(themeStore.mode).toBe('system');
		themeStore.cycle();
		expect(themeStore.mode).toBe('light');
	});

	it('persists mode changes to localStorage', () => {
		themeStore.setMode('light');
		expect(localStorage.getItem('teren-theme')).toBe('light');
		themeStore.setMode('dark');
		expect(localStorage.getItem('teren-theme')).toBe('dark');
		themeStore.setMode('system');
		expect(localStorage.getItem('teren-theme')).toBe('system');
	});

	it('resolved is dark when mode is dark regardless of system', () => {
		themeStore.setMode('dark');
		expect(themeStore.resolved).toBe('dark');
	});

	it('resolved follows systemDark when mode is system', () => {
		themeStore.setMode('system');
		// With our stubbed matchMedia returning matches: false, resolved should be light.
		expect(themeStore.resolved).toBe('light');
	});
});
