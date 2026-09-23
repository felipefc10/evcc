<template>
	<Card data-testid="load-loadpoint">
		<template #title>
			<span class="d-inline-flex align-items-center gap-2">
				<span class="swatch" :style="{ background: color }"></span>
				{{ lp.title }}
			</span>
		</template>
		<template #subtitle>{{ orderLabel }}</template>
		<template #actions>
			<div class="text-end">
				<div class="small text-muted">{{ $t("loadManagement.card.allowed") }}</div>
				<div class="amps">
					{{ lp.paused ? $t("loadManagement.card.paused") : `${lp.setpointA} A` }}
				</div>
			</div>
		</template>

		<p class="reason mb-2" :class="{ 'text-warning': warning }">{{ reason }}</p>
		<div class="bar mb-3" :class="{ 'bar--off': lp.paused }">
			<div class="bar-fill" :style="{ width: `${share}%`, background: color }"></div>
		</div>

		<div class="facts row row-cols-2 row-cols-md-4 g-2 small mb-3">
			<div v-for="f in facts" :key="f.label" class="col" :title="f.title">
				<div class="text-muted">{{ f.label }}</div>
				<div class="fw-bold text-truncate">{{ f.value }}</div>
			</div>
		</div>

		<hr class="my-3" />
		<div class="d-flex flex-wrap align-items-center justify-content-between gap-3">
			<SuperchargeSwitch
				:index="lp.index"
				:title="lp.title"
				:active="lp.supercharge"
				:until="lp.superchargeUntil"
				@open="$emit('open-supercharge', $event)"
			/>
			<div class="d-flex align-items-center gap-2" data-testid="load-priority">
				<span class="small">{{ $t("loadManagement.card.priority") }}</span>
				<div class="btn-group" role="group">
					<button
						type="button"
						class="btn btn-sm btn-outline-secondary"
						:disabled="priority <= 0 || saving"
						:aria-label="$t('loadManagement.card.priorityDown')"
						@click="setPriority(priority - 1)"
					>
						−
					</button>
					<span class="btn btn-sm btn-outline-secondary disabled prio-value">
						{{ priority }}
					</span>
					<button
						type="button"
						class="btn btn-sm btn-outline-secondary"
						:disabled="priority >= priorityMax || saving"
						:aria-label="$t('loadManagement.card.priorityUp')"
						@click="setPriority(priority + 1)"
					>
						+
					</button>
				</div>
			</div>
		</div>
		<p v-if="lp.supercharge && !lp.fast" class="small text-muted mt-2 mb-0">
			{{ $t("loadManagement.card.slowNote") }}
		</p>
		<p v-if="error" class="small text-danger mt-2 mb-0">{{ error }}</p>
	</Card>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import Card from "../Helper/Card.vue";
import SuperchargeSwitch from "./SuperchargeSwitch.vue";
import { loadpointReason } from "./loadpointReason";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

export default defineComponent({
	name: "LoadLoadpointCard",
	components: { Card, SuperchargeSwitch },
	mixins: [formatter],
	props: {
		lp: { type: Object as PropType<LoadLoadpoint>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
		color: { type: String, default: "" },
		rank: { type: Number, default: 0 },
		total: { type: Number, default: 1 },
		tied: Boolean,
	},
	emits: ["open-supercharge"],
	data() {
		return { pending: null as number | null, saving: false, error: "" };
	},
	computed: {
		priority(): number {
			return this.pending ?? this.lp.priority;
		},
		priorityMax(): number {
			return Math.max(10, this.lp.priority);
		},
		orderLabel(): string {
			if (this.total < 2) return "";
			if (this.tied) return this.$t("loadManagement.card.shares");
			return this.$t("loadManagement.card.order", { n: this.rank + 1 });
		},
		reason(): string {
			return loadpointReason(this.lp, this.state, {
				t: (k, v) => this.$t(k, v || {}),
				number: (n, d) => this.fmtNumber(n, d),
				duration: (s) => (s < 90 ? `${Math.round(s)} s` : this.fmtDurationLong(s, "short")),
				time: (iso) => this.fmtAbsoluteDate(new Date(iso)),
			});
		},
		warning(): boolean {
			return (this.lp.paused && this.lp.measuredA > 0.5) || this.lp.stoodOffS > 0;
		},
		share(): number {
			return this.lp.maxA > 0
				? Math.min(100, (100 * this.lp.setpointA) / (this.lp.maxA * (this.lp.phases || 1)))
				: 0;
		},
		facts() {
			const lp = this.lp;
			const res = [];
			if (lp.vehicle) res.push({ label: this.$t("loadManagement.card.car"), value: lp.vehicle });
			if (lp.soc > 0) {
				res.push({
					label: this.$t("loadManagement.card.charge"),
					value: lp.limitSoc ? `${Math.round(lp.soc)} % / ${lp.limitSoc} %` : `${Math.round(lp.soc)} %`,
				});
			}
			if (lp.forecast?.remainingKwh != null) {
				res.push({
					label: this.$t("loadManagement.card.toGo"),
					value: `${this.fmtNumber(lp.forecast.remainingKwh, 1)} kWh`,
				});
			}
			if (lp.forecast?.avgKw) {
				res.push({
					label: this.$t("loadManagement.card.average"),
					value: `${this.fmtNumber(lp.forecast.avgKw, 2)} kW`,
				});
			}
			res.push({
				label: this.$t("loadManagement.card.mode"),
				value: this.$te(`main.mode.${lp.mode}`) ? this.$t(`main.mode.${lp.mode}`) : lp.mode,
			});
			res.push({
				label: this.$t("loadManagement.card.range"),
				value: `${this.fmtNumber(lp.minA, 0)}–${this.fmtNumber(lp.maxA, 0)} A`,
			});
			res.push({
				label: this.$t("loadManagement.card.measured"),
				value: `${this.fmtNumber(lp.measuredA, 1)} A`,
				title: this.$t(`loadManagement.card.src.${lp.measuredSrc}`),
			});
			if (lp.ops) {
				res.push({ label: this.$t("loadManagement.card.pauses"), value: String(lp.ops) });
			}
			return res as { label: string; value: string; title?: string }[];
		},
	},
	watch: {
		"lp.priority"(p: number) {
			if (this.pending === p) this.pending = null;
		},
	},
	methods: {
		async setPriority(p: number) {
			this.error = "";
			this.pending = p;
			this.saving = true;
			try {
				await api.post(`loadpoints/${this.lp.index + 1}/priority/${p}`);
			} catch (e: any) {
				this.pending = null;
				this.error = e?.response?.data?.error || String(e);
			} finally {
				this.saving = false;
			}
		},
	},
});
</script>

<style scoped>
.swatch {
	display: inline-block;
	width: 0.75rem;
	height: 0.75rem;
	border-radius: 3px;
}
.amps {
	font-size: 1.5rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	line-height: 1.1;
}
.bar {
	height: 0.5rem;
	border-radius: 999px;
	background: var(--evcc-gray-15);
	overflow: hidden;
}
.bar-fill {
	height: 100%;
	border-radius: 999px;
	transition: width var(--evcc-transition-medium) linear;
}
.bar--off .bar-fill {
	opacity: 0.3;
}
.prio-value {
	min-width: 2.5rem;
	opacity: 1 !important;
	font-variant-numeric: tabular-nums;
}
</style>
