<template>
	<div data-testid="load-import">
		<p v-if="importedFrom" class="small text-muted mb-3">
			{{ $t("loadManagement.import.already", { from: importedFrom }) }}
		</p>
		<p v-if="step !== 'done'" class="step-line" data-testid="load-import-step">
			{{
				$t("loadManagement.import.stepOf", {
					n: steps.indexOf(step) + 1,
					total: steps.length,
					name: $t(`loadManagement.import.step.${step}`),
				})
			}}
		</p>

		<template v-if="step === 'files'">
			<div class="fw-bold mb-2">{{ $t("loadManagement.import.files") }}</div>
			<input
				id="lmImportFiles"
				ref="files"
				class="visually-hidden"
				type="file"
				accept=".json,application/json"
				multiple
				data-testid="load-import-files"
				@change="read"
			/>
			<label class="btn btn-outline-secondary btn-pill pick-files" for="lmImportFiles">
				{{ $t("loadManagement.import.choose") }}
			</label>
			<div class="form-text">{{ $t("loadManagement.import.filesHelp") }}</div>
			<ul v-if="ignored.length && !unknownOnly" class="small text-muted mt-2 mb-0 ps-3">
				<li v-for="f in ignored" :key="f">
					{{ f }}: {{ $t("loadManagement.import.kind.unknown", 1) }}
				</li>
			</ul>
			<p v-if="unknownOnly" class="small text-danger mt-2 mb-0" role="alert">
				{{ $t("loadManagement.import.nothing", { files: ignored.join(", ") }) }}
			</p>
		</template>

		<template v-else-if="step === 'review'">
			<p class="small mb-3">
				{{ $t("loadManagement.import.read", { files: foundNames }) }}
				<template v-if="ignored.length">
					{{ ignored.join(", ") }}:
					{{ $t("loadManagement.import.kind.unknown", ignored.length) }}.
				</template>
				<button
					type="button"
					class="btn btn-link btn-sm p-0 align-baseline text-reset text-decoration-underline"
					@click="restart"
				>
					{{ $t("loadManagement.import.other") }}
				</button>
			</p>
			<div class="review">
				<div v-if="keys.length">
					<div class="fw-bold mb-1">{{ $t("loadManagement.import.mapTitle") }}</div>
					<div class="form-text mt-0 mb-2">{{ $t("loadManagement.import.mapHelp") }}</div>
					<div v-for="k in keys" :key="k" class="map-row">
						<label class="map-key" :for="`lm-map-${k}`">{{ keyLabel(k) }}</label>
						<select :id="`lm-map-${k}`" v-model="names[k]" class="form-select">
							<option value="">{{ $t("loadManagement.import.skip") }}</option>
							<option v-for="lp in loadpoints" :key="lp.name" :value="lp.name">
								{{ lp.title }}
							</option>
						</select>
					</div>
				</div>
				<div>
					<div class="fw-bold mb-1">{{ $t("loadManagement.import.changes") }}</div>
					<p v-if="!diff.length" class="small text-muted mb-0">
						{{ $t("loadManagement.import.noChanges") }}
					</p>
					<dl v-else class="diff small mb-1" data-testid="load-import-diff">
						<div v-for="d in diff" :key="d.key" class="diff-row">
							<dt>{{ d.label }}</dt>
							<dd>
								<span class="text-muted">{{ d.from }}</span>
								<span class="text-muted"
									>{{ " " }}{{ $t("loadManagement.import.to") }}{{ " " }}</span
								>
								<strong>{{ d.to }}</strong>
							</dd>
						</div>
					</dl>
					<p v-if="settings && sameCount" class="small text-muted mb-0">
						{{ $t("loadManagement.import.same", { n: sameCount }, sameCount) }}
					</p>
					<p v-if="learnedPairs" class="small mb-0 mt-1">
						{{
							$t(
								"loadManagement.import.learnedPairs",
								{ n: learnedPairs },
								learnedPairs
							)
						}}
					</p>
					<div v-if="runtimeNames.length" class="form-check mt-2">
						<input
							id="lmImportRuntime"
							v-model="withRuntime"
							class="form-check-input"
							type="checkbox"
						/>
						<label class="form-check-label" for="lmImportRuntime">
							{{
								$t("loadManagement.import.runtime", {
									names: runtimeNames.join(", "),
								})
							}}
						</label>
					</div>
				</div>
			</div>
			<div v-if="lineMove" class="line-move mt-3" role="alert">
				<p class="mb-2">{{ lineMove }}</p>
				<div class="form-check mb-0">
					<input
						id="lmImportLine"
						v-model="lineOk"
						class="form-check-input"
						type="checkbox"
						data-testid="load-import-line-ok"
					/>
					<label class="form-check-label" for="lmImportLine">{{
						$t("loadManagement.import.lineConfirm", { to: lineTo })
					}}</label>
				</div>
			</div>
			<div class="d-flex justify-content-end align-items-center gap-2 mt-4">
				<button type="button" class="btn btn-link text-muted" @click="restart">
					{{ $t("loadManagement.cancel") }}
				</button>
				<button
					type="button"
					class="btn btn-primary"
					:disabled="busy || (!settings && !learned) || (!!lineMove && !lineOk)"
					data-testid="load-import-run"
					@click="run"
				>
					{{
						$t("loadManagement.import.apply", {
							what: amount(diff.length, learnedPairs),
						})
					}}
				</button>
			</div>
		</template>

		<template v-else>
			<div class="done" role="status" data-testid="load-import-done">
				<svg
					width="18"
					height="18"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2.6"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
				>
					<path d="m5 12 5 5 9-10" />
				</svg>
				<span class="flex-grow-1">{{ doneText }}</span>
				<button type="button" class="btn btn-link btn-sm p-0" @click="restart">
					{{ $t("loadManagement.import.again") }}
				</button>
			</div>
		</template>

		<p v-if="error" class="small text-danger mt-3 mb-0" role="alert">{{ error }}</p>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import type { LoadConfig, LoadLoadpoint } from "@/types/supercharge";

