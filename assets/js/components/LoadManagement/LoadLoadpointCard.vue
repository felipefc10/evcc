<template>
	<article
		class="lp-card"
		:class="{ 'lp-card--lifted': lifted }"
		data-testid="load-loadpoint"
		:data-name="lp.name"
	>
		<div class="head">
			<button
				v-if="movable"
				type="button"
				class="grip"
				:aria-label="$t('loadManagement.order.drag', { name: lp.title })"
				@pointerdown="$emit('grip', $event)"
				@keydown.up.prevent="$emit('move', -1)"
				@keydown.down.prevent="$emit('move', 1)"
			>
				<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
					<circle cx="9" cy="6" r="1.6" />
					<circle cx="15" cy="6" r="1.6" />
					<circle cx="9" cy="12" r="1.6" />
					<circle cx="15" cy="12" r="1.6" />
					<circle cx="9" cy="18" r="1.6" />
					<circle cx="15" cy="18" r="1.6" />
				</svg>
			</button>
			<span
				class="prio"
				:class="{ 'prio--first': first }"
				:title="$t('loadManagement.order.prioTitle', { n: priority })"
				data-testid="load-priority"
			>
				<small>{{ $t("loadManagement.order.prio") }}</small>
				<span>{{ priority }}</span>
			</span>
			<div class="who">
				<h3 class="title">
					<span class="swatch" :style="{ background: color }"></span>
					<span class="title-text">{{ lp.title }}</span>
				</h3>
				<div class="sub" :class="{ 'text-warning': warning }">{{ subline }}</div>
			</div>
			<div class="allowed">
				<div class="label">{{ $t("loadManagement.card.allowed") }}</div>
				<div class="amps">{{ allowed }}</div>
			</div>
		</div>

		<div class="facts">
			<div v-for="f in facts" :key="f.label" :title="f.title">
				<div class="label">{{ f.label }}</div>
				<div class="value text-truncate">{{ f.value }}</div>
			</div>
		</div>

		<div class="foot">
			<SuperchargeSwitch
				:index="lp.index"
				:title="lp.title"
				:active="lp.supercharge"
				:until="lp.superchargeUntil"
				@open="$emit('open-supercharge', $event)"
			/>
			<div v-if="movable" class="arrows">
				<button
					type="button"
					class="arrow"
					:disabled="!canUp"
					:aria-label="$t('loadManagement.order.up', { name: lp.title })"
					@click="$emit('move', -1)"
				>
					<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 15 6-6 6 6" /></svg>
				</button>
				<button
					type="button"
					class="arrow"
					:disabled="!canDown"
					:aria-label="$t('loadManagement.order.down', { name: lp.title })"
					@click="$emit('move', 1)"
				>
					<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6" /></svg>
				</button>
			</div>
		</div>
		<p v-if="lp.supercharge && !lp.fast" class="small text-muted mt-2 mb-0">
			{{ $t("loadManagement.card.slowNote") }}
		</p>
	</article>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import SuperchargeSwitch from "./SuperchargeSwitch.vue";
import { loadpointReason } from "./loadpointReason";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

