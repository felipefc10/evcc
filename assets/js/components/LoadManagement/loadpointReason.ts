import type { LoadLoadpoint, LoadState } from "@/types/supercharge";
import { carAhead } from "./state";

type T = (key: string, values?: Record<string, unknown>) => string;

export interface ReasonFormat {
  t: T;
  number: (n: number, digits: number) => string;
  duration: (s: number) => string;
  time: (iso: string) => string;
}

// Why a car held at 0 A waits: a car ahead of it in the order, or no room under the line.
export function waitReason(lp: LoadLoadpoint, state: LoadState, t: T): string {
  const ahead = carAhead(lp, state);
  return ahead
    ? t("loadManagement.reason.firstOther", { name: ahead.title })
    : t("loadManagement.reason.noRoom");
}

// The one sentence a loadpoint card owes its reader: why a car gets what it gets.
export function loadpointReason(lp: LoadLoadpoint, state: LoadState, f: ReasonFormat): string {
  if (!lp.connected) return f.t("loadManagement.reason.notConnected");
  if (lp.stoodOffS > 0) {
    return f.t("loadManagement.reason.stoodOff", { time: f.duration(lp.stoodOffS) });
  }
  if (!lp.wants) {
    if (lp.mode === "off") return f.t("loadManagement.reason.modeOff");
    if (lp.limitSoc > 0 && lp.soc >= lp.limitSoc) {
      return f.t("loadManagement.reason.atLimit", { soc: Math.round(lp.limitSoc) });
    }
    return f.t("loadManagement.reason.notWanted");
  }
  if (!state.enabled) return f.t("loadManagement.reason.disabled");
  if (!state.running) return f.t("loadManagement.reason.standby");
  if (lp.paused && lp.measuredA > 0.5) {
    return f.t("loadManagement.reason.pausedDrawing", { amps: f.number(lp.measuredA, 1) });
  }
  if (lp.paused) return "";
  const parts: string[] = [];
  if (lp.setpointA < lp.allocatedA) {
    parts.push(f.t("loadManagement.reason.comingUp"));
  } else {
    let measured = f.t("loadManagement.reason.measured", { amps: f.number(lp.measuredA, 1) });
    if (lp.clamped && lp.measuredRawA > lp.measuredA + 0.1) {
      measured += ` ${f.t("loadManagement.reason.sensorSays", { amps: f.number(lp.measuredRawA, 1) })}`;
    }
    parts.push(measured);
    if (lp.forecast?.etaS && lp.forecast.etaAt) {
      parts.push(
        f.t("loadManagement.reason.done", {
          time: f.time(lp.forecast.etaAt),
          duration: f.duration(lp.forecast.etaS),
        })
      );
    }
  }
  if (lp.waitS > 0.5) {
    parts.push(f.t("loadManagement.reason.nextChange", { s: Math.round(lp.waitS) }));
  }
  return parts.join(" · ");
}
