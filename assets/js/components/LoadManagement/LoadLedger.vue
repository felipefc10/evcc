<template>
	<div class="load-ledger" data-testid="load-ledger">
		<div class="d-flex align-items-start justify-content-between gap-3">
			<div class="min-w-0">
				<div class="hero">
					<span class="hero-value">{{ heroValue }}</span>
					<span class="hero-unit">kVA</span>
				</div>
				<p class="status mb-0" data-testid="load-status">{{ statusSentence }}</p>
				<p v-if="superchargeLine" class="text-muted small mb-0">{{ superchargeLine }}</p>
			</div>
			<span class="badge rounded-pill phase-badge flex-shrink-0" :class="phaseClass">
				{{ phaseLabel }}
			</span>
		</div>

		<div class="ledger mt-4" :class="{ 'ledger--burst': showBurstTick }">
			<div class="ledger-bar">
				<div
					v-for="seg in segments"
					:key="seg.id"
					class="ledger-seg"
					:class="seg.cls"
					:style="{ width: seg.width, background: seg.color }"
					:title="seg.title"
				>
					<span v-if="seg.label" class="ledger-seg-label">{{ seg.label }}</span>
				</div>
			</div>
			<div
				v-if="state.thresholdKva > 0"
				class="ledger-tick ledger-tick--line"
				:class="{ 'ledger-tick--flip': linePos > 60 }"
				:style="{ left: `${linePos}%` }"
			>
				<span>{{ $t("loadManagement.ledger.line", { kva: fmtKva(state.thresholdKva) }) }}</span>
			</div>
			<div
				v-if="showBurstTick"
				class="ledger-tick ledger-tick--burst"
				:class="{ 'ledger-tick--flip': burstPos > 60 }"
				:style="{ left: `${burstPos}%` }"
			>
				<span>{{ $t("loadManagement.ledger.burst", { kva: fmtKva(state.burstKva) }) }}</span>
			</div>
		</div>

		<div v-if="!compact" class="legend d-flex flex-wrap gap-3 mt-3 small">
			<span v-for="item in legend" :key="item.id" class="d-inline-flex align-items-center gap-1">
				<span class="swatch" :class="item.cls" :style="{ background: item.color }"></span>
				<span class="text-muted">{{ item.name }}</span>
				<strong>{{ item.value }}</strong>
			</span>
		</div>
		<p v-if="!compact && overclaimNote" class="small text-warning mt-2 mb-0">
			{{ overclaimNote }}
		</p>

		<div class="patience mt-4" data-testid="load-patience">
			<div class="d-flex justify-content-between small mb-1">
				<span class="fw-bold">{{ $t("loadManagement.patience.title") }}</span>
				<span>{{ patienceLabel }}</span>
			</div>
			<div class="gauge">
				<div
					class="gauge-fill"
					:class="gaugeClass"
					:style="{ width: `${closenessPct}%` }"
				></div>
				<div
					v-if="state.burstArmed && state.plannedCloseness > 0"
					class="gauge-tick"
					:style="{ left: `${plannedPct}%` }"
					:title="$t('loadManagement.patience.burstsStop', { pct: plannedPct })"
				></div>
			</div>
			<div class="d-flex justify-content-between small text-muted mt-1">
				<span v-if="state.burstArmed && state.plannedCloseness > 0">
					{{ $t("loadManagement.patience.burstsStop", { pct: plannedPct }) }}
				</span>
				<span v-else></span>
				<span>{{ $t("loadManagement.patience.opens") }}</span>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import colors from "@/colors";
import type { LoadState } from "@/types/supercharge";

