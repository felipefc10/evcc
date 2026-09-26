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
								:ref="`tab-${t}`"
								class="lm-tab"
								:class="{ active: t === tab }"
								role="tab"
								:aria-selected="t === tab"
								:tabindex="t === tab ? 0 : -1"
								@click="selectTab(t)"
								@keydown.left.prevent="stepTab(-1)"
								@keydown.right.prevent="stepTab(1)"
							>
								{{ $t(`loadManagement.tabs.${t}`) }}
								<span
									v-if="t === 'diagnostics' && failedChecks"
									class="ms-1 badge text-bg-danger"
								>
									{{ failedChecks }}
								</span>
							</button>
						</div>
						<span
							v-if="!alert"
							class="phase"
							:class="`phase--${phase}`"
							role="status"
							data-testid="load-phase"
						>
							<span class="phase-dot"></span>{{ $t(`loadManagement.phase.${phase}`) }}
						</span>
					</div>

					<div
						v-if="alert"
						class="lm-alert mb-4"
						:class="`lm-alert--${alert.kind}`"
						role="alert"
						data-testid="load-alert"
					>
						<svg
							width="20"
							height="20"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2.2"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"
							class="flex-shrink-0"
						>
							<path d="M12 3 2 21h20L12 3z" />
							<path d="M12 10v5M12 18h.01" />
						</svg>
						<div class="flex-grow-1 min-w-0">
							<strong class="d-block">{{ alert.title }}</strong>
							<span class="small">{{ alert.text }}</span>
						</div>
						<button
							v-if="alert.tab && alert.tab !== tab"
							type="button"
							class="btn btn-sm btn-outline-secondary flex-shrink-0"
							@click="selectTab(alert.tab)"
						>
							{{ alert.action }}
						</button>
					</div>

					<template v-if="tab === 'overview'">
						<div class="row g-4 mb-4">
							<div class="col-12 col-lg-7">
								<section
									class="lm-box"
									:aria-label="$t('loadManagement.houseLoad')"
								>
									<LoadLedger :state="lm" />
								</section>
							</div>
							<div class="col-12 col-lg-5">
								<LoadOrder :state="lm" @open-supercharge="openSupercharge" />
							</div>
						</div>
					</template>

					<LoadSettings
						v-else-if="tab === 'settings' && config"
						:config="config"
						:state="lm"
						:locked="locked"
						@login="login"
					/>
					<LoadBursts v-else-if="tab === 'bursts'" class="mb-4" :bursts="lm.bursts" />
					<LoadDiagnostics
						v-else-if="tab === 'diagnostics'"
						class="mb-4"
						:state="lm"
						@open-settings="selectTab('settings')"
					/>
				</template>
			</main>
		</div>
		<SuperchargeModal ref="superchargeModal" :load-state="lm" />
	</div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import store from "@/store";
import api from "@/api";
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

