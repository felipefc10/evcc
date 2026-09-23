<template>
	<GenericModal
		id="superchargeModal"
		ref="modal"
		:title="$t('loadManagement.supercharge.modalTitle', { name: title })"
		data-testid="supercharge-modal"
		@closed="error = ''"
	>
		<p class="text-muted">{{ $t("loadManagement.supercharge.modalDescription") }}</p>

		<div class="d-flex flex-wrap gap-2 mb-4">
			<button
				v-for="p in presets"
				:key="p.id"
				type="button"
				class="btn btn-sm"
				:class="p.id === selectedPreset ? 'btn-primary' : 'btn-outline-secondary'"
				@click="choose(p.id)"
			>
				{{ p.label }}
			</button>
		</div>

		<div class="row g-2 align-items-end mb-3">
			<div class="col-7">
				<label class="form-label small" for="superchargeDay">
					{{ $t("loadManagement.supercharge.day") }}
				</label>
				<select
					id="superchargeDay"
					v-model="day"
					class="form-select"
					@change="selectedPreset = 'custom'"
				>
					<option v-for="d in days" :key="d.value" :value="d.value">{{ d.label }}</option>
				</select>
			</div>
			<div class="col-5">
				<label class="form-label small" for="superchargeTime">
					{{ $t("loadManagement.supercharge.time") }}
				</label>
				<select
					id="superchargeTime"
					v-model="time"
					class="form-select"
					@change="selectedPreset = 'custom'"
				>
					<option v-for="t in times" :key="t" :value="t">{{ t }}</option>
				</select>
			</div>
		</div>
		<p class="small mb-3">{{ summary }}</p>
		<p v-if="error" class="small text-danger">{{ error }}</p>

		<div class="d-flex justify-content-between gap-2 flex-wrap">
			<button
				v-if="active"
				type="button"
				class="btn btn-link text-danger px-0"
				:disabled="saving"
				@click="turnOff"
			>
				{{ $t("loadManagement.supercharge.turnOff") }}
			</button>
			<span v-else></span>
			<div class="d-flex gap-2">
				<button
					type="button"
					class="btn btn-outline-secondary"
					data-bs-dismiss="modal"
					:disabled="saving"
				>
					{{ $t("loadManagement.cancel") }}
				</button>
				<button type="button" class="btn btn-primary" :disabled="saving" @click="save">
					{{ $t("loadManagement.supercharge.save") }}
				</button>
			</div>
		</div>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import api from "@/api";
import GenericModal from "../Helper/GenericModal.vue";

const pad = (n: number) => String(n).padStart(2, "0");
const dayValue = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

// Asks until when a loadpoint may supercharge. Every answer is sent as an absolute time,
// so the browser's and the server's clocks cannot disagree about which 06:00 was meant.
export default defineComponent({
	name: "SuperchargeModal",
	components: { GenericModal },
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
				{ id: "0800", label: this.$t("loadManagement.supercharge.until", { time: "08:00" }) },
				{ id: "2", label: this.$t("loadManagement.supercharge.hours", { h: 2 }) },
				{ id: "4", label: this.$t("loadManagement.supercharge.hours", { h: 4 }) },
				{ id: "8", label: this.$t("loadManagement.supercharge.hours", { h: 8 }) },
				{ id: "forever", label: this.$t("loadManagement.supercharge.indefinitely") },
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
			if (this.selectedPreset === "forever") return null;
			const [y, mo, d] = this.day.split("-").map(Number);
			const [h, mi] = this.time.split(":").map(Number);
			return new Date(y!, (mo || 1) - 1, d, h, mi, 0, 0);
		},
		summary(): string {
			if (!this.target) return this.$t("loadManagement.supercharge.summaryForever");
			return this.$t("loadManagement.supercharge.summaryUntil", {
				time: this.target.toLocaleString(this.$i18n.locale, {
					weekday: "long",
					hour: "2-digit",
					minute: "2-digit",
				}),
			});
		},
	},
	methods: {
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
			(this.$refs["modal"] as InstanceType<typeof GenericModal>).open();
		},
		choose(id: string) {
			this.selectedPreset = id;
			const now = new Date();
			if (id === "0800") {
				const at = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 8, 0);
				if (at <= now) at.setDate(at.getDate() + 1);
				this.day = dayValue(at);
				this.time = "08:00";
			} else if (id !== "forever") {
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
				(this.$refs["modal"] as InstanceType<typeof GenericModal>).close();
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
