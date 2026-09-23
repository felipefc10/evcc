<template>
	<div class="settings-layout" data-testid="load-settings">
		<aside
			class="settings-index d-none d-lg-flex"
			:aria-label="$t('loadManagement.settings.onThisPage')"
		>
			<span class="index-title">{{ $t("loadManagement.settings.onThisPage") }}</span>
			<button
				v-for="sec in sections"
				:key="sec.id"
				type="button"
				class="index-link"
				@click="scrollTo(sec.id)"
			>
				{{ sec.title }}
			</button>
			<p class="index-note">{{ $t("loadManagement.settings.instant") }}</p>
		</aside>

		<div class="settings-main">
			<div v-if="locked" class="locked mb-4" role="status" data-testid="load-settings-locked">
				<svg
					width="20"
					height="20"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
				>
					<rect x="5" y="11" width="14" height="10" rx="2" />
					<path d="M8 11V7a4 4 0 0 1 8 0v4" />
				</svg>
				<span class="flex-grow-1">{{ $t("loadManagement.settings.locked") }}</span>
				<button
					type="button"
					class="btn btn-sm btn-light rounded-pill px-3 fw-bold"
					@click="$emit('login')"
				>
					{{ $t("loadManagement.settings.login") }}
				</button>
			</div>

			<fieldset :disabled="locked">
				<section :id="sectionId('installation')" class="lm-box mb-4">
					<h2 class="box-title">{{ $t("loadManagement.settings.general") }}</h2>
					<SettingRow
						id="lmEnabled"
						:label="$t('loadManagement.settings.enabled')"
						:help="$t('loadManagement.settings.enabledHelp')"
						:feedback="feedback['enabled']"
					>
						<div class="form-check form-switch m-0">
							<input
								id="lmEnabled"
								:checked="config.enabled"
								class="form-check-input switch"
								type="checkbox"
								role="switch"
								@change="onEnabled"
							/>
						</div>
					</SettingRow>
					<SettingRow
						id="lmMeterUri"
						wide
						:label="$t('loadManagement.settings.meterUri')"
						:help="$t('loadManagement.settings.meterUriHelp')"
						:feedback="feedback['meterUri']"
					>
						<input
							id="lmMeterUri"
							class="form-control"
							type="url"
							:value="config.meterUri"
							placeholder="http://192.168.1.10/emeter/0"
							@change="onMeterUri"
						/>
					</SettingRow>
					<NumberRow
						v-for="f in curveFields"
						:key="f.key"
						:field="f"
						:value="config[f.key as 'q' | 'k' | 'contractKva' | 'failsafeA']"
						:feedback="feedback[f.key]"
						@change="(v: number) => save({ [f.key]: v }, f.key, v)"
					/>
				</section>

				<section
					v-for="group in groups"
					:id="sectionId(group.id)"
					:key="group.id"
					class="lm-box mb-4"
				>
					<h2 class="box-title">
						{{ $t(`loadManagement.settings.groups.${group.id}.title`) }}
					</h2>
					<p class="box-subtitle">
						{{ $t(`loadManagement.settings.groups.${group.id}.subtitle`) }}
					</p>
					<div v-if="group.id === 'burst'" class="tiles mb-3" data-testid="load-tuning-tiles">
						<div v-for="t in tiles" :key="t.label" class="tile">
							<div class="tile-value">{{ t.value }}</div>
							<div class="tile-label">{{ t.label }}</div>
						</div>
					</div>
					<template v-for="f in group.fields" :key="f.key">
						<SettingRow
							v-if="f.type === 'select'"
							:id="`lm-${f.key}`"
							:label="$t(`loadManagement.settings.fields.${f.key}.label`)"
							:help="$t(`loadManagement.settings.fields.${f.key}.help`)"
							:feedback="feedback[f.key]"
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
						</SettingRow>
						<SettingRow
							v-else-if="f.type === 'text'"
							:id="`lm-${f.key}`"
							wide
							:label="$t(`loadManagement.settings.fields.${f.key}.label`)"
							:help="$t(`loadManagement.settings.fields.${f.key}.help`)"
							:feedback="feedback[f.key]"
						>
							<input
								:id="`lm-${f.key}`"
								class="form-control font-monospace"
								type="text"
								:value="settingValue(f.key)"
								@change="saveSetting(f.key, ($event.target as HTMLInputElement).value)"
							/>
						</SettingRow>
						<NumberRow
							v-else
							:field="f"
							:value="Number(settingValue(f.key))"
							:feedback="feedback[f.key]"
							@change="(v: number) => saveSetting(f.key, v)"
						/>
					</template>
				</section>

				<section :id="sectionId('chargers')" class="lm-box mb-4">
					<h2 class="box-title">{{ $t("loadManagement.settings.loadpoints") }}</h2>
					<p class="box-subtitle">{{ $t("loadManagement.settings.loadpointsHelp") }}</p>
					<div
						v-for="lp in loadpoints"
						:key="lp.name"
						class="charger"
						:data-testid="`load-lpconfig-${lp.index + 1}`"
					>
						<div class="charger-head">
							<h3 class="charger-title">{{ lp.title }}</h3>
							<div class="form-check form-switch m-0">
								<input
									:id="`lm-fast-${lp.index}`"
									:checked="lpConfig(lp.name).fast"
									class="form-check-input"
									type="checkbox"
									role="switch"
									@change="
										saveLp(lp.name, {
											fast: ($event.target as HTMLInputElement).checked,
										})
									"
								/>
								<label
									class="form-check-label fw-bold"
									:for="`lm-fast-${lp.index}`"
									:title="$t('loadManagement.settings.fastHelp')"
								>
									{{ $t("loadManagement.settings.fast") }}
								</label>
							</div>
						</div>
						<div class="feeds">
							<div>
								<label
									class="feed-label"
									:for="`lm-measure-${lp.index}`"
									:title="$t('loadManagement.settings.measureTopicHelp')"
								>
									{{ $t("loadManagement.settings.measureTopic") }}
								</label>
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
							</div>
							<div>
								<label
									class="feed-label"
									:for="`lm-max-${lp.index}`"
									:title="$t('loadManagement.settings.maxTopicHelp')"
								>
									{{ $t("loadManagement.settings.maxTopic") }}
								</label>
								<input
									:id="`lm-max-${lp.index}`"
									class="form-control font-monospace"
									type="text"
									:value="lpConfig(lp.name).maxTopic"
									@change="
										saveLp(lp.name, {
											maxTopic: ($event.target as HTMLInputElement).value,
										})
									"
								/>
							</div>
							<div>
								<label
									class="feed-label"
									:for="`lm-temp-${lp.index}`"
									:title="$t('loadManagement.settings.tempTopicHelp')"
								>
									{{ $t("loadManagement.settings.tempTopic") }}
								</label>
								<input
									:id="`lm-temp-${lp.index}`"
									class="form-control font-monospace"
									type="text"
									:value="lpConfig(lp.name).tempTopic"
									@change="
										saveLp(lp.name, {
											tempTopic: ($event.target as HTMLInputElement).value,
										})
									"
								/>
							</div>
						</div>
						<Feedback :msg="feedback[`lp-${lp.name}`]" />
					</div>
				</section>
			</fieldset>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent, h, type PropType } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import SettingRow from "./SettingRow.vue";
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
						{ class: ["small", "mt-2", props.msg.ok ? "text-primary" : "text-danger"] },
						props.msg.text
					)
				: null;
	},
});

