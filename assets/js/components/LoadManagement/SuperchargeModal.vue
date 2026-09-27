<template>
	<GenericModal
		id="superchargeModal"
		ref="modal"
		:title="modalTitle"
		data-testid="supercharge-modal"
		:autofocus="false"
		@closed="closed"
	>
		<p class="text-muted mb-3">{{ description }}</p>
		<div v-if="movedFirst" class="ahead ahead--done mb-3" :style="aheadStyle" role="status">
			<svg
				width="16"
				height="16"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2.6"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
				class="done-icon"
			>
				<path d="m5 12 5 5 9-10" />
			</svg>
			<span>{{ nowFirstText }}</span>
		</div>
		<div v-else-if="ahead" ref="ahead" class="ahead mb-3" data-testid="supercharge-ahead">
			<span>{{ aheadText }}</span>
			<button
				type="button"
				class="btn btn-sm btn-pill first"
				:disabled="saving"
				data-testid="supercharge-charge-first"
				@click="chargeFirst"
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

		<fieldset class="mb-3">
			<legend class="stop-legend">
				{{ $t("loadManagement.supercharge.stopAt") }}
			</legend>
			<div class="form-check form-switch mb-2">
				<input
					id="superchargeNoEnd"
					v-model="noEnd"
					class="form-check-input"
					type="checkbox"
					role="switch"
					data-testid="supercharge-noend"
				/>
				<label class="form-check-label" for="superchargeNoEnd">
					{{ $t("loadManagement.supercharge.noEndSwitch") }}
				</label>
			</div>
			<div class="stop" :class="{ 'stop--muted': noEnd }">
				<div class="when mb-2">
					<div>
						<label class="form-label" for="superchargeDay">
							{{ $t("loadManagement.supercharge.day") }}
						</label>
						<select
							id="superchargeDay"
							v-model.number="day"
							class="form-select"
							data-testid="supercharge-day"
							@change="edited"
						>
							<option v-for="d in days" :key="d.value" :value="d.value">
								{{ d.label }}
							</option>
						</select>
					</div>
					<div>
						<label class="form-label" for="superchargeHour">
							{{ $t("loadManagement.supercharge.hour") }}
						</label>
						<select
							id="superchargeHour"
							v-model.number="hour"
							class="form-select"
							:class="{ 'select-bad': hourBad }"
							data-testid="supercharge-hour"
							@change="edited"
						>
							<option v-for="h in hours" :key="h" :value="h" :disabled="hourPast(h)">
								{{ pad(h) }}
							</option>
						</select>
					</div>
					<div>
						<label class="form-label" for="superchargeMinute">
							{{ $t("loadManagement.supercharge.minute") }}
						</label>
						<select
							id="superchargeMinute"
							v-model.number="minute"
							class="form-select"
							:class="{ 'select-bad': minuteBad }"
							data-testid="supercharge-minute"
							@change="edited"
						>
							<option
								v-for="m in minutes"
								:key="m"
								:value="m"
								:disabled="minutePast(m)"
							>
								{{ pad(m) }}
							</option>
						</select>
					</div>
				</div>

				<p v-if="active" class="picks-note">
					{{ $t("loadManagement.supercharge.picksFromNow") }}
				</p>
				<div
					class="picks"
					role="group"
					:aria-label="$t('loadManagement.supercharge.quickPicks')"
				>
					<button
						v-for="p in presets"
						:key="p.id"
						type="button"
						class="btn btn-sm btn-pill pick"
						:class="{ active: p.id === selectedPreset }"
						:aria-pressed="p.id === selectedPreset"
						:data-testid="`supercharge-pick-${p.id}`"
						@click="choose(p.id)"
					>
						{{ p.label }}
					</button>
				</div>
			</div>
		</fieldset>

		<p
			class="summary mb-3"
			:class="{ 'summary--bad': inPast }"
			role="status"
			data-testid="supercharge-summary"
		>
			{{ summary }}
		</p>
		<p v-if="error" class="small text-danger" role="alert">{{ error }}</p>

		<div class="footer">
			<button
				v-if="active"
				type="button"
				class="btn btn-outline-danger"
				:disabled="saving"
				data-testid="supercharge-off"
				@click="turnOff"
			>
				{{ $t("loadManagement.supercharge.stopNow") }}
			</button>
			<div class="d-flex align-items-center gap-2 ms-auto">
				<button
					type="button"
					class="btn btn-link text-muted"
					data-bs-dismiss="modal"
					:disabled="saving"
				>
					{{ dismissLabel }}
				</button>
				<button
					type="button"
					class="btn btn-primary"
					:disabled="saving || inPast || (active && !changed)"
					data-testid="supercharge-save"
					@click="save"
				>
					<span
						v-if="saving"
						class="spinner-border spinner-border-sm me-1"
						role="status"
						aria-hidden="true"
					></span>
					{{
						active
							? $t("loadManagement.supercharge.apply")
							: $t("loadManagement.supercharge.start")
					}}
				</button>
			</div>
		</div>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import type { LoadLoadpoint, LoadState, SuperchargeRequest } from "@/types/supercharge";
