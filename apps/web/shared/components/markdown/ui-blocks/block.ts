import type { Issue, Schema, ValidationResult } from "./schema";
import { s, validate } from "./schema";

export const UI_BLOCK_FENCE_LANGUAGE = "deeix-ui";

export type UIBlockEnvelope = {
  component: string;
  version: number;
  id: string;
  props: unknown;
  state?: unknown;
};

// version is resolved by the renderer and defaults to 1; the model only emits
// it when the catalog announces a breaking version for a component.
const ENVELOPE_SCHEMA: Schema = s.object(
  {
    component: s.string(),
    version: s.number(),
    id: s.string(),
    // Props are validated per component; here they only need to be an object.
    props: s.any(),
    state: s.any(),
  },
  ["component", "id", "props"],
);

export type ParsedUIBlock =
  | { status: "incomplete" }
  | { status: "invalid"; issues: Issue[]; raw: string }
  | { status: "ok"; envelope: UIBlockEnvelope; raw: string };

// The fenced body streams in token by token; until it parses as JSON it is
// treated as incomplete rather than invalid so the skeleton stays up. Once the
// braces balance the body is complete, so a parse failure is reported even
// while the message is still streaming instead of leaving the skeleton forever.
export function parseUIBlock(raw: string, streaming: boolean): ParsedUIBlock {
  const text = raw.trim();
  if (text === "") {
    return { status: "incomplete" };
  }
  let json: unknown;
  try {
    json = JSON.parse(text);
  } catch {
    try {
      json = JSON.parse(repairJSON(text));
    } catch (error) {
      if (streaming && !bracesBalanced(text)) {
        return { status: "incomplete" };
      }
      return { status: "invalid", issues: [{ path: "$", message: error instanceof Error ? error.message : "invalid JSON" }], raw };
    }
  }
  const normalized = normalizeEnvelope(json);
  const shape = validate<Omit<UIBlockEnvelope, "props" | "state">>(ENVELOPE_SCHEMA, normalized);
  if (!shape.ok) {
    return { status: "invalid", issues: shape.issues, raw };
  }
  const source = normalized as Record<string, unknown>;
  if (typeof source.props !== "object" || source.props === null || Array.isArray(source.props)) {
    return { status: "invalid", issues: [{ path: "$.props", message: "expected object" }], raw };
  }
  return {
    status: "ok",
    raw,
    envelope: {
      component: shape.value.component,
      version: shape.value.version,
      id: shape.value.id,
      props: source.props,
      state: source.state,
    },
  };
}

// Models emit three common JSON deviations: trailing commas, // or /* */
// comments, and unescaped double quotes inside strings (typical when Chinese
// prose is emitted with ASCII quotes). A quote inside a string is treated as
// the closing quote only when the next significant character is structural.
const STRUCTURAL_AFTER_STRING = new Set([",", "}", "]", ":"]);

function repairJSON(text: string): string {
  let out = "";
  let inString = false;
  let index = 0;
  while (index < text.length) {
    const char = text[index] ?? "";
    if (inString) {
      if (char === "\\") {
        out += char + (text[index + 1] ?? "");
        index += 2;
        continue;
      }
      if (char === '"') {
        let look = index + 1;
        while (look < text.length && /\s/.test(text[look] ?? "")) {
          look += 1;
        }
        const next = text[look];
        if (next === undefined || STRUCTURAL_AFTER_STRING.has(next)) {
          inString = false;
          out += char;
        } else {
          out += '\\"';
        }
        index += 1;
        continue;
      }
      out += char;
      index += 1;
      continue;
    }
    if (char === '"') {
      inString = true;
      out += char;
      index += 1;
    } else if (char === "/" && text[index + 1] === "/") {
      const end = text.indexOf("\n", index);
      index = end === -1 ? text.length : end;
    } else if (char === "/" && text[index + 1] === "*") {
      const end = text.indexOf("*/", index + 2);
      index = end === -1 ? text.length : end + 2;
    } else if (char === ",") {
      // Drop the comma if the next significant character closes a container.
      let look = index + 1;
      while (look < text.length && /\s/.test(text[look] ?? "")) {
        look += 1;
      }
      const next = text[look];
      if (next !== "}" && next !== "]") {
        out += char;
      }
      index += 1;
    } else {
      out += char;
      index += 1;
    }
  }
  return out;
}

function bracesBalanced(text: string): boolean {
  let depth = 0;
  let inString = false;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (inString) {
      if (char === "\\") {
        index += 1;
      } else if (char === '"') {
        inString = false;
      }
      continue;
    }
    if (char === '"') {
      inString = true;
    } else if (char === "{" || char === "[") {
      depth += 1;
    } else if (char === "}" || char === "]") {
      depth -= 1;
    }
  }
  return depth === 0 && !inString;
}

// Models routinely quote numbers ("1") or omit the version. Both are
// unambiguous, so normalise them instead of failing the whole block.
function normalizeEnvelope(json: unknown): unknown {
  if (typeof json !== "object" || json === null || Array.isArray(json)) {
    return json;
  }
  const source = { ...(json as Record<string, unknown>) };
  if (source.version === undefined || source.version === null) {
    source.version = 1;
  } else if (typeof source.version === "string") {
    // "1", "v1", "V1", "1.0" all mean 1.
    const match = /^v?(\d+)(?:\.\d+)?$/i.exec(source.version.trim());
    if (match) {
      source.version = Number.parseInt(match[1] ?? "1", 10);
    }
  } else if (typeof source.version === "number" && !Number.isInteger(source.version)) {
    source.version = Math.trunc(source.version);
  }
  if (typeof source.id === "number") {
    source.id = String(source.id);
  }
  return source;
}

export type UIBlockRenderProps<P> = {
  id: string;
  props: P;
  definition: UIBlockDefinition<P>;
};

export type UIBlockSkeletonProps = { items?: number };

export type UIBlockDefinition<P = unknown> = {
  name: string;
  version: number;
  schema: Schema;
  Component: React.ComponentType<UIBlockRenderProps<P>>;
  // Skeletons get a live item count read from the streaming JSON so they can
  // reserve the component's real height row by row.
  Skeleton: React.ComponentType<UIBlockSkeletonProps>;
  // Present for sandbox-rendered components; the module-level SandboxComponent
  // reads it from the definition instead of closing over it.
  sandbox?: { source: string; title: string };
};

export type UIBlockRegistry = Map<string, UIBlockDefinition>;

function registryKey(name: string, version: number): string {
  return `${name}@${version}`;
}

export function createRegistry(definitions: readonly UIBlockDefinition[]): UIBlockRegistry {
  return new Map(definitions.map((definition) => [registryKey(definition.name, definition.version), definition]));
}

export function resolveDefinition(registry: UIBlockRegistry, name: string, version: number): UIBlockDefinition | undefined {
  return registry.get(registryKey(name, version));
}

export function validateProps<P>(definition: UIBlockDefinition<P>, props: unknown): ValidationResult<P> {
  return validate<P>(definition.schema, props);
}
