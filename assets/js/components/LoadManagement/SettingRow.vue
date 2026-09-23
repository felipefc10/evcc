<template>
	<div class="setting-row">
		<div class="setting-text">
			<label :for="id" class="setting-label">{{ label }}</label>
			<div v-if="help" class="setting-help">{{ help }}</div>
		</div>
		<div class="setting-control" :class="{ 'setting-control--wide': wide }">
			<slot></slot>
			<div v-if="feedback" class="setting-feedback" :class="feedback.ok ? 'ok' : 'bad'" role="status">
				<svg v-if="feedback.ok" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m5 12 5 5 9-10" /></svg>
				{{ feedback.text }}
			</div>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";

// Label and help on the left, the control and what the server made of it on the right.
export default defineComponent({
	name: "SettingRow",
	props: {
		id: { type: String, required: true },
		label: { type: String, required: true },
		help: { type: String, default: "" },
		feedback: { type: Object as PropType<{ ok: boolean; text: string } | undefined> },
		wide: Boolean,
	},
});
</script>

<style scoped>
.setting-row {
	display: flex;
	align-items: center;
	gap: 1.5rem;
	padding: 1.1rem 0;
	border-top: 1px solid var(--evcc-gray-25);
}
.setting-text {
	flex: 1 1 auto;
	min-width: 0;
}
.setting-label {
	font-weight: 700;
	margin: 0;
}
.setting-help {
	font-size: 0.85rem;
	color: var(--evcc-gray);
	margin-top: 0.2rem;
}
.setting-control {
	flex: 0 0 auto;
	width: 13rem;
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	gap: 0.35rem;
}
.setting-control--wide {
	width: 20rem;
}
.setting-control > :deep(.form-control),
.setting-control > :deep(.form-select) {
	width: 100%;
}
.setting-control > :deep(.input-group) {
	width: 100%;
	flex-wrap: nowrap;
}
.setting-feedback {
	font-size: 0.75rem;
	font-weight: 600;
	display: inline-flex;
	align-items: center;
	gap: 0.25rem;
}
.setting-feedback.ok {
	color: var(--bs-primary);
}
.setting-feedback.bad {
	color: var(--bs-danger);
}
@media (max-width: 575px) {
	.setting-row {
		flex-direction: column;
		align-items: stretch;
		gap: 0.6rem;
	}
	.setting-control {
		width: 100%;
		align-items: stretch;
	}
}
</style>
