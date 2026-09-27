<template>
	<div data-testid="load-diagnostics">
		<Card class="mb-4" :title="$t('loadManagement.diagnostics.selftest')">
			<template #actions>
				<div class="actions">
					<button
						type="button"
						class="btn btn-primary text-nowrap"
						:disabled="running"
						@click="runSelftest"
					>
						<span
							v-if="running"
							class="spinner-border spinner-border-sm me-1"
							role="status"
							aria-hidden="true"
						></span>
						{{ $t("loadManagement.diagnostics.run") }}
					</button>
					<a
						class="btn btn-outline-secondary text-nowrap"
						href="./api/supercharging/diagnostics"
						download="supercharging-diagnostics.json"
					>
						{{ $t("loadManagement.diagnostics.download") }}
					</a>
				</div>
			</template>
			<p
				v-if="checks.length"
				role="status"
				class="verdict mb-3"
				:class="failedChecks.length ? 'verdict--bad' : 'verdict--ok'"
			>
				{{ checkedLabel }}
			</p>
			<p v-if="error" class="small text-danger" role="alert">{{ error }}</p>
			<p v-if="!checks.length" class="text-muted mb-0">
				{{ $t("loadManagement.diagnostics.notRun") }}
			</p>
			<template v-else>
				<div v-for="c in failedChecks" :key="c.name" class="attention mb-2" role="alert">
					<svg
						width="20"
						height="20"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2.2"
						stroke-linecap="round"
						stroke-linejoin="round"
						aria-hidden="true"
						class="flex-shrink-0"
					>
						<path d="M12 3 2 21h20L12 3z" />
						<path d="M12 10v5M12 18h.01" />
					</svg>
					<div class="flex-grow-1 min-w-0">
						<strong class="d-block">{{ c.name }}</strong>
						<span class="small">{{ c.detail }}</span>
					</div>
					<button
						type="button"
						class="btn btn-sm btn-outline-secondary flex-shrink-0 text-nowrap"
						@click="$emit('open-settings')"
					>
						{{ $t("loadManagement.diagnostics.checkSettings") }}
					</button>
				</div>
				<details class="passed" :open="!failedChecks.length">
					<summary>
						{{ passedSummary }}
					</summary>
					<ul class="list-unstyled mb-0 mt-2">
						<li v-for="c in passedChecks" :key="c.name" class="d-flex gap-2 py-1">
							<svg
								v-if="c.info"
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2.2"
								stroke-linecap="round"
								aria-hidden="true"
								class="text-muted flex-shrink-0 mt-1"
							>
								<circle cx="12" cy="12" r="9" />
								<path d="M12 11v6M12 7.5h.01" />
							</svg>
							<svg
								v-else
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2.6"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
								class="text-primary flex-shrink-0 mt-1"
							>
								<path d="m5 12 5 5 9-10" />
							</svg>
							<span>
								<span class="fw-bold">{{ c.name }}</span>
								<span class="text-muted"> · {{ c.detail }}</span>
							</span>
						</li>
					</ul>
				</details>
			</template>
		</Card>

		<details class="support evcc-card round-box p-3 p-sm-4" data-testid="load-support">
			<summary>{{ $t("loadManagement.diagnostics.support") }}</summary>
			<p class="small text-muted mt-2">{{ $t("loadManagement.diagnostics.supportHelp") }}</p>
			<section v-for="sec in factSections" :key="sec.id" class="support-section">
				<h3 class="section-title">{{ sec.title }}</h3>
				<p class="section-help">{{ sec.help }}</p>
				<div class="row row-cols-2 row-cols-xl-4 g-3 small" :data-testid="sec.testid">
					<div v-for="f in sec.facts" :key="f.label" class="col">
						<div class="fact-label">{{ f.label }}</div>
						<div class="fact-value">{{ f.value }}</div>
						<div v-if="f.title" class="fact-help">{{ f.title }}</div>
					</div>
				</div>
			</section>

			<section class="support-section">
				<h3 class="section-title">{{ $t("loadManagement.diagnostics.learned") }}</h3>
				<p class="section-help">{{ $t("loadManagement.diagnostics.learnedHelp") }}</p>
				<p v-if="!learned.length" class="text-muted mb-0">
					{{ $t("loadManagement.diagnostics.nothingLearned") }}
				</p>
				<div v-else class="table-responsive">
					<table class="table table-sm small mb-0 learned-table">
						<thead>
							<tr>
								<th>{{ $t("loadManagement.diagnostics.pair") }}</th>
								<th class="text-end d-none d-md-table-cell">
									{{ $t("loadManagement.diagnostics.rampUp") }}
								</th>
								<th class="text-end">
									{{ $t("loadManagement.diagnostics.rampDown") }}
								</th>
								<th class="text-end">
									{{ $t("loadManagement.diagnostics.latencyDown") }}
								</th>
								<th class="text-end d-none d-sm-table-cell">
									{{ $t("loadManagement.diagnostics.latencyUp") }}
								</th>
								<th class="text-end d-none d-sm-table-cell">
									{{ $t("loadManagement.diagnostics.entryOffset") }}
								</th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="b in learned" :key="b.key">
								<td>{{ pairLabel(b) }}</td>
								<td class="text-end d-none d-md-table-cell">
									{{ val(b.rampUp, "A/s", b.ups) }}
								</td>
								<td class="text-end">{{ val(b.rampDown, "A/s", b.downs) }}</td>
								<td class="text-end">{{ val(b.latencyDown, "s", b.latsDown) }}</td>
								<td class="text-end d-none d-sm-table-cell">
									{{ val(b.latencyUp, "s", b.latsUp) }}
								</td>
								<td class="text-end d-none d-sm-table-cell">
									{{ val(b.entryOffsetA, "A", b.entries) }}
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</section>
		</details>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import Card from "../Helper/Card.vue";
