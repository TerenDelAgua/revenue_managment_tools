<script lang="ts">
	// Custom alert para features simuladas
	let showAlert = $state(false);
	let alertFeatureName = $state('');

	function triggerFeatureAlert(name: string) {
		alertFeatureName = name;
		showAlert = true;
		setTimeout(() => {
			showAlert = false;
		}, 4000);
	}
</script>

<div class="flex flex-col gap-6 max-w-6xl mx-auto py-4">
	<!-- Toast Notification (Custom alert) -->
	{#if showAlert}
		<div
			class="fixed bottom-6 right-6 z-50 flex items-center gap-3 rounded-xl border border-[#FF8C42]/30 bg-[#FCFBFA] p-4 shadow-2xl transition-all duration-300 ease-out"
			role="alert"
		>
			<span class="text-2xl">🚧</span>
			<div>
				<h5 class="text-sm font-bold text-[#1C1917]">{alertFeatureName}</h5>
				<p class="text-xs text-[#57534E] mt-0.5">
					Esta funcionalidad se encuentra en desarrollo como parte de la Fase 2 de TEREN Suite.
				</p>
			</div>
			<button
				onclick={() => (showAlert = false)}
				class="ml-4 rounded-lg p-1.5 text-[#A8A29E] transition hover:bg-[#F5F4F1] hover:text-[#1C1917]"
			>
				✕
			</button>
		</div>
	{/if}

	<!-- Header Area -->
	<div class="flex flex-wrap items-center justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold tracking-tight text-[#1C1917]">Executive Dashboard</h2>
			<p class="text-sm text-[#57534E] mt-1">Real-time intelligence and property operations at a glance.</p>
		</div>
		<div class="flex gap-2 bg-[#FCFBFA] border border-[#E7E5E4] rounded-xl p-1.5 shadow-sm">
			<button class="rounded-lg bg-[#F5F4F1] px-3.5 py-1.5 text-xs font-semibold text-[#1C1917] transition hover:brightness-95">
				Today · May 26
			</button>
			<button class="rounded-lg px-3.5 py-1.5 text-xs font-semibold text-[#57534E] transition hover:bg-[#F5F4F1]">
				This Week
			</button>
		</div>
	</div>

	<!-- Stats Grid (TEREN Design System) -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Occupancy</span>
				<span class="text-lg">🛏️</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">78.4%</p>
			<p class="text-xs text-[#16A34A] mt-1 font-medium">↑ +4.2% from last week</p>
		</div>

		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">RevPAR</span>
				<span class="text-lg">💲</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">$84.67</p>
			<p class="text-xs text-[#16A34A] mt-1 font-medium">↑ +$6.10 average</p>
		</div>

		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Average Daily Rate</span>
				<span class="text-lg">📈</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">$108.00</p>
			<p class="text-xs text-[#57534E] mt-1 font-medium">Static strategy active</p>
		</div>

		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-5 shadow-sm hover:shadow-md transition">
			<div class="flex justify-between items-start">
				<span class="text-xs font-bold text-[#FF8C42] tracking-wider uppercase">Active Blocks</span>
				<span class="text-lg">🔧</span>
			</div>
			<p class="text-2xl font-extrabold text-[#1C1917] mt-3 tabular-nums">2 rooms</p>
			<p class="text-xs text-[#D97706] mt-1 font-medium">1 Maintenance · 1 Owner use</p>
		</div>
	</div>

	<!-- Operations & Builder Section -->
	<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-2">
		<!-- Core Feature Card: Floor Map Builder -->
		<div class="lg:col-span-2 rounded-xl border border-[#FF8C42]/30 bg-white p-6 shadow-md flex flex-col justify-between">
			<div>
				<div class="flex justify-between items-center">
					<span class="px-2.5 py-1 rounded bg-[#FFF7ED] text-[10px] font-bold text-[#FF8C42] tracking-wider uppercase">
						Active Module
					</span>
					<span class="text-xs text-[#A8A29E]">Fase 1 Release</span>
				</div>
				<h3 class="text-xl font-bold text-[#1C1917] mt-4">Floor Map & Layout Designer</h3>
				<p class="text-sm text-[#57534E] mt-2 leading-relaxed">
					Design, manage, and scale your hotel layout in an advanced grid system. View real-time room availability, toggle between operations and configuration mode, and set room blocks directly.
				</p>
			</div>

			<div class="flex flex-wrap gap-3 mt-6 pt-6 border-t border-[#E7E5E4]">
				<a
					href="/map?mode=setup"
					class="rounded-xl bg-[#FF8C42] hover:bg-[#E06B20] text-white px-5 py-3 text-sm font-semibold transition active:scale-[0.98] shadow-sm"
				>
					📐 Design Floor Layout
				</a>
				<a
					href="/map"
					class="rounded-xl border border-[#E7E5E4] hover:bg-[#F5F4F1] text-[#1C1917] px-5 py-3 text-sm font-semibold transition active:scale-[0.98]"
				>
					🟢 Operations View
				</a>
			</div>
		</div>

		<!-- Upcoming Features (CRM/Pricing Panel) -->
		<div class="rounded-xl border border-[#E7E5E4] bg-[#FCFBFA] p-6 shadow-sm flex flex-col justify-between">
			<div>
				<h4 class="font-bold text-[#1C1917] border-b border-[#E7E5E4] pb-3">Planned Features</h4>
				
				<div class="space-y-4 mt-4">
					<button
						onclick={() => triggerFeatureAlert('Dynamic Pricing Engine')}
						class="w-full text-left group flex items-start gap-3 rounded-xl p-3 border border-transparent hover:border-[#E7E5E4] hover:bg-[#FCFBFA] transition"
					>
						<span class="text-xl">📈</span>
						<div>
							<div class="flex items-center gap-1.5">
								<span class="text-sm font-bold text-[#1C1917]">Dynamic Pricing</span>
								<span class="px-1.5 py-0.5 rounded bg-[#F5F4F1] text-[9px] font-bold text-[#57534E]">PHASE 2</span>
							</div>
							<p class="text-xs text-[#57534E] mt-0.5 leading-snug">Algorithmic rate pricing engine.</p>
						</div>
					</button>

					<button
						onclick={() => triggerFeatureAlert('Reservations & Bookings Terminal')}
						class="w-full text-left group flex items-start gap-3 rounded-xl p-3 border border-transparent hover:border-[#E7E5E4] hover:bg-[#FCFBFA] transition"
					>
						<span class="text-xl">📅</span>
						<div>
							<div class="flex items-center gap-1.5">
								<span class="text-sm font-bold text-[#1C1917]">Bookings Terminal</span>
								<span class="px-1.5 py-0.5 rounded bg-[#F5F4F1] text-[9px] font-bold text-[#57534E]">PHASE 2</span>
							</div>
							<p class="text-xs text-[#57534E] mt-0.5 leading-snug">Check-in and front desk operations.</p>
						</div>
					</button>

					<button
						onclick={() => triggerFeatureAlert('Guest Intelligence Hub')}
						class="w-full text-left group flex items-start gap-3 rounded-xl p-3 border border-transparent hover:border-[#E7E5E4] hover:bg-[#FCFBFA] transition"
					>
						<span class="text-xl">👥</span>
						<div>
							<div class="flex items-center gap-1.5">
								<span class="text-sm font-bold text-[#1C1917]">Guest Intelligence</span>
								<span class="px-1.5 py-0.5 rounded bg-[#F5F4F1] text-[9px] font-bold text-[#57534E]">PHASE 2</span>
							</div>
							<p class="text-xs text-[#57534E] mt-0.5 leading-snug">Loyalty directory and preferences database.</p>
						</div>
					</button>
				</div>
			</div>

			<p class="text-[10px] text-[#A8A29E] text-center mt-6">
				TEREN Revenue Suite · Crafting Soulful Hotel Systems.
			</p>
		</div>
	</div>
</div>
