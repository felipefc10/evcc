<template>
	<div data-testid="load-diagnostics">
		<Card class="mb-4 box-pull-out" :title="$t('loadManagement.diagnostics.selftest')">
			<template #subtitle>{{ checkedLabel }}</template>
			<template #actions>
				<div class="d-flex gap-2">
					<button
						type="button"
						class="btn btn-sm btn-outline-secondary"
						:disabled="running"
						@click="runSelftest"
					>
						{{ $t("loadManagement.diagnostics.run") }}
					</button>
					<a
						class="btn btn-sm btn-outline-secondary"
						href="./api/supercharging/diagnostics"
						download="supercharging-diagnostics.json"
					>
						{{ $t("loadManagement.diagnostics.download") }}
					</a>
				</div>
			</template>
			<p v-if="!checks.length" class="text-muted mb-0">
				{{ $t("loadManagement.diagnostics.notRun") }}
			</p>
			<ul v-else class="list-unstyled mb-0">
				<li v-for="c in checks" :key="c.name" class="d-flex gap-2 py-1">
					<span :class="c.ok ? 'text-primary' : 'text-danger'" class="fw-bold check">
						{{ c.ok ? "✓" : "✗" }}
					</span>
					<span>
						<span class="fw-bold">{{ c.name }}</span>
						<span class="text-muted"> · {{ c.detail }}</span>
					</span>
				</li>
			</ul>
		</Card>

		<Card class="mb-4 box-pull-out" :title="$t('loadManagement.diagnostics.sensors')">
			<div class="row row-cols-2 row-cols-md-4 g-3 small">
				<div v-for="f in sensorFacts" :key="f.label" class="col" :title="f.title">
					<div class="text-muted">{{ f.label }}</div>
					<div class="fw-bold">{{ f.value }}</div>
				</div>
			</div>
			<p class="small text-muted mt-3 mb-0">{{ $t("loadManagement.diagnostics.sensorsHelp") }}</p>
		</Card>

		<Card class="box-pull-out" :title="$t('loadManagement.diagnostics.learned')">
			<p v-if="!learned.length" class="text-muted mb-0">
				{{ $t("loadManagement.diagnostics.nothingLearned") }}
			</p>
			<div v-else class="table-responsive">
				<table class="table table-sm small mb-0">
					<thead>
						<tr>
							<th>{{ $t("loadManagement.diagnostics.pair") }}</th>
							<th class="text-end">{{ $t("loadManagement.diagnostics.rampDown") }}</th>
							<th class="text-end">{{ $t("loadManagement.diagnostics.latencyDown") }}</th>
							<th class="text-end">{{ $t("loadManagement.diagnostics.latencyUp") }}</th>
							<th class="text-end">{{ $t("loadManagement.diagnostics.entryOffset") }}</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="b in learned" :key="b.key">
							<td>{{ pairLabel(b) }}</td>
							<td class="text-end">{{ val(b.rampDown, "A/s", b.downs) }}</td>
							<td class="text-end">{{ val(b.latencyDown, "s", b.latsDown) }}</td>
							<td class="text-end">{{ val(b.latencyUp, "s", b.latsUp) }}</td>
							<td class="text-end">{{ val(b.entryOffsetA, "A", b.entries) }}</td>
						</tr>
					</tbody>
				</table>
			</div>
			<p class="small text-muted mt-3 mb-0">{{ $t("loadManagement.diagnostics.learnedHelp") }}</p>
		</Card>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import Card from "../Helper/Card.vue";
import type { LoadBehaviour, LoadState } from "@/types/supercharge";

export default defineComponent({
	name: "LoadDiagnostics",
	components: { Card },
	mixins: [formatter],
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	data() {
		return { running: false };
	},
	computed: {
		checks() {
			return this.state.checks || [];
		},
		learned(): LoadBehaviour[] {
			return this.state.learned || [];
		},
		checkedLabel(): string {
			if (!this.state.checkedAt) return "";
			const failed = this.checks.filter((c) => !c.ok).length;
			const when = this.fmtAbsoluteDate(new Date(this.state.checkedAt));
			return failed
				? this.$t("loadManagement.diagnostics.failed", { n: failed, when })
				: this.$t("loadManagement.diagnostics.passed", { when });
		},
		sensorFacts() {
			const s = this.state;
			const m = s.meter || ({} as LoadState["meter"]);
			const na = "—";
			return [
				{ label: this.$t("loadManagement.diagnostics.meter"), value: s.running ? `${this.fmtNumber(m.kva, 2)} kVA` : na },
				{ label: this.$t("loadManagement.diagnostics.watts"), value: s.running ? `${this.fmtNumber(m.watts, 0)} W` : na },
				{ label: this.$t("loadManagement.diagnostics.vars"), value: s.running ? `${this.fmtNumber(m.vars, 0)} var` : na },
				{ label: this.$t("loadManagement.diagnostics.volts"), value: s.running ? `${this.fmtNumber(m.volts, 1)} V` : na },
				{ label: this.$t("loadManagement.diagnostics.latency"), value: s.running ? `${m.latencyMs} ms` : na },
				{ label: this.$t("loadManagement.diagnostics.floor"), value: `${this.fmtNumber(s.floorKva, 2)} kVA`, title: this.$t("loadManagement.diagnostics.floorHelp") },
				{ label: this.$t("loadManagement.diagnostics.claim"), value: `${this.fmtNumber(s.claimKva, 2)} / ${this.fmtNumber(s.creditKva, 2)} kVA`, title: this.$t("loadManagement.diagnostics.claimHelp") },
				{ label: this.$t("loadManagement.diagnostics.disagreements"), value: String(s.disagreements || 0) },
				{ label: this.$t("loadManagement.diagnostics.sourceOhm"), value: s.sourceOhm > 0 ? `${this.fmtNumber(s.sourceOhm, 3)} Ω` : na, title: this.$t("loadManagement.diagnostics.sourceOhmHelp") },
				{ label: this.$t("loadManagement.diagnostics.voltsAtGoal"), value: s.voltsAtGoal > 0 ? `${this.fmtNumber(s.voltsAtGoal, 1)} V` : na },
				{ label: this.$t("loadManagement.diagnostics.temp"), value: s.tempC != null ? `${this.fmtNumber(s.tempC, 0)} °C` : na },
				{ label: this.$t("loadManagement.diagnostics.peak"), value: `${this.fmtNumber(s.peakCloseness, 2)}${s.peakClosenessBlind ? " *" : ""}`, title: s.peakClosenessBlind ? this.$t("loadManagement.diagnostics.peakBlind") : "" },
			];
		},
	},
	methods: {
		async runSelftest() {
			this.running = true;
			try {
				await api.post("supercharging/selftest");
			} finally {
				this.running = false;
			}
		},
		pairLabel(b: LoadBehaviour): string {
			return b.vehicle ? `${b.label || b.charger} · ${b.vehicle}` : b.label || b.charger;
		},
		val(v: number, unit: string, n: number): string {
			return v ? `${this.fmtNumber(v, 2)} ${unit} (${n})` : `— (${n})`;
		},
	},
});
</script>

<style scoped>
.check {
	width: 1rem;
}
</style>
