"use client";

import type * as React from "react";
import { useLocale, useTranslations } from "next-intl";

import {
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu";
import type {
  AdminLLMModelCatalogResolution,
  AdminLLMModelReasoningLevel,
} from "@/features/admin/api/llm-types";
import {
  ModelCapabilityMenuItem,
  ModelCapabilityRow,
} from "@/features/admin/components/sections/models/models-capability-row";
import {
  fallbackReasoningDefault,
  type ModelReasoningDeclaration,
  reasoningNativeValue,
  sortReasoningLevels,
} from "@/features/admin/model/model-reasoning";

type ModelReasoningFieldProps = {
  /** Explicit `reasoning` declaration from the capabilities JSON; null follows automatic detection. */
  override: ModelReasoningDeclaration | "invalid" | null;
  resolution: AdminLLMModelCatalogResolution | null;
  disabled?: boolean;
  onChange: (declaration: ModelReasoningDeclaration | null) => boolean;
};

/**
 * Reasoning levels offered in the chat model control. Automatic detection is what the server resolves
 * without an explicit declaration (legacy thinking parameters, then the models.dev catalog); a custom
 * value writes the explicit `reasoning` declaration built from the server's template.
 */
export function ModelReasoningField({ override, resolution, disabled = false, onChange }: ModelReasoningFieldProps) {
  const t = useTranslations("adminModels");
  const tLevels = useTranslations("common.reasoningEffort.levels");
  const auto = resolution?.reasoning ?? null;
  const template = resolution?.reasoningTemplate ?? null;
  const custom = override !== null;
  const current = override === "invalid" ? null : (override ?? auto);
  const toggleOnly = template?.format === "toggle";
  const locale = useLocale();
  const customBudgets = override !== "invalid" && override?.format === template?.format ? override?.budgets : undefined;
  const budgets = template?.budgets ? { ...template.budgets, ...customBudgets } : null;

  // Menu label: localized name plus the upstream value in parentheses, e.g. 低（low）or 中（8,192 Token）.
  // The value is omitted when it only repeats the name (English level names).
  function menuLevelLabel(level: AdminLLMModelReasoningLevel): string {
    const name = tLevels(level);
    if (!template) return name;
    const native = reasoningNativeValue(template.format, level, budgets);
    const value = typeof native === "string"
      ? native
      : t("sheet.reasoning.budgetTokens", { value: new Intl.NumberFormat(locale).format(native.budget) });
    return value.toLowerCase() === name.toLowerCase() ? name : t("sheet.reasoning.levelWithValue", { name, value });
  }

  // Levels shown as checked, limited to what the template format can declare; edits start from the same set.
  const selectedLevels = template
    ? (toggleOnly ? template.levels : (current?.levels ?? []).filter((level) => template.levels.includes(level)))
    : [];

  function base(): { levels: AdminLLMModelReasoningLevel[]; defaultLevel: AdminLLMModelReasoningLevel | null } {
    return { levels: [...selectedLevels], defaultLevel: current?.default ?? (toggleOnly ? "high" : null) };
  }

  function commit(levels: AdminLLMModelReasoningLevel[], preferredDefault: AdminLLMModelReasoningLevel | null) {
    if (!template || levels.length === 0) return;
    const sorted = sortReasoningLevels(levels);
    const defaultLevel = preferredDefault && sorted.includes(preferredDefault) ? preferredDefault : fallbackReasoningDefault(sorted);
    if (!defaultLevel) return;
    onChange({
      format: template.format,
      levels: sorted,
      default: defaultLevel,
      ...(budgets ? { budgets } : {}),
    });
  }

  function toggleLevel(level: AdminLLMModelReasoningLevel) {
    const { levels, defaultLevel } = base();
    const next = levels.includes(level) ? levels.filter((item) => item !== level) : [...levels, level];
    // At least one level must stay; the backend rejects an empty declaration.
    if (next.length === 0) return;
    commit(next, defaultLevel);
  }

  function selectDefault(level: AdminLLMModelReasoningLevel) {
    const { levels } = base();
    commit(levels.includes(level) ? levels : [...levels, level], level);
  }

  let value: React.ReactNode;
  if (override === "invalid") {
    value = <span className="truncate text-xs text-destructive">{t("sheet.reasoning.invalid")}</span>;
  } else if (current && current.levels.length > 0) {
    value = (
      <span className="truncate text-xs">
        {current.levels.map((level, index) => (
          <span key={level}>
            {index > 0 ? <span className="text-muted-foreground/60"> · </span> : null}
            <span className={level === current.default ? "font-medium text-secondary-foreground" : "text-muted-foreground"}>
              {tLevels(level)}
            </span>
          </span>
        ))}
      </span>
    );
  } else {
    value = (
      <span className="truncate text-xs text-muted-foreground">
        {resolution?.matched ? t("sheet.reasoning.unsupported") : t("sheet.reasoning.unknown")}
      </span>
    );
  }

  return (
    <ModelCapabilityRow
      label={t("sheet.reasoning.label")}
      description={t("sheet.reasoning.description")}
      editLabel={t("sheet.reasoning.edit")}
      custom={custom}
      disabled={disabled}
      menu={
        <>
          <ModelCapabilityMenuItem role="menuitemradio" checked={!custom} onSelect={() => onChange(null)}>
            <span>{t("sheet.capabilityMode.auto")}</span>
          </ModelCapabilityMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-[11px] font-normal text-muted-foreground">
            {t("sheet.reasoning.customTitle")}
          </DropdownMenuLabel>
          {template ? (
            <>
              {template.levels.map((level) => (
                <ModelCapabilityMenuItem
                  key={level}
                  checked={selectedLevels.includes(level)}
                  disabled={toggleOnly}
                  keepOpen
                  onSelect={() => toggleLevel(level)}
                >
                  <span>{menuLevelLabel(level)}</span>
                </ModelCapabilityMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuSub>
                <DropdownMenuSubTrigger className="focus:bg-accent/40 data-[state=open]:bg-accent/40">
                  <span className="flex min-w-0 flex-1 items-center justify-between gap-3">
                    <span>{t("sheet.reasoning.default")}</span>
                    <span className="text-muted-foreground">{current ? tLevels(current.default) : "—"}</span>
                  </span>
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="min-w-40 p-1.5">
                  {selectedLevels.map((level) => (
                    <ModelCapabilityMenuItem
                      key={level}
                      role="menuitemradio"
                      checked={current?.default === level}
                      onSelect={() => selectDefault(level)}
                    >
                      <span>{menuLevelLabel(level)}</span>
                    </ModelCapabilityMenuItem>
                  ))}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </>
          ) : (
            <p className="px-2 py-1.5 text-xs text-muted-foreground">{t("sheet.reasoning.noTemplate")}</p>
          )}
        </>
      }
    >
      {value}
    </ModelCapabilityRow>
  );
}
