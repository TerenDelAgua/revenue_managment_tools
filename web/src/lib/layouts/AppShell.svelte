<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';

	import { onMount } from 'svelte';
	import LanguageSwitcher from '$lib/components/LanguageSwitcher.svelte';
	import ThemeToggle from '$lib/components/ui/ThemeToggle.svelte';
	import { _ } from 'svelte-i18n';

	interface Props {
		children?: import('svelte').Snippet;
	}

	let { children }: Props = $props();

	const navItems = [
		{ icon: '📊', labelKey: 'nav.dashboard', href: '/' },
		{ icon: '🗺️', labelKey: 'nav.floorMap', href: '/map' },
		{ icon: '📅', labelKey: 'nav.bookings', href: '/bookings' },
		{ icon: '🧾', labelKey: 'nav.invoices', href: '/invoices' },
		{ icon: '👥', labelKey: 'nav.guests', href: '/guests' },
		{ icon: '📈', labelKey: 'nav.reports', href: '/reports' },
		{ icon: '⚙️', labelKey: 'nav.settings', href: '/settings' }
	] as const;

	let isCollapsed = $state(false);

	onMount(() => {
		// Collapse by default on tablet/mobile screens
		if (window.innerWidth < 1024) {
			isCollapsed = true;
		}

		// Optional: listen to window resize
		const handleResize = () => {
			if (window.innerWidth < 1024) {
				isCollapsed = true;
			} else {
				isCollapsed = false;
			}
		};
		window.addEventListener('resize', handleResize);
		return () => window.removeEventListener('resize', handleResize);
	});

	function toggleSidebar() {
		isCollapsed = !isCollapsed;
	}
</script>

<div class="flex h-screen bg-teren-background-base font-sans text-teren-text-main">
	<!-- Sidebar — intentionally inverted surface, stays dark in both themes -->
	<aside
		class="z-20 flex flex-col bg-teren-sidebar-bg text-teren-sidebar-fg shadow-xl transition-all duration-300 ease-in-out shrink-0
		{isCollapsed ? 'w-16' : 'w-64'}"
	>
		<!-- Header / Logo -->
		<div class="border-b border-teren-sidebar-border p-4 flex items-center h-16 transition-all duration-300 {isCollapsed ? 'justify-center' : 'justify-between'}">
			{#if !isCollapsed}
				<div class="flex flex-col whitespace-nowrap overflow-hidden">
					<h1 class="text-xl font-bold tracking-tight text-teren-primary">TEREN</h1>
					<p class="text-[9px] text-teren-sidebar-muted">{$_('header.subtitle')}</p>
				</div>
			{/if}
			<button
				onclick={toggleSidebar}
				class="rounded-lg p-1.5 hover:bg-teren-sidebar-hover text-teren-sidebar-muted hover:text-teren-sidebar-fg transition-colors"
				aria-label="Toggle Sidebar"
			>
				{#if isCollapsed}
					<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
					</svg>
				{:else}
					<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
						<path fill-rule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clip-rule="evenodd" />
					</svg>
				{/if}
			</button>
		</div>

		<!-- Nav items -->
		<nav class="flex-1 space-y-1 p-2 md:p-3">
			{#each navItems as item (item.labelKey)}
				<a
					href={resolve(item.href)}
					class="flex items-center gap-3 rounded-lg px-3 py-2.5 transition-all duration-200
					{isCollapsed ? 'justify-center px-0' : ''}
					{page.url.pathname === item.href || (page.url.pathname.startsWith(item.href) && item.href !== '/')
						? 'bg-teren-primary/10 font-semibold text-teren-primary'
						: 'text-teren-sidebar-muted hover:bg-teren-sidebar-hover hover:text-teren-sidebar-fg'}"
					title={isCollapsed ? $_(item.labelKey) : ''}
				>
					<span class="text-lg transition-transform duration-200 hover:scale-110 shrink-0">{item.icon}</span>
					{#if !isCollapsed}
						<span class="text-sm whitespace-nowrap overflow-hidden text-ellipsis">{$_(item.labelKey)}</span>
					{/if}
				</a>
			{/each}
		</nav>

		<!-- Footer / Profile -->
		<div class="border-t border-teren-sidebar-border p-3 bg-teren-sidebar-footer-bg transition-all duration-300">
			<div class="flex items-center gap-3 {isCollapsed ? 'justify-center' : ''}">
				<div
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-teren-primary text-sm font-bold text-white shadow-sm"
				>
					JD
				</div>
				{#if !isCollapsed}
					<div class="overflow-hidden whitespace-nowrap">
						<p class="text-sm font-medium">Juan Del Agua</p>
						<p class="text-xs text-teren-sidebar-muted overflow-hidden text-ellipsis">{$_('header.hotelOwner')}</p>
					</div>
				{/if}
			</div>
		</div>
	</aside>

	<!-- Main Content -->
	<main class="flex flex-1 flex-col overflow-hidden">
		<!-- Topbar -->
		<header
			class="flex h-16 items-center justify-between border-b border-teren-border-subtle bg-teren-surface-base px-4 md:px-6 shadow-sm shrink-0"
		>
			<div class="flex items-center gap-2 text-xs md:text-sm text-teren-text-muted overflow-hidden whitespace-nowrap text-ellipsis mr-2">
				<span class="hidden sm:inline">{$_('header.properties')}</span>
				<span class="hidden sm:inline">/</span>
				<span class="font-medium text-teren-text-main truncate">TEREN Test Hotel</span>
			</div>
			<div class="flex items-center gap-2 md:gap-3 shrink-0">
				<LanguageSwitcher />
				<ThemeToggle />
				<button class="rounded-lg p-2 text-teren-text-muted transition hover:bg-teren-background-base relative">
					🔔
					<span class="absolute top-1.5 right-1.5 h-2 w-2 rounded-full bg-teren-primary"></span>
				</button>
				<button class="rounded-lg p-2 text-teren-text-muted transition hover:bg-teren-background-base">❓</button>
			</div>
		</header>

		<!-- Page Content -->
		<div class="flex-1 overflow-auto p-4 md:p-6">
			{#if children}
				{@render children()}
			{/if}
		</div>
	</main>
</div>

