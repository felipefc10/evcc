import colors from "@/colors";
import type { LoadLoadpoint, LoadState } from "@/types/supercharge";

// The status word shared by the house line and the load page, "attention" aside.
export function loadPhase(s: LoadState): string {
  if (!s.enabled) return "off";
  if (!s.running) return "standby";
  if (s.blind > 0) return "blind";
  if (s.phase === "burst") return "burst";
  // between bursts of a supercharge the house may sit above the line on purpose
  if (superchargeDrawing(s)) return "supercharge";
  return s.vaKva > s.thresholdKva ? "over" : "base";
}

export function superchargeDrawing(s: LoadState): boolean {
  return (s.loadpoints || []).some((lp) => lp.supercharge && !lp.paused && lp.setpointA > 0);
}

export function superchargeBursting(lp: LoadLoadpoint, s: LoadState): boolean {
  return s.running && s.phase === "burst" && lp.supercharge && !lp.paused;
}

// supercharge is on but cannot burst right now
export function superchargePaused(lp: LoadLoadpoint, s: LoadState): boolean {
  return lp.supercharge && (!!s.burstStoodDown || s.blind > 0 || s.tempBlock || lp.paused);
}

// a car higher in the order that is charging now, so this one may wait
export function carAhead(lp: LoadLoadpoint, s: LoadState): LoadLoadpoint | undefined {
  return (s.loadpoints || []).find(
    (o) => o.index !== lp.index && o.priority > lp.priority && !o.paused && o.setpointA > 0
  );
}

// a fixed ruler up past the burst target, so the line mark never moves
export function rulerMax(s: LoadState): number {
  return Math.max(s.thresholdKva * 1.3, (s.burstKva || 0) * 1.05, 0.1);
}

export function lpColor(i: number): string {
  return colors.palette[i % colors.palette.length] || "#60A5FA";
}

// the server's own words for a failed request, else whatever went wrong
export function errorText(e: unknown): string {
  const err = e as { response?: { data?: { error?: string } } } | undefined;
  return err?.response?.data?.error || String(e);
}
