<template>
	<GenericModal
		id="superchargeModal"
		ref="modal"
		:title="$t('loadManagement.supercharge.modalTitle', { name: title })"
		data-testid="supercharge-modal"
		@closed="error = ''"
	>
		<p class="text-muted mb-4">{{ $t("loadManagement.supercharge.modalDescription") }}</p>

		<div class="presets mb-3" role="group" :aria-label="$t('loadManagement.supercharge.time')">
			<button
				v-for="p in presets"
				:key="p.id"
				type="button"
				class="preset"
				:class="{ active: p.id === selectedPreset }"
				:aria-pressed="p.id === selectedPreset"
				@click="choose(p.id)"
			>
				{{ p.label }}
			</button>
		</div>

		<div v-if="selectedPreset === 'custom'" class="row g-2 mb-3">
			<div class="col-7">
				<label class="form-label small" for="superchargeDay">
					{{ $t("loadManagement.supercharge.day") }}
				</label>
				<select id="superchargeDay" v-model="day" class="form-select">
					<option v-for="d in days" :key="d.value" :value="d.value">{{ d.label }}</option>
				</select>
			</div>
			<div class="col-5">
				<label class="form-label small" for="superchargeTime">
					{{ $t("loadManagement.supercharge.time") }}
				</label>
				<select id="superchargeTime" v-model="time" class="form-select">
					<option v-for="t in times" :key="t" :value="t">{{ timeLabel(t) }}</option>
				</select>
			</div>
		</div>

		<div class="summary mb-3">
			<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" class="bolt"><path d="M13 2 4 14h7l-1 8 9-12h-7z" /></svg>
			<span>{{ summary }}</span>
		</div>
		<p v-if="error" class="small text-danger">{{ error }}</p>

		<div class="actions">
			<button
				v-if="active"
				type="button"
				class="btn btn-outline-danger"
				:disabled="saving"
				@click="turnOff"
			>
				{{ $t("loadManagement.supercharge.turnOff") }}
			</button>
			<button
				v-else
				type="button"
				class="btn btn-outline-secondary"
				data-bs-dismiss="modal"
				:disabled="saving"
			>
				{{ $t("loadManagement.cancel") }}
			</button>
			<button type="button" class="btn btn-primary" :disabled="saving" @click="save">
				{{
					active
						? $t("loadManagement.supercharge.apply")
						: $t("loadManagement.supercharge.start")
				}}
			</button>
		</div>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import GenericModal from "../Helper/GenericModal.vue";

const pad = (n: number) => String(n).padStart(2, "0");
const dayValue = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

