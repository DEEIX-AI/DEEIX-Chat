"use client";

import { ImagePlus, Search, Wand2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Input } from "@/components/ui/input";
import { InputGroupAddon, InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { listLobehubIconOptions, lobehubIconURL, ModelIcon, resolveModelIconURL } from "@/entities/model";
import { cn } from "@/lib/utils";

// The grid is taller than the popover, and every icon is an SVG painted through a CSS mask, so
// rendering the whole catalog at once costs a long raster on the opening frame. Offscreen cells
// are skipped by `content-visibility` (see ICON_CELL_CLASS) and the first paint stays cheap.
const MAX_RESULTS = 120;
const ICON_CELL_CLASS =
  "flex aspect-square items-center justify-center rounded-md transition-colors [content-visibility:auto] [contain-intrinsic-size:auto_2.25rem] hover:bg-muted";

/**
 * Icon of a provider, chosen from the bundled icons only: users cannot upload
 * or link images, so nothing they pick loads from elsewhere. An empty `value`
 * means automatic — the icon follows the address, shown as the preview.
 */
export function ModelsIconPicker({
  value,
  autoIcon,
  label,
  disabled = false,
  onChange,
}: {
  value: string;
  autoIcon: string;
  label: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("settings.modelsPage.iconPicker");
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const normalizedQuery = query.trim().toLowerCase();
  const icons = React.useMemo(
    () =>
      listLobehubIconOptions()
        .filter((item) => !normalizedQuery || item.id.includes(normalizedQuery) || item.name.toLowerCase().includes(normalizedQuery))
        .slice(0, MAX_RESULTS),
    [normalizedQuery],
  );
  const previewURL = resolveModelIconURL(value || autoIcon);

  const choose = (next: string) => {
    onChange(next);
    setOpen(false);
  };

  return (
    <>
      <InputGroupAddon align="inline-start" className="pr-0 pl-2">
        <span className="flex size-6 shrink-0 items-center justify-center rounded-sm bg-muted/70 text-muted-foreground">
          {previewURL ? (
            <ModelIcon key={previewURL} iconUrl={previewURL} label={label} size={16} />
          ) : value ? (
            <ModelIcon iconUrl={null} label={label} size={16} />
          ) : (
            <Wand2 className="size-3.5 stroke-1.5" />
          )}
        </span>
      </InputGroupAddon>
      <InputGroupAddon align="inline-end" className="pr-2 pl-0 has-[>button]:mr-0">
        <Popover
          open={open}
          onOpenChange={(next) => {
            setOpen(next);
            if (!next) setQuery("");
          }}
        >
          <PopoverTrigger asChild>
            <InputGroupButton
              size="icon-xs"
              disabled={disabled}
              aria-label={t("choose")}
              title={t("choose")}
              className="bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground"
            >
              <ImagePlus className="size-3.5 stroke-1.5" />
            </InputGroupButton>
          </PopoverTrigger>
          <PopoverContent align="end" className="w-80 p-2">
            <div className="relative mb-2">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input value={query} className="h-8 pl-8 text-xs" placeholder={t("search")} onChange={(event) => setQuery(event.target.value)} />
            </div>
            <div className="max-h-72 touch-pan-y overflow-y-auto overscroll-contain pr-1" onWheel={(event) => event.stopPropagation()}>
              <div className="grid grid-cols-7 gap-1">
                {normalizedQuery ? null : (
                  <button
                    type="button"
                    className={cn(ICON_CELL_CLASS, "text-muted-foreground hover:text-foreground", value === "" && "bg-muted text-foreground")}
                    title={t("auto")}
                    aria-label={t("auto")}
                    onClick={() => choose("")}
                  >
                    <Wand2 className="size-4 stroke-[1.5]" />
                  </button>
                )}
                {icons.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    className={cn(ICON_CELL_CLASS, value === item.id && "bg-muted")}
                    title={item.name}
                    aria-label={item.name}
                    onClick={() => choose(item.id)}
                  >
                    <ModelIcon iconUrl={lobehubIconURL(item.id)} label={item.name} size={20} />
                  </button>
                ))}
              </div>
              {icons.length === 0 ? <p className="py-6 text-center text-xs text-muted-foreground">{t("empty")}</p> : null}
              {icons.length === MAX_RESULTS ? <p className="pt-2 text-center text-[11px] text-muted-foreground">{t("refine")}</p> : null}
            </div>
          </PopoverContent>
        </Popover>
      </InputGroupAddon>
    </>
  );
}
