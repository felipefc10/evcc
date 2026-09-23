<template>
	<section class="order" aria-labelledby="load-order-title" data-testid="load-order">
		<div class="order-head">
			<h2 id="load-order-title" class="order-title">{{ $t("loadManagement.order.title") }}</h2>
			<span v-if="movable" class="order-hint">{{ $t("loadManagement.order.hint") }}</span>
		</div>
		<ol class="order-list">
			<li v-for="(row, i) in rows" :key="row.lp.name">
				<div v-if="movable && i > 0" class="link">
					<button
						type="button"
						class="share"
						:class="{ 'share--on': view.ties[i] }"
						:aria-pressed="view.ties[i]"
						data-testid="load-share"
						@click="toggleShare(i)"
					>
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
							<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" />
							<path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" />
						</svg>
						{{ view.ties[i] ? $t("loadManagement.order.sharing") : $t("loadManagement.order.share") }}
					</button>
				</div>
				<LoadLoadpointCard
					:lp="row.lp"
					:state="state"
					:color="colorOf(row.lp.index)"
					:priority="row.priority"
					:place="row.place"
					:first="row.first"
					:movable="movable"
					:can-up="i > 0"
					:can-down="i < rows.length - 1"
					:lifted="drag === row.lp.name"
					@move="move(row.lp.name, $event)"
					@grip="grip(row.lp.name, $event)"
					@open-supercharge="$emit('open-supercharge', $event)"
				/>
			</li>
		</ol>
		<p v-if="error" class="small text-danger mt-2 mb-0" role="alert">{{ error }}</p>
	</section>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import colors from "@/colors";
import LoadLoadpointCard from "./LoadLoadpointCard.vue";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

interface Arrangement {
	order: string[];
	// ties[i]: loadpoint i shares its priority with loadpoint i-1
	ties: boolean[];
}

const TOP = 10;
const PENDING_MS = 15000;

