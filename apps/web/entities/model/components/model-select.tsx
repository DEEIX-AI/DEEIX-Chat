"use client";

import type * as React from "react";
import { Sparkles } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import { ModelOptionIcon } from "@/entities/model/components/model-option-icon";
import type { ModelSelectOption } from "@/entities/model/types/model-select";
import { OptionSelect, type OptionSelectValueAlign } from "@/shared/components/option-select";

function ModelSelectIcon({
  option,
  fallbackValue,
}: {
  option?: ModelSelectOption;
  fallbackValue: string;
}) {
  if (!option) {
    return <ModelOptionIcon iconUrl={null} label="" size={14} />;
  }

  if (!option.iconUrl && option.value === fallbackValue) {
    return (
      <span className="inline-flex size-3.5 shrink-0 items-center justify-center self-center text-muted-foreground">
        <Sparkles className="size-3.5 stroke-1" />
        <span className="sr-only">{option.label}</span>
      </span>
    );
  }

  return <ModelOptionIcon iconUrl={option.iconUrl} label={option.label} size={14} />;
}

export function ModelSelect({
  id,
  value,
  fallbackValue,
  disabled,
  options,
  align,
  valueAlign = "responsive-end",
  itemAlign,
  contentClassName = "min-w-[320px]",
  triggerClassName,
  valueClassName,
  portalContainer,
  onChange,
}: {
  id?: string;
  value: string;
  fallbackValue: string;
  disabled?: boolean;
  options: ModelSelectOption[];
  align?: "start" | "center" | "end";
  valueAlign?: OptionSelectValueAlign;
  itemAlign?: OptionSelectValueAlign;
  contentClassName?: string;
  triggerClassName?: string;
  valueClassName?: string;
  portalContainer?: HTMLElement | ShadowRoot | null | React.RefObject<HTMLElement | ShadowRoot | null>;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("common.modelSelect");
  const resolvedItemAlign = itemAlign ?? "start";
  const itemTextClass =
    resolvedItemAlign === "start"
      ? "text-left"
      : resolvedItemAlign === "end"
        ? "text-right"
        : "text-left md:text-right";

  return (
    <OptionSelect
      id={id}
      value={value}
      fallbackValue={fallbackValue}
      disabled={disabled}
      options={options}
      searchPlaceholder={t("searchPlaceholder")}
      emptyText={t("empty")}
      align={align}
      valueAlign={valueAlign}
      itemAlign={resolvedItemAlign}
      contentClassName={contentClassName}
      triggerClassName={triggerClassName}
      valueClassName={valueClassName}
      portalContainer={portalContainer}
      renderIcon={(option) => <ModelSelectIcon option={option} fallbackValue={fallbackValue} />}
      renderOption={(item) => (
        <>
          <ModelSelectIcon option={item} fallbackValue={fallbackValue} />
          <span className={cn("min-w-0 flex-1 truncate leading-5", itemTextClass)}>{item.label}</span>
        </>
      )}
      onChange={onChange}
    />
  );
}
