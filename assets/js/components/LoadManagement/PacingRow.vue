<template>
	<SettingRow
		:id="`lm-${field}-0-a`"
		wide
		:label="$t(`loadManagement.settings.fields.${field}.label`)"
		:help="$t(`loadManagement.settings.fields.${field}.help`)"
		:feedback="rowError ? { ok: false, text: rowError } : feedback"
	>
		<table class="pacing" :data-testid="`load-pacing-${field}`">
			<thead>
				<tr>
					<th scope="col">{{ $t("loadManagement.settings.pacing.amps") }}</th>
					<th scope="col">{{ $t("loadManagement.settings.pacing.seconds") }}</th>
					<th scope="col">
						<span class="visually-hidden">{{
							$t("loadManagement.settings.pacing.remove")
						}}</span>
					</th>
				</tr>
			</thead>
			<tbody>
				<tr v-for="(r, i) in rows" :key="i">
					<td>
						<div class="input-group">
							<input
								:id="`lm-${field}-${i}-a`"
								v-model.number="r.a"
								class="form-control text-end"
								type="number"
								min="0.5"
								step="0.5"
								inputmode="decimal"
								:aria-label="
									$t('loadManagement.settings.pacing.ampsRow', { n: i + 1 })
								"
								@change="emit"
								@wheel="($event.target as HTMLInputElement).blur()"
							/>
							<span class="input-group-text">A</span>
						</div>
					</td>
					<td>
						<div class="input-group">
							<input
								v-model.number="r.s"
								class="form-control text-end"
								type="number"
								min="0"
								step="0.1"
								inputmode="decimal"
								:aria-label="
									$t('loadManagement.settings.pacing.secondsRow', { n: i + 1 })
								"
								@change="emit"
								@wheel="($event.target as HTMLInputElement).blur()"
							/>
							<span class="input-group-text">s</span>
						</div>
					</td>
					<td>
						<button
							type="button"
							class="btn btn-link text-muted remove"
							:disabled="rows.length <= 1"
							:aria-label="
								$t('loadManagement.settings.pacing.removeRow', { n: i + 1 })
							"
							@click="remove(i)"
						>
							<svg
								width="14"
								height="14"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2.4"
								stroke-linecap="round"
								aria-hidden="true"
							>
								<path d="M6 6l12 12M18 6 6 18" />
							</svg>
						</button>
					</td>
				</tr>
			</tbody>
		</table>
		<p v-if="hasDraft" class="draft-note">
			{{ $t("loadManagement.settings.pacing.draft") }}
		</p>
		<button type="button" class="btn btn-link btn-sm p-0 add" :disabled="hasDraft" @click="add">
			{{ $t("loadManagement.settings.pacing.add") }}
		</button>
		<template #meta>
			<button v-if="before" type="button" class="btn btn-link btn-sm p-0 reset" @click="undo">
				{{ $t("loadManagement.settings.undoWas", { value: undoText }) }}
			</button>
			<button
				v-else-if="!isDefault"
				type="button"
				class="btn btn-link btn-sm p-0 reset"
				:title="DEFAULT_TEXT"
				@click="resetTable"
			>
				{{ $t("loadManagement.settings.pacing.resetDefault") }}
			</button>
		</template>
	</SettingRow>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import SettingRow from "./SettingRow.vue";

// the server's default cadence, see CadenceDefault in core/supercharge/settings.go
const DEFAULT = "1:10, 2:5, 3:3.3, 5:2, 10:1, 20:0.5";
// "1 A 10 s · 2 A 5 s", as the import review writes a table
const DEFAULT_TEXT = DEFAULT.split(",")
	.map((p) => p.split(":").map((x) => x.trim()))
	.map(([a, t]) => `${a}\u00a0A ${t}\u00a0s`)
	.join(" · ");

interface Row {
	a: number;
	s: number;
}

const parse = (text: string): Row[] =>
	text
		.split(",")
		.map((p) => p.split(":").map((x) => parseFloat(x.trim())))
		.filter((p) => p.length === 2 && p.every((x) => Number.isFinite(x)))
		.map(([a, s]) => ({ a: a!, s: s! }));

