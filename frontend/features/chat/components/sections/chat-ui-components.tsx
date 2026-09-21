"use client";

import { Check, WandSparkles } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Blocks } from "@/components/animate-ui/icons/blocks";
import { InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { UIComponentDTO } from "@/shared/api/ui-components.types";
import { uiComponentIcon } from "@/shared/model/ui-component-icons";

// Builtin names and summaries are localised on the client; the catalog text
// from the backend is the model-facing (Chinese) prompt. Custom components
// show their author-written description.
function headline(description: string): string {
  return description.split("。")[0] ?? description;
}

// One "output" popover for everything that shapes how the reply is rendered:
// the visual-layout prompt and the interactive component catalog.
export function ChatUIComponents({
  components,
  selectedIDs,
  defaultIDs,
  loading,
  placementPreference,
  disabled,
  onChange,
  htmlVisual,
}: {
  components: UIComponentDTO[];
  selectedIDs: number[];
  defaultIDs: number[];
  loading: boolean;
  placementPreference: "top" | "bottom";
  disabled: boolean;
  onChange: (ids: number[]) => void;
  htmlVisual?: { enabled: boolean; onChange: (enabled: boolean) => void };
}) {
  const t = useTranslations("chat.composer");
  const tLibrary = useTranslations("uiComponents");
  const [open, setOpen] = React.useState(false);
  const labelFor = (component: UIComponentDTO) =>
    component.scope === "builtin" && tLibrary.has(`builtin.${component.name}.title`)
      ? { title: tLibrary(`builtin.${component.name}.title`), summary: tLibrary(`builtin.${component.name}.summary`) }
      : { title: headline(component.description), summary: component.description };
  const [hovered, setHovered] = React.useState(false);
  const selectedSet = React.useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const isDefault = selectedIDs.length === defaultIDs.length && selectedIDs.every((id) => defaultIDs.includes(id));
  const customised = !isDefault || Boolean(htmlVisual?.enabled);
  const hasCatalog = loading || components.length > 0;

  if (!hasCatalog && !htmlVisual) {
    return null;
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <InputGroupButton
              type="button"
              variant="ghost"
              size="icon-sm"
              className={cn(
                "size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8",
                customised && "bg-primary/10 text-primary hover:bg-primary/10 hover:text-primary",
              )}
              disabled={disabled}
              aria-label={t("output")}
              onMouseEnter={() => setHovered(true)}
              onMouseLeave={() => setHovered(false)}
            >
              <Blocks size={20} strokeWidth={1.4} animate={hovered || customised ? "default" : undefined} />
            </InputGroupButton>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs">
          {t("output")}
        </TooltipContent>
      </Tooltip>

      <PopoverContent
        side={placementPreference}
        align="start"
        sideOffset={8}
        avoidCollisions={false}
        collisionPadding={8}
        className="flex max-h-[var(--radix-popover-content-available-height)] w-[min(19rem,calc(100vw-1rem))] flex-col p-1.5"
        onPointerDown={(event) => event.stopPropagation()}
        onMouseDown={(event) => event.stopPropagation()}
        onClick={(event) => event.stopPropagation()}
      >
        {htmlVisual ? (
          <label className="flex h-7 shrink-0 cursor-pointer items-center justify-between gap-3 rounded-md px-1.5 text-xs text-foreground/80 transition-colors hover:bg-accent hover:text-accent-foreground">
            <span className="flex items-center gap-1.5">
              <WandSparkles className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.6} />
              <span className="font-medium">{t("htmlVisualPrompt")}</span>
            </span>
            <Switch size="sm" checked={htmlVisual.enabled} onCheckedChange={htmlVisual.onChange} />
          </label>
        ) : null}

        {hasCatalog ? (
          <>
            <div className={cn("flex h-7 shrink-0 items-center justify-between gap-3 px-2 text-[11px] font-medium text-foreground/70", htmlVisual && "mt-1.5 border-t-[0.5px] border-border pt-1.5")}>
              <span>{t("uiComponents")}</span>
              {!isDefault ? (
                <button
                  type="button"
                  className="text-[11px] leading-none text-foreground/55 outline-none transition-colors hover:text-foreground focus-visible:text-foreground"
                  onClick={() => onChange(defaultIDs)}
                >
                  {t("uiComponentsRestoreDefault")}
                </button>
              ) : (
                <button
                  type="button"
                  className="text-[11px] leading-none text-foreground/55 outline-none transition-colors hover:text-foreground focus-visible:text-foreground"
                  onClick={() => onChange([])}
                >
                  {t("clear")}
                </button>
              )}
            </div>
            <div className="min-h-0 max-h-72 space-y-0.5 overflow-y-auto px-0.5">
              {loading ? (
                <div className="flex items-center justify-center py-6">
                  <Spinner className="size-4" />
                </div>
              ) : (
                components.map((component) => {
                  const selected = selectedSet.has(component.id);
                  const Icon = uiComponentIcon(component.name);
                  const label = labelFor(component);
                  return (
                    <button
                      key={component.id}
                      type="button"
                      data-selected={selected}
                      title={label.summary}
                      className="flex h-7 w-full items-center gap-1.5 rounded-md px-1.5 text-left text-foreground/80 transition-colors hover:bg-accent hover:text-accent-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
                      aria-pressed={selected}
                      onClick={() => onChange(selected ? selectedIDs.filter((id) => id !== component.id) : [...selectedIDs, component.id])}
                    >
                      <Icon className={cn("size-3.5 shrink-0", selected ? "text-primary" : "text-muted-foreground")} strokeWidth={1.6} />
                      <span className="min-w-0 flex-1 truncate text-xs font-medium text-current">{label.title}</span>
                      {component.scope !== "builtin" ? <span className="shrink-0 text-[10px] leading-none text-muted-foreground">{t(`uiComponentScope.${component.scope}`)}</span> : null}
                      <Check className={cn("size-3.5 shrink-0 text-primary transition-opacity", selected ? "opacity-100" : "opacity-0")} strokeWidth={2} />
                    </button>
                  );
                })
              )}
            </div>
          </>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
