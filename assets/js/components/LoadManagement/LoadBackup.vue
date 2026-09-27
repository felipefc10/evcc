<template>
	<div data-testid="load-backup">
		<p class="small text-muted mb-3">{{ $t("loadManagement.backup.help") }}</p>
		<div class="d-flex flex-wrap gap-2 mb-3">
			<button
				type="button"
				class="btn btn-outline-primary"
				data-testid="load-export"
				@click="exportFile"
			>
				{{ $t("loadManagement.backup.export") }}
			</button>
			<label class="btn btn-outline-primary mb-0" for="lmImportFile">
				{{ $t("loadManagement.backup.import") }}
			</label>
			<input
				id="lmImportFile"
				ref="file"
				class="d-none"
				type="file"
				accept=".json,application/json"
				@change="pick"
			/>
		</div>
		<LineConfirm
			v-if="pending"
			class="mb-3"
			:text="$t('loadManagement.backup.confirm', { file: pending.name })"
			data-testid="load-import-confirm"
		>
			<button type="button" class="btn btn-outline-secondary" @click="cancel">
				{{ $t("loadManagement.cancel") }}
			</button>
			<button type="button" class="btn btn-danger" @click="importFile">
				{{ $t("loadManagement.backup.replace") }}
			</button>
		</LineConfirm>
		<div
			v-if="result"
			class="small"
			:class="result.ok ? 'text-primary' : 'text-danger'"
			role="status"
		>
			{{ result.text }}
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import api from "@/api";
import LineConfirm from "./LineConfirm.vue";

export default defineComponent({
	name: "LoadBackup",
	components: { LineConfirm },
	data() {
		return {
			pending: null as { name: string; data: unknown } | null,
			result: null as { ok: boolean; text: string } | null,
		};
	},
	methods: {
		async exportFile() {
			this.result = null;
			try {
				const res = await api.get("supercharging/export");
				const blob = new Blob([JSON.stringify(res.data, null, 2)], {
					type: "application/json",
				});
				const a = document.createElement("a");
				a.href = URL.createObjectURL(blob);
				a.download = "load-management.json";
				a.click();
				URL.revokeObjectURL(a.href);
			} catch (e) {
				this.fail(e);
			}
		},
		async pick(e: Event) {
			const input = e.target as HTMLInputElement;
			const file = input.files?.[0];
			input.value = "";
			this.result = null;
			if (!file) return;
			try {
				this.pending = { name: file.name, data: JSON.parse(await file.text()) };
			} catch {
				this.result = { ok: false, text: this.$t("loadManagement.backup.invalid") };
			}
		},
		cancel() {
			this.pending = null;
		},
		async importFile() {
			const data = this.pending?.data;
			this.pending = null;
			try {
				await api.post("supercharging/import", data);
				this.result = { ok: true, text: this.$t("loadManagement.backup.done") };
			} catch (e) {
				this.fail(e);
			}
		},
		fail(e: unknown) {
			const err = e as { response?: { data?: { error?: string } }; message?: string };
			this.result = { ok: false, text: err.response?.data?.error || err.message || "" };
		},
	},
});
</script>
