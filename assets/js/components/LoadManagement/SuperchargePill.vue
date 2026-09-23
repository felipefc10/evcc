<template>
	<button
		type="button"
		class="sc-pill"
		:class="{ 'sc-pill--on': active }"
		:aria-pressed="active"
		:title="active ? $t('loadManagement.supercharge.changeTime') : ''"
		data-testid="supercharge-pill"
		@click.stop="$emit('open', { index, title, active, until })"
	>
		<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M13 2 4 14h7l-1 8 9-12h-7z" /></svg>
		<span class="text-truncate">{{ label }}</span>
	</button>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";

// Supercharge on a loadpoint card: off it reads "Supercharge", on it reads when it ends.
// Either way a press opens the sheet that sets or ends it.
export default defineComponent({
	name: "SuperchargePill",
	mixins: [formatter],
	props: {
		index: { type: Number, required: true },
		title: { type: String, default: "" },
		active: Boolean,
		until: { type: String as PropType<string | null>, default: null },
	},
	emits: ["open"],
	computed: {
		label(): string {
			if (!this.active) return this.$t("loadManagement.supercharge.label");
			if (!this.until) return this.$t("loadManagement.supercharge.indefinitely");
			return this.$t("loadManagement.supercharge.untilMorning", {
				time: this.fmtAbsoluteDate(new Date(this.until)),
			});
		},
	},
});
</script>

<style scoped>
.sc-pill {
	display: inline-flex;
	align-items: center;
	gap: 0.4rem;
	min-height: 2.25rem;
	max-width: 100%;
	padding: 0 0.85rem;
	border-radius: 999px;
	font-size: 0.8rem;
	font-weight: 700;
	white-space: nowrap;
	background: transparent;
	color: var(--evcc-gray);
	border: 1px solid var(--evcc-gray-25);
	flex-shrink: 0;
}
.sc-pill:hover {
	color: var(--evcc-default-text);
}
.sc-pill--on {
	color: var(--evcc-dark-yellow);
	border-color: color-mix(in srgb, var(--evcc-dark-yellow) 50%, transparent);
	background: color-mix(in srgb, var(--evcc-dark-yellow) 14%, transparent);
}
html.dark .sc-pill--on {
	color: var(--evcc-yellow);
	border-color: color-mix(in srgb, var(--evcc-yellow) 50%, transparent);
	background: color-mix(in srgb, var(--evcc-yellow) 14%, transparent);
}
.sc-pill:focus-visible {
	outline: var(--bs-focus-ring-width) solid var(--bs-focus-ring-color);
}
</style>
