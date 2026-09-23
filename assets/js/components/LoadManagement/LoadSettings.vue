<template>
	<div data-testid="load-settings">
		<Card class="mb-4 box-pull-out" :title="$t('loadManagement.settings.general')">
			<div class="form-check form-switch mb-3">
				<input
					id="lmEnabled"
					:checked="config.enabled"
					class="form-check-input"
					type="checkbox"
					role="switch"
					@change="save({ enabled: ($event.target as HTMLInputElement).checked }, 'enabled')"
				/>
				<label class="form-check-label fw-bold" for="lmEnabled">
					{{ $t("loadManagement.settings.enabled") }}
				</label>
				<div class="small text-muted">{{ $t("loadManagement.settings.enabledHelp") }}</div>
			</div>
			<SettingsFormRow
				id="lmMeterUri"
				:label="$t('loadManagement.settings.meterUri')"
				:description="$t('loadManagement.settings.meterUriHelp')"
			>
				<input
					id="lmMeterUri"
					class="form-control"
					type="url"
					:value="config.meterUri"
					placeholder="http://192.168.1.10/emeter/0"
					@change="save({ meterUri: ($event.target as HTMLInputElement).value }, 'meterUri')"
				/>
				<Feedback :msg="feedback['meterUri']" />
			</SettingsFormRow>
			<NumberRow
				v-for="f in curveFields"
				:key="f.key"
				:field="f"
				:value="config[f.key as 'q' | 'k' | 'contractKva' | 'failsafeA']"
				:feedback="feedback[f.key]"
				@change="(v: number) => save({ [f.key]: v }, f.key, v)"
			/>
		</Card>

		<Card
			v-for="group in groups"
			:key="group.id"
			class="mb-4 box-pull-out"
			:title="$t(`loadManagement.settings.groups.${group.id}.title`)"
			:subtitle="$t(`loadManagement.settings.groups.${group.id}.subtitle`)"
		>
			<div
				v-if="group.id === 'burst'"
				class="row row-cols-2 row-cols-md-4 g-3 mb-4"
				data-testid="load-tuning-tiles"
			>
				<div v-for="t in tiles" :key="t.label" class="col">
					<div class="tile p-2 h-100">
						<div class="tile-value">{{ t.value }}</div>
						<div class="small text-muted">{{ t.label }}</div>
					</div>
				</div>
			</div>
			<template v-for="f in group.fields" :key="f.key">
				<SettingsFormRow
					v-if="f.type === 'select'"
					:id="`lm-${f.key}`"
					:label="$t(`loadManagement.settings.fields.${f.key}.label`)"
					:description="$t(`loadManagement.settings.fields.${f.key}.help`)"
				>
					<select
						:id="`lm-${f.key}`"
						class="form-select"
						:value="settingValue(f.key)"
						@change="
							saveSetting(f.key, ($event.target as HTMLSelectElement).value)
						"
					>
						<option v-for="o in f.options" :key="o" :value="o">
							{{ $t(`loadManagement.settings.fields.${f.key}.options.${o}`) }}
						</option>
					</select>
					<Feedback :msg="feedback[f.key]" />
				</SettingsFormRow>
				<SettingsFormRow
					v-else-if="f.type === 'text'"
					:id="`lm-${f.key}`"
					:label="$t(`loadManagement.settings.fields.${f.key}.label`)"
					:description="$t(`loadManagement.settings.fields.${f.key}.help`)"
				>
					<input
						:id="`lm-${f.key}`"
						class="form-control font-monospace"
						type="text"
						:value="settingValue(f.key)"
						@change="saveSetting(f.key, ($event.target as HTMLInputElement).value)"
					/>
					<Feedback :msg="feedback[f.key]" />
				</SettingsFormRow>
				<NumberRow
					v-else
					:field="f"
					:value="Number(settingValue(f.key))"
					:feedback="feedback[f.key]"
					@change="(v: number) => saveSetting(f.key, v)"
				/>
			</template>
		</Card>

		<Card class="mb-4 box-pull-out" :title="$t('loadManagement.settings.loadpoints')">
			<p class="small text-muted">{{ $t("loadManagement.settings.loadpointsHelp") }}</p>
			<div
				v-for="lp in loadpoints"
				:key="lp.name"
				class="mb-4"
				:data-testid="`load-lpconfig-${lp.index + 1}`"
			>
				<h4 class="h6 fw-bold">{{ lp.title }}</h4>
				<div class="form-check form-switch mb-2">
					<input
						:id="`lm-fast-${lp.index}`"
						:checked="lpConfig(lp.name).fast"
						class="form-check-input"
						type="checkbox"
						role="switch"
						@change="
							saveLp(lp.name, { fast: ($event.target as HTMLInputElement).checked })
						"
					/>
					<label class="form-check-label" :for="`lm-fast-${lp.index}`">
						{{ $t("loadManagement.settings.fast") }}
					</label>
					<div class="small text-muted">{{ $t("loadManagement.settings.fastHelp") }}</div>
				</div>
				<SettingsFormRow
					:id="`lm-measure-${lp.index}`"
					:label="$t('loadManagement.settings.measureTopic')"
					:description="$t('loadManagement.settings.measureTopicHelp')"
				>
					<div class="input-group">
						<input
							:id="`lm-measure-${lp.index}`"
							class="form-control font-monospace"
							type="text"
							:value="lpConfig(lp.name).measureTopic"
							@change="
								saveLp(lp.name, {
									measureTopic: ($event.target as HTMLInputElement).value,
								})
							"
						/>
						<select
							class="form-select unit-select"
							:aria-label="$t('loadManagement.settings.measureUnit')"
							:value="lpConfig(lp.name).measureUnit"
							@change="
								saveLp(lp.name, {
									measureUnit: ($event.target as HTMLSelectElement).value as
										| 'A'
										| 'W',
								})
							"
						>
							<option value="A">A</option>
							<option value="W">W</option>
						</select>
					</div>
				</SettingsFormRow>
				<SettingsFormRow
					:id="`lm-temp-${lp.index}`"
					:label="$t('loadManagement.settings.tempTopic')"
					:description="$t('loadManagement.settings.tempTopicHelp')"
				>
					<input
						:id="`lm-temp-${lp.index}`"
						class="form-control font-monospace"
						type="text"
						:value="lpConfig(lp.name).tempTopic"
						@change="
							saveLp(lp.name, { tempTopic: ($event.target as HTMLInputElement).value })
						"
					/>
				</SettingsFormRow>
				<Feedback :msg="feedback[`lp-${lp.name}`]" />
			</div>
		</Card>
	</div>
