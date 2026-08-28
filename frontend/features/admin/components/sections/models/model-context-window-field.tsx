"use client";

import { useEffect, useRef, useState } from "react";
import { Check, ChevronDownIcon, CircleHelp } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
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
    <div className="grid h-8 min-w-0 grid-cols-2 overflow-hidden rounded-md bg-secondary text-secondary-foreground">
      <div className="flex min-w-0 items-center gap-1 px-2">
        <Label className="mb-0 truncate text-xs font-normal leading-4 text-secondary-foreground" htmlFor="model-context-window">
          {t("sheet.contextWindow")}
        </Label>
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="inline-flex size-4 shrink-0 items-center justify-center text-muted-foreground transition-colors hover:text-secondary-foreground"
              aria-label={t("sheet.contextWindowDescription")}
            >
              <CircleHelp className="size-3" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipContent className="max-w-64">
            <div className="space-y-1">
              <p>{t("sheet.contextWindowDescription")}</p>
              <p className="text-background/70">
                {t("sheet.contextWindowEffective", {
                  value: new Intl.NumberFormat(locale).format(effectiveValue),
                })}
              </p>
            </div>
          </TooltipContent>
        </Tooltip>
      </div>
      <div className="flex h-full min-w-0 border-l border-border/60 transition-colors hover:bg-background/30 focus-within:bg-background/50">
        <Input
          ref={inputRef}
          id="model-context-window"
          inputMode="numeric"
          value={inputValue}
          placeholder={t("sheet.contextWindowAuto")}
          disabled={disabled}
          className="h-full min-w-0 flex-1 rounded-none border-0 bg-transparent px-2 text-right text-xs text-secondary-foreground tabular-nums shadow-none focus-visible:ring-0"
          onChange={(event) => setInputValue(event.target.value.replace(/\D/g, ""))}
          onBlur={commitInput}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              event.currentTarget.blur();
            }
          }}
        />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              disabled={disabled}
              className="inline-flex h-full w-8 shrink-0 items-center justify-center border-l border-border/50 text-muted-foreground transition-colors hover:bg-background/40 hover:text-secondary-foreground disabled:pointer-events-none disabled:opacity-50"
              aria-label={t("sheet.contextWindowPresets")}
            >
              <ChevronDownIcon className="size-3.5" aria-hidden="true" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="min-w-32"
            onCloseAutoFocus={(event) => {
              if (!focusInputAfterMenuCloseRef.current) {
                return;
              }
              event.preventDefault();
              focusInputAfterMenuCloseRef.current = false;
              inputRef.current?.focus();
              inputRef.current?.select();
            }}
          >
            <DropdownMenuItem onSelect={() => selectPreset(null)}>
              <span>{t("sheet.contextWindowAuto")}</span>
              <Check className={cn("ml-auto size-3.5", value === null ? "opacity-100" : "opacity-0")} />
            </DropdownMenuItem>
            {MODEL_CONTEXT_WINDOW_PRESETS.map((preset) => (
              <DropdownMenuItem key={preset.value} onSelect={() => selectPreset(preset.value)}>
                <span>{preset.label}</span>
                <Check className={cn("ml-auto size-3.5", value === preset.value ? "opacity-100" : "opacity-0")} />
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => {
              focusInputAfterMenuCloseRef.current = true;
            }}>
              {t("sheet.contextWindowCustom")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