// "Which car charges first" is an order; evcc stores it as a priority number where larger
// is served first and equal numbers share. The list is the order: dragging a card, its arrows
// or the share link between two cards writes 10, 9, 8 … down the list, changed numbers only.
export default defineComponent({
	name: "LoadOrder",
	components: { LoadLoadpointCard },
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	emits: ["open-supercharge"],
	data() {
		return {
			pending: null as Arrangement | null,
			live: null as Arrangement | null,
			drag: null as string | null,
			moved: false,
			error: "",
			timer: undefined as ReturnType<typeof setTimeout> | undefined,
			cleanup: null as null | (() => void),
			// cards that share a priority keep the order they were last shown in
			lastOrder: [] as string[],
		};
	},
	computed: {
		lps(): LoadLoadpoint[] {
			return this.state.loadpoints || [];
		},
		byName(): Record<string, LoadLoadpoint> {
			return Object.fromEntries(this.lps.map((lp) => [lp.name, lp]));
		},
		server(): Arrangement {
			const pos = (name: string) => {
				const i = this.lastOrder.indexOf(name);
				return i < 0 ? Number.MAX_SAFE_INTEGER : i;
			};
			const sorted = [...this.lps].sort(
				(a, b) => b.priority - a.priority || pos(a.name) - pos(b.name) || a.index - b.index
			);
			return {
				order: sorted.map((lp) => lp.name),
				ties: sorted.map((lp, i) => i > 0 && lp.priority === sorted[i - 1]!.priority),
			};
		},
		view(): Arrangement {
			const v = this.live || this.pending || this.server;
			// a loadpoint added or removed meanwhile: fall back to what evcc has
			return v.order.length === this.lps.length && v.order.every((n) => this.byName[n])
				? v
				: this.server;
		},
		movable(): boolean {
			return this.lps.length > 1;
		},
		rows() {
			const prios = this.numbers(this.view);
			const levels = [...new Set(prios)];
			return this.view.order.map((name, i) => {
				const p = prios[i]!;
				const level = levels.indexOf(p);
				const tied = prios.filter((x) => x === p).length > 1;
				let place = "";
				if (this.movable) {
					place = tied
						? this.$t("loadManagement.order.shares")
						: level < 4
							? this.$t(`loadManagement.order.place${level + 1}`)
							: this.$t("loadManagement.order.later");
				}
				return {
					lp: this.byName[name]!,
					priority: this.movable ? p : this.byName[name]!.priority,
					place,
					first: level === 0,
				};
			});
		},
	},
	watch: {
		"view.order": {
			handler(order: string[]) {
				if (!this.live) this.lastOrder = [...order];
			},
			immediate: true,
		},
		server(s: Arrangement) {
			// evcc caught up with what was written
			if (this.pending && this.same(s, this.pending)) this.clearPending();
		},
	},
	beforeUnmount() {
		this.cleanup?.();
		clearTimeout(this.timer);
	},
	methods: {
		colorOf(i: number): string {
			return colors.palette[i % colors.palette.length] || "#60A5FA";
		},
		numbers(a: Arrangement): number[] {
			let p = TOP;
			return a.order.map((_, i) => {
				if (i > 0 && !a.ties[i]) p = Math.max(0, p - 1);
				return p;
			});
		},
		// equal when every loadpoint gets the same number, whatever the order among equals
		same(a: Arrangement, b: Arrangement): boolean {
			const na = this.numbers(a);
			const nb = this.numbers(b);
			return a.order.every((n, i) => nb[b.order.indexOf(n)] === na[i]);
		},
		clearPending() {
			this.pending = null;
			clearTimeout(this.timer);
		},
		// a moved loadpoint no longer shares with anyone; untouched neighbours keep sharing
		rearranged(from: Arrangement, order: string[], moved: string): Arrangement {
			const tied = new Set<string>();
			from.order.forEach((n, i) => {
				if (i > 0 && from.ties[i]) tied.add(`${from.order[i - 1]}|${n}`);
			});
			return {
				order,
				ties: order.map(
					(n, i) =>
						i > 0 && n !== moved && order[i - 1] !== moved && tied.has(`${order[i - 1]}|${n}`)
				),
			};
		},
		async commit(a: Arrangement) {
			this.error = "";
			const want = this.numbers(a);
			const changes = a.order
				.map((name, i) => ({ lp: this.byName[name]!, p: want[i]! }))
				.filter((c) => c.lp && c.lp.priority !== c.p);
			if (!changes.length) {
				this.clearPending();
				return;
			}
			this.pending = { order: [...a.order], ties: [...a.ties] };
			clearTimeout(this.timer);
			this.timer = setTimeout(() => (this.pending = null), PENDING_MS);
			try {
				await Promise.all(
					changes.map((c) => api.post(`loadpoints/${c.lp.index + 1}/priority/${c.p}`))
				);
			} catch (e: any) {
				this.clearPending();
				this.error = this.$t("loadManagement.order.failed", {
					error: e?.response?.data?.error || String(e),
				});
			}
		},
		move(name: string, dir: number) {
			const v = this.view;
			const i = v.order.indexOf(name);
			const j = i + dir;
			if (i < 0 || j < 0 || j >= v.order.length) return;
			const order = [...v.order];
			order[i] = order[j]!;
			order[j] = name;
			this.commit(this.rearranged(v, order, name));
		},
		toggleShare(i: number) {
			const v = this.view;
			const ties = [...v.ties];
			ties[i] = !ties[i];
			this.commit({ order: [...v.order], ties });
		},
		// pointer events rather than HTML5 drag and drop, so a finger on a phone drags too
		grip(name: string, ev: PointerEvent) {
			if (ev.pointerType === "mouse" && ev.button !== 0) return;
			const handle = (ev.target as HTMLElement | null)?.closest<HTMLElement>(".grip");
			if (!handle) return;
			ev.preventDefault();
			try {
				handle.setPointerCapture(ev.pointerId);
			} catch {
				// not every pointer can be captured; the window listeners below still follow it
			}
			const start = { order: [...this.view.order], ties: [...this.view.ties] };
			this.live = { ...start };
			this.drag = name;
			this.moved = false;
			document.body.style.userSelect = "none";

			const onMove = (e: PointerEvent) => {
				const cards = [...(this.$el as HTMLElement).querySelectorAll<HTMLElement>("[data-name]")];
				const others = cards.filter((c) => c.dataset["name"] !== name);
				const target = others.filter((c) => {
					const r = c.getBoundingClientRect();
					return r.top + r.height / 2 < e.clientY;
				}).length;
				const current = this.live!.order.filter((n) => n !== name);
				current.splice(target, 0, name);
				if (current.join("|") !== this.live!.order.join("|")) {
					this.moved = true;
					this.live = this.rearranged(start, current, name);
				}
			};
			const onUp = () => {
				this.cleanup?.();
				const result = this.live;
				this.live = null;
				this.drag = null;
				if (this.moved && result) this.commit(result);
			};
			this.cleanup = () => {
				document.body.style.userSelect = "";
				window.removeEventListener("pointermove", onMove);
				window.removeEventListener("pointerup", onUp);
				window.removeEventListener("pointercancel", onUp);
				this.cleanup = null;
			};
			window.addEventListener("pointermove", onMove);
			window.addEventListener("pointerup", onUp);
			window.addEventListener("pointercancel", onUp);
		},
	},
});
</script>

<style scoped>
.order-head {
	display: flex;
	flex-wrap: wrap;
	align-items: baseline;
	justify-content: space-between;
	gap: 0.25rem 1rem;
	padding: 0 0.5rem 0.75rem;
}
.order-title {
	margin: 0;
	font-size: 0.8rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: var(--evcc-gray);
}
.order-hint {
	font-size: 0.75rem;
	color: var(--evcc-gray);
}
.order-list {
	list-style: none;
	margin: 0;
	padding: 0;
	display: flex;
	flex-direction: column;
}
.link {
	display: flex;
	justify-content: center;
	padding: 0.5rem 0;
}
.share {
	display: inline-flex;
	align-items: center;
	gap: 0.4rem;
	min-height: 2rem;
	padding: 0 0.9rem;
	border-radius: 999px;
	font-size: 0.75rem;
	font-weight: 700;
	background: var(--evcc-background);
	color: var(--evcc-gray);
	border: 1px solid var(--evcc-gray-25);
}
.share:hover {
	color: var(--evcc-default-text);
}
.share--on {
	color: var(--evcc-dark-green);
	border-color: var(--evcc-dark-green);
	background: color-mix(in srgb, var(--evcc-dark-green) 14%, transparent);
}
</style>
