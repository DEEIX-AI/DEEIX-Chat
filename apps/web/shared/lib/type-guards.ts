// Runtime type guards for narrowing `unknown` values (API payloads, storage,
// postMessage data, JSON) without `as` assertions. Prefer these over ad-hoc
// casts; add a new guard here only when it is reused across features.

export type UnknownRecord = Record<string, unknown>;

export function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function isString(value: unknown): value is string {
  return typeof value === "string";
}

export function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim() !== "";
}

/** Finite numbers only; NaN and ±Infinity are rejected. */
export function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

export function isBoolean(value: unknown): value is boolean {
  return typeof value === "boolean";
}

export function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every(isString);
}

/**
 * Builds a guard for a string literal union from its allowed values, e.g.
 * `const isSortKey = isOneOf(["name", "createdAt"] as const)`. Use it to narrow
 * `<Select onValueChange>` strings and query params instead of `value as SortKey`.
 */
export function isOneOf<const T extends readonly string[]>(values: T): (value: unknown) => value is T[number] {
  const allowed: ReadonlySet<string> = new Set(values);
  return (value: unknown): value is T[number] => typeof value === "string" && allowed.has(value);
}

/** Parses JSON without throwing; returns `undefined` on malformed input. Narrow the result with a guard. */
export function parseJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

export function readString(record: UnknownRecord, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" ? value : undefined;
}

export function readFiniteNumber(record: UnknownRecord, key: string): number | undefined {
  const value = record[key];
  return isFiniteNumber(value) ? value : undefined;
}

export function readBoolean(record: UnknownRecord, key: string): boolean | undefined {
  const value = record[key];
  return typeof value === "boolean" ? value : undefined;
}
