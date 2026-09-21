import type { UIComponentDTO, WriteUIComponentRequest } from "@/shared/api/ui-components.types";

export const UI_COMPONENT_LIMITS = {
  name: 64,
  description: 256,
  propsSummary: 1024,
  propsSchema: 16 * 1024,
  rendererSource: 256 * 1024,
} as const;

// Mirrors the backend pattern: kebab-case identifier used both as the prompt
// marker and the frontend dispatch key.
const NAME_PATTERN = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/;

export type UIComponentFormValue = {
  id?: number;
  scope?: UIComponentDTO["scope"];
  name: string;
  description: string;
  propsSummary: string;
  propsSchema: string;
  rendererSource: string;
  enabled: boolean;
};

export const DEFAULT_RENDERER_SOURCE = `<div id="root" style="padding:12px;font:14px/1.5 var(--font-sans);color:var(--foreground)"></div>
<script>
  const root = document.getElementById("root");
  deeix.onProps((props) => {
    root.textContent = JSON.stringify(props, null, 2);
  });
</script>`;

export const EMPTY_UI_COMPONENT_FORM: UIComponentFormValue = {
  name: "",
  description: "",
  propsSummary: "",
  propsSchema: "",
  rendererSource: DEFAULT_RENDERER_SOURCE,
  enabled: true,
};

export function isValidUIComponentName(value: string): boolean {
  return NAME_PATTERN.test(value.trim());
}

export function uiComponentFormFromDTO(item: UIComponentDTO): UIComponentFormValue {
  return {
    id: item.id,
    scope: item.scope,
    name: item.name,
    description: item.description,
    propsSummary: item.propsSummary,
    propsSchema: item.propsSchema,
    rendererSource: item.rendererSource,
    enabled: item.enabled,
  };
}

export function uiComponentPayloadFromForm(form: UIComponentFormValue): WriteUIComponentRequest {
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    propsSummary: form.propsSummary.trim(),
    propsSchema: form.propsSchema.trim(),
    rendererSource: form.rendererSource,
    enabled: form.enabled,
    sortOrder: 0,
  };
}

export function uiComponentFormIsComplete(form: UIComponentFormValue): boolean {
  return isValidUIComponentName(form.name) && form.description.trim() !== "" && form.propsSummary.trim() !== "" && form.rendererSource.trim() !== "";
}

export function uiComponentFormIsWithinLimits(form: UIComponentFormValue): boolean {
  return (
    form.name.trim().length <= UI_COMPONENT_LIMITS.name &&
    form.description.trim().length <= UI_COMPONENT_LIMITS.description &&
    form.propsSummary.trim().length <= UI_COMPONENT_LIMITS.propsSummary &&
    form.propsSchema.length <= UI_COMPONENT_LIMITS.propsSchema &&
    form.rendererSource.length <= UI_COMPONENT_LIMITS.rendererSource
  );
}

export function uiComponentSchemaIsValid(value: string): boolean {
  const text = value.trim();
  if (text === "") {
    return true;
  }
  try {
    const parsed: unknown = JSON.parse(text);
    return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed);
  } catch {
    return false;
  }
}
