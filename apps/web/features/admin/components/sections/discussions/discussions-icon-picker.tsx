"use client";

import * as React from "react";
import { ChevronDown, Search } from "lucide-react";
import { useTranslations } from "next-intl";

import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { listLobehubIconOptions, lobehubIconURL, ModelIcon, resolveModelIconURL } from "@/entities/model";
import { cn } from "@/lib/utils";

// An empty value is a real choice: the picker then falls back to the generic bot icon.
const MAX_RESULTS = 60;

/**
 * Icon of a discussion template, picked from the bundled icon set. The trigger and every row
 * show the icon together with its name, so the field is never a bare slug string.
 */
export function DiscussionsIconPicker({
  value,
  label,
  disabled = false,
  onChange,
}: {
  value: string;
  label: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("adminDiscussions");
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const normalizedQuery = query.trim().toLowerCase();
  const options = React.useMemo(
    () =>
      listLobehubIconOptions()
        .filter(
          (item) =>
            !normalizedQuery ||
            item.id.includes(normalizedQuery) ||
            item.name.toLowerCase().includes(normalizedQuery),
        )
        .slice(0, MAX_RESULTS),
    [normalizedQuery],
  );
  const selected = listLobehubIconOptions().find((item) => item.id === value);
  const previewURL = resolveModelIconURL(value);

  const choose = (next: string) => {
    onChange(next);
    setOpen(false);
    setQuery("");
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className="flex h-9 w-full items-center gap-2 rounded-lg border border-border/70 bg-background px-3 text-left text-sm outline-none focus:border-primary disabled:opacity-60"
        >
          <span className="flex size-5 shrink-0 items-center justify-center">
            <ModelIcon iconUrl={previewURL} label={label} size={16} />
          </span>
          <span className="min-w-0 flex-1 truncate text-xs">
            {selected ? selected.name : t("iconDefault")}
          </span>
          <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 p-2">
        <div className="relative mb-2">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            className="h-8 pl-8 text-xs"
            placeholder={t("iconSearch")}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <div className="max-h-72 overflow-y-auto overscroll-contain pr-1">
          {normalizedQuery ? null : (
            <button
              type="button"
              className={cn(
                "mb-1 flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-muted",
                value === "" && "bg-muted",
              )}
              onClick={() => choose("")}
            >
              <span className="flex size-5 shrink-0 items-center justify-center">
                <ModelIcon iconUrl={null} label={label} size={16} />
              </span>
              <span className="truncate">{t("iconDefault")}</span>
            </button>
          )}
          <div className="grid grid-cols-2 gap-1">
            {options.map((item) => (
              <button
                key={item.id}
                type="button"
                title={item.id}
                className={cn(
                  "flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-muted",
                  value === item.id && "bg-primary/10 text-primary",
                )}
                onClick={() => choose(item.id)}
              >
                <ModelIcon iconUrl={lobehubIconURL(item.id)} label={item.name} size={16} />
                <span className="min-w-0 flex-1 truncate">{item.name}</span>
              </button>
            ))}
          </div>
          {options.length === 0 ? (
            <p className="py-6 text-center text-xs text-muted-foreground">{t("iconEmpty")}</p>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
