"use client";

import { WandSparkles } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Blocks } from "@/components/animate-ui/icons/blocks";
import { InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { UIComponentDTO } from "@/shared/api/ui-components-types";
import { ComposerOptionRow } from "@/features/chat/components/shared/composer-option-row";
import { uiComponentIcon } from "@/features/chat/model/ui-component-icons";
import { useChatPopoverAlignOffset } from "@/features/chat/hooks/use-chat-popover-align-offset";

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
  const popoverShift = useChatPopoverAlignOffset("start");
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
              ref={popoverShift.triggerRef}
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
        ref={popoverShift.contentRef}
        side={placementPreference}
        align="start"
        alignOffset={popoverShift.alignOffset}
        sideOffset={8}
        avoidCollisions={false}
        collisionPadding={8}
        className="flex max-h-[var(--radix-popover-content-available-height)] w-[min(19rem,calc(100vw-1rem))] flex-col p-1.5"
        onPointerDown={(event) => event.stopPropagation()}
        onMouseDown={(event) => event.stopPropagation()}
        onClick={(event) => event.stopPropagation()}
      >
        {hasCatalog ? (
          <>
            <div className="flex h-7 shrink-0 items-center justify-between gap-3 px-2 text-[11px] font-medium text-foreground/70">
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
            <div className="min-h-0 max-h-72 overflow-y-auto px-0.5">
              {loading ? (
                <div className="flex items-center justify-center py-6">
                  <Spinner className="size-4" />
                </div>
              ) : (
                components.map((component) => {
                  const selected = selectedSet.has(component.id);
                  const label = labelFor(component);
                  return (
                    <ComposerOptionRow
                      key={component.id}
                      icon={uiComponentIcon(component.name)}
                      label={label.title}
                      meta={component.scope !== "builtin" ? t(`uiComponentScope.${component.scope}`) : null}
                      selected={selected}
                      title={label.summary}
                      onClick={() => onChange(selected ? selectedIDs.filter((id) => id !== component.id) : [...selectedIDs, component.id])}
                    />
                  );
                })
              )}
            </div>
          </>
        ) : null}

        {/* Outside the scroll area so the switch stays at the bottom however long the list is. */}
        {htmlVisual ? (
          <div className="shrink-0 px-0.5">
            {hasCatalog ? <div className="my-1.5 border-t-[0.5px] border-border" /> : null}
            <label
              className={cn(
                "flex h-7 cursor-pointer items-center justify-between gap-3 rounded-md px-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent/70 hover:text-foreground",
                htmlVisual.enabled && "text-foreground",
              )}
            >
              <span className="flex items-center gap-2">
                <WandSparkles className="size-3.5 shrink-0" strokeWidth={1.6} aria-hidden="true" />
                <span>{t("htmlVisualPrompt")}</span>
              </span>
              <Switch size="sm" checked={htmlVisual.enabled} onCheckedChange={htmlVisual.onChange} />
            </label>
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
