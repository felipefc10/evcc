<template>
	<div class="load-ledger" data-testid="load-ledger">
		<div class="min-w-0">
			<h2 class="ledger-title">
				{{ live ? $t("loadManagement.houseLoad") : $t("loadManagement.ledger.lineTitle") }}
			</h2>
			<div v-if="live" class="hero">
				<span class="hero-value">{{ heroValue }}</span>
				<span class="hero-unit">{{
					$t("loadManagement.ledger.ofLine", { kva: fmtKva(state.thresholdKva) })
				}}</span>
			</div>
			<div v-else class="hero">
				<span class="hero-value">{{ fmtKva(state.thresholdKva) }}</span>
				<span class="hero-unit">kVA</span>
			</div>
			<p class="status mb-0" data-testid="load-status">{{ statusSentence }}</p>
			<p v-if="superchargeLine" class="warn-text small mb-0 mt-1">{{ superchargeLine }}</p>
		</div>

		<template v-if="live">
			<div class="ledger mt-4">
				<div class="ledger-labels" aria-hidden="true">
					<span
						v-if="state.thresholdKva > 0"
						class="ledger-label"
						:class="{ 'ledger-label--flip': linePos > 45 }"
						:style="{ left: `${linePos}%` }"
						>{{
							$t("loadManagement.ledger.line", { kva: fmtKva(state.thresholdKva) })
						}}</span
					>
				</div>
				<div class="ledger-track">
					<div class="ledger-bar">
						<div
							v-for="seg in segments"
							:key="seg.id"
							class="ledger-seg"
							:class="seg.cls"
							:style="{ width: seg.width, background: seg.color }"
							:title="seg.title"
						></div>
					</div>
					<div
						v-if="overWidth > 0"
						class="ledger-over"
						:style="{ left: `${linePos}%`, width: `${overWidth}%` }"
					></div>
					<div
						v-if="state.thresholdKva > 0"
						class="ledger-tick"
						:style="{ left: `${linePos}%` }"
					></div>
					<div
						v-if="showBurstTick"
						class="ledger-tick ledger-tick--burst"
						:class="{ 'ledger-tick--idle': !state.burstArmed }"
						:style="{ left: `${burstPos}%` }"
					></div>
				</div>
				<div
					v-if="showBurstTick"
					class="ledger-labels ledger-labels--below"
					aria-hidden="true"
				>
					<span
						class="ledger-label ledger-label--burst"
						:class="{
							'ledger-label--idle': !state.burstArmed,
							'ledger-label--flip': burstPos > 45,
						}"
						:style="{ left: `${burstPos}%` }"
						>{{
							$t("loadManagement.ledger.burst", { kva: fmtKva(state.burstKva) })
						}}</span
					>
				</div>
			</div>

			<div class="legend d-flex flex-wrap column-gap-3 row-gap-1 mt-3 small">
				<span
					v-for="item in legend"
					:key="item.id"
					class="d-inline-flex align-items-center gap-1"
					:class="{ invisible: item.hidden }"
				>
					<span
						class="swatch"
						:class="item.cls"
						:style="{ background: item.color }"
					></span>
					<span class="text-muted">{{ item.name }}</span>
					<strong>{{ item.value }}</strong>
				</span>
			</div>
			<p v-if="overclaimNote" class="small warn-text mt-2 mb-0">
				{{ overclaimNote }}
			</p>
		</template>

		<div
			v-if="state.running || state.bursts > 0"
			class="figures mt-4 pt-4"
			data-testid="load-figures"
		>
			<div class="figure" data-testid="load-patience">
				<span class="figure-label">{{ $t("loadManagement.patience.title") }}</span>
				<span class="figure-value" :class="gaugeTextClass">{{ patienceValue }}</span>
				<div class="gauge" aria-hidden="true">
					<div
						v-if="closenessPct > 0"
						class="gauge-fill"
						:class="gaugeClass"
						:style="{ width: `${closenessPct}%` }"
					></div>
					<div
						v-if="showPlanned"
						class="gauge-tick"
						:style="{ left: `${plannedPct}%` }"
						:title="$t('loadManagement.patience.tick', { pct: plannedPct })"
					></div>
				</div>
				<span class="figure-note">{{ $t("loadManagement.patience.explain") }}</span>
				<span class="figure-note" :class="{ invisible: !showPlanned }">{{
					$t("loadManagement.patience.tick", { pct: plannedPct })
				}}</span>
			</div>
			<div class="figure">
				<span class="figure-label">{{ $t("loadManagement.stats.average") }}</span>
				<span class="figure-value">{{ averageValue }}</span>
				<span class="figure-note">{{ averageNote }}</span>
			</div>
			<div class="figure">
				<span class="figure-label">{{ $t("loadManagement.stats.bursts") }}</span>
				<span class="figure-value">{{ state.bursts || 0 }}</span>
				<span class="figure-note">
					<span class="text-nowrap"
						>{{ $t("loadManagement.stats.thisSession") }}
						<router-link class="figure-link" :to="{ query: { tab: 'bursts' } }">{{
							$t("loadManagement.stats.historyLink")
						}}</router-link></span
					>
				</span>
			</div>
		</div>
	</div>
