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
				<svg
					width="18"
					height="18"
					viewBox="0 0 24 24"
					fill="currentColor"
					aria-hidden="true"
				>
					<circle cx="9" cy="6" r="1.6" />
					<circle cx="15" cy="6" r="1.6" />
					<circle cx="9" cy="12" r="1.6" />
					<circle cx="15" cy="12" r="1.6" />
					<circle cx="9" cy="18" r="1.6" />
					<circle cx="15" cy="18" r="1.6" />
				</svg>
			</button>
			<div class="who">
				<h3 class="title">
					<span class="swatch" :style="{ background: color }"></span>
					<span class="title-text">{{ lp.title }}</span>
				</h3>
				<div v-if="place" class="place-row">
					<span class="sub">{{ place }}</span>
					<button
						v-if="canFirst"
						type="button"
						class="btn btn-sm btn-pill first"
						:title="$t('loadManagement.order.chargeFirstHelp')"
						data-testid="load-charge-first"
						@click="$emit('charge-first')"
					>
						<svg
							width="11"
							height="11"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2.8"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"
						>
							<path d="m6 15 6-6 6 6" />
						</svg>
						{{ $t("loadManagement.order.chargeFirst") }}
					</button>
				</div>
				<div
					class="sub"
					:class="{ 'warn-text': warning, 'sub--strong': bursting }"
					data-testid="load-reason"
				>
					{{ subline }}
				</div>
			</div>
			<div class="allowed">
				<div class="label">{{ $t("loadManagement.card.allowed") }}</div>
				<div class="amps">{{ allowed }}</div>
			</div>
		</div>

		<div class="foot">
			<div class="foot-actions">
				<SuperchargePill
					v-if="state.enabled"
					:index="lp.index"
					:title="lp.title"
					:active="lp.supercharge"
					:until="lp.superchargeUntil"
					:paused="paused"
					:note="waitNote"
					:reserve="lp.supercharge || lp.paused"
					stacked
					@open="$emit('open-supercharge', $event)"
				/>
				<span v-else class="small text-muted">{{
					$t("loadManagement.supercharge.needsOn")
				}}</span>
			</div>
			<div v-if="movable" class="prio">
				<span class="stepper-label" aria-hidden="true">{{
					$t("loadManagement.order.priority")
				}}</span>
				<div
					class="stepper"
					role="group"
					:aria-label="$t('loadManagement.order.priorityOf', { name: lp.title })"
					data-testid="load-priority"
				>
					<button
						type="button"
						:disabled="priority <= 0"
						:aria-label="$t('loadManagement.order.lower', { name: lp.title })"
						data-testid="load-priority-down"
						@click="$emit('set-priority', priority - 1)"
					>
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2.6"
							stroke-linecap="round"
							aria-hidden="true"
						>
							<path d="M5 12h14" />
						</svg>
					</button>
					<span
						class="stepper-value"
						aria-live="polite"
						data-testid="load-priority-value"
						>{{ priority }}</span
					>
					<button
						type="button"
						:disabled="priority >= 10"
						:aria-label="$t('loadManagement.order.raise', { name: lp.title })"
						data-testid="load-priority-up"
						@click="$emit('set-priority', priority + 1)"
					>
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2.6"
							stroke-linecap="round"
							aria-hidden="true"
						>
							<path d="M12 5v14M5 12h14" />
						</svg>
					</button>
				</div>
			</div>
		</div>
		<p v-if="lp.supercharge && !lp.fast" class="small text-muted mb-0">
			{{ $t("loadManagement.card.slowNote") }}
		</p>
	</article>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import SuperchargePill from "./SuperchargePill.vue";
import { loadpointReason, waitReason } from "./loadpointReason";
import { superchargeBursting, superchargePaused } from "./state";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

