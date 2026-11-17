import colors from "@/colors";
import type { LoadCar, LoadLoadpoint, LoadState } from "@/types/supercharge";

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

// one sentence on where car detection has a car, for the card and the settings
export function carStatus(c: LoadCar, t: (key: string, values?: Record<string, unknown>) => string): string {
  const v = { car: c.vehicleTitle, charger: c.atTitle };
  if (!c.plugged) return t("loadManagement.cars.notPlugged", v);
  if (c.proven) return t("loadManagement.cars.found", v);
  if (c.manual) return t("loadManagement.cars.chosen", v);
  if (c.probing) return t("loadManagement.cars.checking", { ...v, charger: c.probeOnTitle });
  return t("loadManagement.cars.assumed", { ...v, charger: c.loadpointTitle });
}

// why a car cannot be placed, in the owner's words, "" when it can
export function carError(c: LoadCar, t: (key: string, values?: Record<string, unknown>) => string): string {
  if (!c.error) return "";
  const key = c.error.charAt(0).toUpperCase() + c.error.slice(1);
  return t(`loadManagement.cars.error${key}`, { charger: c.errorOn });
}

// the car detection verdict that concerns a loadpoint's card, if any
export function carHere(lp: LoadLoadpoint, s: LoadState): LoadCar | undefined {
  return (s.cars || []).find(
    (c) => !c.error && c.plugged && !c.manual && (c.at === lp.name || (c.probing && c.loadpoint === lp.name))
  );
}
