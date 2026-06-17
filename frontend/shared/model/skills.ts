import type { SkillDTO, WriteSkillRequest } from "@/shared/api/skills.types";

export const SKILL_LIMITS = {
  name: 64,
  description: 256,
  markdown: 10000,
} as const;

export type SkillFormValue = {
  id?: number;
  name: string;
  description: string;
  markdown: string;
  enabled: boolean;
};

export const EMPTY_SKILL_FORM: SkillFormValue = {
  name: "",
  description: "",
  markdown: "",
  enabled: true,
};

export function normalizeSkillName(value: string): string {
  return value
    .trim()
    .replace(/^\/+/, "")
    .trim();
}

export function skillFormFromDTO(item: SkillDTO): SkillFormValue {
  return {
    id: item.id,
    name: item.trigger || item.title,
    description: item.description,
    markdown: item.markdown,
    enabled: item.enabled,
  };
}

export function skillPayloadFromForm(form: SkillFormValue): WriteSkillRequest {
  const name = normalizeSkillName(form.name);
  return {
    title: name,
    trigger: name,
    description: form.description.trim(),
    markdown: form.markdown.trim(),
    enabled: form.enabled,
    sortOrder: 0,
  };
}

export function skillPayloadIsComplete(payload: WriteSkillRequest): boolean {
  return Boolean(payload.title && payload.trigger && payload.markdown);
}

export function skillFormIsWithinLimits(form: SkillFormValue): boolean {
  return (
    normalizeSkillName(form.name).length <= SKILL_LIMITS.name &&
    form.description.trim().length <= SKILL_LIMITS.description &&
    form.markdown.trim().length <= SKILL_LIMITS.markdown
  );
}
