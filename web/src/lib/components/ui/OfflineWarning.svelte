<script lang="ts">
	import { onMount } from 'svelte';
	
	let isOffline = $state(false);

	onMount(() => {
		const handleOnline = () => { isOffline = false; };
		const handleOffline = () => { isOffline = true; };
		
		isOffline = !navigator.onLine;

		window.addEventListener('online', handleOnline);
		window.addEventListener('offline', handleOffline);

		return () => {
			window.removeEventListener('online', handleOnline);
			window.removeEventListener('offline', handleOffline);
		};
	});

	function handleRetry() {
		if (navigator.onLine) {
			isOffline = false;
		} else {
			// Trigger a small shake or feedback if still offline
			// For simplicity we just reload to let the browser handle it if online
			window.location.reload();
		}
	}
</script>

{#if isOffline}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-[#F5F4F1] p-6 text-center animate-in fade-in duration-300">
		<div class="max-w-md w-full flex flex-col items-center">
			<!-- Icon -->
			<div class="mb-8 flex h-24 w-24 items-center justify-center rounded-full bg-[#FFF7ED]">
				<div class="flex h-16 w-16 items-center justify-center rounded-xl bg-gradient-to-br from-[#FF8C42] to-[#E06B20] text-white shadow-lg relative">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<rect width="14" height="20" x="5" y="2" rx="2" ry="2"/>
						<path d="M12 18h.01"/>
					</svg>
					<div class="absolute -bottom-2 -right-2 bg-white px-1.5 py-0.5 rounded text-[10px] font-bold text-[#FF8C42] shadow-sm">
						OFF
					</div>
				</div>
			</div>

			<!-- Typography -->
			<h2 class="mb-4 text-3xl font-extrabold text-[#1C1917]">Sin conexión</h2>
			<p class="mb-10 text-lg leading-relaxed text-[#57534E]">
				No te preocupes. Tu configuración del mapa y datos operativos están guardados localmente. Puedes seguir trabajando y sincronizaremos automáticamente cuando vuelvas a estar en línea.
			</p>

			<!-- Action -->
			<button
				onclick={handleRetry}
				class="w-full max-w-[200px] rounded-xl bg-[#FF8C42] px-6 py-3.5 text-lg font-semibold text-white shadow-md transition hover:bg-[#E06B20] active:scale-95"
			>
				Reintentar
			</button>
		</div>
	</div>
{/if}
