import '@testing-library/jest-dom/vitest';
import { vi, beforeAll } from 'vitest';
import { init, register } from 'svelte-i18n';
import en from '$lib/i18n/locales/en.json';
import es from '$lib/i18n/locales/es.json';
import id from '$lib/i18n/locales/id.json';

/* ------------------------------------------------------------------ */
/* svelte-i18n bootstrap                                               */
/* Components that use $_ / formatMessage need an initial locale. We   */
/* register all three supported locales with the full message map so   */
/* tests can assert on real translated strings.                       */
/* ------------------------------------------------------------------ */
beforeAll(() => {
	register('en', () => Promise.resolve(en));
	register('es', () => Promise.resolve(es));
	register('id', () => Promise.resolve(id));
	init({
		fallbackLocale: 'en',
		initialLocale: 'en'
	});
});

/* ------------------------------------------------------------------ */
/* $env/dynamic/public mock                                            */
/* SvelteKit's runtime env module is virtual — vitest cannot resolve  */
/* it without a stub. We mock it with an empty PUBLIC_API_URL so the   */
/* api client falls back to its hardcoded default                     */
/* (http://localhost:8080/api/v1).                                    */
/* ------------------------------------------------------------------ */
vi.mock('$env/dynamic/public', () => ({
	env: {
		PUBLIC_API_URL: ''
	}
}));

/* ------------------------------------------------------------------ */
/* matchMedia stub                                                     */
/* jsdom does not implement matchMedia. Tests that need a different    */
/* default can override this stub via vi.stubGlobal / vi.spyOn.       */
/* ------------------------------------------------------------------ */
if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		value: vi.fn((query: string) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn()
		}))
	});
}
