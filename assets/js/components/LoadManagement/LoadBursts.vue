<template>
	<Card :title="$t('loadManagement.bursts.title')" data-testid="load-bursts">
		<div v-if="!rows.length" class="empty" data-testid="load-bursts-empty">
			<strong>{{ $t("loadManagement.bursts.none") }}</strong>
			<span class="text-muted small">{{ $t("loadManagement.bursts.noneHelp") }}</span>
		</div>
		<template v-else>
			<div class="chips" data-testid="load-bursts-summary">
				<template v-for="(c, ci) in chips" :key="c.id">
					<span v-if="ci > 0" class="sep" aria-hidden="true">·</span>
					<button
						v-if="c.id === 'u' && c.value !== '0'"
						type="button"
						class="btn btn-sm btn-pill filter"
						:aria-pressed="onlyEarly"
						data-testid="load-bursts-early"
						@click="onlyEarly = !onlyEarly"
					>
						<svg
							width="13"
							height="13"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2.4"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"
						>
							<path d="M3 5h18l-7 8v6l-4 2v-8z" />
						</svg>
						<strong>{{ c.value }}</strong> {{ c.label }}
						<span v-if="onlyEarly" class="chip-x" aria-hidden="true">×</span>
					</button>
					<span v-else class="stat"
						><strong>{{ c.value }}</strong> {{ c.label }}</span
					>
				</template>
			</div>
			<button
				v-if="incoming.length && !onlyEarly"
				type="button"
				class="btn btn-sm btn-pill new-pill mb-3"
				data-testid="load-bursts-new"
				@click="showIncoming"
			>
				{{ $t("loadManagement.bursts.newOnes", { n: newCount }, newCount) }}
			</button>

			<!-- narrow screens: one block per burst, no sideways scrolling -->
			<ul class="stack d-lg-none">
				<template v-for="(r, i) in shown" :key="i">
					<li v-if="newDay(i)" class="stack-day">{{ dayOf(r) }}</li>
					<li class="stack-row" :class="{ 'row--early': early(r) }">
						<div class="d-flex justify-content-between gap-2">
							<strong class="text-truncate">{{
								r.loadpointTitle || r.loadpoint
							}}</strong>
							<span class="text-muted text-nowrap">
								<span v-if="!early(r)" class="exit-ok me-1"
									>✓<span class="visually-hidden">{{
										exitLabel(r.exit)
									}}</span></span
								>{{ fmtAt(r.at) }}</span
							>
						</div>
						<div class="stack-line">
							{{
								$t("loadManagement.bursts.stackLine", {
									s: unit(r.wallS, 0, "s"),
									kva: unit(r.peakKva, 2, "kVA"),
									pct: unit(r.peakCloseness * 100, 0, "%"),
								})
							}}
						</div>
						<div v-if="early(r)" class="mt-1">
							<span :class="exitClass(r)">{{ exitLabel(r.exit) }}</span>
						</div>
					</li>
				</template>
			</ul>

			<div class="d-none d-lg-block">
				<table class="table table-sm align-middle small mb-0 bursts">
					<colgroup>
						<col class="c-time" />
						<col />
						<col class="c-num" />
						<col class="c-peak" />
						<col class="c-num" />
						<col class="c-out" />
					</colgroup>
					<thead>
						<tr>
							<th scope="col">{{ $t("loadManagement.bursts.at") }}</th>
							<th scope="col">{{ $t("loadManagement.bursts.loadpoint") }}</th>
							<th scope="col" class="text-end">
								{{ $t("loadManagement.bursts.lasted") }}
							</th>
							<th scope="col" class="text-end">
								{{ $t("loadManagement.bursts.peak") }}
							</th>
							<th scope="col" class="text-end">
								{{ $t("loadManagement.bursts.closeness") }}
							</th>
							<th scope="col" class="ps-3">{{ $t("loadManagement.bursts.exit") }}</th>
						</tr>
					</thead>
					<tbody>
						<template v-for="(r, i) in shown" :key="i">
							<tr v-if="newDay(i)" class="day-row">
								<th colspan="6" scope="rowgroup">{{ dayOf(r) }}</th>
							</tr>
							<tr :class="{ 'row--early': early(r) }">
								<td class="text-nowrap">{{ fmtAt(r.at) }}</td>
								<td class="text-truncate">{{ r.loadpointTitle || r.loadpoint }}</td>
								<td class="text-end text-nowrap">{{ unit(r.wallS, 0, "s") }}</td>
								<td class="text-end text-nowrap">
									{{ unit(r.peakKva, 2, "kVA") }}
								</td>
								<td class="text-end text-nowrap">
									{{ unit(r.peakCloseness * 100, 0, "%") }}
								</td>
								<td class="ps-3">
									<span
										v-if="early(r)"
										:class="exitClass(r)"
										:title="exitHelp(r.exit)"
										>{{ exitLabel(r.exit) }}</span
									>
									<span v-else class="exit-ok" :title="exitHelp(r.exit)"
										>✓<span class="visually-hidden">{{
											exitLabel(r.exit)
										}}</span></span
									>
								</td>
							</tr>
						</template>
					</tbody>
				</table>
			</div>
			<div
				class="d-flex justify-content-between align-items-center gap-3 mt-3 small text-muted"
			>
				<span>{{
					$t(
						onlyEarly
							? "loadManagement.bursts.showingEarly"
							: rows.length >= KEPT
								? "loadManagement.bursts.showingKept"
								: "loadManagement.bursts.showing",
						{ n: shown.length, total: rows.length }
					)
				}}</span>
				<button
					v-if="shown.length < rows.length"
					type="button"
					class="btn btn-pill older"
					@click="showOlder"
				>
					{{ $t("loadManagement.bursts.older") }}
				</button>
			</div>
			<dl class="endings" data-testid="load-bursts-endings">
				<div v-for="e in endings" :key="e.key" class="ending">
					<dt>
						<span v-if="e.key === 'deadline'" class="exit-ok" aria-hidden="true"
							>✓ </span
						>{{ e.label }}
					</dt>
					<dd>{{ e.help }}</dd>
				</div>
			</dl>
		</template>
	</Card>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import Card from "../Helper/Card.vue";