import { fmtDayShort } from "./format";
import { errorText } from "./state";
import type { LoadBehaviour, LoadCheck, LoadState } from "@/types/supercharge";

interface Fact {
	label: string;
	value: string;
	title?: string;
}

export default defineComponent({
	name: "LoadDiagnostics",
	components: { Card },
	mixins: [formatter],
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	emits: ["open-settings"],
	data() {
		return { running: false, error: "", result: null as LoadCheck[] | null, ranNow: false };
	},
	computed: {
		// the answer to a run shows at once; the websocket confirms it a moment later
		checks(): LoadCheck[] {
			return this.state.checks || this.result || [];
		},
		failedChecks(): LoadCheck[] {
			return this.checks.filter((c) => !c.ok);
		},
		passedChecks(): LoadCheck[] {
			return this.checks.filter((c) => c.ok);
		},
		// notes are limitations, not passes, so they are counted apart
		passedSummary(): string {
			const notes = this.passedChecks.filter((c) => c.info).length;
			const passed = this.passedChecks.length - notes;
			const text = this.$t("loadManagement.diagnostics.passedCount", { n: passed }, passed);
			if (!notes) return text;
			return `${text} · ${this.$t("loadManagement.diagnostics.notesCount", { n: notes }, notes)}`;
		},
		// pairs with nothing observed yet are noise, and one charger shows once per car
		learned(): LoadBehaviour[] {
			const seen = new Set<string>();
			return (this.state.learned || []).filter((b) => {
				const n = b.downs + b.latsDown + b.latsUp + b.entries;
				const id = this.pairLabel(b);
				if (!n || seen.has(id)) return false;
				seen.add(id);
				return true;
			});
		},
		checkedLabel(): string {
			if (!this.state.checkedAt) return "";
			const failed = this.failedChecks.length;
			const at = new Date(this.state.checkedAt);
			const when = this.ranNow
				? this.$t("loadManagement.diagnostics.ranNow")
				: `${this.dayOf(at)} ${this.fmtHourMinute(at)}`;
			return failed
				? this.$t("loadManagement.diagnostics.failed", { n: failed, when })
				: this.$t("loadManagement.diagnostics.passed", { when });
		},
		factSections() {
			return [
				{
					id: "control",
					title: this.$t("loadManagement.diagnostics.control"),
					help: this.$t("loadManagement.diagnostics.controlHelp"),
					testid: "load-control-facts",
					facts: this.controlFacts,
				},
				{
					id: "sensors",
					title: this.$t("loadManagement.diagnostics.sensors"),
					help: this.$t("loadManagement.diagnostics.sensorsHelp"),
					testid: undefined,
					facts: this.sensorFacts,
				},
			];
		},
		controlFacts(): Fact[] {
			const s = this.state;
			const na = "—";
			const run = s.running;
			const handedOut = (s.loadpoints || []).reduce((a, lp) => a + lp.setpointA, 0);
			return [
				{
					label: this.$t("loadManagement.diagnostics.polling"),
					value: this.$te(`loadManagement.diagnostics.pollModes.${s.pollMode}`)
						? this.$t(`loadManagement.diagnostics.pollModes.${s.pollMode}`)
						: s.pollMode || na,
				},
				{
					label: this.$t("loadManagement.stats.budget"),
					value: run ? `${this.fmtNumber(s.budgetA, 0)}\u00a0A` : na,
					title: this.$t("loadManagement.stats.budgetHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.handedOut"),
					value: run ? `${handedOut}\u00a0A` : na,
					title: this.$t("loadManagement.diagnostics.handedOutHelp"),
				},
				{
					label: this.$t("loadManagement.stats.trim"),
					value: run
						? `${s.trimA > 0 ? "+" : ""}${this.fmtNumber(s.trimA, 2)}\u00a0A`
						: na,
					title: this.$t("loadManagement.stats.trimHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.writes"),
					value: this.$t("loadManagement.diagnostics.writesValue", {
						n: s.writes || 0,
						h: this.fmtNumber(s.writesPerH || 0, 0),
					}),
					title: this.$t("loadManagement.stats.commandsHelp"),
				},
				{
					label: this.$t("loadManagement.stats.pauses"),
					value: String(s.contactorOps || 0),
					title: this.$t("loadManagement.stats.pausesHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.blind"),
					value: String(s.blind || 0),
					title: this.$t("loadManagement.diagnostics.blindHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.entryOffsetNow"),
					value: `${this.fmtNumber(s.entryOffsetA || 0, 2)} A${s.entryOffsetHeld ? " *" : ""}`,
					title: this.$t("loadManagement.diagnostics.entryOffsetHelp"),
				},
			];
		},
		sensorFacts(): Fact[] {
			const s = this.state;
			const m = s.meter || ({} as LoadState["meter"]);
			const na = "—";
			return [
				{
					label: this.$t("loadManagement.diagnostics.meter"),
					value: s.running ? `${this.fmtNumber(m.kva, 2)}\u00a0kVA` : na,
				},
				{
					label: this.$t("loadManagement.diagnostics.watts"),
					value: s.running ? `${this.fmtNumber(m.watts / 1000, 2)}\u00a0kW` : na,
				},
				{
					label: this.$t("loadManagement.diagnostics.vars"),
					value: s.running
						? `${this.fmtNumber(m.vars, 0).replace("-", "\u2212")}\u00a0var`
						: na,
				},
				{
					label: this.$t("loadManagement.diagnostics.volts"),
					value: s.running ? `${this.fmtNumber(m.volts, 1)}\u00a0V` : na,
				},
				{
					label: this.$t("loadManagement.diagnostics.latency"),
					value: s.running ? `${m.latencyMs}\u00a0ms` : na,
				},
				{
					label: this.$t("loadManagement.diagnostics.floor"),
					value: `${this.fmtNumber(s.floorKva, 2)}\u00a0kVA`,
					title: this.$t("loadManagement.diagnostics.floorHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.claim"),
					value: `${this.fmtNumber(s.claimKva, 2)} / ${this.fmtNumber(s.creditKva, 2)}\u00a0kVA`,
					title: this.$t("loadManagement.diagnostics.claimHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.disagreements"),
					value: String(s.disagreements || 0),
					title: this.$t("loadManagement.diagnostics.disagreementsHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.sourceOhm"),
					value: s.sourceOhm > 0 ? `${this.fmtNumber(s.sourceOhm, 3)}\u00a0Ω` : na,
					title: this.$t("loadManagement.diagnostics.sourceOhmHelp"),
				},
				{
					label: this.$t("loadManagement.diagnostics.voltsAtGoal"),
					value: s.voltsAtGoal > 0 ? `${this.fmtNumber(s.voltsAtGoal, 1)}\u00a0V` : na,
				},
				{
					label: this.$t("loadManagement.diagnostics.temp"),
					value: s.tempC != null ? `${this.fmtNumber(s.tempC, 0)}\u00a0°C` : na,
				},
				{
					label: this.$t("loadManagement.diagnostics.peak"),
					value: `${this.fmtNumber((s.peakCloseness || 0) * 100, 0)} %${s.peakClosenessBlind ? " *" : ""}`,
					title: s.peakClosenessBlind
						? this.$t("loadManagement.diagnostics.peakBlind")
						: "",
				},
			];
		},
	},
	methods: {
		dayOf(d: Date): string {
			if (d.toDateString() === new Date().toDateString()) {
				return this.$t("loadManagement.supercharge.todayLower");
			}
			return fmtDayShort(d, this.$i18n?.locale);
		},
		async runSelftest() {
			this.running = true;
			try {
				this.error = "";
				const res = await api.post("supercharging/selftest");
				this.result = (res.data as LoadCheck[]) || null;
				this.ranNow = true;
				setTimeout(() => (this.ranNow = false), 60000);
			} catch (e) {
				this.error = errorText(e);
			} finally {
				this.running = false;
			}
		},
		who(b: LoadBehaviour): string {
			const lp = (this.state.loadpoints || []).find((l) => l.name === b.charger);
			return lp?.title || b.label || b.charger;
		},
		// a charger learned with two cars shows twice, so the car tells the rows apart
		pairLabel(b: LoadBehaviour): string {
			const who = this.who(b);
			const twice = (this.state.learned || []).filter((x) => this.who(x) === who).length > 1;
			const car = b.vehicle && b.vehicle !== "?" && b.vehicle !== who ? b.vehicle : "";
			if (car) return `${who} · ${car}`;
			return twice ? `${who} · ${this.$t("loadManagement.diagnostics.unknownCar")}` : who;
		},
		val(v: number, unit: string, n: number): string {
			const digits = unit === "s" ? 1 : 2;
			const shown = v ? `${this.fmtNumber(v, digits)}\u00a0${unit}` : "—";
			return `${shown} (${n})`;
		},
	},
});
</script>

