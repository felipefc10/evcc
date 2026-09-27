<template>
	<router-link
		to="/load"
		class="load-line"
		:aria-label="$t('loadManagement.home.open', { kva, line, phase: phaseLabel })"
		data-testid="load-summary"
	>
		<span class="top-row">
			<span class="reading">
				<span v-if="live" class="reading-text">
					<span class="reading-label">{{ $t("loadManagement.home.label") }}</span>
					<strong>{{ kva }}&nbsp;kVA</strong>
					<span class="line-long">
						·
						<span class="reading-label">{{ $t("loadManagement.home.lineLong") }}</span>
						{{ line }}&nbsp;kVA</span
					><span class="line-short"> / {{ line }}&nbsp;kVA</span>
				</span>
				<span v-else class="reading-text">{{ $t("loadManagement.title") }}</span>
			</span>
			<span class="phase" :class="`phase--${phase}`">
				<span class="dot"></span><span class="phase-text">{{ phaseLabel }}</span>
				<svg
					width="14"
					height="14"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2.4"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
					class="chevron"
				>
					<path d="m9 6 6 6-6 6" />
				</svg>
			</span>
		</span>
		<span
			v-if="live"
			class="meter"
			:title="$t('loadManagement.home.meterTitle', { kva: line })"
			aria-hidden="true"
		>
			<span class="meter-fill" :style="{ width: `${fillPct}%` }"></span>
			<span
				v-if="overPct > 0"
				class="meter-over"
				:style="{ left: `${linePct}%`, width: `${overPct}%` }"
			></span>
			<span class="meter-tick" :style="{ left: `${linePct}%` }"></span>
		</span>
		<span
			v-if="live"
			class="budget"
			:class="`budget--${budgetLevel}`"
			:aria-hidden="budgetPct === 0"
			data-testid="load-summary-budget"
			>{{ $t("loadManagement.home.budget", { pct: budgetPct }) }}</span
		>
	</router-link>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import type { LoadState } from "@/types/supercharge";
import { loadPhase, rulerMax } from "./state";