interface Segment {
	id: string;
	width: string;
	color: string;
	cls: string;
	label: string;
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
		compact: Boolean,
	},
	computed: {
		volts(): number {
			return this.state.meter?.volts > 100 ? this.state.meter.volts : 230;
		},
		cars() {
			return (this.state.loadpoints || [])
				.map((lp, i) => {
					const amps = lp.measuredA > 0 ? lp.measuredA : 0;
					return {
						id: lp.name,
						name: lp.title,
						amps,
						kva: (amps * this.volts) / 1000,
						color: colors.palette[i % colors.palette.length] || "#60A5FA",
					};
				})
				.filter((c) => c.kva > 0.01);
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
			return this.state.burstArmed && this.state.burstKva > this.state.thresholdKva;
		},
		scaleMax(): number {
			const burst = this.showBurstTick ? this.state.burstKva : 0;
			return Math.max(this.state.thresholdKva, burst, this.total, 0.1) * 1.08;
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
					label: this.segLabel(this.house),
					title: `${this.$t("loadManagement.ledger.house")} ${this.fmtKva(this.house)} kVA`,
				},
			];
			for (const c of this.scaledCars) {
				res.push({
					id: c.id,
					width: w(c.kva),
					color: c.color,
					cls: "",
					label: this.segLabel(c.kva),
					title: `${c.name} ${this.fmtKva(c.kva)} kVA · ${this.fmtNumber(c.amps, 1)} A`,
				});
			}
			if (this.free > 0.01) {
				res.push({
					id: "free",
					width: w(this.free),
					color: "",
					cls: "ledger-seg--free",
					label: "",
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
				},
				...this.scaledCars.map((c) => ({
					id: c.id,
					name: c.name,
					value: `${this.fmtKva(c.kva)} kVA · ${this.fmtNumber(c.amps, 1)} A`,
					color: c.color,
					cls: "",
				})),
			];
			if (this.free > 0.01) {
				res.push({
					id: "free",
					name: this.$t("loadManagement.ledger.free"),
					value: `${this.fmtKva(this.free)} kVA`,
					color: "",
					cls: "swatch--free",
				});
			} else {
				res.push({
					id: "over",
					name: this.$t("loadManagement.ledger.over"),
					value: `${this.fmtKva(this.total - this.state.thresholdKva)} kVA`,
					color: "",
					cls: "swatch--over",
				});
			}
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
		phaseLabel(): string {
			if (!this.state.enabled) return this.$t("loadManagement.phase.off");
			if (!this.state.running) return this.$t("loadManagement.phase.standby");
			if (this.blind) return this.$t("loadManagement.phase.blind");
			if (this.state.phase === "burst") return this.$t("loadManagement.phase.burst");
			return this.$t("loadManagement.phase.base");
		},
		phaseClass(): string {
			if (!this.state.enabled) return "text-bg-danger";
			if (!this.state.running) return "text-bg-secondary";
			if (this.blind) return "text-bg-warning";
			if (this.state.phase === "burst") return "phase-badge--burst";
			return "text-bg-success";
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
					this.$t("loadManagement.status.charging", { name: lp.title, amps: lp.setpointA })
				);
			const drawing = lps.filter((lp) => lp.paused && lp.measuredA > 0.5);
			const still = drawing.map((lp) =>
				this.$t("loadManagement.status.stillDrawing", {
					name: lp.title,
					amps: this.fmtNumber(lp.measuredA, 1),
				})
			);
			const waiting = lps
				.filter((lp) => lp.connected && lp.wants && lp.paused && !drawing.includes(lp))
				.map((lp) => this.$t("loadManagement.status.waiting", { name: lp.title }));
			if (!charging.length && !still.length && !waiting.length) {
				return this.$t("loadManagement.status.noCar", { line });
			}
			const risk =
				s.vaKva > s.thresholdKva
					? this.$t("loadManagement.status.over", {
							line,
							kva: this.fmtKva(s.vaKva - s.thresholdKva),
						})
					: this.$t("loadManagement.status.under", { line });
			return `${[...charging, ...still, ...waiting].join("; ")}. ${risk}`;
		},
		superchargeLine(): string {
			const on = (this.state.loadpoints || []).filter((lp) => lp.supercharge);
			if (!on.length) return "";
			if (this.state.burstStoodDown) {
				return this.$t("loadManagement.supercharge.stoodDown", {
					reason: this.state.burstStoodDown,
				});
			}
			const parts = on.map((lp) =>
				lp.superchargeUntil
					? this.$t("loadManagement.supercharge.onUntil", {
							name: lp.title,
							time: this.fmtUntil(lp.superchargeUntil),
						})
					: this.$t("loadManagement.supercharge.onForever", { name: lp.title })
			);
			return `${this.$t("loadManagement.supercharge.on")}: ${parts.join(", ")}`;
		},
		closenessPct(): number {
			return Math.min(100, Math.max(0, Math.round((this.state.closeness || 0) * 100)));
		},
		plannedPct(): number {
			return Math.round((this.state.plannedCloseness || 0) * 100);
		},
		gaugeClass(): string {
			const c = this.state.closeness || 0;
			if (c >= 0.5) return "gauge-fill--danger";
			if (c >= 0.25) return "gauge-fill--warning";
			return "gauge-fill--ok";
		},
		patienceLabel(): string {
			if (!this.state.running) return "—";
			if (this.state.tiS <= 0) return this.$t("loadManagement.patience.clear");
			return this.$t("loadManagement.patience.banked", {
				pct: this.closenessPct,
				s: this.fmtNumber(this.state.tiS, 0),
			});
		},
	},
	methods: {
		pos(kva: number): number {
			return Math.max(0, Math.min(100, (100 * kva) / this.scaleMax));
		},
		fmtKva(kva: number): string {
			return this.fmtNumber(kva || 0, 2);
		},
		segLabel(kva: number): string {
			return kva / this.scaleMax > 0.12 ? this.fmtKva(kva) : "";
		},
		fmtUntil(iso: string): string {
			const d = new Date(iso);
			const soon = d.getTime() - Date.now() < 18 * 3600 * 1000;
			return soon
				? d.toLocaleTimeString(this.$i18n.locale, { hour: "2-digit", minute: "2-digit" })
				: d.toLocaleString(this.$i18n.locale, {
						weekday: "short",
						hour: "2-digit",
						minute: "2-digit",
					});
		},
	},
});
</script>

