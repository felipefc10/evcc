<template>
	<section class="order" aria-labelledby="load-order-title" data-testid="load-order">
		<div class="order-head">
			<h2 id="load-order-title" class="order-title">
				{{ $t("loadManagement.order.title") }}
			</h2>
			<span v-if="movable" class="order-hint">{{ $t("loadManagement.order.hint") }}</span>
		</div>
		<ol class="order-list">
			<li v-for="row in rows" :key="row.lp.name">
				<LoadLoadpointCard
					:lp="row.lp"
					:state="state"
					:color="lpColor(row.lp.index)"
					:priority="row.priority"
					:place="row.place"
					:can-first="row.canFirst"
					:movable="movable"
					:lifted="drag === row.lp.name"
					@move="move(row.lp.name, $event)"
					@grip="grip(row.lp.name, $event)"
					@set-priority="setPriority(row.lp, $event)"
					@charge-first="chargeFirst(row.lp)"
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
import LoadLoadpointCard from "./LoadLoadpointCard.vue";
import { errorText, lpColor } from "./state";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

interface Arrangement {
	order: string[];
	// ties[i]: loadpoint i shares its priority with loadpoint i-1
	ties: boolean[];
}

const TOP = 10;
const PENDING_MS = 10000;

// "Which car charges first" is evcc's priority number: larger is served first and equal
// numbers share. Each card sets its own number with a stepper, "Charge first" puts a car on
// top, and dragging a card writes 10, 9, 8 … down the list. Only changed numbers are written.
export default defineComponent({
	name: "LoadOrder",
	components: { LoadLoadpointCard },
	props: {
		state: { type: Object as PropType<LoadState>, required: true },
	},
	emits: ["open-supercharge"],
	data() {
		return {
			// numbers just written stand for a while: the published state lags a second or two behind
			wanted: {} as Record<string, number>,
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
				(a, b) =>
					this.prio(b) - this.prio(a) || pos(a.name) - pos(b.name) || a.index - b.index
			);
			return {
				order: sorted.map((lp) => lp.name),
				ties: sorted.map((lp, i) => i > 0 && this.prio(lp) === this.prio(sorted[i - 1]!)),
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
			const arranged = this.live || this.pending;
			const prios = arranged
				? this.numbers(this.view)
				: this.view.order.map((n) => this.prio(this.byName[n]!));
			const levels = [...new Set(prios)];
			return this.view.order.map((name, i) => {
				const p = prios[i]!;
				const tied = prios.filter((x) => x === p).length > 1;
				return {
					lp: this.byName[name]!,
					priority: p,
					place: this.movable ? this.placeText(levels.indexOf(p), tied) : "",
					// sharing first place counts as not first yet, so both can step ahead
					canFirst: this.movable && (i > 0 || (tied && p === prios[0])),
				};
			});
		},
	},
	watch: {
		"view.order": {
			handler(order: string[]) {
				if (!this.live && order.join("|") !== this.lastOrder.join("|")) {
					this.lastOrder = [...order];
				}
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
		lpColor,
		placeText(level: number, tied: boolean): string {
			if (tied) return this.$t("loadManagement.order.shares");
			if (level < 4) return this.$t(`loadManagement.order.place${level + 1}`);
			return this.$t("loadManagement.order.later");
		},
		prio(lp: LoadLoadpoint): number {
			return lp.name in this.wanted ? this.wanted[lp.name]! : lp.priority;
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
		async write(changes: { lp: LoadLoadpoint; p: number }[]) {
			this.error = "";
			const wanted = { ...this.wanted };
			changes.forEach((c) => (wanted[c.lp.name] = c.p));
			this.wanted = wanted;
			setTimeout(() => {
				const left = { ...this.wanted };
				changes.forEach((c) => {
					if (left[c.lp.name] === c.p) delete left[c.lp.name];
				});
				this.wanted = left;
			}, PENDING_MS);
			try {
				await Promise.all(
					changes.map((c) => api.post(`loadpoints/${c.lp.index + 1}/priority/${c.p}`))
				);
			} catch (e) {
				const back = { ...this.wanted };
				changes.forEach((c) => delete back[c.lp.name]);
				this.wanted = back;
				this.error = this.$t("loadManagement.order.failed", {
					error: errorText(e),
				});
			}
		},
		setPriority(lp: LoadLoadpoint, p: number) {
			const want = Math.max(0, Math.min(TOP, p));
			if (want === this.prio(lp)) return;
			this.write([{ lp, p: want }]);
		},
		// this car on top: it gets the highest number, anyone already there steps down one
		chargeFirst(lp: LoadLoadpoint) {
			const changes = [{ lp, p: TOP }];
			for (const other of this.lps) {
				if (other.name !== lp.name && this.prio(other) >= TOP) {
					changes.push({ lp: other, p: TOP - 1 });
				}
			}
			this.write(changes.filter((c) => this.prio(c.lp) !== c.p));
		},
		// a drop writes 10, 9, 8 … down the list
		async commit(a: Arrangement) {
			const want = this.numbers(a);
			const changes = a.order
				.map((name, i) => ({ lp: this.byName[name]!, p: want[i]! }))
				.filter((c) => c.lp && this.prio(c.lp) !== c.p);
			if (!changes.length) {
				this.clearPending();
				return;
			}
			this.pending = { order: [...a.order], ties: [...a.ties] };
			clearTimeout(this.timer);
			this.timer = setTimeout(() => (this.pending = null), PENDING_MS);
			await this.write(changes);
			if (this.error) this.clearPending();
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
						i > 0 &&
						n !== moved &&
						order[i - 1] !== moved &&
						tied.has(`${order[i - 1]}|${n}`)
				),
			};
		},
		// keyboard on the drag handle: arrow up and down move the card
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
				const cards = [
					...(this.$el as HTMLElement).querySelectorAll<HTMLElement>("[data-name]"),
				];
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
	flex-direction: column;
	gap: 0.25rem;
	padding: 0 0 0.75rem;
}
.order-title {
	margin: 0;
}
.order-hint {
	font-size: 0.875rem;
	color: var(--evcc-gray);
}
.order-list {
	list-style: none;
	margin: 0;
	padding: 0;
	display: flex;
	flex-direction: column;
	gap: 1rem;
}
</style>