</template>
<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import type { LoadState } from "@/types/supercharge";
import { lpColor, rulerMax, superchargeDrawing } from "./state";

interface Segment {
	id: string;
	width: string;
	color: string;
	cls: string;
	title: string;
}

// The power ledger: the meter reading split into the house and each car, the free headroom,
// the never-trip line and the burst target. The meter is master: the total is the meter reading,
// car claims larger than it are scaled to fit.
export default defineComponent({
	name: "LoadLedger",
	mixins: [formatter],
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	computed: {
		// a meter reading to draw; before that there is only the line to show
		live(): boolean {
			return this.state.running && this.total > 0.01;
		},
		volts(): number {
			return this.state.meter?.volts > 100 ? this.state.meter.volts : 230;
		},
		cars() {
			return (this.state.loadpoints || []).map((lp, i) => {
				const amps = lp.measuredA > 0 ? lp.measuredA : 0;
				return {
					id: lp.name,
					name: lp.title,
					amps,
					kva: (amps * this.volts) / 1000,
					color: lpColor(i),
				};
			});
		},
		carSum(): number {
			return this.cars.reduce((a, c) => a + c.kva, 0);
		},
		total(): number {
			return this.state.vaKva > 0 ? this.state.vaKva : this.carSum;
		},
		overclaim(): number {
			return Math.max(0, this.carSum - Math.max(0, this.total - 0.05));
		},
		scaledCars() {
			if (this.overclaim <= 0 || this.carSum <= 0) return this.cars;
			const scale = Math.max(0, this.total - 0.05) / this.carSum;
			return this.cars.map((c) => ({ ...c, kva: c.kva * scale }));
		},
		house(): number {
			const cars = this.scaledCars.reduce((a, c) => a + c.kva, 0);
			return Math.max(0, this.total - cars);
		},
		free(): number {
			return Math.max(0, this.state.thresholdKva - this.total);
		},
		showBurstTick(): boolean {
			return this.state.burstKva > this.state.thresholdKva;
		},
		// the same fixed ruler as the main screen, so the line mark never moves
		scaleMax(): number {
			return rulerMax(this.state);
		},
		linePos(): number {
			return this.pos(this.state.thresholdKva);
		},
		burstPos(): number {
			return this.pos(this.state.burstKva);
		},
		segments(): Segment[] {
			if (this.total <= 0.01) return [];
			const w = (kva: number) => `${((100 * kva) / this.scaleMax).toFixed(2)}%`;
			const res: Segment[] = [
				{
					id: "house",
					width: w(this.house),
					color: "",
					cls: "ledger-seg--house",
					title: `${this.$t("loadManagement.ledger.house")} ${this.fmtKva(this.house)} kVA`,
				},
			];
			for (const c of this.scaledCars.filter((c) => c.kva > 0.01)) {
				res.push({
					id: c.id,
					width: w(c.kva),
					color: c.color,
					cls: "",
					title: `${c.name} ${this.fmtKva(c.kva)} kVA · ${this.fmtNumber(c.amps, 1)} A`,
				});
			}
			if (this.free > 0.01) {
				res.push({
					id: "free",
					width: w(this.free),
					color: "",
					cls: "ledger-seg--free",
					title: `${this.$t("loadManagement.ledger.free")} ${this.fmtKva(this.free)} kVA`,
				});
			}
			return res;
		},
		legend() {
			if (this.total <= 0.01) return [];
			const res = [
				{
					id: "house",
					name: this.$t("loadManagement.ledger.house"),
					value: `${this.fmtKva(this.house)} kVA`,
					color: "",
					cls: "swatch--house",
					hidden: false,
				},
				...this.scaledCars.map((c) => ({
					id: c.id,
					name: c.name,
					value: `${this.fmtKva(c.kva)} kVA`,
					color: c.color,
					cls: "",
					hidden: false,
				})),
			];
			// every car and the over part are always listed, so the legend never reflows
			const over = this.total - this.state.thresholdKva;
			res.push({
				id: "over",
				name: this.$t("loadManagement.ledger.over"),
				value: `${this.fmtKva(Math.max(0, over))} kVA`,
				color: "",
				cls: "swatch--over",
				hidden: over <= 0,
			});
			return res;
		},
		overclaimNote(): string {
			if (this.total <= 0.01) {
				return this.state.running ? "" : this.$t("loadManagement.ledger.noReading");
			}
			return this.overclaim > 0.05
				? this.$t("loadManagement.ledger.overclaim", { kva: this.fmtKva(this.overclaim) })
				: "";
		},
		heroValue(): string {
			return this.state.running && this.state.vaKva > 0 ? this.fmtKva(this.state.vaKva) : "—";
		},
		blind(): boolean {
			return this.state.running && this.state.blind > 0;
		},
		statusSentence(): string {
			const s = this.state;
			const line = `${this.fmtKva(s.thresholdKva)} kVA`;
			if (!s.enabled) return this.$t("loadManagement.status.off");
			if (!s.configured) return this.$t("loadManagement.status.notConfigured");
			if (!s.armed) return this.$t("loadManagement.status.standby");
			if (!s.running) return this.$t("loadManagement.status.starting");
			if (this.blind) return s.status || this.$t("loadManagement.phase.blind");
			if (s.phase === "burst") {
				return this.$t("loadManagement.status.burst", {
					line,
					target: `${this.fmtKva(s.burstKva)} kVA`,
				});
			}
			const lps = s.loadpoints || [];
			const charging = lps
				.filter((lp) => !lp.paused && lp.setpointA > 0)
				.map((lp) =>
					this.$t("loadManagement.status.charging", {
						name: lp.title,
						amps: lp.setpointA,
					})
				);
			// a paused car that still draws is told on its own card, not here as well
			const drawing = lps.filter((lp) => lp.paused && lp.measuredA > 0.5);
			const waitingNames = lps
				.filter((lp) => lp.connected && lp.wants && lp.paused && !drawing.includes(lp))
				.map((lp) => lp.title);
			const waiting = this.waitingClause(waitingNames);
			if (!charging.length && !waiting.length) {
				return this.$t("loadManagement.status.noCar", { line });
			}
			// a supercharge may sit above the line on purpose, as the status word says
			const overKey = superchargeDrawing(s) ? "overSupercharge" : "over";
			const risk =
				s.vaKva > s.thresholdKva
					? this.$t(`loadManagement.status.${overKey}`, {
							kva: this.fmtKva(s.vaKva - s.thresholdKva),
						})
					: "";
			return `${[...charging, ...waiting].join("; ")}.${risk ? ` ${risk}` : ""}`;
		},
		// each card already says until when; the house only has to say when bursting gave up
		superchargeLine(): string {
			const on = (this.state.loadpoints || []).some((lp) => lp.supercharge);
			if (!on || !this.state.burstStoodDown) return "";
			return this.$t("loadManagement.supercharge.stoodDown", {
				reason: this.state.burstStoodDown,
			});
		},
		closenessPct(): number {
			return Math.min(100, Math.max(0, Math.round((this.state.closeness || 0) * 100)));
		},
		// the mark only means something while a burst runs
		showPlanned(): boolean {
			return this.state.phase === "burst" && this.state.plannedCloseness > 0;
		},
		plannedPct(): number {
			return Math.round((this.state.plannedCloseness || 0) * 100);
		},
		gaugeClass(): string {
			const c = this.state.closeness || 0;
			if (c >= 0.75) return "gauge-fill--danger";
			if (c >= 0.5) return "gauge-fill--warning";
			return "gauge-fill--ok";
		},
		gaugeTextClass(): string {
			return this.gaugeClass.replace("gauge-fill", "figure-value");
		},
		patienceValue(): string {
			return this.state.running ? `${this.closenessPct} %` : "—";
		},
		// share of the bar drawn above the never-trip line
		overWidth(): number {
			if (!this.live || this.total <= this.state.thresholdKva) return 0;
			return this.pos(this.total) - this.linePos;
		},
		averageValue(): string {
			return this.state.avgKva > 0 ? `${this.fmtKva(this.state.avgKva)} kVA` : "—";
		},
		averageNote(): string {
			if (this.state.avgKva <= 0) return this.$t("loadManagement.stats.averageHelp");
			// rounded first, so a small difference never reads "-0 %"
			const g = Math.round(this.state.gainVsLine);
			if (g === 0) return this.$t("loadManagement.stats.atLine");
			return this.$t(`loadManagement.stats.${g > 0 ? "aboveLine" : "belowLine"}`, {
				pct: this.fmtNumber(Math.abs(g), 0),
			});
		},
	},
	methods: {
		// "Wallbox and Little Beast wait for power", not the same clause twice
		waitingClause(names: string[]): string[] {
			if (!names.length) return [];
			if (names.length > 1) {
				const and = ` ${this.$t("loadManagement.ledger.and")} `;
				return [this.$t("loadManagement.status.waitingMany", { names: names.join(and) })];
			}
			const ahead = (this.state.loadpoints || []).find((o) => !o.paused && o.setpointA > 0);
			return [
				ahead
					? this.$t("loadManagement.status.waitingFor", {
							name: names[0],
							first: ahead.title,
						})
					: this.$t("loadManagement.status.waiting", { name: names[0] }),
			];
		},
		pos(kva: number): number {
			return Math.max(0, Math.min(100, (100 * kva) / this.scaleMax));
		},
		fmtKva(kva: number): string {
			return this.fmtNumber(kva || 0, 2);
		},
	},
});
</script>

