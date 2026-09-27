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
				:class="{ active: sec.id === current }"
				:aria-current="sec.id === current ? 'true' : undefined"
				@click="scrollTo(sec.id)"
			>
				{{ sec.title }}
			</button>
			<p class="index-note">{{ $t("loadManagement.settings.instant") }}</p>
		</aside>

		<div class="settings-main">
			<p class="instant-note d-lg-none">{{ $t("loadManagement.settings.instant") }}</p>
			<nav
				class="index-chips d-lg-none"
				:class="{ 'index-chips--end': chipsEnd }"
				@scroll.passive="chipsScrolled"
				:aria-label="$t('loadManagement.settings.onThisPage')"
			>
				<button
					v-for="sec in sections"
					:id="`lm-chip-${sec.id}`"
					:key="sec.id"
					type="button"
					class="btn btn-pill index-chip"
					:class="{ active: sec.id === current }"
					:aria-current="sec.id === current ? 'true' : undefined"
					@click="scrollTo(sec.id)"
				>
					{{ sec.title }}
				</button>
			</nav>
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
						inline
						:label="$t('loadManagement.settings.enabled')"
						:help="$t('loadManagement.settings.enabledHelp')"
						:feedback="feedback['enabled']"
					>
						<div class="form-check form-switch m-0">
							<input
								id="lmEnabled"
								:checked="config.enabled"
								class="form-check-input switch"
								:class="{ 'switch--pending': pendingOff }"
								type="checkbox"
								role="switch"
								@change="onEnabled"
							/>
						</div>
					</SettingRow>
					<div
						v-if="pendingOff"
						class="line-confirm mb-3"
						role="alert"
						data-testid="load-off-confirm"
					>
						<span class="confirm-text">{{
							$t("loadManagement.settings.offConfirm", { a: config.failsafeA })
						}}</span>
						<span class="confirm-actions">
							<button type="button" class="btn btn-outline-secondary" @click="keepOn">
								{{ $t("loadManagement.settings.offKeep") }}
							</button>
							<button type="button" class="btn btn-danger" @click="turnOff">
								{{ $t("loadManagement.settings.offApply") }}
							</button>
						</span>
					</div>
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
							:placeholder="$t('loadManagement.settings.meterUriPlaceholder')"
							@change="onMeterUri"
						/>
					</SettingRow>
					<template v-for="f in lineFields" :key="f.key">
						<NumberRow
							:ref="`row-${f.key}`"
							:field="f"
							:value="curveValue(f.key)"
							:feedback="feedback[f.key]"
							@change="(v: number) => askLine(f.key, v)"
						/>
						<div
							v-if="pendingLine && pendingLine.key === f.key"
							class="line-confirm mb-3"
							role="alert"
							data-testid="load-line-confirm"
						>
							<span class="confirm-text">{{ pendingLineText }}</span>
							<span class="confirm-actions">
								<button
									type="button"
									class="btn btn-outline-secondary"
									@click="undoLine"
								>
									{{ $t("loadManagement.settings.lineUndo", { kva: lineText }) }}
								</button>
								<button type="button" class="btn btn-warning" @click="applyLine">
									{{
										$t("loadManagement.settings.lineApply", {
											kva: fmtNumber(pendingLine.line, 2),
										})
									}}
								</button>
							</span>
						</div>
					</template>
					<SettingRow
						id="lmLine"
						:label="$t('loadManagement.settings.line')"
						:help="$t('loadManagement.settings.lineHelp')"
						:feedback="feedback['line']"
					>
						<div class="input-group">
							<input
								id="lmLine"
								class="form-control text-end fw-bold"
								type="text"
								readonly
								:value="lineText"
								data-testid="load-line-value"
							/>
							<span class="input-group-text unit">kVA</span>
						</div>
					</SettingRow>
					<NumberRow
						v-for="f in otherCurveFields"
						:key="f.key"
						:field="f"
						:value="curveValue(f.key)"
						:feedback="feedback[f.key]"
						@change="(v: number) => save({ [f.key]: v }, f.key, v)"
					/>
				</section>

				<section :id="sectionId('chargers')" class="lm-box mb-4">
					<h2 class="box-title">{{ $t("loadManagement.settings.loadpoints") }}</h2>
					<p class="box-subtitle">
						{{ $t("loadManagement.settings.loadpointsHelp") }}
						<router-link :to="{ query: {} }">{{
							$t("loadManagement.settings.orderLink")
						}}</router-link>
					</p>
					<p class="form-text mt-0 mb-3">
						<strong>{{ $t("loadManagement.settings.fast") }}</strong
						>{{ ": " }}{{ $t("loadManagement.settings.fastHelp") }}
					</p>
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
									:aria-label="
										$t('loadManagement.settings.fastOf', { name: lp.title })
									"
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
								>
									{{ $t("loadManagement.settings.fast") }}
								</label>
							</div>
						</div>
						<div class="feeds">
							<div>
								<label class="feed-label" :for="`lm-measure-${lp.index}`">
									{{ $t("loadManagement.settings.measureTopic") }}
								</label>
								<div class="input-group">
									<input
										:id="`lm-measure-${lp.index}`"
										:placeholder="$t('loadManagement.settings.notSet')"
										class="form-control font-monospace"
										type="text"
										:value="lpConfig(lp.name).measureTopic"
										@change="
											saveLp(lp.name, {
												measureTopic: ($event.target as HTMLInputElement)
													.value,
											})
										"
									/>
									<select
										class="form-select unit-select"
										:aria-label="$t('loadManagement.settings.measureUnit')"
										:value="lpConfig(lp.name).measureUnit"
										@change="
											saveLp(lp.name, {
												measureUnit: ($event.target as HTMLSelectElement)
													.value as 'A' | 'W',
											})
										"
									>
										<option value="A">A</option>
										<option value="W">W</option>
									</select>
								</div>
								<div class="form-text">
									{{ $t("loadManagement.settings.measureTopicShort") }}
								</div>
								<div
									v-if="feedNote(lp)"
									class="feed-note"
									:class="feedNoteClass(lp)"
								>
									{{ feedNote(lp) }}
								</div>
							</div>
							<div>
								<label class="feed-label" :for="`lm-max-${lp.index}`">
									{{ $t("loadManagement.settings.maxTopic") }}
								</label>
								<input
									:id="`lm-max-${lp.index}`"
									:placeholder="$t('loadManagement.settings.notSet')"
									class="form-control font-monospace"
									type="text"
									:value="lpConfig(lp.name).maxTopic"
									@change="
										saveLp(lp.name, {
											maxTopic: ($event.target as HTMLInputElement).value,
										})
									"
								/>
								<div class="form-text">
									{{ $t("loadManagement.settings.maxTopicShort") }}
								</div>
							</div>
							<div>
								<label class="feed-label" :for="`lm-temp-${lp.index}`">
									{{ $t("loadManagement.settings.tempTopic") }}
								</label>
								<input
									:id="`lm-temp-${lp.index}`"
									:placeholder="$t('loadManagement.settings.notSet')"
									class="form-control font-monospace"
									type="text"
									:value="lpConfig(lp.name).tempTopic"
									@change="
										saveLp(lp.name, {
											tempTopic: ($event.target as HTMLInputElement).value,
										})
									"
								/>
								<div class="form-text">
									{{ $t("loadManagement.settings.tempTopicShort") }}
								</div>
							</div>
						</div>
						<Feedback :msg="feedback[`lp-${lp.name}`]" />
					</div>
				</section>

				<section
					v-for="group in groups"
					:id="sectionId(group.id)"
					:key="group.id"
					class="lm-box mb-4"
				>
					<h2 class="box-heading">
						<button
							type="button"
							class="box-toggle"
							:aria-expanded="!!open[group.id]"
							:aria-controls="`${sectionId(group.id)}-body`"
							:data-testid="`load-toggle-${group.id}`"
							@click="toggle(group.id)"
						>
							<span>
								<span class="box-title">
									{{ $t(`loadManagement.settings.groups.${group.id}.title`) }}
								</span>
								<span class="box-subtitle">
									{{ $t(`loadManagement.settings.groups.${group.id}.subtitle`) }}
								</span>
							</span>
							<svg
								class="chevron"
								:class="{ 'chevron--open': open[group.id] }"
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2.4"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path d="m6 9 6 6 6-6" />
							</svg>
						</button>
					</h2>
					<div v-show="open[group.id]" :id="`${sectionId(group.id)}-body`">
						<div
							v-if="group.id === 'burst'"
							class="tiles mb-3"
							data-testid="load-tuning-tiles"
						>
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
										saveSetting(
											f.key,
											($event.target as HTMLSelectElement).value
										)
									"
								>
									<option v-for="o in f.options" :key="o" :value="o">
										{{
											$t(
												`loadManagement.settings.fields.${f.key}.options.${o}`
											)
										}}
									</option>
								</select>
							</SettingRow>
							<PacingRow
								v-else-if="f.type === 'pacing'"
								:field="f.key"
								:value="String(settingValue(f.key))"
								:feedback="feedback[f.key]"
								@change="(v: string) => saveSetting(f.key, v)"
							/>
							<NumberRow
								v-else
								:field="f"
								:disabled="
									f.key === 'blindHoldA' && settingValue('blindAction') !== 'hold'
								"
								:value="Number(settingValue(f.key))"
								:feedback="feedback[f.key]"
								@change="(v: number) => saveSetting(f.key, v)"
							/>
						</template>
					</div>
				</section>

				<section :id="sectionId('import')" class="lm-box mb-4">
					<h2 class="box-heading">
						<button
							type="button"
							class="box-toggle"
							:aria-expanded="!!open['import']"
							:aria-controls="`${sectionId('import')}-body`"
							data-testid="load-toggle-import"
							@click="toggle('import')"
						>
							<span>
								<span class="box-title">{{
									$t("loadManagement.backup.title")
								}}</span>
								<span class="box-subtitle">{{
									$t("loadManagement.backup.subtitle")
								}}</span>
							</span>
							<svg
								class="chevron"
								:class="{ 'chevron--open': open['import'] }"
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2.4"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path d="m6 9 6 6 6-6" />
							</svg>
						</button>
					</h2>
					<div v-show="open['import']" :id="`${sectionId('import')}-body`" class="pb-4">
						<LoadBackup />
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
import LoadBackup from "./LoadBackup.vue";
import PacingRow from "./PacingRow.vue";
import type {
	LoadConfig,
	LoadLoadpoint,
	LoadLpConfig,
	LoadSettings,
	LoadState,
} from "@/types/supercharge";

