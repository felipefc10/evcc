<template>
	<SettingRow
		:id="id"
		:label="$t(`loadManagement.settings.fields.${field.key}.label`)"
		:help="help"
		:feedback="rangeError ? { ok: false, text: rangeError } : feedback"
	>
		<div class="input-group" :class="{ 'no-unit': !field.unit }">
			<input
				:id="id"
				ref="input"
				class="form-control text-end"
				:class="{ 'input-bad': bad }"
				:aria-invalid="bad || undefined"
				type="number"
				inputmode="decimal"
				:min="shownMin"
				:max="shownMax"
				:step="shownStep"
				:value="shown"
				:disabled="disabled"
				@change="changed"
				@wheel="($event.target as HTMLInputElement).blur()"
				@input="clearError"
				@blur="restoreIfBad"
				@keydown.enter.prevent="($event.target as HTMLInputElement).blur()"
			/>
			<span v-if="field.unit" class="input-group-text unit">{{ field.unit }}</span>
		</div>
		<template #meta>
			<button
				v-if="before !== undefined && !canReset"
				type="button"
				class="btn btn-link btn-sm p-0 reset"
				@click="undoReset"
			>
				{{ $t("loadManagement.settings.undoWas", { value: `${fmt(before!)}${unitText}` }) }}
			</button>
			<button
				v-else-if="canReset && !disabled"
				type="button"
				class="btn btn-link btn-sm p-0 reset"
				@click="resetToDefault"
			>
				{{
					$t("loadManagement.settings.resetTo", {
						value: `${fmt(field.def!)}${unitText}`,
					})
				}}
			</button>
		</template>
	</SettingRow>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import SettingRow from "./SettingRow.vue";

export interface NumberField {
	key: string;
	unit: string;
	min?: number;
	max?: number;
	step?: number;
	digits?: number;
	// shown multiplied by this, e.g. 100 for a fraction shown in percent
	scale?: number;
	def?: number;
}

// One numeric setting: label, help with its range and default, box with its unit,
// and what the server made of it.
export default defineComponent({
	name: "NumberRow",
	components: { SettingRow },
	props: {
		field: { type: Object as PropType<NumberField>, required: true },
		value: { type: Number, default: 0 },
		feedback: { type: Object as PropType<{ ok: boolean; text: string } | undefined> },
		helpKey: { type: String, default: "" },
		disabled: Boolean,
	},
	emits: ["change"],
	data() {
		// before: the value a reset replaced, so the reset can be undone
		return {
			rangeError: "",
			bad: false,
			before: undefined as number | undefined,
			timers: {} as Record<string, ReturnType<typeof setTimeout>>,
		};
	},
	computed: {
		id(): string {
			return `lm-${this.field.key}`;
		},
		scale(): number {
			return this.field.scale || 1;
		},
		shownMin(): number | undefined {
			return this.field.min === undefined ? undefined : this.field.min * this.scale;
		},
		shownMax(): number | undefined {
			return this.field.max === undefined ? undefined : this.field.max * this.scale;
		},
		shownStep(): number | undefined {
			return this.field.step === undefined ? undefined : this.field.step * this.scale;
		},
		shown(): string {
			return Number.isFinite(this.value) ? this.fmt(this.value) : "";
		},
		canReset(): boolean {
			const d = this.field.def;
			return d !== undefined && Math.abs(d - this.value) > 1e-9;
		},
		unitText(): string {
			return this.field.unit && this.field.unit !== "×" ? `\u00a0${this.field.unit}` : "";
		},
		help(): string {
			const base = this.$t(
				this.helpKey || `loadManagement.settings.fields.${this.field.key}.help`
			);
			if (this.field.min === undefined || this.field.max === undefined) return base;
			const unit = this.unitText;
			const range = this.$t("loadManagement.settings.range", {
				min: this.fmt(this.field.min),
				max: this.fmt(this.field.max),
				unit,
			});
			return `${base} ${range}`;
		},
	},
	watch: {
		// the server's answer is what stands, even when it equals the old value
		feedback() {
			this.rangeError = "";
			const el = this.$refs["input"] as HTMLInputElement | undefined;
			if (el && document.activeElement !== el) el.value = this.shown;
		},
	},
	methods: {
		fmt(v: number): string {
			const d = Math.max(0, (this.field.digits ?? 2) - Math.round(Math.log10(this.scale)));
			return String(Number((v * this.scale).toFixed(d)));
		},
		changed(e: Event) {
			const el = e.target as HTMLInputElement;
			const v = parseFloat(el.value.replace(",", "."));
			this.rangeError = "";
			if (!Number.isFinite(v)) {
				el.value = this.shown;
				return;
			}
			const lo = this.shownMin ?? -Infinity;
			const hi = this.shownMax ?? Infinity;
			if (v < lo - 1e-9 || v > hi + 1e-9) {
				// safety values are never bent into range silently: the old value stays
				this.rangeError = this.$t("loadManagement.settings.outOfRange", {
					value: `${el.value}${this.unitText}`,
					min: this.fmt(this.field.min!),
					max: this.fmt(this.field.max!),
					unit: this.unitText,
					kept: `${this.shown}${this.unitText}`,
				});
				this.bad = true;
				this.expire("rangeError", 8000);
				return;
			}
			this.$emit("change", v / this.scale);
		},
		// lets a parent put the box back to the stored value, e.g. after an undo
		reset() {
			this.rangeError = "";
			const el = this.$refs["input"] as HTMLInputElement | undefined;
			if (el) el.value = this.shown;
		},
		// leaving a refused value behind puts the stored one back, with the message kept
		restoreIfBad(e: Event) {
			if (!this.bad) return;
			(e.target as HTMLInputElement).value = this.shown;
			this.bad = false;
		},
		clearError() {
			this.rangeError = "";
			this.bad = false;
			this.before = undefined;
		},
		resetToDefault() {
			this.before = this.value;
			this.$emit("change", this.field.def);
			this.expire("before", 15000);
		},
		// messages and undo offers do not outstay their moment
		expire(what: "rangeError" | "before", ms: number) {
			clearTimeout(this.timers[what]);
			this.timers[what] = setTimeout(() => {
				if (what === "rangeError") this.rangeError = "";
				else this.before = undefined;
			}, ms);
		},
		undoReset() {
			const v = this.before;
			this.before = undefined;
			if (v !== undefined) this.$emit("change", v);
		},
	},
});
</script>

<style scoped>
.unit {
	min-width: 3.5rem;
	justify-content: center;
}
.no-unit {
	padding-right: 3.5rem;
}
.input-bad {
	border-color: var(--bs-danger);
}
</style>