</template>

<script lang="ts">
import { defineComponent, h, type PropType } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import Card from "../Helper/Card.vue";
import SettingsFormRow from "../Helper/SettingsFormRow.vue";
import NumberRow, { type NumberField } from "./NumberRow.vue";
import type {
	LoadConfig,
	LoadLoadpoint,
	LoadLpConfig,
	LoadSettings,
	LoadState,
} from "@/types/supercharge";

interface Field extends NumberField {
	type?: "select" | "text";
	options?: string[];
}

const Feedback = defineComponent({
	props: { msg: { type: Object as PropType<{ ok: boolean; text: string } | undefined> } },
	setup(props) {
		return () =>
			props.msg
				? h(
						"div",
						{ class: ["small", "mt-1", props.msg.ok ? "text-primary" : "text-danger"] },
						props.msg.text
					)
				: null;
	},
});

// Every setting is applied the moment it is changed: the control loop reads it on its next second.
export default defineComponent({
	name: "LoadSettings",
	components: { Card, SettingsFormRow, NumberRow, Feedback },
	mixins: [formatter],
	props: {
		config: { type: Object as PropType<LoadConfig>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
	},
	data() {
		return {
			feedback: {} as Record<string, { ok: boolean; text: string } | undefined>,
			timers: {} as Record<string, ReturnType<typeof setTimeout>>,
		};
	},
	computed: {
		loadpoints(): LoadLoadpoint[] {
			return this.state.loadpoints || [];
		},
		curveFields(): Field[] {
			return [
				{ key: "contractKva", unit: "kVA", min: 1, max: 20, step: 0.05, digits: 2 },
				{ key: "k", unit: "×", min: 1, max: 2, step: 0.01, digits: 2 },
				{ key: "q", unit: "", min: 1, max: 200, step: 1, digits: 0 },
				{ key: "failsafeA", unit: "A", min: 0, max: 32, step: 1, digits: 0 },
			];
		},
		groups(): { id: string; fields: Field[] }[] {
			return [
				{
					id: "burst",
					fields: [
						{ key: "burstKva", unit: "kVA", min: 4.2, max: 9, step: 0.05, digits: 2 },
						{ key: "bumpKva", unit: "kVA", min: 1.5, max: 4, step: 0.1, digits: 1 },
						{ key: "marginS", unit: "s", min: 10, max: 120, step: 1, digits: 0 },
						{ key: "resetS", unit: "s", min: 0.5, max: 60, step: 0.5, digits: 1 },
						{ key: "baseMarginKva", unit: "kVA", min: 0, max: 0.5, step: 0.01, digits: 2 },
						{ key: "exitLeadFrac", unit: "0–1", min: 0, max: 1, step: 0.05, digits: 2 },
						{ key: "maxTempC", unit: "°C", min: 30, max: 90, step: 1, digits: 0 },
					],
				},
				{
					id: "control",
					fields: [
						{ key: "maxCloseness", unit: "0–1", min: 0.3, max: 0.95, step: 0.05, digits: 2 },
						{ key: "trimMaxA", unit: "A", min: 0, max: 4, step: 0.25, digits: 2 },
						{ key: "ampMin", unit: "A", min: 2, max: 16, step: 1, digits: 0 },
						{ key: "ampMax", unit: "A", min: 10, max: 32, step: 1, digits: 0 },
						{ key: "raiseTable", type: "text", unit: "A:s" },
						{ key: "reduceTable", type: "text", unit: "A:s" },
					],
				},
				{
					id: "thrift",
					fields: [
						{ key: "floorDwellS", unit: "s", min: 0, max: 60, step: 1, digits: 0 },
						{ key: "restartDwellS", unit: "s", min: 0, max: 300, step: 5, digits: 0 },
						{ key: "baseAbortCloseness", unit: "0–1", min: 0.1, max: 0.8, step: 0.05, digits: 2 },
						{ key: "baseStopCloseness", unit: "0–1", min: 0.2, max: 0.95, step: 0.05, digits: 2 },
						{ key: "baseTiAbortS", unit: "s", min: 5, max: 300, step: 5, digits: 0 },
						{ key: "baseTiStopS", unit: "s", min: 10, max: 600, step: 5, digits: 0 },
						{ key: "blindHoldPolls", unit: "", min: 1, max: 10, step: 1, digits: 0 },
						{ key: "blindPolls", unit: "", min: 2, max: 30, step: 1, digits: 0 },
						{ key: "blindAction", type: "select", unit: "", options: ["stop", "hold"] },
						{ key: "blindHoldA", unit: "A", min: 0, max: 32, step: 1, digits: 0 },
					],
				},
			];
		},
		tiles() {
			const s = this.state;
			return [
				{
					label: this.$t("loadManagement.settings.tiles.expected"),
					value: `${this.fmtNumber(s.expectedKva, 2)} kVA`,
				},
				{
					label: this.$t("loadManagement.settings.tiles.gain"),
					value: `${s.expectedGain > 0 ? "+" : ""}${this.fmtNumber(s.expectedGain, 0)} %`,
				},
				{
					label: this.$t("loadManagement.settings.tiles.window"),
					value: `${this.fmtNumber(s.burstWindowS, 0)} s`,
				},
				{
					label: this.$t("loadManagement.settings.tiles.budget"),
					value: this.fmtNumber(s.plannedCloseness, 2),
				},
			];
		},
	},
	methods: {
		settingValue(key: string): string | number {
			return (this.config.settings as unknown as Record<string, string | number>)[key];
		},
		lpConfig(name: string): LoadLpConfig {
			return (
				this.config.loadpoints?.[name] || {
					fast: false,
					measureTopic: "",
					measureUnit: "A",
					tempTopic: "",
				}
			);
		},
		saveSetting(key: string, value: string | number) {
			this.save({ settings: { [key]: value } as Partial<LoadSettings> }, key, value);
		},
		saveLp(name: string, patch: Partial<LoadLpConfig>) {
			const next = { ...this.lpConfig(name), ...patch };
			this.save({ loadpoints: { [name]: next } }, `lp-${name}`);
		},
		async save(patch: Record<string, unknown>, key: string, asked?: string | number) {
			try {
				const res = await api.post("supercharging/config", patch);
				const cfg = res.data as LoadConfig;
				const settings = cfg.settings as unknown as Record<string, string | number>;
				const got =
					key in settings ? settings[key] : (cfg as unknown as Record<string, string | number>)[key];
				let text = this.$t("loadManagement.settings.saved");
				if (typeof asked === "number" && typeof got === "number" && Math.abs(got - asked) > 1e-9) {
					text = this.$t("loadManagement.settings.clamped", { value: got });
				} else if (typeof asked === "string" && typeof got === "string" && asked.trim() !== got) {
					text = got
						? this.$t("loadManagement.settings.normalised", { value: got })
						: this.$t("loadManagement.settings.saved");
				}
				this.say(key, true, text);
			} catch (e: any) {
				this.say(key, false, e?.response?.data?.error || String(e));
			}
		},
		say(key: string, ok: boolean, text: string) {
			this.feedback = { ...this.feedback, [key]: { ok, text } };
			clearTimeout(this.timers[key]);
			this.timers[key] = setTimeout(() => {
				this.feedback = { ...this.feedback, [key]: undefined };
			}, 4000);
		},
	},
});
</script>

<style scoped>
.tile {
	border-radius: 0.5rem;
	background: var(--evcc-gray-10);
}
.tile-value {
	font-size: 1.25rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}
.unit-select {
	max-width: 5rem;
}
</style>