// A cadence table as rows of "at least this many amps waits this many seconds".
// The server keeps the "1:10, 2:5" text form; this only edits it row by row.
export default defineComponent({
	name: "PacingRow",
	components: { SettingRow },
	props: {
		field: { type: String, required: true },
		value: { type: String, default: "" },
		feedback: { type: Object as PropType<{ ok: boolean; text: string } | undefined> },
	},
	emits: ["change"],
	data() {
		// the table a reset replaced, so one press brings it back
		return {
			DEFAULT,
			DEFAULT_TEXT,
			rows: parse(this.value),
			before: "",
			undoText: "",
			rowError: "",
			timer: undefined as ReturnType<typeof setTimeout> | undefined,
		};
	},
	watch: {
		// a save elsewhere in the table keeps an unfinished draft row
		value(v: string) {
			const drafts = this.rows.filter((r) => !Number.isFinite(r.a) || !Number.isFinite(r.s));
			this.rows = [...parse(v), ...drafts];
		},
	},
	computed: {
		hasDraft(): boolean {
			return this.rows.some((r) => !Number.isFinite(r.a) || !Number.isFinite(r.s));
		},
		isDefault(): boolean {
			return this.value.replace(/\s/g, "") === DEFAULT.replace(/\s/g, "");
		},
	},
	methods: {
		resetTable() {
			// a reset replaces the whole table, unfinished rows included
			this.rows = this.rows.filter((r) => Number.isFinite(r.a) && Number.isFinite(r.s));
			this.undoText = parse(this.value)
				.map((r) => `${r.a}\u00a0A ${r.s}\u00a0s`)
				.join(" · ");
			this.offerUndo();
			this.$emit("change", DEFAULT);
		},
		undo() {
			const v = this.before;
			this.before = "";
			this.$emit("change", v);
		},
		emit() {
			// a row with amps at or below zero, or negative seconds, is refused like any number
			const bad = this.rows.find(
				(r) => (Number.isFinite(r.a) && r.a <= 0) || (Number.isFinite(r.s) && r.s < 0)
			);
			this.rowError = bad ? this.$t("loadManagement.settings.pacing.invalid") : "";
			if (bad) return;
			const valid = this.rows.filter(
				(r) => Number.isFinite(r.a) && Number.isFinite(r.s) && r.a > 0
			);
			if (!valid.length) return;
			this.$emit("change", valid.map((r) => `${r.a}:${r.s}`).join(", "));
		},
		// an empty draft row; it is saved once both boxes hold a value
		add() {
			this.rows.push({
				a: undefined as unknown as number,
				s: undefined as unknown as number,
			});
			const n = this.rows.length - 1;
			this.$nextTick(() => document.getElementById(`lm-${this.field}-${n}-a`)?.focus());
		},
		remove(i: number) {
			const r = this.rows[i]!;
			this.rows.splice(i, 1);
			if (!Number.isFinite(r.a) || !Number.isFinite(r.s)) return;
			this.undoText = `${r.a}\u00a0A ${r.s}\u00a0s`;
			this.offerUndo();
			this.emit();
		},
		offerUndo() {
			this.before = this.value;
			clearTimeout(this.timer);
			this.timer = setTimeout(() => (this.before = ""), 15000);
		},
	},
});
</script>

<style scoped>
.pacing {
	width: 100%;
	border-collapse: collapse;
}
.pacing th {
	font-size: 0.75rem;
	font-weight: 600;
	color: var(--evcc-gray);
	padding: 0 0.25rem 0.25rem;
	text-align: start;
}
.pacing td {
	padding: 0.2rem 0.25rem;
}
.draft-note {
	font-size: 0.75rem;
	color: var(--evcc-gray);
	margin: 0.25rem 0 0;
	text-align: end;
}
.add {
	font-size: 0.8rem;
	color: var(--evcc-default-text);
}
.remove {
	min-width: 2.25rem;
	min-height: 2.25rem;
	padding: 0;
}
.reset {
	font-size: 0.75rem;
	color: var(--evcc-default-text);
}
</style>
