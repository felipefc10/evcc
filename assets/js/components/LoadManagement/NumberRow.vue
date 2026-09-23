<template>
	<SettingsFormRow
		:id="id"
		:label="$t(`loadManagement.settings.fields.${field.key}.label`)"
		:description="$t(`loadManagement.settings.fields.${field.key}.help`)"
	>
		<div class="input-group">
			<input
				:id="id"
				ref="input"
				class="form-control text-end"
				type="number"
				inputmode="decimal"
				:min="field.min"
				:max="field.max"
				:step="field.step"
				:value="shown"
				@change="changed"
				@keydown.enter.prevent="($event.target as HTMLInputElement).blur()"
			/>
			<span v-if="field.unit" class="input-group-text unit">{{ field.unit }}</span>
		</div>
		<div v-if="feedback" class="small mt-1" :class="feedback.ok ? 'text-primary' : 'text-danger'">
			{{ feedback.text }}
		</div>
	</SettingsFormRow>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import SettingsFormRow from "../Helper/SettingsFormRow.vue";

export interface NumberField {
	key: string;
	unit: string;
	min?: number;
	max?: number;
	step?: number;
	digits?: number;
}

// One numeric setting: label, box with its unit, and what the server made of it.
export default defineComponent({
	name: "NumberRow",
	components: { SettingsFormRow },
	props: {
		field: { type: Object as PropType<NumberField>, required: true },
		value: { type: Number, default: 0 },
		feedback: { type: Object as PropType<{ ok: boolean; text: string } | undefined> },
	},
	emits: ["change"],
	computed: {
		id(): string {
			return `lm-${this.field.key}`;
		},
		shown(): string {
			const d = this.field.digits ?? 2;
			return Number.isFinite(this.value) ? String(Number(this.value.toFixed(d))) : "";
		},
	},
	watch: {
		// the server's answer is what stands, even when it equals the old value
		feedback() {
			const el = this.$refs["input"] as HTMLInputElement | undefined;
			if (el && document.activeElement !== el) el.value = this.shown;
		},
	},
	methods: {
		changed(e: Event) {
			const el = e.target as HTMLInputElement;
			const v = parseFloat(el.value.replace(",", "."));
			if (!Number.isFinite(v)) {
				el.value = this.shown;
				return;
			}
			this.$emit("change", v);
		},
	},
});
</script>

<style scoped>
.unit {
	min-width: 3.5rem;
	justify-content: center;
}
</style>
