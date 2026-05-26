<div class="flex flex-col gap-6 max-w-5xl mx-auto py-4">
	<!-- Page Header -->
	<div class="flex justify-between items-center">
		<div>
			<h2 class="text-2xl font-bold text-[#1C1917]">Guest Profiles & Loyalty</h2>
			<p class="text-sm text-[#57534E] mt-1">Detailed directory of customer history, preferences, and guest profiles.</p>
		</div>
		<span class="rounded-full bg-[#FFF7ED] px-3.5 py-1.5 text-xs font-bold text-[#FF8C42] border border-[#FF8C42]/20">
			✨ Phase 2 Feature
		</span>
	</div>

	<!-- Premium Banner -->
	<div class="rounded-xl border border-[#FF8C42]/20 bg-gradient-to-r from-[#FFF7ED] to-white p-6 shadow-sm flex flex-col md:flex-row items-center gap-6">
		<span class="text-4xl">👥</span>
		<div class="flex-1">
			<h3 class="font-semibold text-lg text-[#1C1917]">Guest Intelligence is Under Construction</h3>
			<p class="text-sm text-[#57534E] mt-1">
				We are creating an elegant Guest Intelligence Hub. In Phase 2, this CRM module will give you 360-degree guest insights, historical spending analytics, automated check-in forms, and customized hotel loyalty programs.
			</p>
		</div>
	</div>

	<!-- Interactive Mockup (Locked) -->
	<div class="relative opacity-65 pointer-events-none select-none rounded-xl border border-[#E7E5E4] bg-white p-6 shadow-sm">
		<!-- Lock Screen -->
		<div class="absolute inset-0 bg-[#FCFBFA]/30 backdrop-blur-[1px] flex flex-col items-center justify-center z-10 rounded-xl">
			<span class="text-3xl mb-2">🔒</span>
			<span class="text-sm font-semibold text-[#1C1917]">Scheduled for Phase 2 Release</span>
			<p class="text-xs text-[#57534E] mt-1">Check back soon for the active CRM.</p>
		</div>

		<!-- Mockup Content -->
		<div class="flex justify-between items-center border-b border-[#E7E5E4] pb-4 mb-4">
			<h4 class="font-semibold text-[#1C1917]">Frequent Guests</h4>
			<div class="h-8 w-44 rounded bg-[#F5F4F1]"></div>
		</div>

		<div class="space-y-3">
			{#each Array(3) as _, i}
				<div class="flex items-center justify-between border-b border-[#F5F4F1] pb-3 last:border-b-0 last:pb-0">
					<div class="flex items-center gap-3">
						<div class="h-10 w-10 rounded-full bg-[#FF8C42]/10 flex items-center justify-center font-bold text-xs text-[#FF8C42]">
							{i === 0 ? 'AM' : i === 1 ? 'SL' : 'KH'}
						</div>
						<div>
							<div class="text-sm font-semibold text-[#1C1917]">
								{i === 0 ? 'Alex Miller' : i === 1 ? 'Sophia Loren' : 'Kenji Hayashi'}
							</div>
							<div class="text-xs text-[#57534E]">
								{i === 0 ? 'alex@miller.com · United Kingdom' : i === 1 ? 'sophia@loren.it · Italy' : 'k.hayashi@kobe.jp · Japan'}
							</div>
						</div>
					</div>
					<div class="flex gap-6 items-center">
						<div class="text-right">
							<div class="text-sm font-semibold text-[#1C1917]">{15 - i * 3} stays</div>
							<div class="text-xs text-[#57534E]">${1500 - i * 300} Total Spent</div>
						</div>
						<span class="px-2.5 py-1 rounded-full bg-[#FFF7ED] text-[10px] font-bold text-[#FF8C42]">VIP LEVEL {3 - i}</span>
					</div>
				</div>
			{/each}
		</div>
	</div>
</div>
