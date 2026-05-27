<script lang="ts">
	import { page } from '$app/stores';

	interface Props {
		children?: import('svelte').Snippet;
	}

	let { children }: Props = $props();

	import LanguageSwitcher from '$lib/components/LanguageSwitcher.svelte';
	import { _ } from 'svelte-i18n';

	const navItems = [
		{ icon: '📊', labelKey: 'nav.dashboard', href: '/' },
		{ icon: '🗺️', labelKey: 'nav.floorMap', href: '/map' },
		{ icon: '📅', labelKey: 'nav.bookings', href: '/bookings' },
		{ icon: '👥', labelKey: 'nav.guests', href: '/guests' },
		{ icon: '⚙️', labelKey: 'nav.settings', href: '/settings' }
	];
</script>

<div class="flex h-screen bg-[#F5F4F1] font-sans text-[#1C1917]">
	<!-- Sidebar -->
	<aside class="z-20 flex w-64 flex-col bg-[#1C1917] text-[#FCFBFA] shadow-xl">
		<div class="border-b border-[#3F3D38] p-6">
			<h1 class="text-xl font-bold tracking-tight text-[#FF8C42]">TEREN</h1>
			<p class="mt-1 text-xs text-[#A8A29E]">{$_('header.subtitle')}</p>
		</div>

		<nav class="flex-1 space-y-1 p-4">
			{#each navItems as item}
				<a
					href={item.href}
					class="flex items-center gap-3 rounded-lg px-3 py-2.5 transition-all duration-200
          {$page.url.pathname === item.href || ($page.url.pathname.startsWith(item.href) && item.href !== '/')
						? 'bg-[#FF8C42]/10 font-semibold text-[#FF8C42]'
						: 'text-[#A8A29E] hover:bg-[#3F3D38] hover:text-[#FCFBFA]'}"
				>
					<span class="text-lg">{item.icon}</span>
					<span class="text-sm">{$_(item.labelKey)}</span>
				</a>
			{/each}
		</nav>

		<div class="border-t border-[#3F3D38] p-4 bg-[#141211]">
			<div class="flex items-center gap-3">
				<div
					class="flex h-9 w-9 items-center justify-center rounded-full bg-[#FF8C42] text-sm font-bold text-white shadow-sm"
				>
					JD
				</div>
				<div>
					<p class="text-sm font-medium">Juan Del Agua</p>
					<p class="text-xs text-[#A8A29E]">{$_('header.hotelOwner')}</p>
				</div>
			</div>
		</div>
	</aside>

	<!-- Main Content -->
	<main class="flex flex-1 flex-col overflow-hidden">
		<!-- Topbar -->
		<header
			class="flex h-16 items-center justify-between border-b border-[#E7E5E4] bg-[#FCFBFA] px-6 shadow-sm"
		>
			<div class="flex items-center gap-2 text-sm text-[#57534E]">
				<span>{$_('header.properties')}</span>
				<span>/</span>
				<span class="font-medium text-[#1C1917]">TEREN Test Hotel</span>
			</div>
			<div class="flex items-center gap-3">
				<LanguageSwitcher />
				<button class="rounded-lg p-2 text-[#57534E] transition hover:bg-[#F5F4F1] relative">
					🔔
					<span class="absolute top-1.5 right-1.5 h-2 w-2 rounded-full bg-[#FF8C42]"></span>
				</button>
				<button class="rounded-lg p-2 text-[#57534E] transition hover:bg-[#F5F4F1]">❓</button>
			</div>
		</header>

		<!-- Page Content -->
		<div class="flex-1 overflow-auto p-6">
			{#if children}
				{@render children()}
			{/if}
		</div>
	</main>
</div>
