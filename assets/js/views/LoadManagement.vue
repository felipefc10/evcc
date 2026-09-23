<template>
	<div class="container px-4 safe-area-inset">
		<TopHeader :title="$t('loadManagement.title')" />
		<div class="row">
			<main class="col-12">
				<p v-if="!lm" class="my-4 text-muted">{{ $t("loadManagement.loading") }}</p>
				<template v-else>
					<div class="toolbar mb-4">
						<div class="lm-tabs" role="tablist" data-testid="load-tabs">
							<button
								v-for="t in tabs"
								:key="t"
								type="button"
								class="lm-tab"
								:class="{ active: t === tab }"
								role="tab"
								:aria-selected="t === tab"
								@click="selectTab(t)"
							>
								{{ $t(`loadManagement.tabs.${t}`) }}
								<span v-if="t === 'diagnostics' && failedChecks" class="ms-1 badge text-bg-danger">
									{{ failedChecks }}
								</span>
							</button>
						</div>
						<span class="phase" :class="`phase--${phase}`" data-testid="load-phase">
							<span class="phase-dot"></span>{{ $t(`loadManagement.phase.${phase}`) }}
						</span>
					</div>

					<div
						v-if="lm.lastError && lm.running"
						class="alert alert-warning py-2 small"
						role="alert"
					>
						{{ lm.lastError }}
					</div>

					<template v-if="tab === 'overview'">
						<div class="row g-4 mb-4">
							<div class="col-12 col-lg-7">
								<section class="lm-box" :aria-label="$t('loadManagement.houseLoad')">
									<LoadLedger :state="lm" />
								</section>
							</div>
							<div class="col-12 col-lg-5">
								<LoadOrder :state="lm" @open-supercharge="openSupercharge" />
							</div>
						</div>

						<section class="lm-box figures mb-4" data-testid="load-figures">
							<div v-for="s in stats" :key="s.label" class="figure" :title="s.help">
								<div class="figure-value">{{ s.value }}</div>
								<div class="figure-label">{{ s.label }}</div>
								<div class="figure-note">{{ s.note }}</div>
							</div>
						</section>
					</template>

					<LoadSettings
						v-else-if="tab === 'settings' && config"
						:config="config"
						:state="lm"
						:locked="locked"
						@login="login"
					/>
					<LoadBursts v-else-if="tab === 'bursts'" class="mb-4" :bursts="lm.bursts" />
					<LoadDiagnostics v-else-if="tab === 'diagnostics'" class="mb-4" :state="lm" />
				</template>
			</main>
		</div>
		<SuperchargeModal ref="superchargeModal" />
	</div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import store from "@/store";
import auth, { openLoginModal } from "../components/Auth/auth";
import formatter from "@/mixins/formatter";
import Header from "../components/Top/Header.vue";
import LoadLedger from "../components/LoadManagement/LoadLedger.vue";
import LoadOrder from "../components/LoadManagement/LoadOrder.vue";
import LoadSettings from "../components/LoadManagement/LoadSettings.vue";
import LoadBursts from "../components/LoadManagement/LoadBursts.vue";
import LoadDiagnostics from "../components/LoadManagement/LoadDiagnostics.vue";
import SuperchargeModal from "../components/LoadManagement/SuperchargeModal.vue";
import type { LoadConfig, LoadState } from "@/types/supercharge";

const TABS = ["overview", "settings", "bursts", "diagnostics"] as const;
type Tab = (typeof TABS)[number];

export default defineComponent({
	name: "LoadManagement",
	components: {
		TopHeader: Header,
		LoadLedger,
		LoadOrder,
		LoadSettings,
		LoadBursts,
		LoadDiagnostics,
		SuperchargeModal,
	},
	mixins: [formatter],
	props: {
		tab: { type: String as () => Tab, default: "overview" },
	},
	head() {
		return { title: this.$t("loadManagement.title") };
	},
	computed: {
		tabs() {
			return TABS;
		},
		lm(): LoadState | undefined {
			return store.state.supercharging;
		},
		config(): LoadConfig | undefined {
			return store.state.superchargingConfig;
		},
		failedChecks(): number {
			return (this.lm?.checks || []).filter((c) => !c.ok).length;
		},
		// changing the installation needs the admin login, like the rest of the configuration
		locked(): boolean {
			return auth.loggedIn === false;
		},
		phase(): string {
			const s = this.lm!;
			if (!s.enabled) return "off";
			if (!s.running) return "standby";
			if (s.blind > 0) return "blind";
			return s.phase === "burst" ? "burst" : "base";
		},
		stats() {
			const s = this.lm!;
			const handedOut = (s.loadpoints || []).reduce((a, lp) => a + lp.setpointA, 0);
			return [
				{
					label: this.$t("loadManagement.stats.budget"),
					value: s.running ? `${this.fmtNumber(s.budgetA, 0)} A` : "—",
					note: s.running ? this.$t("loadManagement.stats.handedOut", { a: handedOut }) : "",
					help: this.$t("loadManagement.stats.budgetHelp"),
				},
				{
					label: this.$t("loadManagement.stats.trim"),
					value: s.running ? `${s.trimA > 0 ? "+" : ""}${this.fmtNumber(s.trimA, 2)} A` : "—",
					note: s.burstArmed ? this.$t("loadManagement.stats.trimAside") : "",
					help: this.$t("loadManagement.stats.trimHelp"),
				},
				{
					label: this.$t("loadManagement.stats.average"),
					value: s.avgKva > 0 ? `${this.fmtNumber(s.avgKva, 2)} kVA` : "—",
					note:
						s.avgKva > 0
							? this.$t("loadManagement.stats.vsLine", {
									pct: `${s.gainVsLine > 0 ? "+" : ""}${this.fmtNumber(s.gainVsLine, 0)}`,
								})
							: "",
					help: this.$t("loadManagement.stats.averageHelp"),
				},
				{
					label: this.$t("loadManagement.stats.bursts"),
					value: String(s.bursts || 0),
					note:
						s.bursts > 0
							? this.$t("loadManagement.stats.lastChanges", { n: s.burstCmds })
							: "",
					help: this.$t("loadManagement.stats.burstsHelp"),
				},
				{
					label: this.$t("loadManagement.stats.window"),
					value: `${this.fmtNumber(s.burstWindowS, 0)} s`,
					note: `${this.fmtNumber(s.burstKva, 2)} kVA`,
					help: this.$t("loadManagement.stats.windowHelp"),
				},
			];
		},
	},
	methods: {
		selectTab(t: Tab) {
			this.$router.replace({ query: t === "overview" ? {} : { tab: t } });
		},
		login() {
			openLoginModal(this.$route.fullPath);
		},
		openSupercharge(e: { index: number; title: string; active: boolean; until: string | null }) {
			(this.$refs["superchargeModal"] as InstanceType<typeof SuperchargeModal> | undefined)?.open(
				e.index,
				e.title,
				e.active,
				e.until
			);
		},
	},
});
</script>

