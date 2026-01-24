<template>
	<GenericModal
		id="priorityModal"
		:title="$t('priority.modal.title')"
		data-testid="priority-modal"
		@open="open"
	>
		<div v-if="localLoadpoints.length > 0">
			<p class="mb-3 text-secondary">
				{{ $t("priority.modal.description") }}
			</p>
			<div class="list-group list-group-flush">
				<div
					v-for="lp in localLoadpoints"
					:key="lp.id"
					class="list-group-item d-flex align-items-center justify-content-between px-0"
				>
					<span class="fw-bold">{{ lp.title }}</span>
					<select
						v-model.number="lp.priority"
						class="form-select w-auto"
						@change="updatePriority"
					>
						<option v-for="p in 11" :key="p - 1" :value="p - 1">
							{{ $t("priority.level", { priority: p - 1 }) }}
						</option>
					</select>
				</div>
			</div>
		</div>
		<div v-else>
			<p>{{ $t("priority.modal.noLoadpoints") }}</p>
		</div>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import GenericModal from "../Helper/GenericModal.vue";
import api from "../../api";
import store from "../../store";

export interface PriorityLoadpoint {
	id: number;
	title: string;
	priority: number;
}

export default defineComponent({
	name: "PriorityModal",
	components: { GenericModal },
	data() {
		return {
			localLoadpoints: [] as PriorityLoadpoint[],
		};
	},
	computed: {
		loadpoints() {
			return store.uiLoadpoints.value || [];
		},
	},
	methods: {
		open() {
			// Initialize local copy, sorted by priority (descending)
			this.localLoadpoints = this.loadpoints
				.map((lp: any) => ({
					id: lp.id,
					title: lp.title || `Loadpoint ${lp.id}`,
					priority: lp.priority || 0,
				}))
				.sort((a: PriorityLoadpoint, b: PriorityLoadpoint) => b.priority - a.priority);
		},
		async updatePriority() {
			const priorities: Record<number, number> = {};
			this.localLoadpoints.forEach((lp) => {
				priorities[lp.id] = lp.priority;
			});

			try {
				await api.post("/priority", priorities);
			} catch (e) {
				console.error("Failed to update priority", e);
			}
		},
	},
});
</script>
