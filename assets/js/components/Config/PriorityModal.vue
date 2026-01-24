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
			<DragDropList :values="localLoadpoints" @reorder="reorder">
				<DragDropItem
					v-for="lp in localLoadpoints"
					:key="lp.id"
					:title="lp.title"
					class="mb-2"
				>
					<div class="d-flex align-items-center">
						<span class="flex-grow-1 fw-bold">{{ lp.title }}</span>
						<span class="badge bg-secondary ms-2">
							{{ $t("priority.level", { priority: lp.priority }) }}
						</span>
					</div>
				</DragDropItem>
			</DragDropList>
		</div>
		<div v-else>
			<p>{{ $t("priority.modal.noLoadpoints") }}</p>
		</div>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import GenericModal from "../Helper/GenericModal.vue";
import DragDropList from "../Helper/DragDropList.vue";
import DragDropItem from "../Helper/DragDropItem.vue";
import api from "../../api";
import store from "../../store";

interface PriorityLoadpoint {
	id: number;
	title: string;
	priority: number;
}

export default defineComponent({
	name: "PriorityModal",
	components: { GenericModal, DragDropList, DragDropItem },
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
		async reorder(newOrder: PriorityLoadpoint[]) {
			this.localLoadpoints = newOrder;
			// Update priorities based on new order (First = Max Priority)
			const maxPrio = this.localLoadpoints.length;
			const ids = this.localLoadpoints.map((lp: PriorityLoadpoint) => lp.id);

			try {
				await api.post("/priority", ids);
				// Optimistic update of local priority display
				this.localLoadpoints.forEach((lp: PriorityLoadpoint, index: number) => {
					lp.priority = maxPrio - index;
				});
			} catch (e) {
				console.error("Failed to update priority", e);
			}
		},
	},
});
</script>
