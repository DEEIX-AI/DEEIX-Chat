"use client";

import { Check, ChevronLeft, ChevronRight, Ellipsis, Lock, Minus, Pin, Plus, Settings2 } from "lucide-react";
import { useMessages, useTranslations } from "next-intl";
import * as React from "react";

import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Slider } from "@/components/ui/slider";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  type ChatModelControlSelections,
  isControlChanged,
  resolveControlValue,
} from "@/features/chat/model/chat-model-controls";
import {
  type ComposerControlItem,
  type ModelControlPlacements,
  nextModelControlPlacements,
  splitComposerControlItems,
} from "@/features/chat/model/chat-model-control-placements";
import type { NativeToolVisualOption } from "@/features/chat/model/chat-native-tools";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { useChatPopoverAlignOffset } from "@/features/chat/hooks/use-chat-popover-align-offset";
import { cn } from "@/lib/utils";
import {
  isReasoningEffortLevel,
  localizedNativeToolText,
  MAX_PINNED_MODEL_CONTROLS,
  MODEL_CONTROL_REASONING_AUTO,
  MODEL_CONTROL_TOGGLE_OFF,
  MODEL_CONTROL_TOGGLE_ON,
  modelControlNumberStep,
  type ModelControl,
  type ModelControlValue,
  normalizeModelControlValue,
  resolveModelControlIcon,
  resolveNativeToolIcon,
} from "@/entities/model";

// Same trigger and panel styling as the other composer popovers (tools, knowledge bases, output).
const TRIGGER_CLASSNAME = "size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8";
const TRIGGER_ACTIVE_CLASSNAME = "bg-primary/10 text-primary hover:bg-primary/10 hover:text-primary";
const PANEL_CLASSNAME = "flex max-h-[var(--radix-popover-content-available-height)] flex-col p-1.5";
// Full-row option button (inside second-level panels) and the trailing "advanced" entry.
const OPTION_ROW_CLASSNAME =
  "flex h-7 w-full items-center gap-1.5 rounded-md px-1.5 text-left text-foreground/80 outline-none transition-colors hover:bg-accent hover:text-accent-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground disabled:pointer-events-none disabled:opacity-45";
// Parent-list row: the whole row is the control, so the pin button sits next to it, not inside.
const ROW_CONTAINER_CLASSNAME =
  "flex h-7 w-full items-center gap-1 rounded-md pr-1 pl-1.5 text-foreground/80 transition-colors hover:bg-accent hover:text-accent-foreground";
const ROW_MAIN_CLASSNAME = "flex min-w-0 flex-1 items-center gap-1.5 text-left outline-none disabled:cursor-not-allowed disabled:opacity-60";
const PIN_BUTTON_CLASSNAME =
  "inline-flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground/60 outline-none transition-colors hover:text-foreground focus-visible:text-foreground disabled:pointer-events-none disabled:opacity-30";
const HEADER_CLASSNAME = "flex h-7 shrink-0 items-center justify-between gap-3 px-2 text-[11px] font-medium text-foreground/70";
const HEADER_BACK_CLASSNAME =
  "-ml-1.5 flex h-7 min-w-0 items-center gap-0.5 rounded-md px-0.5 text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-foreground focus-visible:bg-accent focus-visible:text-foreground";
const HEADER_ACTION_CLASSNAME =
  "text-[11px] leading-none text-foreground/55 outline-none transition-colors hover:text-foreground focus-visible:text-foreground";

// Composer controls: the administrator's model controls plus provider-native tool toggles.
type ComposerItem =
  | (ComposerControlItem & { source: "control"; control: ModelControl; label: string })
  | (ComposerControlItem & { source: "tool"; tool: NativeToolVisualOption; label: string; description: string });

