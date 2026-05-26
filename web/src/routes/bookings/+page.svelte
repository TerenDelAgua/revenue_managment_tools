<div class="flex flex-col gap-6 max-w-5xl mx-auto py-4">
	<!-- Page Header -->
	<div class="flex justify-between items-center">
		<div>
			<h2 class="text-2xl font-bold text-[#1C1917]">Reservations & Bookings</h2>
			<p class="text-sm text-[#57534E] mt-1">Manage guest bookings, check-ins, and reservations history.</p>
		</div>
		<span class="rounded-full bg-[#FFF7ED] px-3.5 py-1.5 text-xs font-bold text-[#FF8C42] border border-[#FF8C42]/20">
			✨ Phase 2 Feature
		</span>
	</div>

	<!-- Premium Banner -->
	<div class="rounded-xl border border-[#FF8C42]/20 bg-gradient-to-r from-[#FFF7ED] to-white p-6 shadow-sm flex flex-col md:flex-row items-center gap-6">
		<span class="text-4xl">📅</span>
		<div class="flex-1">
			<h3 class="font-semibold text-lg text-[#1C1917]">Bookings Engine is Under Construction</h3>
			<p class="text-sm text-[#57534E] mt-1">
				We are currently perfecting the real-time reservations system. In Phase 2, this terminal will let you instantly book rooms, handle walk-ins, and synchronise with major Channel Managers (OTAs) natively.
			</p>
		</div>
	</div>

	<!-- Interactive Mockup (Greyed out/locked state) -->
	<div class="relative opacity-65 pointer-events-none select-none rounded-xl border border-[#E7E5E4] bg-white p-6 shadow-sm">
		<!-- Lock Screen -->
		<div class="absolute inset-0 bg-[#FCFBFA]/30 backdrop-blur-[1px] flex flex-col items-center justify-center z-10 rounded-xl">
			<span class="text-3xl mb-2">🔒</span>
			<span class="text-sm font-semibold text-[#1C1917]">Scheduled for Phase 2 Release</span>
			<p class="text-xs text-[#57534E] mt-1">Check back soon for the active terminal.</p>
		</div>

		<!-- Mockup Content -->
		<div class="flex justify-between items-center border-b border-[#E7E5E4] pb-4 mb-4">
			<h4 class="font-semibold text-[#1C1917]">Upcoming Arrivals</h4>
			<div class="flex gap-2">
				<div class="h-8 w-24 rounded bg-[#F5F4F1]"></div>
				<div class="h-8 w-32 bg-[#F5F4F1] rounded"></div>
			</div>
		</div>

		<div class="space-y-3">
			{#each Array(3) as _, i}
				<div class="flex items-center justify-between border-b border-[#F5F4F1] pb-3 last:border-b-0 last:pb-0">
					<div class="flex items-center gap-3">
						<div class="h-10 w-10 rounded-full bg-[#F5F4F1] flex items-center justify-center font-bold text-xs">
							G{i+1}
						</div>
						<div>
							<div class="text-sm font-semibold text-[#1C1917]">Guest Name {i+1}</div>
							<div class="text-xs text-[#57534E]">Room 10{i+1} · Standard Room</div>
						</div>
					</div>
					<div class="flex gap-4 items-center">
						<div class="text-right">
							<div class="text-sm font-semibold text-[#1C1917]">May 28 - Jun 02</div>
							<div class="text-xs text-[#57534E]">5 nights</div>
						</div>
						<span class="px-2 py-1 rounded bg-[#E7E5E4] text-[10px] font-bold text-[#57534E]">CONFIRMED</span>
					</div>
				</div>
			{/each}
		</div>
	</div>
</div>