import api from "@/api";
import formatter from "@/mixins/formatter";
import GenericModal from "../Helper/GenericModal.vue";
import { dayOffset, fmtDayShort } from "./format";
import { carAhead, errorText } from "./state";

const DAYS = 8;
const MORNING = 8;
const pad = (n: number) => String(n).padStart(2, "0");

// Asks until when a loadpoint may supercharge. Day, hour and minute are always on screen and
// already filled in: a quick pick only fills them, and touching one of them means "until then".
// Every answer is sent as an absolute time, so the browser's and the server's clocks cannot
// disagree about which 08:00 was meant. Hours are 24-hour selects on purpose: a native time
// input follows the browser's locale and can show AM/PM.
export default defineComponent({
	name: "SuperchargeModal",
	components: { GenericModal },
	mixins: [formatter],
	props: {
		// the whole load state, to tell a car behind another in the charging order
		loadState: { type: Object as PropType<LoadState | undefined>, default: undefined },
	},
	data() {
		return {
			index: 0,
			title: "",
			active: false,
			day: 0,
			hour: MORNING,
			minute: 0,
			noEnd: false,
			picked: "",
			// what an active supercharge had when the dialog opened
			initial: "",
			initialText: "",
			// Charge first was pressed here: say so instead of dropping the note
			movedFirst: false,
			// the car whose supercharge Charge first paused
			pausedOther: "",
			// the note keeps its height when it turns into the confirmation
			aheadH: 0,
			// the stop was moved forward to the earliest time; the summary says so
			moved: false,
			now: new Date(),
			saving: false,
			error: "",
			ticker: undefined as ReturnType<typeof setInterval> | undefined,
		};
	},
	computed: {
		presets() {
			return [
				{ id: "1", label: this.$t("loadManagement.supercharge.hours", { h: 1 }, 1) },
				{ id: "2", label: this.$t("loadManagement.supercharge.hours", { h: 2 }, 2) },
				{ id: "4", label: this.$t("loadManagement.supercharge.hours", { h: 4 }, 4) },
				{
					id: "morning",
					label: this.$t(
						`loadManagement.supercharge.${this.morningToday ? "todayAt" : "tomorrowAt"}`,
						{ time: this.fmtHourMinute(new Date(2000, 0, 1, MORNING, 0)) }
					),
				},
			];
		},
		// the quick pick the fields currently match, if any
		selectedPreset(): string {
			if (this.noEnd) return "";
			if (this.picked) return this.same(this.presetTarget(this.picked)) ? this.picked : "";
			// the fixed morning pick may light up by itself; relative ones only when chosen
			return this.same(this.presetTarget("morning")) ? "morning" : "";
		},
		// before 07:55 the next morning is still today
		morningToday(): boolean {
			const n = this.now;
			return n.getHours() * 60 + n.getMinutes() < MORNING * 60 - 5;
		},
		days() {
			return Array.from({ length: DAYS }, (_, i) => ({ value: i, label: this.dayLabel(i) }));
		},
		hours(): number[] {
			return Array.from({ length: 24 }, (_, i) => i);
		},
		minutes(): number[] {
			// five-minute steps, plus the exact minute of a stop set elsewhere
			const res = Array.from({ length: 12 }, (_, i) => i * 5);
			if (!res.includes(this.minute)) res.push(this.minute);
			return res.sort((a, b) => a - b);
		},
		target(): Date | null {
			if (this.noEnd) return null;
			const n = this.now;
			return new Date(
				n.getFullYear(),
				n.getMonth(),
				n.getDate() + this.day,
				this.hour,
				this.minute
			);
		},
		hourBad(): boolean {
			return this.inPast && this.hourPast(this.hour);
		},
		minuteBad(): boolean {
			return this.inPast && !this.hourBad;
		},
		inPast(): boolean {
			return !!this.target && this.target.getTime() <= this.now.getTime();
		},
		stateKey(): string {
			return this.noEnd ? "none" : `${this.day}-${this.hour}-${this.minute}`;
		},
		changed(): boolean {
			return this.stateKey !== this.initial;
		},
		untilText(): string {
			if (this.noEnd || !this.target) return "";
			return `${this.dayLabel(this.day, true)} ${this.fmtHourMinute(this.target)}`;
		},
		me(): LoadLoadpoint | undefined {
			return (this.loadState?.loadpoints || []).find((lp) => lp.index === this.index);
		},
		// a car higher in the order that is charging now, so this one may wait
		ahead(): string {
			if (!this.me || !this.loadState) return "";
			return carAhead(this.me, this.loadState)?.title || "";
		},
		// Charge first stands the car ahead down, a running supercharge included: say so first
		aheadSupercharging(): boolean {
			const lps = this.loadState?.loadpoints || [];
			return !!lps.find((o) => o.title === this.ahead && o.supercharge);
		},
		aheadText(): string {
			const key = this.aheadSupercharging ? "aheadSupercharge" : "ahead";
			return this.$t(`loadManagement.supercharge.${key}`, {
				first: this.ahead,
				name: this.title,
			});
		},
		nowFirstText(): string {
			const key = this.pausedOther ? "nowFirstPaused" : "nowFirst";
			return this.$t(`loadManagement.supercharge.${key}`, {
				name: this.title,
				other: this.pausedOther,
			});
		},
		aheadStyle(): Record<string, string> | undefined {
			return this.aheadH ? { minHeight: `${this.aheadH}px` } : undefined;
		},
		// Charge first applies at once, so there is nothing left to cancel
		dismissLabel(): string {
			return this.$t(`loadManagement.${this.movedFirst ? "close" : "cancel"}`);
		},
		modalTitle(): string {
			if (!this.title) return "";
			if (!this.active)
				return this.$t("loadManagement.supercharge.modalTitle", { name: this.title });
			if (this.me?.paused)
				return this.$t("loadManagement.supercharge.pausedTitle", { name: this.title });
			return this.$t("loadManagement.supercharge.activeTitle", { name: this.title });
		},
		description(): string {
			if (!this.active) {
				return this.$t("loadManagement.supercharge.modalDescription");
			}
			// the summary shows the stop time; after an edit this keeps the one still in force
			if (!this.changed) return this.$t("loadManagement.supercharge.activeDescription");
			if (!this.initialText) return this.$t("loadManagement.supercharge.activeNoEnd");
			return this.$t("loadManagement.supercharge.activeWas", { time: this.initialText });
		},
		summary(): string {
			if (!this.target) return this.$t("loadManagement.supercharge.summaryForever");
			if (this.inPast) return this.$t("loadManagement.supercharge.inPast");
			// a "+2 h" pick reads 2:00 h, not the 1:58 h its rounding to five minutes leaves
			const rel = ["1", "2", "4"].includes(this.selectedPreset);
			const secs = rel
				? Number(this.selectedPreset) * 3600
				: Math.round((this.target.getTime() - this.now.getTime()) / 1000);
			const text = this.$t("loadManagement.supercharge.summaryUntil", {
				time: this.untilText,
				duration: this.fmtLeft(secs),
			});
			return this.moved
				? `${this.$t("loadManagement.supercharge.movedEarliest")} ${text}`
				: text;
		},
	},
	watch: {
		noEnd() {
			this.moved = false;
		},
	},
	beforeUnmount() {
		clearInterval(this.ticker);
	},
	methods: {
		pad,
		// "45 min", "15 h 16 min", "3 days 2 h"
		fmtLeft(secs: number): string {
			const mins = Math.max(1, Math.round(secs / 60));
			if (mins < 60) return `${mins}\u202Fmin`;
			// past two days, days and hours read better than "73:10 h"
			if (mins >= 48 * 60) {
				const d = Math.floor(mins / 1440);
				const h = Math.round((mins % 1440) / 60);
				return this.$t("loadManagement.supercharge.daysHours", { d, h }, d);
			}
			const h = Math.floor(mins / 60);
			const m = mins % 60;
			return m ? `${h} h ${m} min` : `${h} h`;
		},
		hourPast(h: number): boolean {
			return this.day === 0 && h < this.now.getHours();
		},
		minutePast(m: number): boolean {
			return (
				this.day === 0 && this.hour === this.now.getHours() && m <= this.now.getMinutes()
			);
		},
		dayLabel(i: number, lower = false): string {
			if (i === 0)
				return this.$t(`loadManagement.supercharge.${lower ? "todayLower" : "today"}`);
			if (i === 1) {
				return this.$t(
					`loadManagement.supercharge.${lower ? "tomorrowLower" : "tomorrow"}`
				);
			}
			const n = this.now;
			return fmtDayShort(
				new Date(n.getFullYear(), n.getMonth(), n.getDate() + i),
				this.$i18n.locale
			);
		},
		open({ index, title, active, until }: SuperchargeRequest) {
			this.index = index;
			this.title = title;
			this.active = active;
			this.error = "";
			this.picked = "";
			this.movedFirst = false;
			this.pausedOther = "";
			this.aheadH = 0;
			this.moved = false;
			this.now = new Date();
			// an active supercharge without a stop shows the morning, switched off
			this.set(active && until ? new Date(until) : this.presetTarget("morning"));
			this.noEnd = active && !until;
			this.initial = this.stateKey;
			this.initialText = active && until ? this.untilText : "";
			clearInterval(this.ticker);
			this.ticker = setInterval(() => {
				this.now = new Date();
				if (!this.active || this.changed) this.keepAhead();
			}, 30000);
			(this.$refs["modal"] as InstanceType<typeof GenericModal> | undefined)?.open();
		},
		closed() {
			this.error = "";
			clearInterval(this.ticker);
		},
		// the first five-minute step at least five minutes away
		earliest(): Date {
			const step = 5 * 60 * 1000;
			return new Date(Math.ceil((this.now.getTime() + step) / step) * step);
		},
		// a stop that is gone or too close moves to the earliest one
		keepAhead() {
			if (this.noEnd || !this.target) return;
			const first = this.earliest();
			if (this.target.getTime() < first.getTime()) {
				this.set(first);
				this.moved = true;
			}
		},
		presetTarget(id: string): Date {
			const n = this.now;
			if (id === "morning") {
				const day = n.getDate() + (this.morningToday ? 0 : 1);
				return new Date(n.getFullYear(), n.getMonth(), day, MORNING, 0);
			}
			// "+2 h" lands on the nearest five minutes, as the minute list has them
			const at = new Date(n.getTime() + Number(id) * 3600 * 1000);
			at.setMinutes(Math.round(at.getMinutes() / 5) * 5, 0, 0);
			return at;
		},
		same(d: Date): boolean {
			return (
				dayOffset(d, this.now) === this.day &&
				d.getHours() === this.hour &&
				d.getMinutes() === this.minute
			);
		},
		set(d: Date) {
			// a stop further out than the day list reaches is shown on its last day
			this.day = Math.max(0, Math.min(DAYS - 1, dayOffset(d, this.now)));
			this.hour = d.getHours();
			this.minute = d.getMinutes();
		},
		choose(id: string) {
			this.error = "";
			this.noEnd = false;
			this.moved = false;
			this.picked = id;
			this.set(this.presetTarget(id));
		},
		// touching day, hour or minute always means "until then"
		edited() {
			this.error = "";
			this.noEnd = false;
			this.picked = "";
			this.moved = false;
			// a time already gone, or only minutes away, moves to the next five minutes past +5 min
			this.keepAhead();
		},
		// the same move as the Charge first button in the charging order
		async chargeFirst() {
			const me = this.me;
			if (!me) return;
			this.saving = true;
			this.error = "";
			try {
				for (const o of this.loadState?.loadpoints || []) {
					if (o.index !== me.index && o.priority >= 10) {
						await api.post(`loadpoints/${o.index + 1}/priority/9`);
					}
				}
				await api.post(`loadpoints/${me.index + 1}/priority/10`);
				this.aheadH = (this.$refs["ahead"] as HTMLElement | undefined)?.offsetHeight || 0;
				this.pausedOther = this.aheadSupercharging ? this.ahead : "";
				this.movedFirst = true;
			} catch (e) {
				this.error = errorText(e);
			} finally {
				this.saving = false;
			}
		},
		async post(on: boolean, until: string) {
			this.saving = true;
			this.error = "";
			try {
				await api.post(`supercharging/loadpoints/${this.index + 1}/supercharge`, {
					on,
					until,
				});
				(this.$refs["modal"] as InstanceType<typeof GenericModal> | undefined)?.close();
			} catch (e) {
				this.error = errorText(e);
			} finally {
				this.saving = false;
			}
		},
		save() {
			this.now = new Date();
			if (this.inPast) return;
			this.post(true, this.target ? this.target.toISOString() : "");
		},
		turnOff() {
			this.post(false, "");
		},
	},
});
</script>