export type ChatModelControlsProps = {
  controls: ModelControl[];
  selections: ChatModelControlSelections;
  nativeTools: NativeToolVisualOption[];
  isNativeToolEnabled: (tool: NativeToolVisualOption) => boolean;
  placements: ModelControlPlacements;
  disabled: boolean;
  placementPreference: "top" | "bottom";
  onControlChange: (controlID: string, value: ModelControlValue | null) => void;
  onNativeToolChange: (tool: NativeToolVisualOption, enabled: boolean) => void;
  onPlacementsChange: (placements: ModelControlPlacements) => void;
  // Entry at the bottom of the popover that opens the parameter dialog (raw JSON).
  advancedOptions?: { label: string; onOpen: () => void };
  // Popover title and trigger tooltip; chat wording by default, media tasks name their own.
  labels?: { title: string; more: string };
};

/**
 * Tooltip state for a button that opens a popover: hidden while the popover is open, and kept hidden
 * after it closes (focus returns to the trigger) until the pointer moves onto or off the trigger, or
 * focus moves away.
 */
function useMenuTriggerTooltip(menuOpen: boolean) {
  const [tooltipOpen, setTooltipOpen] = React.useState(false);
  const [suppressed, setSuppressed] = React.useState(false);
  const wasMenuOpen = React.useRef(menuOpen);
  React.useEffect(() => {
    if (menuOpen) {
      setTooltipOpen(false);
    } else if (wasMenuOpen.current) {
      setSuppressed(true);
    }
    wasMenuOpen.current = menuOpen;
  }, [menuOpen]);
  return {
    tooltipProps: {
      open: tooltipOpen && !menuOpen && !suppressed,
      onOpenChange: setTooltipOpen,
    },
    triggerProps: {
      onPointerEnter: () => setSuppressed(false),
      onPointerLeave: () => setSuppressed(false),
      onBlur: () => setSuppressed(false),
    },
  };
}

function nativeToolItemID(tool: NativeToolVisualOption): string {
  return `tool:${tool.primary.type.trim() || tool.primary.toolKey.trim() || tool.primary.provider.trim()}`;
}

function stopPopoverPropagation() {
  return {
    onPointerDown: (event: React.PointerEvent) => event.stopPropagation(),
    onMouseDown: (event: React.MouseEvent) => event.stopPropagation(),
    onClick: (event: React.MouseEvent) => event.stopPropagation(),
  };
}

/**
 * Pinned controls as icon buttons (at most MAX_PINNED_MODEL_CONTROLS) plus a "…" popover listing every
 * control. Controls that take a value (select, number) open a second-level panel; toggles and
 * provider tools are switched by clicking the row. Each row carries its own pin button. Values are
 * validated and remembered by the caller.
 */