<style scoped>
.toolbar {
	display: flex;
	align-items: center;
	justify-content: space-between;
	flex-wrap: wrap;
	gap: 1rem;
}
/* the same segmented control as a loadpoint's mode */
.lm-tabs {
	display: inline-flex;
	border: 2px solid var(--evcc-default-text);
	border-radius: 20px;
	padding: 4px;
	max-width: 100%;
	overflow-x: auto;
}
.lm-tab {
	border: none;
	background: none;
	white-space: nowrap;
	border-radius: 18px;
	padding: 0.25em 1em;
	color: var(--evcc-default-text);
	font-weight: 600;
}
.lm-tab:hover {
	color: var(--evcc-gray);
}
.lm-tab.active {
	color: var(--evcc-background);
	background: var(--evcc-default-text);
}
.lm-tab:focus-visible {
	outline: var(--bs-focus-ring-width) solid var(--bs-focus-ring-color);
}
@media (max-width: 575px) {
	.lm-tabs {
		display: flex;
		width: 100%;
	}
	.lm-tab {
		flex: 1 1 0;
		padding: 0.25em 0.4em;
		font-size: 0.875rem;
	}
}
.phase {
	display: inline-flex;
	align-items: center;
	gap: 0.5rem;
	padding: 0.3rem 0.9rem;
	border-radius: 999px;
	font-size: 0.85rem;
	font-weight: 700;
	color: var(--evcc-gray);
	background: var(--evcc-gray-15);
}
.phase-dot {
	width: 0.5rem;
	height: 0.5rem;
	border-radius: 50%;
	background: currentColor;
}
.phase--base {
	color: var(--evcc-darker-green);
	background: color-mix(in srgb, var(--evcc-darker-green) 14%, transparent);
}
html.dark .phase--base {
	color: var(--evcc-dark-green);
}
.phase--burst {
	color: var(--evcc-orange);
	background: color-mix(in srgb, var(--evcc-orange) 14%, transparent);
}
.phase--blind,
.phase--off {
	color: var(--evcc-red);
	background: color-mix(in srgb, var(--evcc-red) 12%, transparent);
}
.lm-box {
	background: var(--evcc-box);
	border-radius: 2rem;
	padding: 2rem;
}
@media (max-width: 575px) {
	.lm-box {
		padding: 1.25rem;
		border-radius: 1.5rem;
	}
}
.figures {
	display: grid;
	grid-template-columns: repeat(5, minmax(0, 1fr));
	padding-top: 1.5rem;
	padding-bottom: 1.5rem;
}
.figure {
	padding: 0.25rem 1.25rem;
	border-left: 1px solid var(--evcc-gray-25);
}
.figure:first-child {
	padding-left: 0;
	border-left: 0;
}
.figure-value {
	font-size: 1.5rem;
	font-weight: 800;
	font-variant-numeric: tabular-nums;
}
.figure-label {
	font-size: 0.85rem;
	font-weight: 600;
}
.figure-note {
	font-size: 0.75rem;
	color: var(--evcc-gray);
}
@media (max-width: 991px) {
	.figures {
		grid-template-columns: repeat(2, minmax(0, 1fr));
		padding-top: 0.5rem;
		padding-bottom: 0.5rem;
	}
	.figure,
	.figure:first-child {
		padding: 0.85rem 0 0.85rem 1rem;
		border-left: 1px solid var(--evcc-gray-25);
		border-top: 1px solid var(--evcc-gray-25);
	}
	.figure:nth-child(odd) {
		padding-left: 0;
		border-left: 0;
	}
	.figure:nth-child(-n + 2) {
		border-top: 0;
	}
}
</style>
