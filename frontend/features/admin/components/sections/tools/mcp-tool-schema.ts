export type MCPToolSchemaStringArgument = {
  name: string;
  description: string;
  required: boolean;
};

type JSONSchemaObject = Record<string, unknown>;

const MAX_SCHEMA_REFERENCE_DEPTH = 32;

export function toolSchemaArgumentMetadata(inputSchemaJSON: string): {
  requiredArguments: string[];
  stringArguments: MCPToolSchemaStringArgument[];
} {
  try {
    const root = asObject(JSON.parse(inputSchemaJSON));
    const properties = asObject(root?.properties);
    if (!root || !properties) {
      return { requiredArguments: [], stringArguments: [] };
    }
    const requiredArguments = Array.isArray(root.required)
      ? root.required
        .filter((name): name is string => typeof name === "string")
        .map((name) => name.trim())
        .filter(Boolean)
      : [];
    const required = new Set(requiredArguments);
    const stringArguments = Object.entries(properties)
      .filter(([, property]) => schemaNodeAcceptsString(root, property, new Set(), 0))
      .map(([name, property]) => ({
        name,
        description: schemaNodeDescription(root, property, new Set(), 0),
        required: required.has(name),
      }))
      .sort((left, right) => left.name.localeCompare(right.name));
    return { requiredArguments, stringArguments };
  } catch {
    return { requiredArguments: [], stringArguments: [] };
  }
}

function schemaNodeAcceptsString(
  root: JSONSchemaObject,
  value: unknown,
  resolving: Set<string>,
  depth: number,
): boolean {
  if (depth > MAX_SCHEMA_REFERENCE_DEPTH) {
    return false;
  }
  const schema = asObject(value);
  if (!schema) {
    return false;
  }
  if (typeof schema.$ref === "string") {
    const reference = schema.$ref.trim();
    if (!reference || resolving.has(reference)) {
      return false;
    }
    const resolved = resolveLocalReference(root, reference);
    if (resolved === undefined) {
      return false;
    }
    resolving.add(reference);
    const acceptsReference = schemaNodeAcceptsString(root, resolved, resolving, depth + 1);
    resolving.delete(reference);
    if (!acceptsReference) {
      return false;
    }
    const siblings = Object.fromEntries(Object.entries(schema).filter(([key]) => key !== "$ref"));
    return Object.keys(siblings).length === 0 || schemaNodeAcceptsString(root, siblings, resolving, depth + 1);
  }
  if ("type" in schema && !schemaTypeIncludesString(schema.type)) {
    return false;
  }
  if ("const" in schema && typeof schema.const !== "string") {
    return false;
  }
  if ("enum" in schema && (!Array.isArray(schema.enum) || !schema.enum.some((item) => typeof item === "string"))) {
    return false;
  }
  for (const keyword of ["anyOf", "oneOf"] as const) {
    if (keyword in schema) {
      const branches = schema[keyword];
      if (!Array.isArray(branches) || !branches.some((branch) => schemaNodeAcceptsString(root, branch, resolving, depth + 1))) {
        return false;
      }
    }
  }
  if ("allOf" in schema) {
    if (!Array.isArray(schema.allOf) || schema.allOf.length === 0 ||
      !schema.allOf.every((branch) => schemaNodeAcceptsString(root, branch, resolving, depth + 1))) {
      return false;
    }
  }
  return true;
}

function schemaNodeDescription(
  root: JSONSchemaObject,
  value: unknown,
  resolving: Set<string>,
  depth: number,
): string {
  if (depth > MAX_SCHEMA_REFERENCE_DEPTH) {
    return "";
  }
  const schema = asObject(value);
  if (!schema) {
    return "";
  }
  if (typeof schema.description === "string" && schema.description.trim()) {
    return schema.description.trim();
  }
  if (typeof schema.$ref !== "string") {
    return "";
  }
  const reference = schema.$ref.trim();
  if (!reference || resolving.has(reference)) {
    return "";
  }
  const resolved = resolveLocalReference(root, reference);
  if (resolved === undefined) {
    return "";
  }
  resolving.add(reference);
  const description = schemaNodeDescription(root, resolved, resolving, depth + 1);
  resolving.delete(reference);
  return description;
}

function schemaTypeIncludesString(value: unknown): boolean {
  return value === "string" || (Array.isArray(value) && value.includes("string"));
}

function resolveLocalReference(root: JSONSchemaObject, reference: string): unknown {
  if (reference === "#") {
    return root;
  }
  if (!reference.startsWith("#/")) {
    return undefined;
  }
  let current: unknown = root;
  for (const rawSegment of reference.slice(2).split("/")) {
    const segment = rawSegment.replaceAll("~1", "/").replaceAll("~0", "~");
    const object = asObject(current);
    if (!object || !(segment in object)) {
      return undefined;
    }
    current = object[segment];
  }
  return current;
}

function asObject(value: unknown): JSONSchemaObject | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as JSONSchemaObject
    : null;
}