export function ChatModelControls({
  controls,
  selections,
  nativeTools,
  isNativeToolEnabled,
  placements,
  disabled,
  placementPreference,
  onControlChange,
  onNativeToolChange,
  onPlacementsChange,
  advancedOptions,
  labels,
}: ChatModelControlsProps) {
  const t = useTranslations("chat.modelControls");
  const title = labels?.title ?? t("title");
  const moreLabel = labels?.more ?? t("more");
  const tReasoning = useTranslations("chat.reasoningEffort");
  const tLevels = useTranslations("common.reasoningEffort.levels");
  const tOptionLabels = useTranslations("chat.optionLabels");
  const messages = useMessages();

  const controlLabel = React.useCallback((control: ModelControl) => {
    if (control.label) return control.label;
    if (control.kind === "reasoning") return tReasoning("title");
    // Controls compiled from optionControls are identified by their parameter path; the option
    // labels use the path with dots replaced by "__" (same convention as the advanced dialog).
    const key = control.id.replaceAll(".", "__");
    return tOptionLabels.has(key) ? tOptionLabels(key) : control.id;
  }, [tOptionLabels, tReasoning]);

  const items = React.useMemo<ComposerItem[]>(() => [
    ...controls.map((control): ComposerItem => ({
      id: control.id,
      defaultPlacement: control.placement,
      pinnable: true,
      source: "control",
      control,
      label: controlLabel(control),
    })),
    ...nativeTools.map((tool): ComposerItem => ({
      id: nativeToolItemID(tool),
      defaultPlacement: "menu",
      pinnable: true,
      source: "tool",
      tool,
      label: localizedNativeToolText(messages, "nativeToolLabels", tool.primary.toolKey)
        || tool.primary.label || tool.primary.type || tool.primary.toolKey,
      description: localizedNativeToolText(messages, "nativeToolDescriptions", tool.primary.toolKey) || tool.primary.description,
    })),
  ], [controlLabel, controls, messages, nativeTools]);
  const { toolbar } = React.useMemo(() => splitComposerControlItems(items, placements), [items, placements]);
  const toolbarIDs = React.useMemo(() => new Set(toolbar.map((item) => item.id)), [toolbar]);

  // Unpinned controls first, pinned ones collected at the bottom (a filled pin marks them).
  const { unpinnedItems, pinnedItems } = React.useMemo(() => {
    const pinned: ComposerItem[] = [];
    const unpinned: ComposerItem[] = [];
    for (const item of items) {
      (toolbarIDs.has(item.id) ? pinned : unpinned).push(item);
    }
    return { unpinnedItems: unpinned, pinnedItems: pinned };
  }, [items, toolbarIDs]);

  const valueLabel = React.useCallback((control: ModelControl, value: ModelControlValue | null) => {
    if (value === null) return t("unset");
    if (control.type === "number") return String(value);
    if (control.type === "toggle") return value === MODEL_CONTROL_TOGGLE_ON ? t("on") : t("off");
    const option = control.options.find((candidate) => candidate.value === value);
    if (option?.label) return option.label;
    if (control.kind === "reasoning") {
      if (value === MODEL_CONTROL_REASONING_AUTO) return tReasoning("auto");
      if (isReasoningEffortLevel(value)) return tLevels(value);
    }
    return String(value);
  }, [t, tLevels, tReasoning]);

  const isItemActive = React.useCallback((item: ComposerItem) => {
    if (item.source === "tool") return isNativeToolEnabled(item.tool);
    if (item.control.type === "toggle") return resolveControlValue(item.control, selections) === MODEL_CONTROL_TOGGLE_ON;
    return isControlChanged(item.control, selections);
  }, [isNativeToolEnabled, selections]);

  const [moreOpen, setMoreOpen] = React.useState(false);
  const moreTooltip = useMenuTriggerTooltip(moreOpen);
  const moreShift = useChatPopoverAlignOffset("end");
  // Phones have no room for a side panel: a value row swaps the list for its options, like the model picker.
  const isMobile = useIsMobile();
  const [drillID, setDrillID] = React.useState<string | null>(null);

  const togglePin = React.useCallback((item: ComposerItem) => {
    const next = nextModelControlPlacements(items, placements, item, !toolbarIDs.has(item.id));
    if (next) onPlacementsChange(next);
  }, [items, onPlacementsChange, placements, toolbarIDs]);

  const changedControls = controls.filter((control) => isControlChanged(control, selections));
  const drillTarget = isMobile ? items.find((item) => item.id === drillID) : undefined;
  const drillItem = drillTarget?.source === "control" && drillTarget.control.type !== "toggle" ? drillTarget : null;
  const pinnedCount = toolbar.length;

  const itemTooltip = (item: ComposerItem) => {
    if (item.source === "tool") return item.label;
    const value = resolveControlValue(item.control, selections);
    return item.control.locked
      ? t("lockedTooltip", { label: item.label, value: valueLabel(item.control, value) })
      : t("valueTooltip", { label: item.label, value: valueLabel(item.control, value) });
  };

  // Trailing pin button: one click to pin or unpin; disabled once the pin limit is reached.
  const renderPinButton = (item: ComposerItem) => {
    if (!item.pinnable) {
      return null;
    }
    const pinned = toolbarIDs.has(item.id);
    const full = !pinned && pinnedCount >= MAX_PINNED_MODEL_CONTROLS;
    const label = full ? t("pinLimit", { max: MAX_PINNED_MODEL_CONTROLS }) : pinned ? t("unpin") : t("pin");
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className={cn(PIN_BUTTON_CLASSNAME, pinned && "text-primary hover:text-primary")}
            disabled={full}
            aria-label={label}
            aria-pressed={pinned}
            onClick={() => togglePin(item)}
          >
            <Pin className={cn("size-3", pinned && "fill-current")} strokeWidth={1.8} />
          </button>
        </TooltipTrigger>
        <TooltipContent side="left" className="text-xs">{label}</TooltipContent>
      </Tooltip>
    );
  };

  const renderOptionRows = (control: ModelControl, close: () => void) => {
    const value = resolveControlValue(control, selections);
    if (control.type === "number") {
      return <ChatModelControlNumber control={control} value={value} onChange={(next) => onControlChange(control.id, next)} />;
    }
    return control.options.map((option) => {
      const selected = value === option.value;
      return (
        <button
          key={option.value}
          type="button"
          data-selected={selected}
          title={option.description || undefined}
          className={OPTION_ROW_CLASSNAME}
          onClick={() => {
            onControlChange(control.id, option.value);
            close();
          }}
        >
          <span className="min-w-0 flex-1 truncate text-xs font-medium text-current">{valueLabel(control, option.value)}</span>
          {option.value === control.default ? (
            <span className="shrink-0 text-[10px] leading-none text-muted-foreground">{t("default")}</span>
          ) : null}
          <Check className={cn("size-3.5 shrink-0 text-primary transition-opacity", selected ? "opacity-100" : "opacity-0")} strokeWidth={2} />
        </button>
      );
    });
  };

  // Value controls (select, number) open their options in a second-level panel.
  const renderValueRow = (item: ComposerItem & { source: "control" }) => {
    const { control } = item;
    const value = resolveControlValue(control, selections);
    const Icon = resolveModelControlIcon(control);
    const active = isItemActive(item);
    const row = (
      <button
        type="button"
        className={ROW_MAIN_CLASSNAME}
        disabled={disabled || control.locked}
        title={control.description || undefined}
        aria-label={itemTooltip(item)}
        onClick={isMobile ? () => setDrillID(item.id) : undefined}
      >
        <Icon className={cn("size-3.5 shrink-0", active ? "text-primary" : "text-muted-foreground")} strokeWidth={1.6} />
        <span className="min-w-0 flex-1 truncate text-xs font-medium text-current">{item.label}</span>
        <span className="max-w-24 shrink-0 truncate text-[11px] text-muted-foreground">{valueLabel(control, value)}</span>
        {control.locked ? (
          <Lock className="size-3 shrink-0 text-muted-foreground" strokeWidth={1.6} />
        ) : (
          <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.6} />
        )}
      </button>
    );
    return (
      <div key={item.id} className={ROW_CONTAINER_CLASSNAME}>
        {isMobile ? row : (
          <ChatModelControlSubmenu header={item.label} trigger={row} renderContent={(close) => renderOptionRows(control, close)} />
        )}
        {renderPinButton(item)}
      </div>
    );
  };

  // Toggles and provider tools switch by clicking anywhere on the row.
  const renderToggleRow = (item: ComposerItem) => {
    const isTool = item.source === "tool";
    const Icon = isTool ? resolveNativeToolIcon(item.tool.primary.type) : resolveModelControlIcon(item.control);
    const active = isItemActive(item);
    const checked = isTool ? active : resolveControlValue(item.control, selections) === MODEL_CONTROL_TOGGLE_ON;
    const locked = !isTool && item.control.locked;
    const description = isTool ? item.description : item.control.description;
    const toggle = () => {
      if (isTool) {
        onNativeToolChange(item.tool, !active);
      } else {
        onControlChange(item.control.id, checked ? MODEL_CONTROL_TOGGLE_OFF : MODEL_CONTROL_TOGGLE_ON);
      }
    };
    return (
      <div key={item.id} className={ROW_CONTAINER_CLASSNAME} title={description || undefined}>
        <button
          type="button"
          role="switch"
          aria-checked={checked}
          aria-label={item.label}
          disabled={disabled || locked}
          className={ROW_MAIN_CLASSNAME}
          onClick={toggle}
        >
          <Icon className={cn("size-3.5 shrink-0", active ? "text-primary" : "text-muted-foreground")} strokeWidth={1.6} />
          <span className="min-w-0 flex-1 truncate text-xs font-medium text-current">{item.label}</span>
          {locked ? <Lock className="size-3 shrink-0 text-muted-foreground" strokeWidth={1.6} /> : <SwitchIndicator checked={checked} />}
        </button>
        {renderPinButton(item)}
      </div>
    );
  };

  const renderMenuItem = (item: ComposerItem) =>
    item.source === "control" && item.control.type !== "toggle" ? renderValueRow(item) : renderToggleRow(item);

  const renderToolbarItem = (item: ComposerItem) => {
    const Icon = item.source === "tool" ? resolveNativeToolIcon(item.tool.primary.type) : resolveModelControlIcon(item.control);
    const active = isItemActive(item);
    const locked = item.source === "control" && item.control.locked;
    const button = (
      <InputGroupButton
        type="button"
        variant="ghost"
        size="icon-sm"
        className={cn(TRIGGER_CLASSNAME, active && TRIGGER_ACTIVE_CLASSNAME)}
        disabled={disabled || locked}
        aria-label={itemTooltip(item)}
        aria-pressed={item.source === "tool" || item.control.type === "toggle" ? active : undefined}
        onClick={item.source === "tool"
          ? () => onNativeToolChange(item.tool, !active)
          : item.control.type === "toggle"
            ? () => onControlChange(item.control.id, active ? MODEL_CONTROL_TOGGLE_OFF : MODEL_CONTROL_TOGGLE_ON)
            : undefined}
      >
        <Icon className="size-4" strokeWidth={1.6} />
      </InputGroupButton>
    );
    if (item.source === "tool" || item.control.type === "toggle" || locked) {
      return (
        <Tooltip key={item.id}>
          <TooltipTrigger asChild>{button}</TooltipTrigger>
          <TooltipContent side="top" className="text-xs">{itemTooltip(item)}</TooltipContent>
        </Tooltip>
      );
    }
    const { control } = item;
    return (
      <ChatModelControlSubmenu
        key={item.id}
        header={item.label}
        tooltip={itemTooltip(item)}
        side={placementPreference}
        align="end"
        avoidCollisions={false}
        trigger={button}
        renderContent={(close) => renderOptionRows(control, close)}
      />
    );
  };

  return (
    <>
      {toolbar.map((item, index) => (
        // Keep the composer compact on phones: only the first two pinned items stay visible there.
        <span key={item.id} className={cn("contents", index >= 2 && "max-sm:hidden")}>
          {renderToolbarItem(item)}
        </span>
      ))}
      <Popover
        open={moreOpen}
        onOpenChange={(next) => {
          // Reset on open, not on close, so the list does not flash back during the closing fade.
          if (next) setDrillID(null);
          setMoreOpen(next);
        }}
      >
        <Tooltip {...moreTooltip.tooltipProps}>
          <TooltipTrigger asChild>
            <PopoverTrigger asChild>
              <InputGroupButton
                ref={moreShift.triggerRef}
                type="button"
                variant="ghost"
                size="icon-sm"
                className={cn(TRIGGER_CLASSNAME, changedControls.some((control) => !toolbarIDs.has(control.id)) && TRIGGER_ACTIVE_CLASSNAME)}
                disabled={disabled}
                aria-label={moreLabel}
                {...moreTooltip.triggerProps}
              >
                <Ellipsis className="size-4" strokeWidth={1.6} />
              </InputGroupButton>
            </PopoverTrigger>
          </TooltipTrigger>
          <TooltipContent side="top" className="text-xs">{moreLabel}</TooltipContent>
        </Tooltip>
        {/* The trigger sits at the right end of the toolbar (next to the model picker), so the panel
            grows leftwards over the composer instead of past its right edge. On a phone the trigger is
            mid-screen and the panel is nearly as wide as the viewport: the offset keeps it on screen. */}
        <PopoverContent
          ref={moreShift.contentRef}
          side={placementPreference}
          align="end"
          alignOffset={moreShift.alignOffset}
          sideOffset={8}
          avoidCollisions={false}
          collisionPadding={8}
          className={cn(PANEL_CLASSNAME, "w-[min(19rem,calc(100vw-1rem))]")}
          {...stopPopoverPropagation()}
        >
          {drillItem ? (
            <>
              <div className={HEADER_CLASSNAME}>
                <button type="button" className={HEADER_BACK_CLASSNAME} onClick={() => setDrillID(null)}>
                  <ChevronLeft className="size-3.5" strokeWidth={1.8} />
                  <span>{title}</span>
                </button>
                <span className="min-w-0 truncate text-right text-foreground/70">{drillItem.label}</span>
              </div>
              <div className="min-h-0 max-h-80 space-y-0.5 overflow-y-auto px-0.5">
                {renderOptionRows(drillItem.control, () => setDrillID(null))}
              </div>
            </>
          ) : (
            <>
              <div className={HEADER_CLASSNAME}>
                <span>{title}</span>
                {changedControls.length > 0 ? (
                  <button
                    type="button"
                    className={HEADER_ACTION_CLASSNAME}
                    onClick={() => {
                      for (const control of changedControls) onControlChange(control.id, null);
                    }}
                  >
                    {t("resetAll")}
                  </button>
                ) : null}
              </div>
              {items.length > 0 ? (
                <div className="min-h-0 max-h-80 space-y-0.5 overflow-y-auto px-0.5">
                  {unpinnedItems.map(renderMenuItem)}
                  {pinnedItems.map(renderMenuItem)}
                </div>
              ) : (
                <p className="px-2.5 pb-1 text-[11px] leading-4 text-muted-foreground">{t("empty")}</p>
              )}
              {/* Outside the scroll area so the entry stays at the bottom however long the list is. */}
              {advancedOptions ? (
                <div className="shrink-0 px-0.5">
                  <div className="my-1.5 border-t-[0.5px] border-border" />
                  <button type="button" className={OPTION_ROW_CLASSNAME} onClick={advancedOptions.onOpen}>
                    <Settings2 className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.6} />
                    <span className="min-w-0 flex-1 truncate text-xs font-medium text-current">{advancedOptions.label}</span>
                    <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.6} />
                  </button>
                </div>
              ) : null}
            </>
          )}
        </PopoverContent>
      </Popover>
    </>
  );
}

