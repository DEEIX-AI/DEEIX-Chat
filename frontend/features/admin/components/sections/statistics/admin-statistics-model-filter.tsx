"use client";

import { Bot, Sparkles } from "lucide-react";
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
} from "@/components/ui/combobox";
import { cn } from "@/lib/utils";
import type { ModelSelectOption } from "@/shared/components/model-select";
import { ModelOptionIcon } from "@/shared/components/model-option-icon";

export function AdminStatisticsModelFilter({
  value,
  fallbackValue,
  options,
  label,
  triggerClassName,
  disabled = false,
  onChange,
}: {
  value: string;
  fallbackValue: string;
  options: ModelSelectOption[];
  label: string;
  triggerClassName?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("common.modelSelect");
  const selectedOption = options.find((option) => option.value === value) ?? options.find((option) => option.value === fallbackValue);

  return (
    <Combobox
      items={options}
      value={selectedOption ?? null}
      disabled={disabled}
      itemToStringLabel={(option) => option.label}
      isItemEqualToValue={(option, selected) => option.value === selected.value}
      onValueChange={(option) => onChange(option?.value ?? fallbackValue)}
    >
      <ComboboxTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            disabled={disabled}
            className={cn(
              "h-8 w-full min-w-0 justify-start gap-2 rounded-md px-2 text-xs font-normal shadow-none data-[popup-open]:bg-accent/50 [&_[data-slot=combobox-trigger-icon]]:ml-0 [&_[data-slot=combobox-trigger-icon]]:size-3.5 [&_[data-slot=combobox-trigger-icon]]:-rotate-90",
              triggerClassName,
            )}
          >
            <Bot className="size-3.5 shrink-0 text-muted-foreground" />
            <span>{label}</span>
            <span className="ml-auto max-w-32 truncate text-[11px] text-muted-foreground">
              {selectedOption?.label ?? t("empty")}
            </span>
          </Button>
        }
      />
      <ComboboxContent side="right" align="start" sideOffset={8} className="w-[min(300px,calc(100vw-32px))]">
        <ComboboxInput placeholder={t("searchPlaceholder")} showTrigger={false} showClear={false} disabled={disabled} />
        <ComboboxEmpty>{t("empty")}</ComboboxEmpty>
        <ComboboxList>
          {(option: ModelSelectOption) => (
            <ComboboxItem key={option.value} value={option} className="h-8 py-0">
              {option.value === fallbackValue ? (
                <span className="flex size-4 shrink-0 items-center justify-center text-foreground">
                  <Sparkles className="size-4" strokeWidth={1.8} />
                </span>
              ) : (
                <ModelOptionIcon iconUrl={option.iconUrl} label={option.label} />
              )}
              <span className={cn("min-w-0 flex-1 truncate", option.value === fallbackValue && "font-medium")}>
                {option.label}
              </span>
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}
