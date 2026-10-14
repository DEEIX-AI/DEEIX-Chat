// Layout and control primitives of a live block (`panel` and its children).
// The backend prompt lists the same names (backend/internal/application/
// uicomponent); a custom component may not reuse one.

const UI_CONTAINER_PRIMITIVES = ["panel", "row", "stack", "grid", "card", "tabs", "collapse"] as const;

const UI_LEAF_PRIMITIVES = [
  "divider",
  "text",
  "stat",
  "badge",
  "progress",
  "callout",
  "formula",
  "scene",
  "slider",
  "select",
  "segmented",
  "switch",
  "input",
  "button",
] as const;

export type UIPrimitive = (typeof UI_CONTAINER_PRIMITIVES)[number] | (typeof UI_LEAF_PRIMITIVES)[number];

export const UI_PRIMITIVES: readonly UIPrimitive[] = [...UI_CONTAINER_PRIMITIVES, ...UI_LEAF_PRIMITIVES];

const PRIMITIVE_SET: ReadonlySet<string> = new Set(UI_PRIMITIVES);
const CONTAINER_SET: ReadonlySet<string> = new Set(UI_CONTAINER_PRIMITIVES);

export function isUIPrimitive(name: string): name is UIPrimitive {
  return PRIMITIVE_SET.has(name);
}

// Containers render as soon as they open; everything else waits for its
// closing brace while streaming, so half-written props never flash.
export function isUIContainer(name: string): boolean {
  return CONTAINER_SET.has(name);
}