// Decorative switch for rows that are switched by clicking the whole row; a real Switch would nest
// buttons inside a button. Mirrors components/ui/switch at size="sm".
function SwitchIndicator({ checked }: { checked: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex h-3.5 w-6 shrink-0 items-center rounded-full border border-transparent shadow-xs transition-all",
        checked ? "bg-primary" : "bg-input dark:bg-input/80",
      )}
    >
      <span
        className={cn(
          "block size-3 rounded-full ring-0 transition-transform",
          checked
            ? "translate-x-[calc(100%-2px)] bg-primary-foreground"
            : "translate-x-0 bg-background dark:bg-foreground",
        )}
      />
    </span>
  );
}

/**
 * A row that opens a second-level (side) panel. Value controls (level lists, numbers) use this so the
 * parent list stays short and matches the rest of the composer menus.
 */
function ChatModelControlSubmenu({
  header,
  trigger,
  renderContent,
  side = "right",
  align = "start",
  avoidCollisions = true,
  tooltip,
}: {
  header: string;
  trigger: React.ReactElement<React.ComponentProps<"button">>;
  renderContent: (close: () => void) => React.ReactNode;
  side?: "left" | "right" | "top" | "bottom";
  align?: "start" | "end";
  avoidCollisions?: boolean;
  tooltip?: string;
}) {
  const [open, setOpen] = React.useState(false);
  const menuTooltip = useMenuTriggerTooltip(open);
  // Only fixed-side panels need this; the others let Radix shift them.
  const shift = useChatPopoverAlignOffset(align);
  const anchoredTrigger = avoidCollisions ? trigger : React.cloneElement(trigger, { ref: shift.triggerRef });
  return (
    <Popover open={open} onOpenChange={setOpen}>
      {tooltip ? (
        <Tooltip {...menuTooltip.tooltipProps}>
          <TooltipTrigger asChild>
            <PopoverTrigger asChild>{React.cloneElement(anchoredTrigger, menuTooltip.triggerProps)}</PopoverTrigger>
          </TooltipTrigger>
          <TooltipContent side="top" className="text-xs">{tooltip}</TooltipContent>
        </Tooltip>
      ) : <PopoverTrigger asChild>{anchoredTrigger}</PopoverTrigger>}
      <PopoverContent
        ref={avoidCollisions ? undefined : shift.contentRef}
        side={side}
        align={align}
        alignOffset={avoidCollisions ? 0 : shift.alignOffset}
        sideOffset={side === "top" || side === "bottom" ? 8 : 6}
        avoidCollisions={avoidCollisions}
        collisionPadding={8}
        className={cn(PANEL_CLASSNAME, "w-[min(13rem,calc(100vw-1rem))]")}
        {...stopPopoverPropagation()}
      >
        <div className={HEADER_CLASSNAME}>{header}</div>
        <div className="min-h-0 max-h-72 space-y-0.5 overflow-y-auto px-0.5">{renderContent(() => setOpen(false))}</div>
      </PopoverContent>
    </Popover>
  );
}

