import type { PublicModelControlDTO } from "@/shared/api/model-types";
import { isFiniteNumber, isOneOf, isRecord } from "@/shared/lib/type-guards";

// User-facing model controls. The administrator declares what each option does in the model
// capabilities; users only pick a value and the request carries `{controlID: value}`.
export type ModelControl = PublicModelControlDTO;
export type ModelControlType = ModelControl["type"];
export type ModelControlPlacement = ModelControl["placement"];

// Value submitted for a control: an option value (select), "on"/"off" (toggle) or a number.
export type ModelControlValue = string | number;

export const MODEL_CONTROL_TOGGLE_ON = "on";
export const MODEL_CONTROL_TOGGLE_OFF = "off";
export const MODEL_CONTROL_REASONING_ID = "reasoning";
export const MODEL_CONTROL_REASONING_AUTO = "auto";

// Users can pin at most this many controls to the composer toolbar (mirrors the backend setting validation).
export const MAX_PINNED_MODEL_CONTROLS = 6;

const isModelControlType = isOneOf(["select", "toggle", "number"] as const satisfies readonly ModelControlType[]);
const isModelControlPlacement = isOneOf(["toolbar", "menu"] as const satisfies readonly ModelControlPlacement[]);
const isModelControlKind = isOneOf(["generic", "reasoning"] as const satisfies readonly ModelControl["kind"][]);

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function optionalNumber(value: unknown): number | null {
  return isFiniteNumber(value) ? value : null;
}

/** Narrows the `controls` array of a public model DTO; malformed entries are dropped. */
export function parseModelControls(value: unknown): ModelControl[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const seen = new Set<string>();
  const controls: ModelControl[] = [];
  for (const item of value) {
    if (!isRecord(item)) continue;
    const id = text(item.id);
    if (!id || seen.has(id) || !isModelControlType(item.type)) continue;
    const options = Array.isArray(item.options)
      ? item.options.flatMap((option) => {
        if (!isRecord(option) || !text(option.value)) return [];
        return [{ value: text(option.value), label: text(option.label), description: text(option.description) }];
      })
      : [];
    if (item.type !== "number" && options.length === 0) continue;
    seen.add(id);
    controls.push({
      id,
      kind: isModelControlKind(item.kind) ? item.kind : "generic",
      type: item.type,
      label: text(item.label),
      description: text(item.description),
      icon: text(item.icon),
      placement: isModelControlPlacement(item.placement) ? item.placement : "menu",
      default: text(item.default) || null,
      locked: item.locked === true,
      options,
      min: optionalNumber(item.min),
      max: optionalNumber(item.max),
      step: optionalNumber(item.step),
      integer: item.integer === true,
      protocols: Array.isArray(item.protocols) ? item.protocols.map(text).filter(Boolean) : [],
    });
  }
  return controls;
}

/** Returns the value when the control accepts it, otherwise null (callers fall back to the default). */
export function normalizeModelControlValue(control: ModelControl, value: unknown): ModelControlValue | null {
  if (control.type === "number") {
    const numeric = typeof value === "number" ? value : typeof value === "string" && value.trim() ? Number(value) : Number.NaN;
    if (!Number.isFinite(numeric)) return null;
    if (control.min !== null && numeric < control.min) return null;
    if (control.max !== null && numeric > control.max) return null;
    if (control.integer && !Number.isInteger(numeric)) return null;
    if (control.step !== null && control.step > 0) {
      const steps = (numeric - (control.min ?? 0)) / control.step;
      if (Math.abs(steps - Math.round(steps)) > 1e-6) return null;
    }
    return numeric;
  }
  const raw = typeof value === "boolean"
    ? (value ? MODEL_CONTROL_TOGGLE_ON : MODEL_CONTROL_TOGGLE_OFF)
    : typeof value === "string" ? value.trim() : "";
  return control.options.some((option) => option.value === raw) ? raw : null;
}

/** Default value of a control; null means the control leaves the request untouched. */
export function modelControlDefaultValue(control: ModelControl): ModelControlValue | null {
  return control.default === null ? null : normalizeModelControlValue(control, control.default);
}

// Sampling parameters that take fractional values; matched on the last path segment of the control
// id (controls compiled from optionControls use the parameter path as their id).
const FRACTIONAL_NUMBER_PARAMETERS = new Set([
  "temperature",
  "top_p",
  "topp",
  "min_p",
  "minp",
  "top_a",
  "frequency_penalty",
  "frequencypenalty",
  "presence_penalty",
  "presencepenalty",
  "repetition_penalty",
  "repetitionpenalty",
]);

/**
 * Stepper increment of a number control. The administrator's step wins; otherwise integers step by 1,
 * known sampling parameters (temperature, top_p, penalties…) and narrow ranges by 0.1, the rest by 1.
 */
export function modelControlNumberStep(control: ModelControl): number {
  if (control.step !== null && control.step > 0) return control.step;
  if (control.integer) return 1;
  const parameter = (control.id.split(".").pop() ?? "").toLowerCase();
  if (FRACTIONAL_NUMBER_PARAMETERS.has(parameter)) return 0.1;
  if (control.min !== null && control.max !== null && control.max - control.min <= 2) return 0.1;
  const defaultValue = control.default === null ? Number.NaN : Number(control.default);
  if (Number.isFinite(defaultValue) && !Number.isInteger(defaultValue)) return 0.1;
  return 1;
}