// One loadpoint in the charging order: its priority, what it is allowed and why, and supercharge.
export default defineComponent({
	name: "LoadLoadpointCard",
	components: { SuperchargeSwitch },
	mixins: [formatter],
	props: {
		lp: { type: Object as PropType<LoadLoadpoint>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
		color: { type: String, default: "" },
		priority: { type: Number, required: true },
		place: { type: String, default: "" },
		first: Boolean,
		movable: Boolean,
		canUp: Boolean,
		canDown: Boolean,
		lifted: Boolean,
	},
	emits: ["open-supercharge", "move", "grip"],
	computed: {
		warning(): boolean {
			return (this.lp.paused && this.lp.measuredA > 0.5) || this.lp.stoodOffS > 0;
		},
		// charging as planned: say where it stands in the order; otherwise say why not
		normal(): boolean {
			const lp = this.lp;
			return this.state.running && lp.connected && lp.wants && !lp.paused && !this.warning;
		},
		subline(): string {
			const parts: string[] = [];
			if (this.lp.vehicle) parts.push(this.lp.vehicle);
			if (this.normal) {
				if (this.place) parts.push(this.place);
				const eta = this.lp.forecast?.etaAt;
				if (eta) {
					parts.push(
						this.$t("loadManagement.reason.done", {
							time: this.fmtAbsoluteDate(new Date(eta)),
							duration: this.fmtDurationLong(this.lp.forecast.etaS || 0, "short"),
						})
					);
				}
			} else {
				parts.push(
					loadpointReason(this.lp, this.state, {
						t: (k, v) => this.$t(k, v || {}),
						number: (n, d) => this.fmtNumber(n, d),
						duration: (s) =>
							s < 90 ? `${Math.round(s)} s` : this.fmtDurationLong(s, "short"),
						time: (iso) => this.fmtAbsoluteDate(new Date(iso)),
					})
				);
			}
			return parts.join(" · ");
		},
		allowed(): string {
			if (!this.state.running || !this.lp.wants) return "—";
			// the reason line already says why a paused car gets nothing
			return `${this.lp.paused ? 0 : this.lp.setpointA} A`;
		},
		facts() {
			const lp = this.lp;
			const res: { label: string; value: string; title?: string }[] = [
				{
					label: this.$t("loadManagement.card.mode"),
					value: this.$te(`main.mode.${lp.mode}`) ? this.$t(`main.mode.${lp.mode}`) : lp.mode,
				},
				{
					label: this.$t("loadManagement.card.measured"),
					value: `${this.fmtNumber(lp.measuredA, 1)} A`,
					title: this.$t(`loadManagement.card.src.${lp.measuredSrc}`),
				},
			];
			if (lp.soc > 0) {
				res.push({
					label: this.$t("loadManagement.card.charge"),
					value: lp.limitSoc
						? `${Math.round(lp.soc)} → ${lp.limitSoc} %`
						: `${Math.round(lp.soc)} %`,
				});
			}
			return res;
		},
	},
});
</script>

<style scoped>
.lp-card {
	background: var(--evcc-box);
	border-radius: 2rem;
	padding: 1.25rem 1.5rem;
	display: flex;
	flex-direction: column;
	gap: 1rem;
	border: 2px solid transparent;
	transition:
		box-shadow var(--evcc-transition-fast),
		border-color var(--evcc-transition-fast);
}
.lp-card--lifted {
	border-color: var(--evcc-dark-green);
	box-shadow: 0 12px 32px rgba(0, 0, 0, 0.35);
	position: relative;
	z-index: 2;
}
.head {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}
.grip {
	border: 0;
	background: none;
	color: var(--evcc-gray);
	padding: 0.5rem 0.25rem;
	margin-left: -0.5rem;
	cursor: grab;
	touch-action: none;
	display: flex;
	border-radius: 8px;
}
.grip:active {
	cursor: grabbing;
}
.grip:focus-visible {
	outline: var(--bs-focus-ring-width) solid var(--bs-focus-ring-color);
}
.prio {
	flex-shrink: 0;
	width: 2.75rem;
	height: 2.75rem;
	border-radius: 12px;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	line-height: 1.05;
	font-weight: 800;
	font-size: 1.05rem;
	font-variant-numeric: tabular-nums;
	background: var(--evcc-gray-15);
	color: var(--evcc-default-text);
}
.prio small {
	font-size: 0.55rem;
	font-weight: 700;
	letter-spacing: 0.06em;
	text-transform: uppercase;
	opacity: 0.75;
}
.prio--first {
	background: var(--evcc-dark-green);
	color: var(--bs-dark);
}
.who {
	flex-grow: 1;
	min-width: 0;
}
.title {
	margin: 0;
	font-size: 1.15rem;
	font-weight: 700;
	display: flex;
	align-items: center;
	gap: 0.5rem;
	min-width: 0;
}
.title-text {
	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
	overflow: hidden;
	overflow-wrap: anywhere;
}
.swatch {
	flex-shrink: 0;
	width: 0.65rem;
	height: 0.65rem;
	border-radius: 50%;
}
.sub {
	font-size: 0.85rem;
	color: var(--evcc-gray);
}
.allowed {
	text-align: end;
	flex-shrink: 0;
}
.label {
	font-size: 0.7rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: var(--evcc-gray);
}
.amps {
	font-size: 1.6rem;
	font-weight: 800;
	line-height: 1.1;
	font-variant-numeric: tabular-nums;
}
.facts {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 0.75rem;
}
.value {
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}
.foot {
	display: flex;
	align-items: center;
	justify-content: space-between;
	flex-wrap: wrap;
	gap: 0.75rem;
	padding-top: 0.85rem;
	border-top: 1px solid var(--evcc-gray-25);
}
.arrows {
	display: flex;
	gap: 0.5rem;
}
.arrow {
	width: 2.5rem;
	height: 2.5rem;
	border-radius: 10px;
	border: 1px solid var(--evcc-gray-25);
	background: transparent;
	color: var(--evcc-default-text);
	display: flex;
	align-items: center;
	justify-content: center;
}
.arrow:hover:not(:disabled) {
	background: var(--evcc-gray-15);
}
.arrow:disabled {
	opacity: 0.3;
}
@media (max-width: 575px) {
	.lp-card {
		padding: 1rem 1.1rem;
		border-radius: 1.5rem;
	}
	.head {
		gap: 0.5rem;
	}
	.title {
		font-size: 1.05rem;
	}
	.amps {
		font-size: 1.35rem;
	}
	.arrow {
		width: 2.75rem;
		height: 2.75rem;
	}
}
@media (prefers-reduced-motion: reduce) {
	.lp-card {
		transition: none;
	}
}
</style>
