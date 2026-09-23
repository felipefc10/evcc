<template>
	<div class="load-row small" data-testid="loadpoint-load-management">
		<div class="d-flex justify-content-between align-items-baseline gap-2">
			<span class="text-muted text-truncate">{{ $t("loadManagement.row.title") }}</span>
			<span class="fw-bold text-nowrap">{{ allowance }}</span>
		</div>
		<div class="text-muted reason" :class="{ 'text-warning': warning }">{{ reason }}</div>
		<SuperchargeSwitch
			class="mt-1"
			:index="lp.index"
			:title="lp.title"
			:active="lp.supercharge"
			:until="lp.superchargeUntil"
			@open="$emit('open-supercharge', $event)"
		/>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import formatter from "@/mixins/formatter";
import SuperchargeSwitch from "./SuperchargeSwitch.vue";
import { loadpointReason } from "./loadpointReason";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

// What whole-house load management gives this loadpoint, on its own card.
export default defineComponent({
	name: "LoadpointLoadRow",
	components: { SuperchargeSwitch },
	mixins: [formatter],
	props: {
		lp: { type: Object as PropType<LoadLoadpoint>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
	},
	emits: ["open-supercharge"],
	computed: {
		allowance(): string {
			if (!this.state.running || !this.lp.wants) return "—";
			if (this.lp.paused) return this.$t("loadManagement.card.paused");
			return this.$t("loadManagement.row.allowed", { a: this.lp.setpointA });
		},
		reason(): string {
			return loadpointReason(this.lp, this.state, {
				t: (k, v) => this.$t(k, v || {}),
				number: (n, d) => this.fmtNumber(n, d),
				duration: (s) => (s < 90 ? `${Math.round(s)} s` : this.fmtDurationLong(s, "short")),
				time: (iso) =>
					new Date(iso).toLocaleTimeString(this.$i18n.locale, {
						hour: "2-digit",
						minute: "2-digit",
					}),
			});
		},
		warning(): boolean {
			return (this.lp.paused && this.lp.measuredA > 0.5) || this.lp.stoodOffS > 0;
		},
	},
});
</script>

<style scoped>
.reason {
	min-height: 1.25rem;
}
</style>
