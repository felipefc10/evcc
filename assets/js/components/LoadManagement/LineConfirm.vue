<template>
	<div class="line-confirm" role="alert">
		<span class="confirm-text">{{ text }}</span>
		<span class="confirm-actions">
			<slot></slot>
		</span>
	</div>
</template>

<script lang="ts">
import { defineComponent } from "vue";

// A question asked in place, under the row it is about, with its answers as buttons.
export default defineComponent({
	name: "LineConfirm",
	props: {
		text: { type: String, required: true },
	},
});
</script>

<style scoped>
.line-confirm {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 0.5rem;
	padding: 0.75rem 1rem;
	border-radius: 1rem;
	background: color-mix(in srgb, var(--evcc-orange) 12%, transparent);
	font-size: 0.875rem;
}
.confirm-text {
	flex: 1 1 100%;
}
.confirm-actions {
	display: flex;
	justify-content: flex-end;
	align-items: center;
	gap: 0.5rem;
	margin-left: auto;
}
</style>
