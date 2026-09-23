<template>
	<router-link
		to="/load"
		class="load-line"
		:aria-label="$t('loadManagement.home.open', { kva, line, phase: phaseLabel })"
		data-testid="load-summary"
	>
		<span class="reading">
			<strong>{{ kva }}</strong> {{ $t("loadManagement.ledger.ofLine", { kva: line }) }}
		</span>
		<span class="phase" :class="`phase--${phase}`">
			<span class="dot"></span>{{ phaseLabel }}
			<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" class="chevron"><path d="m9 6 6 6-6 6" /></svg>
		</span>
	</router-link>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import type { LoadState } from "@/types/supercharge";

// The whole of load management on the home page: one line under the energy flow.
export default defineComponent({
	name: "LoadHomeLine",
	mixins: [formatter],
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	computed: {
		kva(): string {
			return this.state.running && this.state.vaKva > 0 ? this.fmtNumber(this.state.vaKva, 2) : "—";
		},
		line(): string {
			return this.fmtNumber(this.state.thresholdKva || 0, 2);
		},
		phase(): string {
			const s = this.state;
			if (!s.enabled) return "off";
			if (!s.running) return "standby";
			if (s.blind > 0) return "blind";
			return s.phase === "burst" ? "burst" : "base";
		},
		phaseLabel(): string {
			return this.$t(`loadManagement.phase.${this.phase}`);
		},
	},
});
</script>

<style scoped>
.load-line {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	padding: 0.5rem 0;
	font-size: 0.875rem;
	color: var(--evcc-gray);
	text-decoration: none;
}
.load-line strong {
	color: var(--evcc-default-text);
	font-variant-numeric: tabular-nums;
}
.phase {
	display: inline-flex;
	align-items: center;
	gap: 0.4rem;
	font-weight: 700;
}
.dot {
	width: 0.45rem;
	height: 0.45rem;
	border-radius: 50%;
	background: currentColor;
}
.chevron {
	color: var(--evcc-gray);
}
.phase--base {
	color: var(--evcc-darker-green);
}
html.dark .phase--base {
	color: var(--evcc-dark-green);
}
.phase--burst {
	color: var(--evcc-orange);
}
.phase--blind,
.phase--off {
	color: var(--evcc-red);
}
.load-line:hover .chevron {
	color: var(--evcc-default-text);
}
</style>
