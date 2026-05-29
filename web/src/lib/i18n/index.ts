import { browser } from '$app/environment';
import { init, register, locale } from 'svelte-i18n';

const defaultLocale = 'en';

register('en', () => import('./locales/en.json'));
register('id', () => import('./locales/id.json'));

export function setupI18n() {
	let initialLocale = defaultLocale;
	
	if (browser) {
		const storedLocale = window.localStorage.getItem('locale');
		if (storedLocale) {
			initialLocale = storedLocale;
		} else {
			initialLocale = window.navigator.language.split('-')[0] === 'id' ? 'id' : 'en';
		}
		
		// Sync the svelte-i18n locale store changes to localStorage
		locale.subscribe((value) => {
			if (value) {
				window.localStorage.setItem('locale', value);
			}
		});
	}

	init({
		fallbackLocale: defaultLocale,
		initialLocale: initialLocale,
	});
}
