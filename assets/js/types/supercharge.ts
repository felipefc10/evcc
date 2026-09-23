// Whole-house load management (ICP never-trip line and supercharging), see core/supercharge

export type LoadPhase = "idle" | "base" | "burst";

export interface LoadForecast {
  avgKw: number;
  deliveredKwh: number;
  remainingKwh: number | null;
  etaS: number | null;
  etaAt: string | null;
}

export interface LoadLoadpoint {
  index: number;
  name: string;
  title: string;
  vehicle: string;
  mode: string;
  priority: number;
  connected: boolean;
  charging: boolean;
  wants: boolean;
  demandA: number;
  minA: number;
  maxA: number;
  phases: number;
  setpointA: number;
  allocatedA: number;
  paused: boolean;
  ops: number;
  waitS: number;
  measuredA: number;
  measuredRawA: number;
  measuredSrc: string;
  clamped: boolean;
  supercharge: boolean;
  superchargeUntil: string | null;
  fast: boolean;
  stoodOffS: number;
  soc: number;
  limitSoc: number;
  forecast: LoadForecast;
  feedAgeS: number | null;
}

export interface LoadMeter {
  ok: boolean;
  kva: number;
  watts: number;
  vars: number;
  volts: number;
  latencyMs: number;
  error?: string;
}

export interface LoadBehaviour {
  key: string;
  label: string;
  charger: string;
  vehicle: string;
  rampUp: number;
  rampDown: number;
  latencyDown: number;
  latencyUp: number;
  entryOffsetA: number;
  ups: number;
  downs: number;
  latsUp: number;
  latsDown: number;
  entries: number;
}

export interface LoadCheck {
  name: string;
  ok: boolean;
  detail: string;
}

export interface LoadState {
  enabled: boolean;
  configured: boolean;
  armed: boolean;
  running: boolean;
  status: string;
  phase: LoadPhase;
  pollMode: string;
  lastError: string;

  vaKva: number;
  houseKva: number;
  budgetA: number;
  trimA: number;
  tiS: number;
  closeness: number;
  peakCloseness: number;
  peakClosenessPhase: string;
  peakClosenessBlind: boolean;
  setpointA: number;
  bursts: number;
  burstCmds: number;
  contactorOps: number;
  writes: number;
  writesPerH: number;
  blind: number;

  thresholdKva: number;
  thresholdA: number;
  baseKva: number;
  burstKva: number;
  burstWindowS: number;
  canBurst: boolean;
  burstArmed: boolean;
  burstStoodDown: string;
  stillbornLimit: number;
  expectedKva: number;
  expectedGain: number;
  plannedCloseness: number;
  plannedLeadS: number;
  exitCloseness: number;
  exitLeadS: number;

  tempC: number | null;
  tempBlock: boolean;
  maxTempC: number;
  entryOffsetA: number;
  entryOffsetHeld: boolean;
  sourceOhm: number;
  voltsAtGoal: number;

  avgKva: number;
  kvah: number;
  gainVsLine: number;
  elapsedH: number;

  meter: LoadMeter;
  floorKva: number;
  idleHouseKva: number;
  claimKva: number;
  creditKva: number;
  disagreements: number;

  loadpoints: LoadLoadpoint[];
  learned: LoadBehaviour[];
  checks: LoadCheck[] | null;
  checkedAt: string | null;
}

export interface LoadSettings {
  burstKva: number;
  bumpKva: number;
  marginS: number;
  resetS: number;
  maxCloseness: number;
  exitLeadFrac: number;
  baseMarginKva: number;
  ampMin: number;
  ampMax: number;
  raiseTable: string;
  reduceTable: string;
  floorDwellS: number;
  restartDwellS: number;
  blindHoldPolls: number;
  blindPolls: number;
  blindAction: "stop" | "hold";
  blindHoldA: number;
  baseAbortCloseness: number;
  baseStopCloseness: number;
  baseTiAbortS: number;
  baseTiStopS: number;
  trimMaxA: number;
  maxTempC: number;
  supercharge: Record<string, number>;
}

export interface LoadLpConfig {
  fast: boolean;
  measureTopic: string;
  measureUnit: "A" | "W";
  tempTopic: string;
  maxTopic: string;
}

export interface LoadConfig {
  enabled: boolean;
  failsafeA: number;
  meterUri: string;
  q: number;
  k: number;
  contractKva: number;
  loadpoints: Record<string, LoadLpConfig>;
  settings: LoadSettings;
  importedFrom?: string;
}

export interface BurstRecord {
  startedAt: number;
  targetKva: number;
  targetA: number;
  lastedS: number;
  windowS: number;
  peakKva: number;
  peakCloseness: number;
  exit: string;
  usedContactor: boolean;
  loadpoint: string;
  wallS: number;
  settledKva: number;
  settledVolts: number;
  baseKva: number;
  baseVolts: number;
  peakVolts: number;
  entryOffsetA: number;
  cmds: number;
  at?: string;
  loadpointTitle?: string;
}
