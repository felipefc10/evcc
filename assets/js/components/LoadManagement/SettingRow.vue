<template>
	<div class="setting-row" :class="{ 'setting-row--inline': inline }">
		<div class="setting-text">
			<label :for="id" class="setting-label">{{ label }}</label>
			<div v-if="help" class="setting-help">{{ help }}</div>
		</div>
		<div class="setting-control" :class="{ 'setting-control--wide': wide }">
			<slot></slot>
			<div class="setting-meta" :class="{ 'setting-meta--flow': feedback && !feedback.ok }">
				<div
					v-if="feedback"
					class="setting-feedback"
					:class="feedback.ok ? 'ok' : 'bad'"
					role="status"
				>
					<svg
						v-if="feedback.ok"
						width="12"
						height="12"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="3"
						stroke-linecap="round"
						stroke-linejoin="round"
						aria-hidden="true"
					>
						<path d="m5 12 5 5 9-10" />
					</svg>
					{{ feedback.text }}
				</div>
				<slot name="meta"></slot>
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
		// a switch: label and control side by side even on phones
		inline: Boolean,
	},
});
</script>

<style scoped>
.setting-row {
	display: flex;
	align-items: flex-start;
	gap: 1.5rem;
	padding: 1.1rem 0 1.6rem;
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
	position: relative;
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
/* short notes (Saved, Reset) sit in the row's bottom padding instead of reserving a line;
   an error can run long, so it takes its place in the flow */
.setting-meta {
	position: absolute;
	top: 100%;
	left: 0;
	right: 0;
	margin-top: 0.2rem;
	display: flex;
	flex-wrap: wrap;
	justify-content: flex-end;
	align-items: center;
	gap: 0.25rem 0.75rem;
	line-height: 1.2;
}
.setting-meta--flow {
	position: static;
	margin-top: 0;
}
.setting-feedback {
	font-size: 0.8125rem;
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
/* the undo and reset links the rows put in the meta slot */
:slotted(.reset) {
	font-size: 0.75rem;
	color: var(--evcc-default-text);
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
	.setting-row--inline {
		flex-direction: row;
		align-items: flex-start;
	}
	.setting-row--inline .setting-control {
		width: auto;
		align-items: flex-end;
	}
}
</style>
