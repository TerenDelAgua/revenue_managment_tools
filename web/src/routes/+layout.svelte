<script lang="ts">
	import './layout.css';
	import AppShell from '$lib/layouts/AppShell.svelte';
	import OfflineWarning from '$lib/components/ui/OfflineWarning.svelte';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
<OfflineWarning />
<AppShell>
	{@render children()}
</AppShell>