<style scoped>
fieldset {
	border: 0;
	margin: 0;
	padding: 0;
	min-width: 0;
}
.stop-legend {
	font-size: 1rem;
	font-weight: 700;
	margin-bottom: 0.5rem;
}
.when {
	display: grid;
	grid-template-columns: minmax(0, 1fr) 5rem 5rem;
	gap: 0.5rem;
}
.stop {
	transition: opacity var(--evcc-transition-fast);
}
.when .form-label {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin-bottom: 0.25rem;
}
.stop--muted {
	opacity: 0.4;
}
.select-bad {
	border-color: var(--bs-danger);
}
.picks {
	display: flex;
	flex-wrap: wrap;
	gap: 0.5rem;
}
.pick {
	min-height: 2.25rem;
}
@media (max-width: 575px) {
	.picks {
		display: grid;
		grid-template-columns: 1fr 1fr;
	}
}
.picks-note {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin: 0 0 0.35rem;
}
.pick:focus:not(:focus-visible),
.ahead .first:focus:not(:focus-visible) {
	box-shadow: none;
}
.ahead .first {
	display: inline-flex;
	align-items: center;
	gap: 0.3rem;
}
.pick.active {
	background: var(--evcc-default-text);
	border-color: var(--evcc-default-text);
	color: var(--evcc-background);
}
.ahead.ahead--done {
	flex-wrap: nowrap;
	color: var(--evcc-default-text);
	background: color-mix(in srgb, var(--evcc-green) 14%, transparent);
}
.done-icon {
	flex-shrink: 0;
	color: var(--evcc-darker-green);
}
.ahead {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.5rem 0.75rem;
	padding: 0.6rem 0.9rem;
	border-radius: 0.75rem;
	background: var(--evcc-gray-10);
	font-size: 0.875rem;
}
@media (max-width: 575px) {
	.footer > .btn-outline-danger {
		flex-basis: 100%;
	}
	.footer > div {
		width: 100%;
		justify-content: flex-end;
	}
}
.footer {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.5rem;
}
.summary {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin: 0;
}
.form-check-input:focus:not(:focus-visible) {
	box-shadow: none;
}
.summary--bad {
	color: var(--bs-danger);
}
@media (prefers-reduced-motion: reduce) {
	.stop {
		transition: none;
	}
}
</style>
