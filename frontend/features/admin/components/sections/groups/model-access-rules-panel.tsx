"use client";

import * as React from "react";
import { X } from "lucide-react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { AdminLLMUpstreamView } from "@/features/admin/api/llm.types";
import { ADAPTER_LABELS } from "@/features/admin/types/llm";
import type {
  PermissionGroupModelRule,
  PermissionGroupModelRuleType,
} from "@/features/admin/api/permission-groups";

type ModelAccessRulesPanelProps = {
  rules: PermissionGroupModelRule[];
  onRulesChange: React.Dispatch<React.SetStateAction<PermissionGroupModelRule[]>>;
  upstreamOptions: AdminLLMUpstreamView[];
  vendorOptions: Array<{ label: string; value: string }>;
  disabled: boolean;
};

export function ModelAccessRulesPanel({
  rules,
  onRulesChange,
  upstreamOptions,
  vendorOptions,
  disabled,
}: ModelAccessRulesPanelProps) {
  const t = useTranslations("adminGroups");
  const [draftType, setDraftType] = React.useState<PermissionGroupModelRuleType>("all");
  const [draftValue, setDraftValue] = React.useState("");

  const protocolOptions = React.useMemo(
    () => Object.entries(ADAPTER_LABELS).map(([value, label]) => ({ label, value })),
    [],
  );
  const upstreamSelectOptions = React.useMemo(
    () => upstreamOptions.map((upstream) => ({ label: upstream.name, value: String(upstream.id) })),
    [upstreamOptions],
  );
  const valueOptions = React.useMemo(() => {
    switch (draftType) {
      case "vendor":
        return vendorOptions;
      case "protocol":
        return protocolOptions;
      case "upstream":
        return upstreamSelectOptions;
      default:
        return [];
    }
  }, [draftType, protocolOptions, upstreamSelectOptions, vendorOptions]);
  const labelByValue = React.useMemo(() => {
    const map = new Map<string, string>();
    for (const option of [...vendorOptions, ...protocolOptions, ...upstreamSelectOptions]) {
      map.set(option.value, option.label);
    }
    return map;
  }, [protocolOptions, upstreamSelectOptions, vendorOptions]);
  const hasAllRule = rules.some((rule) => rule.type === "all");
  const requiresValue = draftType !== "all";
  const normalizedDraftValue = requiresValue ? draftValue : "";
  const draftKey = `${draftType}:${normalizedDraftValue}`;
  const duplicate = rules.some((rule) => `${rule.type}:${rule.value}` === draftKey);
  const canAdd =
    !disabled &&
    !duplicate &&
    (!hasAllRule || draftType === "all") &&
    (!requiresValue || normalizedDraftValue.trim() !== "");

  React.useEffect(() => {
    setDraftValue("");
  }, [draftType]);

  const addRule = React.useCallback(() => {
    if (!canAdd) {
      return;
    }
    const nextRule: PermissionGroupModelRule = {
      type: draftType,
      value: normalizedDraftValue,
    };
    onRulesChange((current) => {
      if (nextRule.type === "all") {
        return [nextRule];
      }
      if (current.some((rule) => rule.type === "all")) {
        return current;
      }
      if (current.some((rule) => rule.type === nextRule.type && rule.value === nextRule.value)) {
        return current;
      }
      return [...current, nextRule];
    });
  }, [canAdd, draftType, normalizedDraftValue, onRulesChange]);

  const removeRule = React.useCallback((index: number) => {
    onRulesChange((current) => current.filter((_, currentIndex) => currentIndex !== index));
  }, [onRulesChange]);

  const ruleLabel = React.useCallback(
    (rule: PermissionGroupModelRule) => {
      switch (rule.type) {
        case "all":
          return t("ruleAllModels");
        case "vendor":
          return `${t("ruleVendor")}: ${labelByValue.get(rule.value) ?? rule.value}`;
        case "protocol":
          return `${t("ruleProtocol")}: ${labelByValue.get(rule.value) ?? rule.value}`;
        case "upstream":
          return `${t("ruleUpstream")}: ${labelByValue.get(rule.value) ?? rule.value}`;
        default:
          return rule.value;
      }
    },
    [labelByValue, t],
  );

  return (
    <div className="space-y-2 rounded-md bg-muted/30 px-3 py-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <p className="mr-auto text-xs font-medium text-foreground">{t("autoRules")}</p>
        <Select
          value={draftType}
          disabled={disabled}
          onValueChange={(value) => setDraftType(value as PermissionGroupModelRuleType)}
        >
          <SelectTrigger size="xs" className="h-7 w-[140px] bg-background text-[11px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="end" className="z-[100]">
            <SelectItem value="all" className="text-[11px]">{t("ruleAllModels")}</SelectItem>
            <SelectItem value="vendor" className="text-[11px]">{t("ruleVendor")}</SelectItem>
            <SelectItem value="protocol" className="text-[11px]">{t("ruleProtocol")}</SelectItem>
            <SelectItem value="upstream" className="text-[11px]">{t("ruleUpstream")}</SelectItem>
          </SelectContent>
        </Select>
        {requiresValue ? (
          <Select
            value={draftValue}
            disabled={disabled || valueOptions.length === 0 || hasAllRule}
            onValueChange={setDraftValue}
          >
            <SelectTrigger size="xs" className="h-7 w-[156px] bg-background text-[11px]">
              <SelectValue placeholder={t("selectRuleValue")} />
            </SelectTrigger>
            <SelectContent
              position="popper"
              align="end"
              className="z-[100]"
              viewportClassName="max-h-[220px]"
            >
              {valueOptions.map((option) => (
                <SelectItem key={`${draftType}:${option.value}`} value={option.value} className="text-[11px]">
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : null}
        <Button
          type="button"
          size="sm"
          variant="secondary"
          className="h-7 px-2 text-xs shadow-none"
          disabled={!canAdd}
          onClick={addRule}
        >
          {t("addRule")}
        </Button>
      </div>

      {rules.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {rules.map((rule, index) => (
            <Badge
              key={`${rule.type}:${rule.value}:${index}`}
              variant="secondary"
              className="gap-1 rounded-sm px-1.5 py-0 text-[10px] font-normal leading-5"
            >
              <span>{ruleLabel(rule)}</span>
              <button
                type="button"
                className="rounded-sm text-muted-foreground hover:text-foreground disabled:opacity-50"
                disabled={disabled}
                onClick={() => removeRule(index)}
                aria-label={t("removeRule")}
              >
                <X className="size-3 stroke-1" />
              </button>
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-[11px] text-muted-foreground">{t("noAutoRules")}</p>
      )}
    </div>
  );
}
