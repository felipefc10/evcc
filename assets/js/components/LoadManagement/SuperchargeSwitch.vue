<template>
	<div class="d-flex align-items-center flex-wrap gap-2" data-testid="supercharge-switch">
		<div class="form-check form-switch m-0">
			<input
				:id="`supercharge-${index}`"
				:checked="active"
				class="form-check-input"
				type="checkbox"
				role="switch"
				:aria-label="$t('loadManagement.supercharge.switchAria', { name: title })"
				@click.prevent="toggle"
			/>
			<label class="form-check-label" :for="`supercharge-${index}`">
				{{ $t("loadManagement.supercharge.label") }}
			</label>
		</div>
		<button
			v-if="active"
			type="button"
			class="btn btn-sm btn-link p-0 until"
			:title="$t('loadManagement.supercharge.changeTime')"
			@click="$emit('open', { index, title, active, until })"
		>
			{{ untilLabel }}
		</button>
		<span v-if="error" class="small text-danger">{{ error }}</span>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";

// Switching supercharging on asks until when (the parent opens SuperchargeModal);
// switching it off is immediate. The switch only ever shows what the server says.
export default defineComponent({
	name: "SuperchargeSwitch",
	props: {
		index: { type: Number, required: true },
		title: { type: String, default: "" },
		active: Boolean,
		until: { type: String as PropType<string | null>, default: null },
	},
	emits: ["open"],
	data() {
		return { error: "" };
	},
	computed: {
		untilLabel(): string {
			if (!this.until) return this.$t("loadManagement.supercharge.indefinitelyShort");
			const d = new Date(this.until);
			const soon = d.getTime() - Date.now() < 18 * 3600 * 1000;
			const time = soon
				? d.toLocaleTimeString(this.$i18n.locale, { hour: "2-digit", minute: "2-digit" })
				: d.toLocaleString(this.$i18n.locale, {
						weekday: "short",
						hour: "2-digit",
						minute: "2-digit",
					});
			return this.$t("loadManagement.supercharge.until", { time });
		},
	},
	methods: {
		async toggle() {
			this.error = "";
			if (!this.active) {
				this.$emit("open", {
					index: this.index,
					title: this.title,
					active: false,
					until: null,
				});
				return;
			}
			try {
				await api.post(`supercharging/loadpoints/${this.index + 1}/supercharge`, {
					on: false,
					until: "",
				});
			} catch (e: any) {
				this.error = e?.response?.data?.error || String(e);
			}
		},
	},
});
</script>

<style scoped>
.until {
	text-decoration: none;
}
</style>
