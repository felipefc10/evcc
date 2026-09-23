<template>
	<Card
		class="box-pull-out"
		:title="$t('loadManagement.bursts.title')"
		:subtitle="subtitle"
		data-testid="load-bursts"
	>
		<template #actions>
			<button type="button" class="btn btn-sm btn-outline-secondary" @click="load">
				{{ $t("loadManagement.bursts.refresh") }}
			</button>
		</template>
		<p v-if="!rows.length" class="text-muted mb-0">{{ $t("loadManagement.bursts.none") }}</p>
		<div v-else class="table-responsive">
			<table class="table table-sm align-middle small mb-0">
				<thead>
					<tr>
						<th>{{ $t("loadManagement.bursts.at") }}</th>
						<th>{{ $t("loadManagement.bursts.loadpoint") }}</th>
						<th class="text-end">{{ $t("loadManagement.bursts.lasted") }}</th>
						<th class="text-end">{{ $t("loadManagement.bursts.peak") }}</th>
						<th class="text-end">{{ $t("loadManagement.bursts.closeness") }}</th>
						<th class="text-end">{{ $t("loadManagement.bursts.changes") }}</th>
						<th>{{ $t("loadManagement.bursts.exit") }}</th>
					</tr>
				</thead>
				<tbody>
					<tr v-for="(r, i) in rows" :key="i">
						<td class="text-nowrap">{{ fmtAt(r.at) }}</td>
						<td class="text-truncate">{{ r.loadpointTitle || r.loadpoint }}</td>
						<td class="text-end">{{ fmtNumber(r.wallS, 0) }} s</td>
						<td class="text-end">{{ fmtNumber(r.peakKva, 2) }} kVA</td>
						<td class="text-end">{{ fmtNumber(r.peakCloseness, 2) }}</td>
						<td class="text-end" :class="{ 'text-warning': r.cmds > 2 }">{{ r.cmds }}</td>
						<td :class="{ 'text-danger': r.usedContactor }" :title="exitHelp(r.exit)">
							{{ exitLabel(r.exit) }}
						</td>
					</tr>
				</tbody>
			</table>
		</div>
		<p class="small text-muted mt-3 mb-0">{{ $t("loadManagement.bursts.help") }}</p>
	</Card>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import api from "@/api";
import formatter from "@/mixins/formatter";
import Card from "../Helper/Card.vue";
import type { BurstRecord } from "@/types/supercharge";

const EXITS = ["deadline", "bump margin", "closeness", "meter blind", "no draw", "icp alarm"];

export default defineComponent({
	name: "LoadBursts",
	components: { Card },
	mixins: [formatter],
	props: {
		bursts: { type: Number, default: 0 },
	},
	data() {
		return { records: [] as BurstRecord[] };
	},
	computed: {
		rows(): BurstRecord[] {
			return [...this.records].reverse().slice(0, 100);
		},
		subtitle(): string {
			return this.records.length
				? this.$t("loadManagement.bursts.count", { n: this.records.length })
				: "";
		},
	},
	watch: {
		bursts() {
			this.load();
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		async load() {
			try {
				const res = await api.get("supercharging/bursts");
				this.records = (res.data as BurstRecord[]) || [];
			} catch {
				// keep what is shown
			}
		},
		exitKey(exit: string): string {
			return EXITS.includes(exit) ? exit.replace(/ /g, "_") : "other";
		},
		exitLabel(exit: string): string {
			const k = this.exitKey(exit);
			return k === "other" ? exit : this.$t(`loadManagement.bursts.exits.${k}.label`);
		},
		exitHelp(exit: string): string {
			const k = this.exitKey(exit);
			return k === "other" ? "" : this.$t(`loadManagement.bursts.exits.${k}.help`);
		},
		fmtAt(at?: string): string {
			if (!at) return "";
			return this.fmtAbsoluteDate(new Date(at));
		},
	},
});
</script>
