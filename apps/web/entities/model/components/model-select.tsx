"use client";

import type * as React from "react";
import { Sparkles } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import { ModelOptionIcon } from "@/entities/model/components/model-option-icon";
import type { ModelSelectOption } from "@/entities/model/types/model-select";
import { OptionSelect, type OptionSelectValueAlign } from "@/shared/components/option-select";

// Like the rows of the chat model picker: a row is muted at rest and takes the highlight text color once
// it is hovered, keyboard-highlighted or selected (the plain foreground sits too close to the muted tone
// in the default dark theme). Monochrome icons are painted with the text color, so they follow the row;
// brand-colored icons keep their colors.
const MODEL_OPTION_ITEM_CLASS_NAME = "text-muted-foreground transition-colors data-[selected]:text-accent-foreground";

// Model names are mostly lowercase, whose visual middle (the x-height) sits about 0.8px below the middle
// of the 12px line the icon is centered on; a 1px drop lines the icon up with it and stays on whole pixels.
const MODEL_OPTION_ICON_OPTICAL_CLASS_NAME = "translate-y-px";

function ModelSelectIcon({
  option,
  fallbackValue,
  className,
}: {
  option?: ModelSelectOption;
  fallbackValue: string;
  className?: string;
}) {
  if (!option) {
    return <ModelOptionIcon iconUrl={null} label="" size={14} className={cn(MODEL_OPTION_ICON_OPTICAL_CLASS_NAME, className)} />;
  }

  if (!option.iconUrl && option.value === fallbackValue) {
    return (
      <span
        className={cn(
          "inline-flex size-3.5 shrink-0 items-center justify-center self-center text-muted-foreground",
          MODEL_OPTION_ICON_OPTICAL_CLASS_NAME,
          className,
        )}
      >
        <Sparkles className="size-3.5 stroke-1" />
        <span className="sr-only">{option.label}</span>
      </span>
    );
  }

  return (
    <ModelOptionIcon
      iconUrl={option.iconUrl}
      label={option.label}
      size={14}
      className={cn(MODEL_OPTION_ICON_OPTICAL_CLASS_NAME, className)}
    />
  );
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
      itemClassName={MODEL_OPTION_ITEM_CLASS_NAME}
      portalContainer={portalContainer}
      renderIcon={(option) => <ModelSelectIcon option={option} fallbackValue={fallbackValue} />}
      renderOption={(item) => (
        <>
          <ModelSelectIcon option={item} fallbackValue={fallbackValue} className="text-current" />
          <span className={cn("min-w-0 flex-1 truncate leading-5", itemTextClass)}>{item.label}</span>
        </>
      )}
      onChange={onChange}
    />
  );
}