// Asks until when a loadpoint may supercharge. Every answer is sent as an absolute time,
// so the browser's and the server's clocks cannot disagree about which 06:00 was meant.
export default defineComponent({
	name: "SuperchargeModal",
	components: { GenericModal },
	mixins: [formatter],
	emits: ["updated"],
	data() {
		return {
			index: 0,
			title: "",
			active: false,
			day: "",
			time: "08:00",
			selectedPreset: "0800" as string,
			saving: false,
			error: "",
		};
	},
	computed: {
		presets() {
			return [
				{
					id: "0800",
					label: this.$t("loadManagement.supercharge.untilMorning", {
						time: this.fmtHourMinute(new Date(2000, 0, 1, 8, 0)),
					}),
				},
				{ id: "2", label: this.$t("loadManagement.supercharge.hours", { h: 2 }) },
				{ id: "4", label: this.$t("loadManagement.supercharge.hours", { h: 4 }) },
				{ id: "8", label: this.$t("loadManagement.supercharge.hours", { h: 8 }) },
				{ id: "custom", label: this.$t("loadManagement.supercharge.custom") },
				{ id: "forever", label: this.$t("loadManagement.supercharge.noEnd") },
			];
		},
		days() {
			const res = [];
			const now = new Date();
			for (let i = 0; i < 8; i++) {
				const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + i);
				res.push({
					value: dayValue(d),
					label:
						i === 0
							? this.$t("loadManagement.supercharge.today")
							: i === 1
								? this.$t("loadManagement.supercharge.tomorrow")
								: d.toLocaleDateString(this.$i18n.locale, {
										weekday: "short",
										day: "numeric",
										month: "short",
									}),
				});
			}
			return res;
		},
		times() {
			const res = [];
			for (let h = 0; h < 24; h++) {
				for (let m = 0; m < 60; m += 15) res.push(`${pad(h)}:${pad(m)}`);
			}
			return res;
		},
		target(): Date | null {
			if (this.selectedPreset === "forever" || !this.day) return null;
			const [y, mo, d] = this.day.split("-").map(Number);
			const [h, mi] = this.time.split(":").map(Number);
			return new Date(y!, (mo || 1) - 1, d, h, mi, 0, 0);
		},
		summary(): string {
			if (!this.target) return this.$t("loadManagement.supercharge.summaryForever");
			return this.$t("loadManagement.supercharge.summaryUntil", {
				time: this.fmtDayTime(this.target),
			});
		},
	},
	methods: {
		timeLabel(t: string): string {
			const [h, m] = t.split(":").map(Number);
			return this.fmtHourMinute(new Date(2000, 0, 1, h, m));
		},
		open(index: number, title: string, active: boolean, until: string | null) {
			this.index = index;
			this.title = title;
			this.active = active;
			this.error = "";
			if (active && until) {
				const d = new Date(until);
				this.day = dayValue(d);
				this.time = `${pad(d.getHours())}:${pad(Math.floor(d.getMinutes() / 15) * 15)}`;
				this.selectedPreset = "custom";
			} else if (active) {
				this.choose("forever");
			} else {
				this.choose("0800");
			}
			(this.$refs["modal"] as InstanceType<typeof GenericModal> | undefined)?.open();
		},
		choose(id: string) {
			this.selectedPreset = id;
			const now = new Date();
			if (id === "0800") {
				const at = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 8, 0);
				if (at <= now) at.setDate(at.getDate() + 1);
				this.day = dayValue(at);
				this.time = "08:00";
			} else if (id !== "forever" && id !== "custom") {
				const at = new Date(now.getTime() + Number(id) * 3600 * 1000);
				const mins = Math.round(at.getMinutes() / 15) * 15;
				at.setMinutes(mins, 0, 0);
				this.day = dayValue(at);
				this.time = `${pad(at.getHours())}:${pad(at.getMinutes())}`;
			}
		},
		async post(on: boolean, until: string) {
			this.saving = true;
			this.error = "";
			try {
				await api.post(`supercharging/loadpoints/${this.index + 1}/supercharge`, { on, until });
				this.$emit("updated");
				(this.$refs["modal"] as InstanceType<typeof GenericModal> | undefined)?.close();
			} catch (e: any) {
				this.error = e?.response?.data?.error || String(e);
			} finally {
				this.saving = false;
			}
		},
		save() {
			const target = this.target;
			if (target && target.getTime() <= Date.now()) {
				this.error = this.$t("loadManagement.supercharge.inPast");
				return;
			}
			this.post(true, target ? target.toISOString() : "");
		},
		turnOff() {
			this.post(false, "");
		},
	},
});
</script>

<style scoped>
.presets {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 0.5rem;
}
.preset {
	min-height: 2.75rem;
	border-radius: 12px;
	font-weight: 700;
	font-size: 0.875rem;
	border: 1px solid var(--evcc-gray-25);
	background: transparent;
	color: var(--evcc-default-text);
}
.preset:hover {
	background: var(--evcc-gray-10);
}
.preset.active {
	background: var(--evcc-default-text);
	border-color: var(--evcc-default-text);
	color: var(--evcc-background);
}
.preset:focus-visible {
	outline: var(--bs-focus-ring-width) solid var(--bs-focus-ring-color);
}
.summary {
	display: flex;
	gap: 0.75rem;
	padding: 0.85rem 1rem;
	border-radius: 16px;
	background: var(--evcc-gray-10);
	font-size: 0.875rem;
}
.bolt {
	flex-shrink: 0;
	margin-top: 1px;
	color: var(--evcc-dark-yellow);
}
html.dark .bolt {
	color: var(--evcc-yellow);
}
.actions {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 0.75rem;
}
.actions .btn {
	border-radius: 999px;
	min-height: 2.75rem;
	font-weight: 700;
}
</style>
