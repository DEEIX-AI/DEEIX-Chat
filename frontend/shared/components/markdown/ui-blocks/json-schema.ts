import { type Schema, s } from "./schema";

// Converts the JSON Schema subset custom components declare into the internal
// validator shape. Unsupported constructs degrade to permissive checks rather
// than rejecting the component: authors get the structure they described,
// and anything exotic is simply not validated.
export function schemaFromJSONSchema(raw: string): Schema | null {
  const text = raw.trim();
  if (text === "") {
    return null;
  }
  let json: unknown;
  try {
    json = JSON.parse(text);
  } catch {
    return null;
  }
  return convert(json, 0);
}

const MAX_DEPTH = 8;

function convert(node: unknown, depth: number): Schema | null {
  if (typeof node !== "object" || node === null || Array.isArray(node) || depth > MAX_DEPTH) {
    return null;
  }
  const schema = node as Record<string, unknown>;
  // anyOf / oneOf map onto the validator's union; unsupported branches are dropped.
  const union = Array.isArray(schema.anyOf) ? schema.anyOf : Array.isArray(schema.oneOf) ? schema.oneOf : undefined;
  if (union) {
    const options = union.map((option) => convert(option, depth + 1)).filter((option): option is Schema => option !== null);
    return options.length > 0 ? s.oneOf(options) : null;
  }
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  const enumValues = Array.isArray(schema.enum) ? schema.enum.filter((value): value is string => typeof value === "string") : undefined;

  switch (type) {
    case "string":
      return s.string(enumValues && enumValues.length > 0 ? { enum: enumValues } : undefined);
    case "number":
    case "integer":
      return s.number();
    case "boolean":
      return s.boolean();
    case "array": {
      const items = convert(schema.items, depth + 1) ?? s.cell();
      const minItems = typeof schema.minItems === "number" ? schema.minItems : undefined;
      return s.array(items, minItems !== undefined ? { minItems } : undefined);
    }
    case "object": {
      const properties = typeof schema.properties === "object" && schema.properties !== null ? (schema.properties as Record<string, unknown>) : {};
      const fields: Record<string, Schema> = {};
      for (const [key, value] of Object.entries(properties)) {
        const converted = convert(value, depth + 1);
        if (converted) {
          fields[key] = converted;
        }
      }
      const required = Array.isArray(schema.required) ? schema.required.filter((value): value is string => typeof value === "string") : undefined;
      return s.object(fields, required);
    }
    default:
      return enumValues && enumValues.length > 0 ? s.string({ enum: enumValues }) : null;
  }
}
