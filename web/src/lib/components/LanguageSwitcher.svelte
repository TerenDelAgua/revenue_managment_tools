<script lang="ts">
	import { locale } from 'svelte-i18n';
	
	function toggleLocale() {
		$locale = $locale === 'en' ? 'id' : 'en';
	}
</script>

<button 
	class="flex items-center justify-center rounded-lg px-2 py-1.5 text-xs font-bold text-[#57534E] transition hover:bg-[#F5F4F1] hover:text-[#1C1917] border border-[#E7E5E4] uppercase w-9 h-9" 
	onclick={toggleLocale}
	title="Switch Language"
>
	{$locale || 'en'}
</button>
