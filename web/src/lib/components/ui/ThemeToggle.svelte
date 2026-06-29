<script lang="ts">
	import { themeStore, type ThemeMode } from '$lib/store/themeStore.svelte';
	import { _ } from 'svelte-i18n';

	const icons: Record<ThemeMode, string> = {
		light: '☀️',
		dark: '🌙',
		system: '💻'
	};

	const labels: Record<ThemeMode, string> = {
		light: 'theme.toggle.light',
		dark: 'theme.toggle.dark',
		system: 'theme.toggle.system'
	};
</script>

<button
	type="button"
	class="flex items-center justify-center rounded-lg border border-teren-border-subtle w-9 h-9 text-base
		text-teren-text-muted transition hover:bg-teren-primary-subtle hover:text-teren-text-main
		focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teren-primary focus-visible:ring-offset-2"
	aria-label={$_(labels[themeStore.mode])}
	title={$_(labels[themeStore.mode])}
	onclick={() => themeStore.cycle()}
	data-testid="theme-toggle"
>
	<span aria-hidden="true">{icons[themeStore.mode]}</span>
</button>
