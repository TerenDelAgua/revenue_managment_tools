<script lang="ts">
	import { toasts, removeToast, type Toast } from '$lib/store/toastStore';
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';

	// Colores y estilos según Design System 3.9
	const styles = {
		success: { border: 'border-[#16A34A]/50', icon: '✅', text: 'text-[#16A34A]' },
		error: { border: 'border-[#DC2626]/50', icon: '⚠️', text: 'text-[#DC2626]' },
		info: { border: 'border-[#FF8C42]/50', icon: '️', text: 'text-[#FF8C42]' }
	};
</script>

<!-- Contenedor Flotante -->
<div
	class="pointer-events-none fixed top-0 right-0 bottom-0 z-[100] flex w-full flex-col
  justify-end gap-3 p-4 sm:top-4 sm:right-4
  sm:bottom-auto sm:w-auto sm:items-center
  sm:items-end sm:justify-start"
>
	{#each $toasts as toast (toast.id)}
		<div
			class="pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-lg border bg-[#FCFBFA] p-4 shadow-lg transition-all sm:w-80
      {styles[toast.type].border}"
			transition:fly={{ duration: 250, easing: cubicOut, y: 20 }}
			role="alert"
		>
			<span class="mt-0.5 text-lg">{styles[toast.type].icon}</span>
			<div class="flex-1">
				<p class="text-sm leading-snug font-medium text-[#1C1917]">
					{toast.message}
				</p>
			</div>

			<!-- Botón de cerrar (X) -->
			<button
				class="rounded p-1 text-[#57534E] transition-colors hover:bg-[#F5F4F1] hover:text-[#1C1917]"
				onclick={() => removeToast(toast.id)}
				aria-label="Cerrar notificación"
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
					><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"
					></line></svg
				>
			</button>
		</div>
	{/each}
</div>