<style scoped>
.min-w-0 {
	min-width: 0;
}
.ledger-title {
	margin: 0 0 0.35rem;
}
.hero {
	display: flex;
	align-items: baseline;
	gap: 0.4rem;
	line-height: 1.1;
}
.hero-value {
	font-size: 3rem;
	font-weight: 800;
	font-variant-numeric: tabular-nums;
}
.hero-unit {
	font-size: 1.25rem;
	color: var(--evcc-gray);
}
/* two lines kept, so the bar does not jump when the sentence gets shorter */
.status {
	margin-top: 0.5rem;
	min-height: 3em;
}
.ledger-labels {
	position: relative;
	height: 1.25rem;
	font-size: 0.75rem;
	font-weight: 700;
}
.ledger-label {
	position: absolute;
	white-space: nowrap;
	padding-left: 6px;
}
.ledger-label--flip {
	transform: translateX(-100%);
	padding-left: 0;
	padding-right: 6px;
}
.ledger-labels--below {
	margin-top: 0.9rem;
}
/* yellow text is unreadable on white, so the mark carries the colour, not the words */
.ledger-label--burst {
	display: inline-flex;
	align-items: center;
	gap: 0.35rem;
}
.ledger-label--burst::before {
	content: "";
	width: 0.9rem;
	border-top: 2px dashed var(--evcc-gray);
}
.ledger-track {
	position: relative;
}
.ledger-bar {
	height: 2.5rem;
	border-radius: 10px;
	display: flex;
	overflow: hidden;
	background: var(--evcc-gray-15);
}
.ledger-seg {
	transition: width var(--evcc-transition-medium) linear;
}
.ledger-seg--house {
	background-color: var(--evcc-grid);
}
/* over the line is a strip under the bar, so the cars' colours stay readable */
.ledger-over {
	position: absolute;
	top: calc(2.5rem + 4px);
	height: 6px;
	border-radius: 3px;
	background: repeating-linear-gradient(
		135deg,
		var(--evcc-orange) 0 3px,
		var(--evcc-box) 3px 5px
	);
	pointer-events: none;
	transition:
		left var(--evcc-transition-medium) linear,
		width var(--evcc-transition-medium) linear;
}
.ledger-tick {
	position: absolute;
	top: -0.35rem;
	height: calc(2.5rem + 0.7rem);
	width: 2px;
	margin-left: -1px;
	background: var(--evcc-default-text);
}
.ledger-tick--idle,
.ledger-labels--below:has(.ledger-label--idle) {
	opacity: 0.45;
}
.ledger-tick--burst {
	width: 0;
	background: none;
	border-left: 2px dashed var(--evcc-gray);
}
.swatch {
	display: inline-block;
	width: 0.65rem;
	height: 0.65rem;
	border-radius: 50%;
}
.swatch--house {
	background-color: var(--evcc-grid);
}
.swatch--over {
	border-radius: 2px;
	background: repeating-linear-gradient(
		135deg,
		var(--evcc-orange) 0 3px,
		var(--evcc-box) 3px 5px
	) !important;
}
.figures {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	border-top: 1px solid var(--evcc-gray-25);
}
.figure {
	display: flex;
	flex-direction: column;
	gap: 0.2rem;
	padding: 0 1.25rem;
	border-left: 1px solid var(--evcc-gray-25);
	min-width: 0;
}
.figure:first-child {
	padding-left: 0;
	border-left: 0;
}
/* labels keep two lines of room, so the values sit on one baseline */
.figure-label {
	min-height: 2.6em;
	line-height: 1.3;
	text-transform: uppercase;
	color: var(--evcc-gray);
	font-size: 14px;
	font-weight: normal;
}
.figure-note {
	font-size: 0.75rem;
	color: var(--evcc-gray);
}
.figure-value {
	font-size: 1.35rem;
	font-weight: 800;
	font-variant-numeric: tabular-nums;
}
.figure-value--warning {
	color: var(--evcc-orange);
}
.figure-value--danger {
	color: var(--evcc-red);
}
.figure-link {
	font-size: 0.875rem;
	color: var(--evcc-default-text);
	text-decoration: underline;
	white-space: nowrap;
}
.gauge {
	position: relative;
	height: 0.375rem;
	border-radius: 999px;
	background: var(--evcc-gray-15);
	margin: 0.15rem 0;
}
.gauge-fill {
	height: 100%;
	border-radius: 999px;
	transition: width var(--evcc-transition-medium) linear;
}
.gauge-fill--ok {
	background-color: var(--evcc-darker-green);
}
.gauge-fill--warning {
	background-color: var(--evcc-orange);
}
.gauge-fill--danger {
	background-color: var(--evcc-red);
}
.gauge-tick {
	position: absolute;
	top: -0.2rem;
	width: 2px;
	height: 0.775rem;
	background: var(--evcc-default-text);
}
@media (max-width: 575px) {
	.hero-value {
		font-size: 2.5rem;
	}
	.figures {
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem 0;
	}
	.figure:first-child {
		grid-column: span 2;
	}
	.figure:nth-child(2) {
		padding-left: 0;
		border-left: 0;
	}
}
@media (prefers-reduced-motion: reduce) {
	.ledger-seg,
	.gauge-fill {
		transition: none;
	}
}
.warn-text {
	color: #9a5200;
}
html.dark .warn-text {
	color: var(--evcc-orange);
}
</style>
