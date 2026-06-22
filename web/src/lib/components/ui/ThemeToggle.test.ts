import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import { init, register } from 'svelte-i18n';
import ThemeToggle from './ThemeToggle.svelte';
import { themeStore } from '$lib/store/themeStore.svelte';

// svelte-i18n requires init() to be called before any $_ / $t usage.
// Register only the theme keys we need for these tests.
register('en', () =>
	Promise.resolve({
		'theme.toggle.light': 'Light theme',
		'theme.toggle.dark': 'Dark theme',
		'theme.toggle.system': 'Follow system theme'
	})
);
init({
	fallbackLocale: 'en',
	initialLocale: 'en'
});

describe('ThemeToggle Component', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.className = '';
		themeStore.setMode('light');
	});

	afterEach(() => {
		cleanup();
	});

	it('renders a button with the theme-toggle testid', () => {
		const { getByTestId } = render(ThemeToggle);
		const btn = getByTestId('theme-toggle');
		expect(btn).toBeInTheDocument();
		expect(btn.tagName).toBe('BUTTON');
	});

	it('shows the sun icon when mode is light', () => {
		themeStore.setMode('light');
		const { getByTestId } = render(ThemeToggle);
		expect(getByTestId('theme-toggle').textContent?.trim()).toBe('☀️');
	});

	it('shows the moon icon when mode is dark', () => {
		themeStore.setMode('dark');
		const { getByTestId } = render(ThemeToggle);
		expect(getByTestId('theme-toggle').textContent?.trim()).toBe('🌙');
	});

	it('shows the laptop icon when mode is system', () => {
		themeStore.setMode('system');
		const { getByTestId } = render(ThemeToggle);
		expect(getByTestId('theme-toggle').textContent?.trim()).toBe('💻');
	});

	it('cycles mode when clicked', async () => {
		themeStore.setMode('light');
		const { getByTestId } = render(ThemeToggle);

		await fireEvent.click(getByTestId('theme-toggle'));
		expect(themeStore.mode).toBe('dark');

		await fireEvent.click(getByTestId('theme-toggle'));
		expect(themeStore.mode).toBe('system');

		await fireEvent.click(getByTestId('theme-toggle'));
		expect(themeStore.mode).toBe('light');
	});

	it('has an aria-label for accessibility', () => {
		const { getByTestId } = render(ThemeToggle);
		const btn = getByTestId('theme-toggle');
		expect(btn).toHaveAttribute('aria-label');
		expect(btn.getAttribute('aria-label')).toBeTruthy();
	});

	it('icon span is hidden from screen readers via aria-hidden', () => {
		const { container } = render(ThemeToggle);
		const iconSpan = container.querySelector('[aria-hidden="true"]');
		expect(iconSpan).toBeInTheDocument();
	});

	it('has a focus-visible ring for keyboard navigation', () => {
		const { getByTestId } = render(ThemeToggle);
		const btn = getByTestId('theme-toggle');
		expect(btn.className).toContain('focus-visible:ring-2');
	});
});