// Every setting is applied the moment it is changed: the control loop reads it on its next second.
export default defineComponent({
	name: "LoadSettings",
	components: { SettingRow, NumberRow, Feedback },
	mixins: [formatter],
	props: {
		config: { type: Object as PropType<LoadConfig>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
		locked: Boolean,
	},
	emits: ["login"],
	data() {
		return {
			feedback: {} as Record<string, { ok: boolean; text: string } | undefined>,
			timers: {} as Record<string, ReturnType<typeof setTimeout>>,
		};
	},
	computed: {
		sections(): { id: string; title: string }[] {
			return [
				{ id: "installation", title: this.$t("loadManagement.settings.general") },
				...this.groups.map((g) => ({
					id: g.id,
					title: this.$t(`loadManagement.settings.groups.${g.id}.title`),
				})),
				{ id: "chargers", title: this.$t("loadManagement.settings.loadpoints") },
			];
		},
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
		sectionId(id: string): string {
			return `lm-section-${id}`;
		},
		// the app routes by hash, so the index scrolls instead of linking
		scrollTo(id: string) {
			document
				.getElementById(this.sectionId(id))
				?.scrollIntoView({ behavior: "smooth", block: "start" });
		},
		onEnabled(e: Event) {
			this.save({ enabled: (e.target as HTMLInputElement).checked }, "enabled");
		},
		onMeterUri(e: Event) {
			this.save({ meterUri: (e.target as HTMLInputElement).value }, "meterUri");
		},
		settingValue(key: string): string | number {
			return (this.config.settings as unknown as Record<string, string | number>)[key]!;
		},
		lpConfig(name: string): LoadLpConfig {
			return (
				this.config.loadpoints?.[name] || {
					fast: false,
					measureTopic: "",
					measureUnit: "A",
					tempTopic: "",
					maxTopic: "",
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
					key in settings
						? settings[key]
						: (cfg as unknown as Record<string, string | number>)[key];
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
.settings-layout {
	display: grid;
	grid-template-columns: minmax(0, 1fr);
	gap: 2.5rem;
}
@media (min-width: 992px) {
	.settings-layout {
		grid-template-columns: 13rem minmax(0, 1fr);
	}
}
.settings-index {
	flex-direction: column;
	gap: 0.25rem;
	position: sticky;
	top: 1rem;
	align-self: start;
}
.index-title {
	font-size: 0.7rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: var(--evcc-gray);
	padding: 0 0.75rem 0.5rem;
}
.index-link {
	border: 0;
	background: none;
	text-align: start;
	padding: 0.6rem 0.75rem;
	border-radius: 10px;
	font-weight: 600;
	color: var(--evcc-gray);
}
.index-link:hover {
	background: var(--evcc-box);
	color: var(--evcc-default-text);
}
.index-note {
	margin: 1rem 0.75rem 0;
	font-size: 0.75rem;
	color: var(--evcc-gray);
}
fieldset {
	min-width: 0;
}
.locked {
	display: flex;
	align-items: center;
	gap: 0.9rem;
	padding: 1rem 1.25rem;
	border-radius: 1rem;
	color: var(--evcc-orange);
	background: color-mix(in srgb, var(--evcc-orange) 10%, transparent);
	border: 1px solid color-mix(in srgb, var(--evcc-orange) 35%, transparent);
}
.locked span {
	color: var(--evcc-default-text);
}
.lm-box {
	background: var(--evcc-box);
	border-radius: 2rem;
	padding: 1.75rem 2rem 1rem;
	scroll-margin-top: 1rem;
}
@media (max-width: 575px) {
	.lm-box {
		padding: 1.25rem 1.25rem 0.5rem;
		border-radius: 1.5rem;
	}
}
.box-title {
	font-size: 1.25rem;
	font-weight: 700;
	margin: 0 0 0.25rem;
}
.box-subtitle {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin-bottom: 1rem;
}
.switch {
	width: 2.75rem;
	height: 1.5rem;
}
.tiles {
	display: grid;
	grid-template-columns: repeat(4, minmax(0, 1fr));
	gap: 0.75rem;
}
@media (max-width: 767px) {
	.tiles {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}
}
.tile {
	padding: 0.9rem 1rem;
	border-radius: 1rem;
	background: var(--evcc-gray-10);
}
.tile-value {
	font-size: 1.35rem;
	font-weight: 800;
	font-variant-numeric: tabular-nums;
}
.tile-label {
	font-size: 0.75rem;
	color: var(--evcc-gray);
}
.charger {
	border: 1px solid var(--evcc-gray-25);
	border-radius: 1.25rem;
	padding: 1.1rem 1.25rem;
	margin-bottom: 1rem;
}
.charger-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	margin-bottom: 0.9rem;
}
.charger-title {
	font-size: 1rem;
	font-weight: 700;
	margin: 0;
}
.feeds {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 1rem;
}
@media (max-width: 991px) {
	.feeds {
		grid-template-columns: minmax(0, 1fr);
	}
}
.feed-label {
	display: block;
	font-size: 0.7rem;
	font-weight: 700;
	letter-spacing: 0.06em;
	text-transform: uppercase;
	color: var(--evcc-gray);
	margin-bottom: 0.35rem;
}
.unit-select {
	max-width: 4.5rem;
}
</style>
