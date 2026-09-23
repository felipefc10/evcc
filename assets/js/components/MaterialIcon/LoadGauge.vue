<template>
	<svg :style="svgStyle" viewBox="0 0 24 24">
		<path
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			d="M4.2 18a9 9 0 1 1 15.6 0"
		/>
		<path
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			d="M12 15l4.5-5.5"
		/>
		<circle cx="12" cy="15" r="1.6" fill="currentColor" />
	</svg>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import icon from "@/mixins/icon";

export default defineComponent({
	name: "LoadGauge",
	mixins: [icon],
});
</script>
