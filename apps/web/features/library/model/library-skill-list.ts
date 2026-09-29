import type { SkillDTO, SkillSummaryDTO } from "@/shared/api/skills-types";

export type SkillListItem = SkillDTO | SkillSummaryDTO;

export function skillKey(item: SkillListItem): string {
  return `${item.scope}-${item.id}`;
}

export function hasSkillMarkdown(item: SkillListItem): item is SkillDTO {
  return "markdown" in item;
}

export function orderSkills<T extends SkillListItem>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    const rank = (item: SkillListItem) => {
      if (!item.enabled) return 2;
      return item.scope === "builtin" ? 1 : 0;
    };
    return rank(a) - rank(b) || a.sortOrder - b.sortOrder || b.id - a.id;
  });
}

export function skillMatchesQuery(item: SkillListItem, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  return [item.title, item.trigger, item.description].join(" ").toLowerCase().includes(normalized);
}
