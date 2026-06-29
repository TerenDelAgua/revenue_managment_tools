/**
 * TEREN Design System v1.1 — Theme Store
 *
 * Runtime state for the active color mode. Backed by Svelte 5 runes
 * (`$state`, `$derived`) and persisted to localStorage.
 *
 * Three explicit modes: 'light', 'dark', 'system'.
 * One derived value: `resolved` (always 'light' or 'dark'), which is
 * the one consumers actually read.
 *
 * The <html> element gets `.light` / `.dark` / both-neither markers.
 * The bootstrap script in app.html applies the right class before the
 * first paint, so this store does not cause a FOUC on reload.
 */

export type ThemeMode = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

const STORAGE_KEY = 'teren-theme';
const MEDIA_QUERY = '(prefers-color-scheme: dark)';

function readStoredMode(): ThemeMode {
	if (typeof localStorage === 'undefined') return 'system';
	const stored = localStorage.getItem(STORAGE_KEY);
	if (stored === 'light' || stored === 'dark' || stored === 'system') {
		return stored;
	}
	return 'system';
}

function writeStoredMode(mode: ThemeMode): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY, mode);
	} catch {
		// localStorage may throw in private mode; degrade silently.
	}
}

function systemPrefersDark(): boolean {
	if (typeof window === 'undefined' || !window.matchMedia) return false;
	return window.matchMedia(MEDIA_QUERY).matches;
}

function applyDocumentClasses(mode: ThemeMode, resolved: ResolvedTheme): void {
	if (typeof document === 'undefined') return;
	const root = document.documentElement;
	root.classList.remove('light', 'dark');
	root.classList.add(resolved);
	if (mode === 'light') root.classList.add('light');
	if (mode === 'dark') root.classList.add('dark');
}

function createThemeStore() {
	let mode = $state<ThemeMode>(readStoredMode());
	let systemDark = $state<boolean>(systemPrefersDark());

	const resolved = $derived<ResolvedTheme>(
		mode === 'dark' || (mode === 'system' && systemDark) ? 'dark' : 'light'
	);

	// Sync the document with the initial state on the client only.
	if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
		// Defer until the derived value is computed.
		$effect.root(() => {
			$effect(() => {
				applyDocumentClasses(mode, resolved);
			});
		});

		// Keep systemDark in sync with the OS preference.
		const mql = window.matchMedia(MEDIA_QUERY);
		const onChange = (event: MediaQueryListEvent) => {
			systemDark = event.matches;
		};
		mql.addEventListener('change', onChange);
	}

	function setMode(next: ThemeMode): void {
		mode = next;
		writeStoredMode(next);
	}

	function cycle(): void {
		const order: ThemeMode[] = ['light', 'dark', 'system'];
		const idx = order.indexOf(mode);
		const next = order[(idx + 1) % order.length];
		setMode(next);
	}

	return {
		get mode() {
			return mode;
		},
		get resolved() {
			return resolved;
		},
		get systemDark() {
			return systemDark;
		},
		setMode,
		cycle
	};
}

export const themeStore = createThemeStore();