function decimalPlaces(value: number): number {
  const text = String(value);
  const exponent = text.match(/e-(\d+)$/);
  if (exponent) return Number(exponent[1]);
  return text.includes(".") ? text.split(".")[1]?.length ?? 0 : 0;
}

/**
 * Number control: a native number input with steppers (and a slider when the administrator set a
 * range). Typing commits on Enter / blur, Escape reverts; clearing the input returns to the default.
 */
function ChatModelControlNumber({
  control,
  value,
  onChange,
}: {
  control: ModelControl;
  value: ModelControlValue | null;
  onChange: (value: number | null) => void;
}) {
  const t = useTranslations("chat.modelControls");
  const isMobile = useIsMobile();
  const numeric = typeof value === "number" ? value : null;
  const parsedDefault = control.default !== null && control.default.trim() !== "" ? Number(control.default) : Number.NaN;
  const defaultValue = Number.isFinite(parsedDefault) ? parsedDefault : null;
  const [draft, setDraft] = React.useState(numeric === null ? "" : String(numeric));
  const [invalid, setInvalid] = React.useState(false);
  React.useEffect(() => {
    setDraft(numeric === null ? "" : String(numeric));
    setInvalid(false);
  }, [numeric]);

  const hasRange = control.min !== null && control.max !== null;
  const step = modelControlNumberStep(control);
  // The server only enforces the step the administrator configured; an inferred step just drives
  // the steppers and must not round values the user typed.
  const snapToGrid = control.step !== null && control.step > 0;
  // Clamp to the range and, with a configured step, snap to its grid (anchored at min, as the server
  // validates), so steppers and the slider never produce a value the server would reject.
  const clamp = (next: number, base = 0) => {
    const precision = Math.max(decimalPlaces(step), decimalPlaces(control.min ?? 0), decimalPlaces(base));
    const origin = control.min ?? 0;
    let result = snapToGrid ? origin + Math.round((next - origin) / step) * step : next;
    if (control.max !== null && result > control.max) {
      // max may sit off the grid: use the last grid point below it.
      result = snapToGrid ? origin + Math.floor((control.max - origin) / step + 1e-9) * step : control.max;
    }
    if (control.min !== null && result < control.min) result = control.min;
    return Number(result.toFixed(precision));
  };

  const commit = (input: HTMLInputElement) => {
    const raw = input.value;
    // type="number" reports "" for text it cannot parse; treat that as invalid, not as "cleared".
    if (input.validity.badInput) {
      setInvalid(true);
      return;
    }
    if (!raw.trim()) {
      setInvalid(false);
      if (numeric !== null) onChange(null);
      return;
    }
    const next = normalizeModelControlValue(control, raw);
    if (typeof next === "number") {
      setInvalid(false);
      if (next !== numeric) onChange(next);
    } else {
      setInvalid(true);
    }
  };
  const stepBy = (direction: 1 | -1) => {
    const draftNumber = Number(draft);
    const base = draft.trim() && Number.isFinite(draftNumber) ? draftNumber : numeric ?? defaultValue ?? control.min ?? 0;
    const next = clamp(base + direction * step, base);
    setDraft(String(next));
    setInvalid(false);
    onChange(next);
  };

  const draftNumber = Number(draft);
  const sliderValue = draft.trim() && Number.isFinite(draftNumber) ? draftNumber : numeric ?? defaultValue ?? control.min ?? 0;
  const atMin = control.min !== null && sliderValue <= control.min;
  const atMax = control.max !== null && sliderValue >= control.max;
  const placeholder = defaultValue !== null
    ? t("numberDefault", { value: String(defaultValue) })
    : t("unset");
  const hint = invalid
    ? hasRange
      ? t("numberInvalidRange", { min: String(control.min), max: String(control.max) })
      : t("numberInvalid")
    : hasRange
      ? t("numberRange", { min: String(control.min), max: String(control.max) })
      : null;

  const showFooter = Boolean(hint) || numeric !== null;

  return (
    <div className="space-y-1.5 px-1.5 pb-1">
      {/* Same borderless, filled field as the search inputs of the other composer popovers. The ring
          is inset: the panel body scrolls (overflow clips), so an outer ring loses its top edge. */}
      <InputGroup className="h-7 border-0 bg-muted/45 ring-inset has-[[aria-invalid=true]]:ring-[1px] dark:bg-muted/35">
        <InputGroupInput
          type="number"
          inputMode={control.integer ? "numeric" : "decimal"}
          // Focus the field on desktop; on phones that would pop the keyboard over the panel.
          autoFocus={!isMobile}
          min={control.min ?? undefined}
          max={control.max ?? undefined}
          step={step}
          aria-invalid={invalid || undefined}
          className="h-7 px-2 tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
          value={draft}
          placeholder={placeholder}
          onChange={(event) => {
            setDraft(event.target.value);
            setInvalid(false);
          }}
          onBlur={(event) => commit(event.currentTarget)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              commit(event.currentTarget);
            } else if (event.key === "Escape" && draft !== (numeric === null ? "" : String(numeric))) {
              // Revert the draft first; a second Escape closes the panel.
              event.preventDefault();
              event.stopPropagation();
              setDraft(numeric === null ? "" : String(numeric));
              setInvalid(false);
            }
          }}
        />
        {/* The addon's default negative margin pulls buttons to the field edge; keep them inside. */}
        <InputGroupAddon align="inline-end" className="gap-0.5 py-0 pr-1 has-[>button]:mr-0">
          <InputGroupButton
            size="icon-xs"
            className="size-5 rounded-[4px] text-muted-foreground hover:text-foreground"
            aria-label={t("decrease")}
            disabled={atMin}
            onClick={() => stepBy(-1)}
          >
            <Minus className="size-3" strokeWidth={1.8} />
          </InputGroupButton>
          <InputGroupButton
            size="icon-xs"
            className="size-5 rounded-[4px] text-muted-foreground hover:text-foreground"
            aria-label={t("increase")}
            disabled={atMax}
            onClick={() => stepBy(1)}
          >
            <Plus className="size-3" strokeWidth={1.8} />
          </InputGroupButton>
        </InputGroupAddon>
      </InputGroup>
      {hasRange ? (
        <Slider
          className="px-0.5 py-1"
          min={control.min ?? 0}
          max={control.max ?? 1}
          step={step}
          value={[sliderValue]}
          onValueChange={([next]) => {
            if (typeof next === "number") {
              setDraft(String(clamp(next)));
              setInvalid(false);
            }
          }}
          onValueCommit={([next]) => typeof next === "number" && onChange(clamp(next))}
        />
      ) : null}
      {showFooter ? (
        <div className="flex items-center justify-between gap-2 px-0.5 text-[10px] leading-4">
          <span className={cn("min-w-0 truncate", invalid ? "text-destructive" : "text-muted-foreground")}>{hint}</span>
          {numeric !== null ? (
            <button
              type="button"
              className={HEADER_ACTION_CLASSNAME}
              onClick={() => {
                setDraft("");
                setInvalid(false);
                onChange(null);
              }}
            >
              {t("resetAll")}
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