const TABS = ["overview", "bursts", "diagnostics", "settings"] as const;
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
			if (s.phase === "burst") return "burst";
			const lps = s.loadpoints || [];
			if (lps.some((lp) => lp.supercharge && !lp.paused && lp.setpointA > 0)) {
				return "supercharge";
			}
			return s.vaKva > s.thresholdKva ? "over" : "base";
		},
		// one message at a time, the most serious first
		alert(): { kind: string; title: string; text: string; tab?: Tab; action?: string } | null {
			const s = this.lm;
			if (!s) return null;
			if (!s.enabled) {
				return {
					kind: "off",
					title: this.$t("loadManagement.alert.offTitle"),
					text: this.$t("loadManagement.alert.offText", {
						a: this.config?.failsafeA ?? 0,
					}),
					tab: "settings",
					action: this.$t("loadManagement.alert.offAction"),
				};
			}
			if (s.running && s.blind > 0) {
				return {
					kind: "bad",
					title: this.$t("loadManagement.alert.blindTitle"),
					text: this.$t("loadManagement.alert.blindText"),
					tab: "diagnostics",
					action: this.$t("loadManagement.alert.blindAction"),
				};
			}
			const failed = (s.checks || []).filter((c) => !c.ok);
			if (failed.length) {
				return {
					kind: "warn",
					title: this.$t("loadManagement.alert.checkTitle", { name: failed[0]!.name }),
					text: failed[0]!.detail,
					tab: "diagnostics",
					action: this.$t("loadManagement.alert.checkAction"),
				};
			}
			if (s.lastError && s.running) {
				return {
					kind: "warn",
					title: this.$t("loadManagement.alert.errorTitle"),
					text: s.lastError,
				};
			}
			return null;
		},
	},
	mounted() {
		window.scrollTo({ top: 0 });
		this.fetch();
	},
	watch: {
		// arriving on a tab by link starts at its top too
		tab() {
			window.scrollTo({ top: 0 });
		},
	},
	methods: {
		// the websocket normally delivers both; ask directly so the page never waits on it
		async fetch() {
			try {
				const [state, config] = await Promise.all([
					this.lm ? null : api.get("supercharging"),
					this.config ? null : api.get("supercharging/config"),
				]);
				const msg: Record<string, unknown> = {};
				if (state && !this.lm) msg["supercharging"] = state.data;
				if (config && !this.config) msg["superchargingConfig"] = config.data;
				store.update(msg);
			} catch {
				// the websocket will bring it
			}
		},
		selectTab(t: Tab) {
			if (t === this.tab) return;
			this.$router.replace({ query: t === "overview" ? {} : { tab: t } });
		},
		// arrow keys move along the tabs, as a tablist does
		stepTab(dir: number) {
			const i = this.tabs.indexOf(this.tab);
			const next = this.tabs[(i + dir + this.tabs.length) % this.tabs.length]!;
			this.selectTab(next);
			this.$nextTick(() => {
				const el = this.$refs[`tab-${next}`] as HTMLElement[] | undefined;
				el?.[0]?.focus();
			});
		},
		login() {
			openLoginModal(this.$route.fullPath);
		},
		openSupercharge(e: {
			index: number;
			title: string;
			active: boolean;
			until: string | null;
		}) {
			(
				this.$refs["superchargeModal"] as InstanceType<typeof SuperchargeModal> | undefined
			)?.open(e.index, e.title, e.active, e.until);
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
	min-height: 2.25rem;
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
		flex: 1 1 auto;
		padding: 0.25em 0.3em;
		font-size: 0.8rem;
	}
}
/* the same dot and word as the house line on the main screen */
.phase {
	display: inline-flex;
	align-items: center;
	gap: 0.45rem;
	font-weight: 700;
	color: var(--evcc-default-text);
}
.phase-dot {
	width: 0.5rem;
	height: 0.5rem;
	border-radius: 50%;
	background: var(--evcc-gray);
}
.phase--base .phase-dot {
	background: var(--evcc-darker-green);
}
.phase--burst .phase-dot {
	background: var(--evcc-dark-yellow);
}
html.dark .phase--burst .phase-dot {
	background: var(--evcc-yellow);
}
.phase--blind {
	color: var(--evcc-red);
}
.phase--over {
	color: #9a5200;
}
html.dark .phase--over {
	color: var(--evcc-orange);
}
.phase--supercharge .phase-dot {
	background: var(--evcc-dark-yellow);
}
.phase--over .phase-dot {
	background: var(--evcc-orange);
}
.phase--blind .phase-dot {
	background: var(--evcc-red);
}
.phase--off,
.phase--standby {
	color: var(--evcc-gray);
	font-weight: normal;
}
.lm-alert {
	display: flex;
	align-items: center;
	gap: 0.9rem;
	padding: 0.9rem 1.1rem;
	border-radius: 1rem;
}
.lm-alert--warn {
	color: var(--evcc-orange);
	background: color-mix(in srgb, var(--evcc-orange) 12%, transparent);
}
.lm-alert--bad {
	color: var(--evcc-red);
	background: color-mix(in srgb, var(--evcc-red) 10%, transparent);
}
.lm-alert--off {
	color: var(--evcc-gray);
	background: var(--evcc-gray-15);
}
.lm-alert > div,
.lm-alert .btn {
	color: var(--evcc-default-text);
}
.min-w-0 {
	min-width: 0;
}
/* phones: tabs take the row, the status sits small under them */
@media (max-width: 575px) {
	.phase {
		font-size: 0.875rem;
	}
	.toolbar {
		gap: 0.6rem;
	}
	.lm-tab {
		min-height: 2.5rem;
	}
}
/* the same box as evcc's own cards (Card.vue, round-box) */
.lm-box {
	background: var(--evcc-box);
	border: 1px solid var(--bs-border-color-translucent);
	border-radius: 1rem;
	padding: 1.5rem;
}
@media (max-width: 575px) {
	.lm-box {
		padding: 1rem;
	}
}
</style>