<style scoped>
.min-w-0 {
	min-width: 0;
}
.hero {
	display: flex;
	align-items: baseline;
	gap: 0.4rem;
	line-height: 1.1;
}
.hero-value {
	font-size: 2.5rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}
.hero-unit {
	font-size: 1.25rem;
	color: var(--evcc-gray);
}
.status {
	margin-top: 0.5rem;
}
.phase-badge--burst {
	background-color: var(--evcc-orange);
	color: var(--bs-dark);
}
.ledger {
	position: relative;
	padding-top: 1.5rem;
}
.ledger--burst {
	padding-bottom: 1.5rem;
}
.ledger-bar {
	--height: 2.5rem;
	height: var(--height);
	border-radius: 10px;
	display: flex;
	overflow: hidden;
	background: var(--evcc-gray-15);
	-webkit-font-smoothing: antialiased;
}
.ledger-seg {
	display: flex;
	align-items: center;
	justify-content: center;
	overflow: hidden;
	white-space: nowrap;
	color: var(--bs-dark);
	transition: width var(--evcc-transition-medium) linear;
	font-variant-numeric: tabular-nums;
}
.ledger-seg-label {
	margin: 0 0.2rem;
	overflow: hidden;
}
.ledger-seg--house {
	background-color: var(--evcc-grid);
	color: var(--bs-white);
}
html.dark .ledger-seg--house {
	color: var(--bs-dark);
}
.ledger-seg--free {
	background: repeating-linear-gradient(
		135deg,
		var(--evcc-gray-25) 0 4px,
		transparent 4px 9px
	);
}
.ledger-tick {
	position: absolute;
	top: 0;
	width: 0;
	white-space: nowrap;
	font-size: 0.75rem;
}
.ledger-tick::before {
	content: "";
	position: absolute;
	left: -1px;
	top: 1.25rem;
	height: calc(2.5rem + 0.5rem);
	border-left: 2px solid var(--evcc-default-text);
}
.ledger-tick span {
	position: absolute;
	left: 6px;
	font-weight: 600;
}
.ledger-tick--flip span {
	left: auto;
	right: 6px;
}
.ledger-tick--burst::before {
	border-left: 2px dashed var(--evcc-orange);
}
.ledger-tick--burst span {
	top: calc(1.5rem + 2.5rem + 0.35rem);
	color: var(--evcc-gray);
	font-weight: normal;
}
.swatch {
	display: inline-block;
	width: 0.75rem;
	height: 0.75rem;
	border-radius: 3px;
}
.swatch--house {
	background-color: var(--evcc-grid);
}
.swatch--free {
	background: repeating-linear-gradient(135deg, var(--evcc-gray-50) 0 2px, transparent 2px 4px);
	border: 1px solid var(--evcc-gray-25);
}
.swatch--over {
	background-color: var(--evcc-red);
}
.gauge {
	position: relative;
	height: 0.5rem;
	border-radius: 999px;
	background: var(--evcc-gray-15);
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
	top: -0.25rem;
	width: 2px;
	height: 1rem;
	background: var(--evcc-default-text);
}
@media (prefers-reduced-motion: reduce) {
	.ledger-seg,
	.gauge-fill {
		transition: none;
	}
}
</style>
