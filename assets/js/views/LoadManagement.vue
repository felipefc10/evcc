<template>
	<div class="container px-4 safe-area-inset">
		<TopHeader :title="$t('loadManagement.title')" />
		<div class="row">
			<main class="col-12">
				<p v-if="!lm" class="my-4 text-muted">{{ $t("loadManagement.loading") }}</p>
				<template v-else>
					<div class="btn-group mb-4 tabs" role="tablist" data-testid="load-tabs">
						<button
							v-for="t in tabs"
							:key="t"
							type="button"
							class="btn btn-sm"
							:class="t === tab ? 'btn-secondary' : 'btn-outline-secondary'"
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

					<div
						v-if="lm.lastError && lm.running"
						class="alert alert-warning py-2 small"
						role="alert"
					>
						{{ lm.lastError }}
					</div>

					<template v-if="tab === 'overview'">
						<Card class="mb-4 box-pull-out" :title="$t('loadManagement.houseLoad')">
							<LoadLedger :state="lm" />
						</Card>

						<LoadLoadpointCard
							v-for="(lp, i) in lm.loadpoints"
							:key="lp.name"
							class="mb-4 box-pull-out"
							:lp="lp"
							:state="lm"
							:color="colorOf(i)"
							:rank="rankOf(lp.name)"
							:total="lm.loadpoints.length"
							:tied="tied(lp.priority)"
							@open-supercharge="openSupercharge"
						/>

						<div class="row row-cols-2 row-cols-md-4 g-3 mb-4 box-pull-out">
							<div v-for="s in stats" :key="s.label" class="col">
								<div class="stat round-box p-3 h-100" :title="s.help">
									<div class="stat-value">{{ s.value }}</div>
									<div class="small">{{ s.label }}</div>
									<div class="small text-muted">{{ s.note }}</div>
								</div>
							</div>
						</div>
					</template>

					<LoadSettings v-else-if="tab === 'settings' && config" :config="config" :state="lm" />
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
import colors from "@/colors";
import formatter from "@/mixins/formatter";
import Header from "../components/Top/Header.vue";
import Card from "../components/Helper/Card.vue";
import LoadLedger from "../components/LoadManagement/LoadLedger.vue";
import LoadLoadpointCard from "../components/LoadManagement/LoadLoadpointCard.vue";
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
		Card,
		LoadLedger,
		LoadLoadpointCard,
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
		ranked() {
			return [...(this.lm?.loadpoints || [])].sort((a, b) => b.priority - a.priority);
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
					label: this.$t("loadManagement.stats.house"),
					value: s.running ? `${this.fmtNumber(s.houseKva, 2)} kVA` : "—",
					note: s.running
						? this.$t("loadManagement.stats.ofLine", {
								pct: Math.round((100 * s.houseKva) / (s.thresholdKva || 1)),
							})
						: "",
					help: this.$t("loadManagement.stats.houseHelp"),
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
					label: this.$t("loadManagement.stats.pauses"),
					value: String(s.contactorOps || 0),
					note: "",
					help: this.$t("loadManagement.stats.pausesHelp"),
				},
				{
					label: this.$t("loadManagement.stats.commands"),
					value: String(s.writes || 0),
					note: s.writesPerH ? `${this.fmtNumber(s.writesPerH, 1)} /h` : "",
					help: this.$t("loadManagement.stats.commandsHelp"),
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
		colorOf(i: number): string {
			return colors.palette[i % colors.palette.length] || "#60A5FA";
		},
		rankOf(name: string): number {
			return this.ranked.findIndex((lp) => lp.name === name);
		},
		tied(priority: number): boolean {
			return (this.lm?.loadpoints || []).filter((lp) => lp.priority === priority).length > 1;
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
.tabs {
	flex-wrap: wrap;
}
.stat-value {
	font-size: 1.5rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}
</style>