<style scoped>
:deep(.evcc-card-title) {
	font-weight: 700 !important;
	text-transform: uppercase;
	font-size: 1.25rem;
}

.min-w-0 {
	min-width: 0;
}
.attention {
	display: flex;
	align-items: center;
	gap: 0.9rem;
	padding: 0.85rem 1rem;
	border-radius: 1rem;
	color: var(--evcc-orange);
	background: color-mix(in srgb, var(--evcc-orange) 12%, transparent);
}
.attention > div,
.attention .btn {
	color: var(--evcc-default-text);
}
details > summary {
	list-style: none;
	display: flex;
	align-items: center;
	gap: 0.5rem;
}
details > summary::-webkit-details-marker {
	display: none;
}
details > summary::after {
	content: "";
	width: 0.55rem;
	height: 0.55rem;
	border-right: 2px solid currentColor;
	border-bottom: 2px solid currentColor;
	transform: rotate(45deg);
	margin-top: -0.25rem;
	transition: transform var(--evcc-transition-fast);
}
details[open] > summary::after {
	transform: rotate(-135deg);
	margin-top: 0.2rem;
}
.fact-label {
	text-transform: uppercase;
	color: var(--evcc-gray);
	font-size: 14px;
	font-weight: normal;
}
.fact-value {
	font-size: 1.1rem;
	font-weight: 700;
	color: var(--evcc-default-text);
	font-variant-numeric: tabular-nums;
}
.verdict {
	font-weight: 700;
}
.learned-table th {
	white-space: nowrap;
	font-weight: normal;
	text-transform: uppercase;
	color: var(--evcc-gray);
}
.learned-table td + td {
	white-space: nowrap;
}
.actions {
	display: flex;
	flex-wrap: wrap;
	gap: 0.5rem;
}
.support-section + .support-section {
	margin-top: 1.5rem;
	padding-top: 1.5rem;
	border-top: 1px solid var(--evcc-gray-25);
}
.section-title {
	font-size: 1rem;
	font-weight: 700;
	margin-bottom: 0.25rem;
}
.section-help {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin-bottom: 1rem;
}
@media (max-width: 575px) {
	.actions {
		width: 100%;
	}
	.actions > * {
		flex: 1 1 auto;
	}
}
.support > summary {
	justify-content: space-between;
	font-weight: 700;
	font-size: 1.25rem;
	text-transform: uppercase;
	cursor: pointer;
}
.support[open] > summary {
	margin-bottom: 1rem;
}
.passed summary {
	justify-content: space-between;
	font-weight: 700;
	cursor: pointer;
	padding: 0.35rem 0;
}
.actions .btn {
	min-height: 2.5rem;
	display: inline-flex;
	align-items: center;
	justify-content: center;
}
.verdict--ok {
	color: var(--evcc-darker-green);
}
html.dark .verdict--ok {
	color: var(--evcc-dark-green);
}
.verdict--bad {
	color: var(--bs-danger);
}
.fact-help {
	font-size: 0.8125rem;
	color: var(--evcc-gray);
	line-height: 1.35;
	margin-top: 0.15rem;
}
</style>
