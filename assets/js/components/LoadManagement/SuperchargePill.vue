<template>
	<span class="sc-wrap" :class="{ 'sc-wrap--stacked': stacked }">
		<button
			type="button"
			class="btn btn-pill sc-pill"
			:class="{ 'sc-pill--on': active && !paused, 'sc-pill--paused': active && paused }"
			:aria-pressed="active"
			:aria-label="aria"
			data-testid="supercharge-pill"
			@click.stop="$emit('open', { index, title, active, until })"
		>
			<svg
				width="13"
				height="13"
				viewBox="0 0 24 24"
				:fill="active && !paused ? 'currentColor' : 'none'"
				stroke="currentColor"
				stroke-width="2.2"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
				class="bolt"
			>
				<path d="M13 2 4 14h7l-1 8 9-12h-7z" />
			</svg>
			<span class="sc-label">{{ stacked ? shortLabel : fullLabel }}</span>
		</button>
		<span
			v-if="stacked && (reserve || line)"
			class="sc-until"
			data-testid="supercharge-until"
			:title="line"
			>{{ line || " " }}</span
		>
	</span>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import { dayOffset, fmtDayShort } from "./format";

// Supercharge on a loadpoint: off it reads "Supercharge", on it says until when, paused it
// says so. Either way a press opens the dialog that sets or ends it. Narrow columns use
// `stacked`: the word in the pill, the stop time on a line of its own below.
export default defineComponent({
	name: "SuperchargePill",
	mixins: [formatter],
	props: {
		index: { type: Number, required: true },
		title: { type: String, default: "" },
		active: Boolean,
		paused: Boolean,
		until: { type: String as PropType<string | null>, default: null },
		stacked: Boolean,
		// shown under a stacked pill when supercharge is off, e.g. why the car waits
		note: { type: String, default: "" },
		// keep the line's height even when empty, so neighbouring cards line up
		reserve: Boolean,
	},
	emits: ["open"],
	computed: {
		// the same words as the dialog: "22:00", "tomorrow 08:00", "Sat 3 Oct 08:00"
		when(): string {
			if (!this.until) return "";
			const d = new Date(this.until);
			const days = dayOffset(d, new Date());
			const time = this.fmtHourMinute(d);
			if (days <= 0) return time;
			if (days === 1) return `${this.$t("loadManagement.supercharge.tomorrowLower")} ${time}`;
			return `${fmtDayShort(d, this.$i18n.locale)} ${time}`;
		},
		fullLabel(): string {
			if (!this.active) return this.$t("loadManagement.supercharge.label");
			if (this.paused) return this.$t("loadManagement.supercharge.paused");
			if (!this.until) return this.$t("loadManagement.supercharge.pillNoEnd");
			return this.$t("loadManagement.supercharge.pillUntil", { time: this.when });
		},
		shortLabel(): string {
			if (!this.active) return this.$t("loadManagement.supercharge.label");
			if (this.paused) return this.$t("loadManagement.supercharge.paused");
			return this.$t("loadManagement.supercharge.active");
		},
		untilLine(): string {
			if (!this.active) return "";
			if (!this.until) return this.$t("loadManagement.supercharge.noEndLower");
			return this.$t("loadManagement.supercharge.until", { time: this.when });
		},
		line(): string {
			if (this.active && this.paused && this.note) return this.note;
			return this.untilLine || this.note;
		},
		aria(): string {
			return this.active
				? `${this.title}: ${this.fullLabel}. ${this.$t("loadManagement.supercharge.changeTime")}`
				: this.$t("loadManagement.supercharge.switchAria", { name: this.title });
		},
	},
});
</script>

<style scoped>
.sc-wrap {
	display: inline-flex;
	max-width: 100%;
}
.sc-wrap--stacked {
	flex-direction: column;
	align-items: center;
	gap: 0.2rem;
}
.sc-pill {
	display: inline-flex;
	align-items: center;
	gap: 0.4rem;
	min-height: 2.25rem;
	max-width: 100%;
	padding: 0 0.9rem;
	flex-shrink: 0;
	white-space: nowrap;
}
.sc-pill:focus:not(:focus-visible) {
	box-shadow: none;
}
.sc-until {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	max-width: 100%;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}
.bolt {
	flex-shrink: 0;
	color: var(--evcc-gray);
}
.sc-pill--on {
	--bs-btn-border-color: var(--evcc-dark-yellow);
	--bs-btn-hover-border-color: var(--evcc-dark-yellow);
	--bs-btn-bg: color-mix(in srgb, var(--evcc-dark-yellow) 16%, transparent);
	--bs-btn-hover-bg: color-mix(in srgb, var(--evcc-dark-yellow) 24%, transparent);
}
.sc-pill--on .bolt {
	color: var(--evcc-dark-yellow);
}
html.dark .sc-pill--on {
	--bs-btn-border-color: var(--evcc-yellow);
	--bs-btn-hover-border-color: var(--evcc-yellow);
	--bs-btn-bg: color-mix(in srgb, var(--evcc-yellow) 14%, transparent);
	--bs-btn-hover-bg: color-mix(in srgb, var(--evcc-yellow) 22%, transparent);
}
html.dark .sc-pill--on .bolt {
	color: var(--evcc-yellow);
}
.sc-pill--paused {
	--bs-btn-border-color: var(--evcc-orange);
	--bs-btn-hover-border-color: var(--evcc-orange);
}
.sc-pill--paused .bolt {
	color: var(--evcc-orange);
}
</style>
