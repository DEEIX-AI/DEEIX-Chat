"use client";

import * as React from "react";
import { Sparkles } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
  ComboboxValue,
} from "@/components/ui/combobox";
import { cn } from "@/lib/utils";
import { ModelOptionIcon } from "@/shared/components/model-option-icon";

export type OptionSelectOption = {
  label: string;
  value: string;
};

export type ModelSelectOption = OptionSelectOption & {
  iconUrl?: string | null;
};

type OptionSelectAlign = "start" | "end";
type OptionSelectValueAlign = OptionSelectAlign | "responsive-end";

type OptionSelectProps<TOption extends OptionSelectOption> = {
  id?: string;
  value: string;
  options: TOption[];
  disabled?: boolean;
  fallbackValue?: string;
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  align?: "start" | "center" | "end";
  valueAlign?: OptionSelectValueAlign;
  itemAlign?: OptionSelectValueAlign;
  contentClassName?: string;
  triggerClassName?: string;
  valueClassName?: string;
  portalContainer?: HTMLElement | ShadowRoot | null | React.RefObject<HTMLElement | ShadowRoot | null>;
  renderIcon?: (option: TOption | undefined) => React.ReactNode;
  renderOption?: (option: TOption) => React.ReactNode;
  onChange: (value: string) => void;
};

export function OptionSelect<TOption extends OptionSelectOption>({
  id,
  value,
  options,
  disabled,
  fallbackValue,
  placeholder,
  searchPlaceholder,
  emptyText,
  align,
  valueAlign = "responsive-end",
  itemAlign = "start",
  contentClassName = "min-w-[320px]",
  triggerClassName,
  valueClassName,
  portalContainer,
  renderIcon,
  renderOption,
  onChange,
}: OptionSelectProps<TOption>) {
  const t = useTranslations("common.select");
  const normalizedValue = value.trim();
  const resolvedPlaceholder = placeholder ?? t("placeholder");
  const resolvedSearchPlaceholder = searchPlaceholder ?? t("searchPlaceholder");
  const resolvedEmptyText = emptyText ?? t("empty");
  const hasCurrentValue = !normalizedValue || options.some((item) => item.value === normalizedValue);
  const selectedValue = hasCurrentValue ? normalizedValue || fallbackValue : fallbackValue;
  const resolvedItemAlign = itemAlign ?? "start";
  const contentAlign = align ?? (valueAlign === "end" ? "end" : "start");
  const valueJustifyClass =
    valueAlign === "start" ? "justify-start" : valueAlign === "end" ? "justify-end" : "justify-start md:justify-end";
  const valueTextClass =
    valueAlign === "start" ? "text-left" : valueAlign === "end" ? "text-right" : "text-left md:text-right";
  const itemClass =
    resolvedItemAlign === "start"
      ? "text-left"
      : resolvedItemAlign === "end"
        ? "justify-end text-right"
        : "justify-start text-left md:justify-end md:text-right";
  const itemTextClass =
    resolvedItemAlign === "start"
      ? "text-left"
      : resolvedItemAlign === "end"
        ? "text-right"
        : "text-left md:text-right";

  React.useEffect(() => {
    if (fallbackValue && !disabled && normalizedValue && !hasCurrentValue && normalizedValue !== fallbackValue) {
      onChange(fallbackValue);
    }
  }, [disabled, fallbackValue, hasCurrentValue, normalizedValue, onChange]);

  const selectedItem = React.useMemo(() => {
    if (!selectedValue) return undefined;
    return options.find((item) => item.value === selectedValue);
  }, [options, selectedValue]);

  return (
    <Combobox
      id={id}
      items={options}
      value={selectedItem}
      onValueChange={(item) => onChange(item?.value ?? fallbackValue ?? "")}
      itemToStringLabel={(item) => item?.label ?? ""}
      disabled={disabled}
    >
      <ComboboxTrigger
        render={
          <Button
            type="button"
            variant="outline"
            className={cn(
              "w-full justify-between border-input/40 bg-transparent px-3 py-1 font-normal hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 dark:border-input/40 dark:bg-input/30 dark:hover:bg-input/30 [&_[data-slot=combobox-trigger-icon]]:size-3 [&_[data-slot=combobox-trigger-icon]]:opacity-50",
              triggerClassName,
            )}
            disabled={disabled}
          >
            <span className={cn("flex min-w-0 flex-1 items-center gap-2", valueJustifyClass)}>
              {renderIcon?.(selectedItem)}
              <span
                className={cn(
                  "min-w-0 truncate leading-5",
                  valueTextClass,
                  selectedItem ? "text-foreground" : "text-muted-foreground",
                  valueClassName,
                )}
              >
                {selectedItem ? <ComboboxValue /> : resolvedPlaceholder}
              </span>
            </span>
          </Button>
        }
      />
      <ComboboxContent align={contentAlign} className={contentClassName} portalContainer={portalContainer}>
        <ComboboxInput placeholder={resolvedSearchPlaceholder} showTrigger={false} showClear={false} disabled={disabled} />
        <ComboboxEmpty>{resolvedEmptyText}</ComboboxEmpty>
        <ComboboxList>
          {(item: TOption) => (
            <ComboboxItem
              key={item.value}
              value={item}
              className={cn(itemClass)}
            >
              {renderOption ? (
                renderOption(item)
              ) : (
                <span className={cn("min-w-0 flex-1 truncate leading-5", itemTextClass)}>
                  {item.label}
                </span>
              )}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}

function ModelSelectIcon({
  option,
  fallbackValue,
}: {
  option?: ModelSelectOption;
  fallbackValue: string;
}) {
  if (!option) {
    return <ModelOptionIcon iconUrl={null} label="" />;
  }

  if (!option.iconUrl && option.value === fallbackValue) {
    return (
      <span className="inline-flex size-4 shrink-0 items-center justify-center self-center text-foreground">
        <Sparkles className="size-4" strokeWidth={1.8} />
        <span className="sr-only">{option.label}</span>
      </span>
    );
  }

  return <ModelOptionIcon iconUrl={option.iconUrl} label={option.label} />;
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