type Kind = "settings" | "options" | "learned" | "unknown";
type Step = "files" | "review" | "done";

// the add-on's installation keys and where they live in evcc
const TOP: Record<string, string> = {
	icp_q: "q",
	icp_k: "k",
	contracted_kva: "contractKva",
	meter_url: "meterUri",
};
const RUNTIME = ["supercharge_scope", "burst_until_by_key", "amp_max_by_key"];
const camel = (k: string) => k.replace(/_([a-z])/g, (_, c: string) => c.toUpperCase());
const UNITS: Record<string, string> = {
	contractKva: "kVA",
	failsafeA: "A",
	burstKva: "kVA",
	bumpKva: "kVA",
	baseMarginKva: "kVA",
	marginS: "s",
	resetS: "s",
	floorDwellS: "s",
	restartDwellS: "s",
	baseTiAbortS: "s",
	baseTiStopS: "s",
	ampMin: "A",
	ampMax: "A",
	blindHoldA: "A",
	trimMaxA: "A",
	maxTempC: "°C",
	maxCloseness: "%",
	exitLeadFrac: "%",
	baseAbortCloseness: "%",
	baseStopCloseness: "%",
};

const same = (a: unknown, b: unknown) =>
	typeof a === "number" && typeof b === "number"
		? Math.abs(a - b) < 1e-9
		: String(a).replace(/\s/g, "") === String(b).replace(/\s/g, "");

