"use client";

import { useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  ModelCapabilityMenuItem,
  ModelCapabilityRow,
} from "@/features/admin/components/sections/models/models-capability-row";
import {
  isValidModelContextWindow,
  MODEL_CONTEXT_WINDOW_PRESETS,
} from "@/features/admin/model/model-context-window";

type ModelContextWindowFieldProps = {
  value: number | null;
  effectiveValue: number;
  disabled?: boolean;
  onChange: (value: number | null) => boolean;
};

/**
 * Context window row. In automatic mode the detected value is shown; typing a number or picking a
 * preset switches to a custom value written to the capabilities JSON.
 */
export function ModelContextWindowField({
  value,
  effectiveValue,
  disabled = false,
  onChange,
}: ModelContextWindowFieldProps) {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const focusInputAfterMenuCloseRef = useRef(false);
  const [inputValue, setInputValue] = useState(value === null ? "" : String(value));

  useEffect(() => {
    setInputValue(value === null ? "" : String(value));
  }, [value]);

  function commitInput() {
    const normalized = inputValue.trim();
    if (!normalized) {
      if (!onChange(null)) {
        setInputValue(value === null ? "" : String(value));
      }
      return;
    }
    const parsed = Number(normalized);
    if (!isValidModelContextWindow(parsed)) {
      toast.error(t("sheet.contextWindowInvalid"));
      setInputValue(value === null ? "" : String(value));
      return;
    }
    if (!onChange(parsed)) {
      setInputValue(value === null ? "" : String(value));
    }
  }

  function selectPreset(nextValue: number | null) {
    if (onChange(nextValue)) {
      setInputValue(nextValue === null ? "" : String(nextValue));
    }
  }

  return (
    <ModelCapabilityRow
      label={t("sheet.contextWindow")}
      description={t("sheet.contextWindowDescription")}
      editLabel={t("sheet.contextWindowPresets")}
      custom={value !== null}
      disabled={disabled}
      interactiveValue
      menuClassName="w-40"
      onMenuCloseAutoFocus={(event) => {
        if (!focusInputAfterMenuCloseRef.current) {
          return;
        }
        event.preventDefault();
        focusInputAfterMenuCloseRef.current = false;
        inputRef.current?.focus();
        inputRef.current?.select();
      }}
      menu={
        <>
          <ModelCapabilityMenuItem role="menuitemradio" checked={value === null} onSelect={() => selectPreset(null)}>
            <span>{t("sheet.capabilityMode.auto")}</span>
          </ModelCapabilityMenuItem>
          <DropdownMenuSeparator />
          {MODEL_CONTEXT_WINDOW_PRESETS.map((preset) => (
            <ModelCapabilityMenuItem
              key={preset.value}
              role="menuitemradio"
              checked={value === preset.value}
              onSelect={() => selectPreset(preset.value)}
            >
              <span>{preset.label}</span>
            </ModelCapabilityMenuItem>
          ))}
          <ModelCapabilityMenuItem
            role="menuitemradio"
            checked={value !== null && !MODEL_CONTEXT_WINDOW_PRESETS.some((preset) => preset.value === value)}
            onSelect={() => {
              focusInputAfterMenuCloseRef.current = true;
            }}
          >
            <span>{t("sheet.capabilityMode.custom")}</span>
          </ModelCapabilityMenuItem>
        </>
      }
    >
      <Input
        ref={inputRef}
        id="model-context-window"
        inputMode="numeric"
        aria-label={t("sheet.contextWindow")}
        value={inputValue}
        // In automatic mode the detected value is shown as the content of the row.
        placeholder={new Intl.NumberFormat(locale).format(effectiveValue)}
        disabled={disabled}
        className="h-full min-w-0 flex-1 rounded-none border-0 bg-transparent px-0 text-right text-xs text-secondary-foreground tabular-nums shadow-none placeholder:text-secondary-foreground focus-visible:ring-0 focus-visible:placeholder:text-muted-foreground"
        onChange={(event) => setInputValue(event.target.value.replace(/\D/g, ""))}
        onBlur={commitInput}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.preventDefault();
            event.currentTarget.blur();
          }
        }}
      />
    </ModelCapabilityRow>
  );
}
