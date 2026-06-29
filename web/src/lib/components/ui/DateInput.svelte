<script lang="ts">
	interface Props {
		label?: string;
		value: string; // YYYY-MM-DD
		onChange: (value: string) => void;
		placeholder?: string;
		error?: boolean;
	}

	let { label, value, onChange, placeholder = 'Select date', error = false }: Props = $props();
	let inputEl: HTMLInputElement;
	const inputId = $derived(
		`date-input-${(label || placeholder).toLowerCase().replace(/[^a-z0-9]+/g, '-')}`
	);

	function handleInput(e: Event) {
		const target = e.target as HTMLInputElement;
		onChange(target.value);
	}
</script>

<div class="w-full space-y-1.5">
	{#if label}
		<label for={inputId} class="ml-1 text-xs font-semibold tracking-wide text-[#57534E] uppercase">
			{label}
		</label>
	{/if}

	<!-- Contenedor Unificado -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		onclick={() => inputEl?.showPicker()}
		class="group relative flex h-12 w-full cursor-pointer items-center overflow-hidden rounded-lg border transition-all duration-200 ease-out
    {error
			? 'border-[#DC2626] bg-[#FEF2F2]'
			: 'border-[#E7E5E4] bg-[#FCFBFA] focus-within:border-[#FF8C42] focus-within:ring-2 focus-within:ring-[#FF8C42]/30 hover:bg-[#FFF7ED]'}"
	>
		<!-- Icono Izquierdo -->
		<div
			class="pointer-events-none absolute left-0 pl-3.5 text-[#57534E] transition-colors group-focus-within:text-[#FF8C42]"
		>
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="20"
				height="20"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
				><rect x="3" y="4" width="18" height="18" rx="2" ry="2"></rect><line
					x1="16"
					y1="2"
					x2="16"
					y2="6"
				></line><line x1="8" y1="2" x2="8" y2="6"></line><line x1="3" y1="10" x2="21" y2="10"
				></line></svg
			>
		</div>

		<!-- Input Real (Transparente) -->
		<input
			bind:this={inputEl}
			id={inputId}
			type="date"
			bind:value
			oninput={handleInput}
			class="h-full w-full cursor-pointer border-none bg-transparent pr-4 pl-12 text-base font-medium text-[#1C1917] outline-none
      [&::-webkit-calendar-picker-indicator]:absolute [&::-webkit-calendar-picker-indicator]:inset-0 [&::-webkit-calendar-picker-indicator]:cursor-pointer [&::-webkit-calendar-picker-indicator]:opacity-0"
			{placeholder}
		/>

		<!-- Placeholder Visual (si está vacío) -->
		{#if !value}
			<span class="pointer-events-none absolute left-12 text-[#A8A29E] select-none">
				{placeholder}
			</span>
		{/if}
	</div>

	{#if error}
		<p class="ml-1 text-xs font-medium text-[#DC2626]">Please select a valid date.</p>
	{/if}
</div>