import { fmtDayShort } from "./format";
import type { BurstRecord } from "@/types/supercharge";

const PAGE = 25;
// the server keeps this many bursts, see burstLogKeep
const KEPT = 200;
const EXITS = ["deadline", "bump margin", "closeness", "meter blind", "no draw", "icp alarm"];

export default defineComponent({
	name: "LoadBursts",
	components: { Card },
	mixins: [formatter],
	props: {
		bursts: { type: Number, default: 0 },
	},
	data() {
		// bursts that arrive while the list is open wait behind a pill instead of pushing rows
		return {
			records: [] as BurstRecord[],
			incoming: [] as BurstRecord[],
			limit: PAGE,
			PAGE,
			KEPT,
			onlyEarly: false,
		};
	},
	computed: {
		rows(): BurstRecord[] {
			const all = [...this.records].reverse();
			return this.onlyEarly ? all.filter((r) => this.early(r)) : all;
		},
		shown(): BurstRecord[] {
			return this.rows.slice(0, this.limit);
		},
		chips() {
			const r = this.records;
			const early = r.filter((x) => this.early(x)).length;
			const avg = r.reduce((a, x) => a + (x.wallS || 0), 0) / (r.length || 1);
			const peak = Math.max(0, ...r.map((x) => x.peakCloseness || 0));
			return [
				{
					id: "n",
					value: String(r.length),
					label: this.$t("loadManagement.bursts.chipCount", r.length),
					warn: false,
				},
				{
					id: "s",
					value: this.unit(avg, 0, "s"),
					label: this.$t("loadManagement.bursts.chipAverage"),
					warn: false,
				},
				{
					id: "p",
					value: this.unit(peak * 100, 0, "%"),
					label: this.$t("loadManagement.bursts.chipPeak"),
					warn: false,
				},
				{
					id: "u",
					value: String(early),
					label: this.$t("loadManagement.bursts.chipEarly"),
					warn: early > 0,
				},
			];
		},
		// every way a burst in the list ended, explained where touch can read it too
		endings() {
			const seen = [...new Set(this.rows.map((r) => this.exitKey(r.exit)))];
			return seen
				.filter((k) => k !== "other")
				.map((k) => ({
					key: k,
					label: this.$t(`loadManagement.bursts.exits.${k}.label`),
					help: this.$t(`loadManagement.bursts.exits.${k}.help`),
				}));
		},
		newCount(): number {
			return Math.max(0, this.incoming.length - this.records.length);
		},
	},
	watch: {
		bursts() {
			this.load();
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		async load() {
			try {
				const res = await api.get("supercharging/bursts");
				const next = (res.data as BurstRecord[]) || [];
				if (this.records.length && next.length > this.records.length) {
					this.incoming = next;
				} else {
					this.records = next;
				}
			} catch {
				// keep what is shown
			}
		},
		showIncoming() {
			this.records = this.incoming;
			this.incoming = [];
		},
		newDay(i: number): boolean {
			return i === 0 || this.dayOf(this.shown[i]!) !== this.dayOf(this.shown[i - 1]!);
		},
		early(r: BurstRecord): boolean {
			return r.exit !== "deadline";
		},
		exitClass(r: BurstRecord): string {
			return this.early(r) ? "exit exit--warn" : "exit-ok";
		},
		unit(v: number, digits: number, u: string): string {
			return `${this.fmtNumber(v, digits)}\u00a0${u}`;
		},
		// the button stays under the pointer: the new rows appear above it
		showOlder(e: Event) {
			const btn = e.currentTarget as HTMLElement;
			const before = btn.getBoundingClientRect().top;
			this.limit += PAGE;
			this.$nextTick(() => {
				const after = btn.getBoundingClientRect().top;
				window.scrollBy({ top: after - before });
			});
		},
		exitKey(exit: string): string {
			return EXITS.includes(exit) ? exit.replace(/ /g, "_") : "other";
		},
		exitLabel(exit: string): string {
			const k = this.exitKey(exit);
			return k === "other" ? exit : this.$t(`loadManagement.bursts.exits.${k}.label`);
		},
		exitHelp(exit: string): string {
			const k = this.exitKey(exit);
			return k === "other" ? "" : this.$t(`loadManagement.bursts.exits.${k}.help`);
		},
		dayOf(r: BurstRecord): string {
			if (!r.at) return "";
			const d = new Date(r.at);
			if (d.toDateString() === new Date().toDateString()) {
				return this.$t("loadManagement.supercharge.today");
			}
			return fmtDayShort(d, this.$i18n?.locale);
		},
		fmtAt(at?: string): string {
			if (!at) return "";
			return new Intl.DateTimeFormat(this.$i18n?.locale, {
				hour: "2-digit",
				minute: "2-digit",
				second: "2-digit",
				hour12: false,
			}).format(new Date(at));
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
.stat {
	color: var(--evcc-gray);
	font-size: 0.875rem;
}
.stat strong {
	color: var(--evcc-default-text);
}
/* fixed columns, so filtering never shifts them or widens the table */
.bursts {
	table-layout: fixed;
}
.c-time {
	width: 6rem;
}
.c-num {
	width: 9.5rem;
}
.c-peak {
	width: 7rem;
}
.c-out {
	width: 11rem;
}
.chips {
	align-items: center;
	column-gap: 0.6rem;
	display: flex;
	flex-wrap: wrap;
	gap: 0.5rem;
	padding: 0;
	margin: 0 0 1rem;
	font-size: 0.875rem;
}
/* the one control among the figures: an evcc pill with a funnel */
.filter {
	display: inline-flex;
	align-items: center;
	gap: 0.35rem;
	--bs-btn-border-color: var(--evcc-orange);
	--bs-btn-hover-border-color: var(--evcc-orange);
}
.filter[aria-pressed="true"] {
	--bs-btn-bg: var(--evcc-orange);
	--bs-btn-hover-bg: var(--evcc-orange);
	--bs-btn-color: #1b1b24;
	--bs-btn-hover-color: #1b1b24;
	--bs-btn-active-bg: var(--evcc-orange);
	--bs-btn-active-color: #1b1b24;
}
.bursts > :not(caption) > * > * {
	border-bottom-color: var(--evcc-gray-15);
}
.endings {
	margin: 1.5rem 0 0;
	padding-top: 1rem;
	border-top: 1px solid var(--evcc-gray-25);
	font-size: 0.875rem;
	display: grid;
	gap: 0.5rem;
}
.sep {
	color: var(--evcc-gray);
}
/* a dot would dangle where the figures wrap */
@media (max-width: 575px) {
	.chips {
		column-gap: 1.25rem;
	}
	.sep {
		display: none;
	}
}
@media (min-width: 576px) {
	.ending {
		display: grid;
		grid-template-columns: 11rem minmax(0, 1fr);
		gap: 0 1rem;
	}
}
.ending dt {
	font-weight: 700;
}
.ending dd {
	margin: 0;
	color: var(--evcc-gray);
}
.new-pill {
	display: block;
}
.empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 0.35rem;
	padding: 2.5rem 0;
	text-align: center;
}
.exit {
	display: inline-flex;
	padding: 0.1rem 0.6rem;
	border-radius: 999px;
	font-size: 0.75rem;
	font-weight: 700;
}
.exit-ok {
	color: var(--evcc-darker-green);
	font-weight: 700;
}
.stack-line {
	margin-top: 0.15rem;
	color: var(--evcc-gray);
	font-variant-numeric: tabular-nums;
}
.chip-x {
	font-size: 1.1rem;
	line-height: 1;
	padding-left: 0.15rem;
}
.exit--warn {
	color: #9a5200;
	background: color-mix(in srgb, var(--evcc-orange) 14%, transparent);
}
html.dark .exit--warn {
	color: var(--evcc-orange);
}
.row--early > td,
.stack-row.row--early {
	background: color-mix(in srgb, var(--evcc-orange) 12%, transparent);
}
.bursts .day-row > th {
	padding-top: 1rem;
	font-weight: 700;
	border-bottom-width: 2px;
	text-transform: none;
	color: var(--evcc-default-text);
}
.bursts th {
	white-space: nowrap;
	font-weight: normal;
	text-transform: uppercase;
	color: var(--evcc-gray);
}
.bursts td,
.bursts th {
	padding-left: 0.75rem;
	padding-right: 0.75rem;
}
.stack {
	list-style: none;
	margin: 0;
	padding: 0;
	font-size: 0.875rem;
}
.stack-day {
	font-weight: 700;
	padding: 1rem 0 0.4rem;
	border-bottom: 2px solid var(--evcc-gray-25);
}
.stack-day:first-child {
	padding-top: 0;
}
.stack-row {
	border-radius: 0.5rem;
	padding: 0.6rem 0.5rem;
	border-bottom: 1px solid var(--evcc-gray-15);
}
</style>
