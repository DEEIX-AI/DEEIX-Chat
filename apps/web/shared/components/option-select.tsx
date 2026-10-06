"use client";

import * as React from "react";
import { ChevronDownIcon } from "lucide-react";
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

export type OptionSelectOption = {
  label: string;
  value: string;
};

type OptionSelectAlign = "start" | "end";
export type OptionSelectValueAlign = OptionSelectAlign | "responsive-end";

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
  /** Extra classes on every list item, e.g. a group name the rendered option styles against. */
  itemClassName?: string;
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
  itemClassName,
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
    if (fallbackValue !== undefined && !disabled && normalizedValue && !hasCurrentValue && normalizedValue !== fallbackValue) {
      onChange(fallbackValue);
    }
  }, [disabled, fallbackValue, hasCurrentValue, normalizedValue, onChange]);

  const selectedItem = React.useMemo(() => {
    return options.find((item) => item.value === selectedValue);
  }, [options, selectedValue]);

  return (
    <Combobox
      id={id}
      items={options}
      value={selectedItem}
      onValueChange={(item) => onChange(item?.value ?? fallbackValue ?? "")}
      itemToStringLabel={(item) => item?.label ?? ""}
      itemToStringValue={(item) => item?.value ?? ""}
      isItemEqualToValue={(item, selected) => item.value === selected.value}
      disabled={disabled}
    >
      <ComboboxTrigger
        render={
          <Button
            type="button"
            variant="outline"
            className={cn(
              "w-full justify-between border-input/40 bg-transparent px-3 py-1 font-normal hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 dark:border-input/40 dark:bg-input/30 dark:hover:bg-input/30",
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
            {/* The trigger's own chevron is dropped when it renders through this Button, so it is drawn here,
                exactly like the Select trigger's: 12px, muted, half opacity. */}
            <ChevronDownIcon
              data-slot="combobox-trigger-icon"
              className="size-3 shrink-0 text-muted-foreground opacity-50"
              aria-hidden="true"
            />
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
              className={cn(itemClass, itemClassName)}
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