// One loadpoint in the charging order: where it stands, what it is allowed and why,
// its supercharge and its priority. The priority is evcc's own number, 0 to 10.
export default defineComponent({
	name: "LoadLoadpointCard",
	components: { SuperchargePill },
	mixins: [formatter],
	props: {
		lp: { type: Object as PropType<LoadLoadpoint>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
		color: { type: String, default: "" },
		priority: { type: Number, required: true },
		place: { type: String, default: "" },
		canFirst: Boolean,
		movable: Boolean,
		lifted: Boolean,
	},
	emits: ["open-supercharge", "move", "grip", "set-priority", "charge-first"],
	computed: {
		warning(): boolean {
			return (this.lp.paused && this.lp.measuredA > 0.5) || this.lp.stoodOffS > 0;
		},
		bursting(): boolean {
			return superchargeBursting(this.lp, this.state);
		},
		paused(): boolean {
			return superchargePaused(this.lp, this.state);
		},
		normal(): boolean {
			const lp = this.lp;
			return this.state.running && lp.connected && lp.wants && !lp.paused && !this.warning;
		},
		waitNote(): string {
			if (!this.lp.paused || !this.state.running || !this.lp.wants) return "";
			return waitReason(this.lp, this.state, (k, v) => this.$t(k, v || {}));
		},
		// supercharge is on, yet no burst runs: only the unusual reasons, the pill says the rest
		superchargeWait(): string {
			const s = this.state;
			if (!this.lp.supercharge || this.bursting || !s.running) return "";
			if (s.burstStoodDown) {
				return this.$t("loadManagement.reason.scStoodDown", { reason: s.burstStoodDown });
			}
			if (s.tempBlock) return this.$t("loadManagement.reason.scTooWarm");
			return "";
		},
		subline(): string {
			const parts: string[] = [];
			if (this.lp.vehicle) parts.push(this.lp.vehicle);
			if (this.superchargeWait) parts.push(this.superchargeWait);
			if (this.bursting) {
				parts.push(this.$t("loadManagement.reason.bursting"));
			} else if (this.normal) {
				parts.push(
					this.$t("loadManagement.reason.measured", {
						amps: this.fmtNumber(this.lp.measuredA, 1),
					})
				);
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
		// the same words as the main screen's Allowed
		allowed(): string {
			if (!this.state.running || !this.lp.wants) return "—";
			return `${this.lp.paused || this.state.blind > 0 ? 0 : this.lp.setpointA} A`;
		},
	},
});
</script>

<style scoped>
.lp-card {
	background: var(--evcc-box);
	border-radius: 1rem;
	padding: 1.25rem 1.5rem;
	display: flex;
	flex-direction: column;
	gap: 1rem;
	border: 1px solid var(--bs-border-color-translucent);
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
	align-items: flex-start;
	gap: 0.75rem;
}
.grip {
	min-width: 2.25rem;
	min-height: 2.75rem;
	justify-content: center;
	align-items: center;
	border: 0;
	background: none;
	color: var(--evcc-default-text);
	opacity: 0.75;
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
.who {
	flex-grow: 1;
	min-width: 0;
	display: flex;
	flex-direction: column;
	gap: 0.15rem;
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
.place-row .sub {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}
.place-row .first {
	flex-shrink: 0;
}
/* phones: the place text stays whole, the pill goes under it */
@media (max-width: 419px) {
	.place-row {
		flex-wrap: wrap;
	}
	.place-row .sub {
		white-space: normal;
	}
}
.sub::first-letter {
	text-transform: uppercase;
}
.sub--strong {
	color: var(--evcc-default-text);
	font-weight: 600;
}
.place-row {
	display: flex;
	flex-wrap: nowrap;
	min-width: 0;
	align-items: center;
	gap: 0.25rem 0.6rem;
}
.first {
	display: inline-flex;
	align-items: center;
	gap: 0.3rem;
	padding: 0.1rem 0.7rem;
	min-height: 1.9rem;
	white-space: nowrap;
}
.allowed {
	text-align: end;
	flex-shrink: 0;
}
.label {
	text-transform: uppercase;
	color: var(--evcc-gray);
	font-size: 14px;
	font-weight: normal;
}
.amps {
	font-size: 1.6rem;
	font-weight: 800;
	line-height: 1.1;
	font-variant-numeric: tabular-nums;
}
.foot {
	display: flex;
	flex-wrap: wrap;
	row-gap: 0.75rem;
	align-items: flex-start;
	justify-content: space-between;
	gap: 0.75rem;
	padding-top: 0.85rem;
	border-top: 1px solid var(--evcc-gray-25);
}
.stepper {
	display: inline-flex;
	align-items: center;
	border: 1px solid var(--evcc-gray-25);
	border-radius: var(--bs-border-radius);
	overflow: hidden;
	flex-shrink: 0;
}
.foot-actions :deep(.sc-wrap--stacked) {
	align-items: flex-start;
}
.foot-actions {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.5rem;
	min-width: 0;
	margin-right: auto;
}
.prio {
	margin-left: auto;
	display: inline-flex;
	flex-direction: column;
	align-items: flex-end;
	gap: 0.2rem;
	flex-shrink: 0;
}
.stepper-label {
	text-transform: uppercase;
	color: var(--evcc-gray);
	font-size: 14px;
	line-height: 1;
}
.stepper button {
	width: 2.25rem;
	height: 2.5rem;
	border: 0;
	background: transparent;
	color: var(--evcc-default-text);
	display: flex;
	align-items: center;
	justify-content: center;
}
.stepper button:hover:not(:disabled) {
	background: var(--evcc-gray-15);
}
.stepper button:disabled {
	opacity: 0.3;
}
.stepper button:focus-visible {
	outline: var(--bs-focus-ring-width) solid var(--bs-focus-ring-color);
	outline-offset: -2px;
}
.stepper-value {
	min-width: 2rem;
	line-height: 2.5rem;
	text-align: center;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	border-left: 1px solid var(--evcc-gray-25);
	border-right: 1px solid var(--evcc-gray-25);
}
@media (max-width: 575px) {
	.lp-card {
		padding: 1rem 1.1rem;
		border-radius: 1rem;
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
	.stepper button {
		width: 2.75rem;
		height: 2.75rem;
	}
}
@media (prefers-reduced-motion: reduce) {
	.lp-card {
		transition: none;
	}
}
.warn-text {
	color: #9a5200;
}
html.dark .warn-text {
	color: var(--evcc-orange);
}
/* phones: the pill column gives way, the stepper keeps its place on every card */
@media (max-width: 575px) {
	.foot {
		flex-wrap: nowrap;
		column-gap: 0.5rem;
	}
	.foot-actions {
		flex: 1 1 auto;
		min-width: 0;
	}
}
</style>
