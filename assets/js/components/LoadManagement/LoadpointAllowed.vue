<template>
	<div class="allowed d-flex flex-column align-items-center text-center">
		<LabelAndValue
			:label="$t('loadManagement.card.allowed')"
			:value="allowedValue"
			align="center"
			data-testid="loadpoint-allowed"
		/>
		<SuperchargePill
			class="mt-1 allowed-pill"
			:index="lp.index"
			:title="lp.title"
			:active="lp.supercharge"
			:paused="paused"
			:until="lp.superchargeUntil"
			:note="allowedWhy"
			:reserve="lineUsed"
			stacked
			@open="$emit('open-supercharge', $event)"
		/>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import LabelAndValue from "../Helper/LabelAndValue.vue";
import SuperchargePill from "./SuperchargePill.vue";
import { waitReason } from "./loadpointReason";
import { superchargeBursting, superchargePaused } from "./state";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

// What whole-house load management allows a loadpoint on the main screen, with its supercharge.
export default defineComponent({
	name: "LoadpointAllowed",
	components: { LabelAndValue, SuperchargePill },
	props: {
		lp: { type: Object as PropType<LoadLoadpoint>, required: true },
		state: { type: Object as PropType<LoadState>, required: true },
	},
	emits: ["open-supercharge"],
	computed: {
		// a held-back car gets 0 A; the line under the pill says why
		allowedValue(): string {
			const lp = this.lp;
			const s = this.state;
			if (!s.running || !lp.wants) return "—";
			if (lp.paused || s.blind > 0) return "0 A";
			// a car that is not drawing yet may take up to this, not more
			return lp.charging
				? `${lp.setpointA} A`
				: this.$t("loadManagement.card.upTo", { a: lp.setpointA });
		},
		paused(): boolean {
			return superchargePaused(this.lp, this.state);
		},
		// any card with a line under its pill makes every card keep one, so the cards line up
		lineUsed(): boolean {
			const s = this.state;
			if (!s.running) return false;
			return (s.loadpoints || []).some((lp) => lp.supercharge || (lp.wants && lp.paused));
		},
		// said only when load management holds this car back
		allowedWhy(): string {
			const lp = this.lp;
			const s = this.state;
			if (!s.running || !lp.wants) return "";
			if (superchargeBursting(lp, s)) return this.$t("loadManagement.card.whyBursting");
			if (s.blind > 0) return this.$t("loadManagement.card.whyBlind");
			if (lp.paused) return waitReason(lp, s, (k, v) => this.$t(k, v || {}));
			return "";
		},
	},
});
</script>

<style scoped>
.allowed {
	overflow: visible !important;
}
.allowed-pill {
	width: max-content;
	max-width: none;
}
</style>