// Reads the Supercharging add-on's own files and hands them to evcc after showing what changes.
// The add-on named loadpoints lp1, lp2 …; each is matched to an evcc loadpoint first.
export default defineComponent({
	name: "LoadImport",
	mixins: [formatter],
	props: {
		loadpoints: { type: Array as PropType<LoadLoadpoint[]>, default: () => [] },
		importedFrom: { type: String, default: "" },
		config: { type: Object as PropType<LoadConfig>, required: true },
	},
	data() {
		return {
			step: "files" as Step,
			found: [] as { name: string; kind: Kind }[],
			settings: null as Record<string, unknown> | null,
			learned: null as Record<string, unknown> | null,
			names: {} as Record<string, string>,
			withRuntime: false,
			busy: false,
			error: "",
			doneText: "",
			// a line move is applied only once it is ticked, as the settings page confirms it
			lineOk: false,
		};
	},
	computed: {
		steps(): Step[] {
			// "done" is the result, not a step to count
			return ["files", "review"];
		},
		unknownOnly(): boolean {
			return this.found.length > 0 && this.found.every((f) => f.kind === "unknown");
		},
		ignored(): string[] {
			return this.found.filter((f) => f.kind === "unknown").map((f) => f.name);
		},
		foundNames(): string {
			return this.found
				.filter((f) => f.kind !== "unknown")
				.map((f) => f.name)
				.join(", ");
		},
		keys(): string[] {
			const res = new Set<string>();
			const s = this.settings || {};
			const text = (v: unknown) =>
				v && typeof v === "object"
					? Object.entries(v as Record<string, unknown>)
							.map(([k, x]) => `${k}:${x}`)
							.join(",")
					: String(v || "");
			const list = (v: unknown, pairs: boolean) =>
				text(v)
					.split(/[,;]/)
					.map((p) => (pairs ? p.split(":")[0]! : p).trim())
					.filter((k) => k && k !== "all")
					.forEach((k) => res.add(k));
			list(s["supercharge_scope"], false);
			list(s["burst_until_by_key"], true);
			list(s["amp_max_by_key"], true);
			const learnedKeys = (this.learned?.["keys"] || {}) as Record<string, unknown>;
			Object.keys(learnedKeys).forEach((k) => res.add(k.split(":")[0]!));
			return [...res].sort();
		},
		current(): Record<string, unknown> {
			return {
				...(this.config.settings as unknown as Record<string, unknown>),
				...this.config,
			};
		},
		comparable(): { key: string; from: unknown; to: unknown }[] {
			const s = this.settings || {};
			return Object.entries(s)
				.filter(([k]) => !RUNTIME.includes(k))
				.map(([k, v]) => ({ key: TOP[k] || camel(k), to: v }))
				.filter((e) => e.key in this.current)
				.map((e) => ({ ...e, from: this.current[e.key] }));
		},
		diff(): { key: string; label: string; from: string; to: string }[] {
			return this.comparable
				.filter((e) => !same(e.from, e.to))
				.map((e) => ({
					key: e.key,
					label: this.labelOf(e.key),
					from: this.fmtValue(e.from, e.key),
					to: this.fmtValue(e.to, e.key),
				}));
		},
		lineAfter(): number {
			const c = this.config;
			const next = (key: string, now: number) => {
				const raw = this.comparable.find((x) => x.key === key)?.to;
				return typeof raw === "number" ? raw : now;
			};
			return next("contractKva", c.contractKva) * next("k", c.k);
		},
		lineTo(): string {
			return this.fmtNumber(this.lineAfter, 2);
		},
		// the import may move the never-trip line: say so, as the settings page does
		lineMove(): string {
			const c = this.config;
			const before = c.contractKva * c.k;
			const after = this.lineAfter;
			if (Math.abs(after - before) < 0.005) return "";
			return this.$t("loadManagement.import.lineMove", {
				from: this.fmtNumber(before, 2),
				to: this.fmtNumber(after, 2),
			});
		},
		sameCount(): number {
			return this.comparable.length - this.diff.length;
		},
		learnedPairs(): number {
			const learnedKeys = (this.learned?.["keys"] || {}) as Record<string, unknown>;
			return Object.keys(learnedKeys).filter((k) => this.names[k.split(":")[0]!]).length;
		},
		runtimeNames(): string[] {
			const scope = String(this.settings?.["supercharge_scope"] || "");
			const keys = scope
				.split(",")
				.map((k) => k.trim())
				.filter(Boolean);
			const mapped = keys.includes("all") ? Object.keys(this.names) : keys;
			return mapped
				.map((k) => this.loadpoints.find((lp) => lp.name === this.names[k])?.title)
				.filter((t): t is string => !!t);
		},
	},
	watch: {
		keys(keys: string[]) {
			const names: Record<string, string> = {};
			keys.forEach((k) => {
				const n = /^lp(\d+)$/.exec(k);
				// "lp2" is the second loadpoint; a key named like a loadpoint is that one
				const lp = n
					? this.loadpoints[Number(n[1]) - 1]
					: this.loadpoints.find(
							(l) =>
								l.name.toLowerCase() === k.toLowerCase() ||
								l.title.toLowerCase() === k.toLowerCase()
						);
				names[k] = this.names[k] ?? lp?.name ?? "";
			});
			this.names = names;
		},
	},
	methods: {
		// "3 changes and 1 learned pair", with each part in its own plural
		amount(n: number, m: number): string {
			const parts = [this.$t("loadManagement.import.nChanges", { n }, n)];
			if (m) parts.push(this.$t("loadManagement.import.nPairs", { n: m }, m));
			return parts.join(` ${this.$t("loadManagement.import.and")} `);
		},
		// the add-on called its loadpoints lp1, lp2; say that in words
		keyLabel(k: string): string {
			const m = /^lp(\d+)$/.exec(k);
			if (m) return this.$t("loadManagement.import.addonLoadpoint", { n: m[1] });
			return this.$t("loadManagement.import.addonNamed", { key: k });
		},
		labelOf(key: string): string {
			const field = `loadManagement.settings.fields.${key}.label`;
			if (this.$te(field)) return this.$t(field);
			if (key === "meterUri") return this.$t("loadManagement.settings.meterUri");
			return key;
		},
		// with the unit the settings page shows, and pacing tables as "1 A 30 s · 2 A 10 s"
		fmtValue(v: unknown, key: string): string {
			if (typeof v === "string" && /^\s*[\d.]+\s*:/.test(v)) {
				return v
					.split(",")
					.map((p) => p.split(":").map((x) => x.trim()))
					.map(([a, t]) => `${a} A ${t} s`)
					.join(" · ");
			}
			if (typeof v !== "number") return String(v ?? "");
			const unit = UNITS[key];
			if (unit === "%") return `${Number((v * 100).toFixed(1))} %`;
			const n = String(Number(v.toFixed(3)));
			return unit ? `${n} ${unit}` : n;
		},
		kindOf(data: unknown): Kind {
			if (!data || typeof data !== "object" || Array.isArray(data)) return "unknown";
			const d = data as Record<string, unknown>;
			if ("schema" in d && "keys" in d) return "learned";
			if ("meter_url" in d || "icp_q" in d || "contracted_kva" in d) return "options";
			if ("burst_kva" in d || "raise_table" in d) return "settings";
			return "unknown";
		},
		async read(e: Event) {
			this.error = "";
			this.found = [];
			this.learned = null;
			const settings: Record<string, unknown> = {};
			let anySettings = false;
			const files = [...((e.target as HTMLInputElement).files || [])];
			for (const file of files) {
				let kind: Kind = "unknown";
				try {
					const data = JSON.parse(await file.text());
					kind = this.kindOf(data);
					if (kind === "learned") this.learned = data;
					// options.json and settings.json both hold settings, the later file wins per key
					if (kind === "settings" || kind === "options") {
						Object.assign(settings, data);
						anySettings = true;
					}
				} catch {
					kind = "unknown";
				}
				this.found.push({ name: file.name, kind });
			}
			this.settings = anySettings ? settings : null;
			this.withRuntime = false;
			if (this.settings || this.learned) this.step = "review";
		},
		restart() {
			this.step = "files";
			this.lineOk = false;
			this.found = [];
			this.settings = null;
			this.learned = null;
			this.error = "";
		},
		async run() {
			this.busy = true;
			this.error = "";
			const names = Object.fromEntries(Object.entries(this.names).filter(([, v]) => v));
			let settings = this.settings;
			if (settings && !this.withRuntime) {
				settings = Object.fromEntries(
					Object.entries(settings).filter(
						([k]) => k !== "supercharge_scope" && k !== "burst_until_by_key"
					)
				);
			}
			const changes = this.diff.length;
			try {
				const res = await api.post("supercharging/import", {
					settings: settings ?? undefined,
					learned: this.learned ?? undefined,
					names,
				});
				this.doneText = this.$t("loadManagement.import.done", {
					what: this.amount(changes, res.data?.learnedKeys ?? 0),
				});
				this.step = "done";
			} catch (e: any) {
				this.error = e?.response?.data?.error || String(e);
			} finally {
				this.busy = false;
			}
		},
	},
});
</script>

