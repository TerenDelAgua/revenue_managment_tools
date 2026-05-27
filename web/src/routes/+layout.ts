import { browser } from '$app/environment';
import { setupI18n } from '$lib/i18n';
import { waitLocale } from 'svelte-i18n';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async () => {
	if (browser) {
		setupI18n();
	} else {
        // on server, we might setup i18n without relying on window/navigator.
        // For simple CSR or SSR fallback we still need setup.
        setupI18n();
    }
	await waitLocale();
};