// The whole of load management on the home page: one line under the energy flow.
export default defineComponent({
	name: "LoadHomeLine",
	mixins: [formatter],
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	computed: {
		live(): boolean {
			return this.state.running && this.state.vaKva > 0;
		},
		kva(): string {
			return this.live ? this.fmtNumber(this.state.vaKva, 2) : "—";
		},
		line(): string {
			return this.fmtNumber(this.state.thresholdKva || 0, 2);
		},
		scaleMax(): number {
			return rulerMax(this.state);
		},
		linePct(): number {
			return Math.min(100, (100 * this.state.thresholdKva) / this.scaleMax);
		},
		fillPct(): number {
			return Math.min(this.linePct, (100 * this.state.vaKva) / this.scaleMax);
		},
		overPct(): number {
			const total = Math.min(100, (100 * this.state.vaKva) / this.scaleMax);
			return Math.max(0, total - this.linePct);
		},
		failed(): number {
			return (this.state.checks || []).filter((c) => !c.ok).length;
		},
		// failed checks show only when nothing more pressing does
		phase(): string {
			const phase = loadPhase(this.state);
			return phase === "base" && this.failed ? "attention" : phase;
		},
		// the breaker budget in use, shown only while the meter is spending it
		budgetPct(): number {
			return Math.min(100, Math.round((this.state.closeness || 0) * 100));
		},
		// the line keeps its place while idle, so the card does not jump at every burst
		budgetLevel(): string {
			if (this.budgetPct === 0) return "idle";
			if (this.budgetPct >= 75) return "danger";
			if (this.budgetPct >= 50) return "warning";
			return "ok";
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
	flex-wrap: wrap;
	row-gap: 0.4rem;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	min-height: 2.75rem;
	padding: 0.5rem;
	margin: 0 -0.5rem 0.75rem;
	font-size: 0.875rem;
	color: var(--evcc-gray);
	text-decoration: none;
}
.reading {
	display: flex;
	flex-direction: column;
	gap: 0.35rem;
	flex: 0 0 auto;
}
.phase {
	margin-left: auto;
}
/* the status word changes often; a steady width keeps the row from twitching */
@media (min-width: 420px) {
	.phase {
		min-width: 9.5rem;
		justify-content: flex-end;
	}
}
.reading-text {
	white-space: nowrap;
}
.reading-label {
	text-transform: uppercase;
	font-size: 14px;
	margin-right: 0.35rem;
}
.reading strong {
	color: var(--evcc-default-text);
	font-size: 1.1rem;
	font-variant-numeric: tabular-nums;
}
.meter {
	position: relative;
	display: block;
	flex-basis: 100%;
	width: 100%;
	height: 5px;
	border-radius: 3px;
	background: var(--evcc-gray-15);
}
.meter-fill {
	position: absolute;
	left: 0;
	top: 0;
	height: 100%;
	border-radius: 3px 0 0 3px;
	transition: width var(--evcc-transition-medium) linear;
	background: var(--evcc-grid);
}
/* the part above the line is orange everywhere, planned burst or not */
.meter-over {
	position: absolute;
	top: 0;
	height: 100%;
	background: repeating-linear-gradient(
		135deg,
		var(--evcc-orange) 0 3px,
		var(--evcc-box) 3px 5px
	);
}

/* a gap on each side keeps the line mark apart from a fill that reaches it */
.meter-tick {
	position: absolute;
	top: -3px;
	width: 2px;
	height: 11px;
	margin-left: -1px;
	background: var(--evcc-default-text);
	box-shadow: 0 0 0 2px var(--evcc-background);
}
.phase {
	display: inline-flex;
	align-items: center;
	gap: 0.4rem;
	font-weight: 700;
	flex: 0 1 auto;
	min-width: 0;
}
/* phones keep reading and status on one row: the word label goes, the numbers stay */
.top-row {
	display: flex;
	align-items: center;
	gap: 1rem;
	flex-basis: 100%;
	min-width: 0;
}
.line-short {
	display: none;
}
@media (max-width: 419px) {
	.reading-label,
	.line-long {
		display: none;
	}
	.line-short {
		display: inline;
	}
}
.phase-text {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}
.dot {
	flex-shrink: 0;
	width: 0.45rem;
	height: 0.45rem;
	border-radius: 50%;
	background: currentColor;
}
.chevron {
	flex-shrink: 0;
	color: var(--evcc-gray);
}
.phase--base {
	color: var(--evcc-default-text);
}
.phase--base .dot {
	background: var(--evcc-darker-green);
}
.phase--burst {
	color: var(--evcc-default-text);
}
.phase--burst .dot {
	background: var(--evcc-dark-yellow);
}
html.dark .phase--burst .dot {
	background: var(--evcc-yellow);
}
.phase--attention,
.phase--over {
	color: #9a5200;
}
html.dark .phase--attention,
html.dark .phase--over {
	color: var(--evcc-orange);
}
.phase--supercharge {
	color: var(--evcc-default-text);
}
.phase--supercharge .dot {
	background: var(--evcc-dark-yellow);
}
html.dark .phase--supercharge .dot {
	background: var(--evcc-yellow);
}
.phase--blind {
	color: var(--evcc-red);
}
.phase--off,
.phase--standby {
	color: var(--evcc-gray);
	font-weight: normal;
}
.load-line {
	border-radius: 0.5rem;
	transition: background-color var(--evcc-transition-fast);
}
.load-line:hover {
	background: var(--evcc-gray-10);
}
.load-line:hover .chevron {
	color: var(--evcc-default-text);
}
@media (prefers-reduced-motion: reduce) {
	.meter-fill {
		transition: none;
	}
}
.budget {
	flex-basis: 100%;
	margin-top: -0.25rem;
	line-height: 1.2;
	font-size: 0.8125rem;
	font-variant-numeric: tabular-nums;
	color: var(--evcc-gray);
}
.budget--warning {
	color: #9a5200;
}
html.dark .budget--warning {
	color: var(--evcc-orange);
}
.budget--idle {
	visibility: hidden;
}
.budget--danger {
	color: var(--bs-danger);
}
</style>