<style scoped>
.step-line {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin: 0 0 1rem;
}
.pick-files {
}
.review {
	display: grid;
	grid-template-columns: minmax(0, 1fr);
	gap: 1.5rem;
}
@media (max-width: 767px) {
	.review {
		grid-template-columns: minmax(0, 1fr);
		gap: 1.25rem;
	}
}
.map-row {
	display: grid;
	grid-template-columns: 11rem minmax(0, 1fr);
	align-items: center;
	gap: 0.75rem;
	margin-bottom: 0.5rem;
}
.map-key {
	font-weight: 700;
	margin: 0;
}
.done {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	padding: 0.75rem 1rem;
	border-radius: 1rem;
	color: var(--bs-primary);
	background: color-mix(in srgb, var(--bs-primary) 10%, transparent);
}
.done span {
	color: var(--evcc-default-text);
}
@media (max-width: 575px) {
	.map-row {
		grid-template-columns: minmax(0, 1fr);
		gap: 0.25rem;
	}
}
.line-move {
	padding: 0.75rem 1rem;
	border-radius: 1rem;
	background: color-mix(in srgb, var(--evcc-orange) 12%, transparent);
	font-size: 0.875rem;
}
.map-row .form-select {
	max-width: 20rem;
}
.diff {
	margin: 0;
}
/* the same grey as every other help line in Settings */
.form-text {
	color: var(--evcc-gray);
}
.diff-row {
	padding: 0.4rem 0;
	border-bottom: 1px solid var(--evcc-gray-15);
}
.diff-row dt {
	font-weight: normal;
	color: var(--evcc-default-text);
}
.diff-row strong {
	color: var(--evcc-default-text);
}
.diff-row dd {
	margin: 0;
	overflow-wrap: anywhere;
}
</style>