interface Field extends NumberField {
	type?: "select" | "pacing";
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
	components: { SettingRow, NumberRow, Feedback, LoadBackup, PacingRow },
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
			// tuning sections start folded: the installation and the chargers are what most visits need
			open: {} as Record<string, boolean>,
			// a contract or factor change waits for a confirm: it moves the never-trip line
			pendingLine: null as { key: "contractKva" | "k"; value: number; line: number } | null,
			pendingOff: false,
			current: "installation",
			// the fade hints at more chips; at the end it would only hide the last one
			chipsEnd: false,
			spy: null as IntersectionObserver | null,
		};
	},
	computed: {
		sections(): { id: string; title: string }[] {
			return [
				{ id: "installation", title: this.$t("loadManagement.settings.general") },
				{ id: "chargers", title: this.$t("loadManagement.settings.loadpoints") },
				...this.groups.map((g) => ({
					id: g.id,
					title: this.$t(`loadManagement.settings.groups.${g.id}.title`),
				})),
				{ id: "import", title: this.$t("loadManagement.backup.title") },
			];
		},
		loadpoints(): LoadLoadpoint[] {
			return this.state.loadpoints || [];
		},
		lineFields(): Field[] {
			return [
				{ key: "contractKva", unit: "kVA", min: 1, max: 20, step: 0.05, digits: 2 },
				{ key: "k", unit: "×", min: 1, max: 2, step: 0.01, digits: 2, def: 1.2 },
			];
		},
		otherCurveFields(): Field[] {
			return [
				{ key: "q", unit: "s", min: 1, max: 200, step: 1, digits: 0, def: 50 },
				{ key: "failsafeA", unit: "A", min: 0, max: 32, step: 1, digits: 0 },
			];
		},
		lineText(): string {
			return this.fmtNumber(this.config.contractKva * this.config.k, 2);
		},
		pendingLineText(): string {
			if (!this.pendingLine) return "";
			return this.$t("loadManagement.settings.lineConfirm", {
				kva: this.fmtNumber(this.pendingLine.line, 2),
				now: this.lineText,
			});
		},
		groups(): { id: string; fields: Field[] }[] {
			return [
				{
					id: "burst",
					fields: [
						{
							key: "burstKva",
							unit: "kVA",
							min: 4.2,
							max: 9,
							step: 0.05,
							digits: 2,
							def: 6.5,
						},
						{
							key: "bumpKva",
							unit: "kVA",
							min: 1.5,
							max: 4,
							step: 0.1,
							digits: 1,
							def: 2,
						},
						{
							key: "marginS",
							unit: "s",
							min: 10,
							max: 120,
							step: 1,
							digits: 0,
							def: 30,
						},
						{
							key: "resetS",
							unit: "s",
							min: 0.5,
							max: 60,
							step: 0.5,
							digits: 1,
							def: 10,
						},
						{
							key: "baseMarginKva",
							unit: "kVA",
							min: 0,
							max: 0.5,
							step: 0.01,
							digits: 2,
							def: 0.1,
						},
						{
							key: "exitLeadFrac",
							unit: "%",
							min: 0,
							max: 1,
							step: 0.05,
							digits: 2,
							scale: 100,
							def: 0.5,
						},
						{
							key: "maxTempC",
							unit: "°C",
							min: 30,
							max: 90,
							step: 1,
							digits: 0,
							def: 55,
						},
					],
				},
				{
					id: "control",
					fields: [
						{
							key: "trimMaxA",
							unit: "A",
							min: 0,
							max: 4,
							step: 0.25,
							digits: 2,
							def: 2,
						},
						{ key: "ampMin", unit: "A", min: 2, max: 16, step: 1, digits: 0, def: 6 },
						{ key: "ampMax", unit: "A", min: 10, max: 32, step: 1, digits: 0, def: 32 },
						{ key: "raiseTable", type: "pacing", unit: "" },
						{ key: "reduceTable", type: "pacing", unit: "" },
					],
				},
				{
					id: "thrift",
					fields: [
						{
							key: "floorDwellS",
							unit: "s",
							min: 0,
							max: 60,
							step: 1,
							digits: 0,
							def: 6,
						},
						{
							key: "restartDwellS",
							unit: "s",
							min: 0,
							max: 300,
							step: 5,
							digits: 0,
							def: 30,
						},
						{
							key: "maxCloseness",
							unit: "%",
							min: 0.3,
							max: 0.95,
							step: 0.05,
							digits: 2,
							scale: 100,
							def: 0.7,
						},
						{
							key: "baseAbortCloseness",
							unit: "%",
							min: 0.1,
							max: 0.8,
							step: 0.05,
							digits: 2,
							scale: 100,
							def: 0.4,
						},
						{
							key: "baseStopCloseness",
							unit: "%",
							min: 0.2,
							max: 0.95,
							step: 0.05,
							digits: 2,
							scale: 100,
							def: 0.75,
						},
						{
							key: "baseTiAbortS",
							unit: "s",
							min: 5,
							max: 300,
							step: 5,
							digits: 0,
							def: 30,
						},
						{
							key: "baseTiStopS",
							unit: "s",
							min: 10,
							max: 600,
							step: 5,
							digits: 0,
							def: 90,
						},
						{
							key: "blindHoldPolls",
							unit: this.$t("loadManagement.settings.unitReadings"),
							min: 1,
							max: 10,
							step: 1,
							digits: 0,
							def: 2,
						},
						{
							key: "blindPolls",
							unit: this.$t("loadManagement.settings.unitReadings"),
							min: 2,
							max: 30,
							step: 1,
							digits: 0,
							def: 8,
						},
						{ key: "blindAction", type: "select", unit: "", options: ["stop", "hold"] },
						{
							key: "blindHoldA",
							unit: "A",
							min: 0,
							max: 32,
							step: 1,
							digits: 0,
							def: 10,
						},
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
					value: `${this.fmtNumber(s.plannedCloseness * 100, 0)} %`,
				},
			];
		},
	},
	watch: {
		// the chip row follows the section on screen
		current(id: string) {
			document
				.getElementById(`lm-chip-${id}`)
				?.scrollIntoView({ block: "nearest", inline: "nearest" });
		},
	},
	mounted() {
		// the index follows the section at the top of the screen
		this.spy = new IntersectionObserver(
			(entries) => {
				const top = entries
					.filter((e) => e.isIntersecting)
					.sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
				if (top) this.current = top.target.id.replace("lm-section-", "");
			},
			{ rootMargin: "0px 0px -70% 0px" }
		);
		for (const sec of this.sections) {
			const el = document.getElementById(this.sectionId(sec.id));
			if (el) this.spy.observe(el);
		}
		window.addEventListener("scroll", this.atEnd, { passive: true });
	},
	beforeUnmount() {
		this.spy?.disconnect();
		window.removeEventListener("scroll", this.atEnd);
	},
	methods: {
		chipsScrolled(e: Event) {
			const el = e.target as HTMLElement;
			this.chipsEnd = el.scrollLeft + el.clientWidth >= el.scrollWidth - 2;
		},
		atEnd() {
			const doc = document.documentElement;
			if (window.innerHeight + window.scrollY >= doc.scrollHeight - 4) {
				this.current = this.sections[this.sections.length - 1]!.id;
			}
		},
		sectionId(id: string): string {
			return `lm-section-${id}`;
		},
		// the app routes by hash, so the index scrolls instead of linking
		toggle(id: string) {
			this.open = { ...this.open, [id]: !this.open[id] };
		},
		scrollTo(id: string) {
			this.current = id;
			if (id in this.open || this.groups.some((g) => g.id === id) || id === "import") {
				this.open = { ...this.open, [id]: true };
			}
			this.$nextTick(() =>
				document
					.getElementById(this.sectionId(id))
					?.scrollIntoView({ behavior: "smooth", block: "start" })
			);
		},
		curveValue(key: string): number {
			return (this.config as unknown as Record<string, number>)[key]!;
		},
		askLine(key: string, value: number) {
			const k = key as "contractKva" | "k";
			const contract = k === "contractKva" ? value : this.config.contractKva;
			const factor = k === "k" ? value : this.config.k;
			this.pendingLine = { key: k, value, line: contract * factor };
		},
		applyLine() {
			const p = this.pendingLine;
			if (!p) return;
			this.pendingLine = null;
			this.save({ [p.key]: p.value }, p.key, p.value);
		},
		undoLine() {
			const p = this.pendingLine;
			this.pendingLine = null;
			if (!p) return;
			const row = this.$refs[`row-${p.key}`] as { reset: () => void }[] | undefined;
			row?.[0]?.reset();
			this.$nextTick(() => document.getElementById(`lm-${p.key}`)?.focus());
		},
		feedNote(lp: LoadLoadpoint): string {
			if (!this.lpConfig(lp.name).measureTopic) return "";
			if (lp.feedAgeS == null) return this.$t("loadManagement.settings.feedNone");
			return this.$t("loadManagement.settings.feedAge", {
				s: this.fmtNumber(lp.feedAgeS, 0),
			});
		},
		feedNoteClass(lp: LoadLoadpoint): string {
			return lp.feedAgeS == null || lp.feedAgeS > 30 ? "warn-text" : "text-primary";
		},
		onEnabled(e: Event) {
			const el = e.target as HTMLInputElement;
			if (el.checked) {
				this.pendingOff = false;
				this.save({ enabled: true }, "enabled");
				return;
			}
			el.checked = true;
			this.pendingOff = true;
		},
		keepOn() {
			this.pendingOff = false;
			this.$nextTick(() => document.getElementById("lmEnabled")?.focus());
		},
		turnOff() {
			this.pendingOff = false;
			this.save({ enabled: false }, "enabled");
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
				if (
					typeof asked === "number" &&
					typeof got === "number" &&
					Math.abs(got - asked) > 1e-9
				) {
					const scale =
						this.groups.flatMap((g) => g.fields).find((f) => f.key === key)?.scale || 1;
					text = this.$t("loadManagement.settings.clamped", {
						value: Number((got * scale).toFixed(2)),
					});
				} else if (
					typeof asked === "string" &&
					typeof got === "string" &&
					asked.trim() !== got
				) {
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
.index-link.active {
	background: var(--evcc-box);
	color: var(--evcc-default-text);
}
.index-link:hover {
	color: var(--evcc-default-text);
	text-decoration: underline;
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
	border: 1px solid var(--bs-border-color-translucent);
	border-radius: 1rem;
	padding: 1.5rem 1.5rem 0.75rem;
	scroll-margin-top: 1rem;
}
/* the sticky chip row covers the top of a section it scrolls to */
@media (max-width: 991px) {
	.lm-box {
		scroll-margin-top: 4.5rem;
	}
}
.box-title + .setting-row {
	border-top: 0;
}
@media (max-width: 575px) {
	.lm-box {
		padding: 1.25rem 1.25rem 0.5rem;
		border-radius: 1rem;
	}
}
.box-title {
	font-size: 1.25rem;
	font-weight: 700;
	margin: 0 0 0.25rem;
}
.box-subtitle a {
	color: inherit;
	text-decoration: underline;
}
.box-subtitle {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin-bottom: 1rem;
}
.box-heading {
	margin: 0;
}
.box-heading .box-title {
	display: block;
}
.box-heading .box-subtitle {
	display: block;
	text-transform: none;
	letter-spacing: normal;
	font-weight: normal;
}
.box-toggle {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	width: 100%;
	border: 0;
	background: none;
	color: inherit;
	text-align: start;
	text-transform: inherit;
	padding: 0 0 0.75rem;
}
.box-toggle .box-subtitle {
	margin-bottom: 0;
}
.box-toggle:focus-visible {
	outline: var(--bs-focus-ring-width) solid var(--bs-focus-ring-color);
	border-radius: 8px;
}
.chevron {
	flex-shrink: 0;
	color: var(--evcc-gray);
	transition: transform var(--evcc-transition-fast);
}
.chevron--open {
	transform: rotate(180deg);
}
@media (prefers-reduced-motion: reduce) {
	.chevron {
		transition: none;
	}
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
	line-height: 1.3;
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
	flex-wrap: nowrap;
	align-items: center;
	justify-content: space-between;
	gap: 0.5rem 1rem;
	margin-bottom: 0.9rem;
}
.charger-title {
	font-size: 1rem;
	font-weight: 700;
	margin: 0;
}
.feeds {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(17rem, 1fr));
	gap: 1rem;
}
@media (max-width: 991px) {
	.feeds {
		grid-template-columns: minmax(0, 1fr);
	}
}
.feed-label {
	display: block;
	font-weight: 700;
	margin-bottom: 0.35rem;
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
.instant-note {
	font-size: 0.875rem;
	color: var(--evcc-gray);
	margin-bottom: 0.75rem;
}
/* one row that scrolls sideways and stays at the top while the page scrolls */
.index-chips {
	position: sticky;
	top: 0;
	z-index: 20;
	padding: 0.5rem 0;
	background: var(--evcc-background);
	display: flex;
	flex-wrap: nowrap;
	overflow-x: auto;
	scrollbar-width: none;
	gap: 0.5rem;
	margin-bottom: 1.5rem;
}
.index-chip {
	flex-shrink: 0;
}
/* the fade sits over the chips only, so rows scrolling under the bar never show through */
.index-chips::after {
	content: "";
	position: sticky;
	right: 0;
	flex: 0 0 2.5rem;
	margin-left: -2.5rem;
	background: linear-gradient(to right, transparent, var(--evcc-background));
	pointer-events: none;
}
.index-chips--end::after {
	visibility: hidden;
}
.index-chip.active {
	background: var(--evcc-default-text);
	border-color: var(--evcc-default-text);
	color: var(--evcc-background);
}
.topics-help {
	font-size: 0.85rem;
	margin: 0 0 1rem;
	display: grid;
	gap: 0.4rem;
}
.topics-help dt {
	display: inline;
	font-weight: 700;
}
.topics-help dt::after {
	content: ": ";
}
.topics-help dd {
	display: inline;
	margin: 0;
	color: var(--evcc-gray);
	overflow-wrap: anywhere;
}
.feeds :deep(input) {
	overflow-wrap: anywhere;
}
.feed-help {
	font-size: 0.8125rem;
	color: var(--evcc-gray);
	margin-top: 0.3rem;
	overflow-wrap: anywhere;
}
.switch--pending {
	opacity: 0.5;
}
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
.unit {
	min-width: 3.5rem;
	justify-content: center;
}
.feeds :deep(input::placeholder) {
	font-family: var(--bs-body-font-family);
	opacity: 0.55;
}
.fast-help {
	font-size: 0.8rem;
	color: var(--evcc-gray);
	margin: -0.4rem 0 0.9rem;
}
.feed-note {
	font-size: 0.75rem;
	margin-top: 0.3rem;
}
.unit-select {
	max-width: 4.5rem;
}
.warn-text {
	color: #9a5200;
}
html.dark .warn-text {
	color: var(--evcc-orange);
}
</style>
