// Minimal structural validator for ui-block props. Deliberately not a full
// JSON Schema implementation: builtin components only need object/array/
// primitive/enum checks, and adding a validator dependency for three
// components is not justified. Revisit when custom components (P2) arrive.

export type Schema =
  | { kind: "string"; enum?: readonly string[] }
  | { kind: "number" }
  | { kind: "boolean" }
  | { kind: "cell" }
  | { kind: "any" }
  | { kind: "oneOf"; options: Schema[] }
  | { kind: "array"; items: Schema; minItems?: number }
  | { kind: "object"; fields: Record<string, Schema>; required?: readonly string[] };

export type Issue = { path: string; message: string };

export type ValidationResult<T> = { ok: true; value: T } | { ok: false; issues: Issue[] };

export const s = {
  string: (options?: { enum?: readonly string[] }): Schema => ({ kind: "string", enum: options?.enum }),
  number: (): Schema => ({ kind: "number" }),
  boolean: (): Schema => ({ kind: "boolean" }),
  cell: (): Schema => ({ kind: "cell" }),
  any: (): Schema => ({ kind: "any" }),
  // First option that validates wins; issues are reported only when none does.
  oneOf: (options: Schema[]): Schema => ({ kind: "oneOf", options }),
  array: (items: Schema, options?: { minItems?: number }): Schema => ({ kind: "array", items, minItems: options?.minItems }),
  object: (fields: Record<string, Schema>, required?: readonly string[]): Schema => ({ kind: "object", fields, required }),
};

export function validate<T>(schema: Schema, input: unknown): ValidationResult<T> {
  const issues: Issue[] = [];
  const value = walk(schema, input, "$", issues);
  return issues.length === 0 ? { ok: true, value: value as T } : { ok: false, issues };
}

// Array items that fail validation are dropped instead of failing the whole
// block, so a single malformed row does not hide an otherwise valid list.
function walk(schema: Schema, input: unknown, path: string, issues: Issue[]): unknown {
  switch (schema.kind) {
    case "string": {
      if (typeof input !== "string") {
        issues.push({ path, message: "expected string" });
        return undefined;
      }
      if (schema.enum && !schema.enum.includes(input)) {
        issues.push({ path, message: `expected one of ${schema.enum.join(", ")}` });
        return undefined;
      }
      return input;
    }
    case "number": {
      if (typeof input !== "number" || !Number.isFinite(input)) {
        issues.push({ path, message: "expected number" });
        return undefined;
      }
      return input;
    }
    case "boolean": {
      if (typeof input !== "boolean") {
        issues.push({ path, message: "expected boolean" });
        return undefined;
      }
      return input;
    }
    case "any":
      return input;
    case "oneOf": {
      for (const option of schema.options) {
        const attempt: Issue[] = [];
        const value = walk(option, input, path, attempt);
        if (attempt.length === 0) {
          return value;
        }
      }
      issues.push({ path, message: "matches none of the allowed shapes" });
      return undefined;
    }
    case "cell": {
      if (typeof input === "string" || typeof input === "boolean" || (typeof input === "number" && Number.isFinite(input))) {
        return input;
      }
      issues.push({ path, message: "expected string, number, or boolean" });
      return undefined;
    }
    case "array": {
      if (!Array.isArray(input)) {
        issues.push({ path, message: "expected array" });
        return undefined;
      }
      const kept: unknown[] = [];
      for (let index = 0; index < input.length; index += 1) {
        const itemIssues: Issue[] = [];
        const item = walk(schema.items, input[index], `${path}[${index}]`, itemIssues);
        if (itemIssues.length === 0) {
          kept.push(item);
        }
      }
      if (schema.minItems !== undefined && kept.length < schema.minItems) {
        issues.push({ path, message: `expected at least ${schema.minItems} valid items` });
        return undefined;
      }
      return kept;
    }
    case "object": {
      if (typeof input !== "object" || input === null || Array.isArray(input)) {
        issues.push({ path, message: "expected object" });
        return undefined;
      }
      const source = input as Record<string, unknown>;
      const output: Record<string, unknown> = {};
      for (const [key, fieldSchema] of Object.entries(schema.fields)) {
        const required = schema.required?.includes(key) ?? false;
        if (source[key] === undefined || source[key] === null) {
          if (required) {
            issues.push({ path: `${path}.${key}`, message: "required" });
          }
          continue;
        }
        const fieldIssues: Issue[] = [];
        const fieldValue = walk(fieldSchema, source[key], `${path}.${key}`, fieldIssues);
        if (fieldIssues.length > 0) {
          if (required) {
            issues.push(...fieldIssues);
          }
          continue;
        }
        output[key] = fieldValue;
      }
      return output;
    }
    default:
      return undefined;
  }
}
