"use client";

import * as React from "react";
import { closestCenter, DndContext, type DragEndEvent, type Modifier, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { arrayMove, horizontalListSortingStrategy, SortableContext, useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Copy, Globe, Laptop, Minus, Plus, Square, X } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import {
  activateTab,
  closeTab,
  closeWindow,
  isWindowMaximized,
  listTabs,
  minimizeWindow,
  moveTab,
  onTabsChanged,
  onWindowResized,
  openTab,
  type ShellTab,
  type ShellTabs,
  toggleMaximizeWindow,
} from "@/shared/platform/desktop-shell";

// Tab strip for the desktop shell, in its own 38px webview (STRIP_HEIGHT in
// tabs.rs). State is owned by Rust; this is a view over `tabs_list` + `tabs:changed`.

const CONTROL_WIDTH = 46;

// Tabs only ever move along the strip.
const horizontalOnly: Modifier = ({ transform }) => ({ ...transform, y: 0 });

export function TabStrip() {
  const t = useTranslations("desktopTabs");
  const [state, setState] = React.useState<ShellTabs>({ tabs: [], active: null, platform: "" });
  // Optimistic order until Rust confirms, so the tab does not flash back.
  const [pending, setPending] = React.useState<string[] | null>(null);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  React.useEffect(() => {
    let disposed = false;
    void listTabs().then((next) => {
      if (!disposed) setState(next);
    });
    const unsubscribe = onTabsChanged((next) => {
      setState(next);
      setPending(null);
    });
    return () => {
      disposed = true;
      unsubscribe();
    };
  }, []);

  const ids = React.useMemo(() => pending ?? state.tabs.map((tab) => tab.id), [pending, state.tabs]);
  const tabs = React.useMemo(
    () => ids.map((id) => state.tabs.find((tab) => tab.id === id)).filter((tab): tab is ShellTab => tab !== undefined),
    [ids, state.tabs],
  );

  const onDragEnd = React.useCallback(
    ({ active, over }: DragEndEvent) => {
      if (!over || active.id === over.id) return;
      const from = ids.indexOf(String(active.id));
      const to = ids.indexOf(String(over.id));
      if (from < 0 || to < 0) return;
      setPending(arrayMove(ids, from, to));
      void moveTab(String(active.id), to);
    },
    [ids],
  );

  return (
    <div
      data-tauri-drag-region
      className={cn(
        "relative flex h-[38px] w-screen select-none items-end overflow-hidden bg-muted px-2 pt-1 text-foreground",
        // Room for the traffic lights under the overlay title bar.
        state.platform === "macos" && "pl-[84px]",
      )}
      style={state.platform === "windows" ? { paddingRight: CONTROL_WIDTH * 3 + 8 } : undefined}
    >
      {/* Next.js dev badge: one per webview; the strip is shell chrome, not a page. */}
      <style>{"nextjs-portal{display:none}"}</style>
      <DndContext sensors={sensors} collisionDetection={closestCenter} modifiers={[horizontalOnly]} onDragEnd={onDragEnd}>
        <SortableContext items={ids} strategy={horizontalListSortingStrategy}>
          {tabs.map((tab, index) => (
            <TabItem
              key={tab.id}
              tab={tab}
              active={tab.id === state.active}
              // Chrome hides the divider on both sides of the active tab.
              divider={index > 0 && tab.id !== state.active && tabs[index - 1]?.id !== state.active}
              label={labelFor(tab, t)}
              closable={state.tabs.length > 1 || tab.server !== null}
              onClose={() => void closeTab(tab.id)}
            />
          ))}
        </SortableContext>
      </DndContext>
      <button
        type="button"
        aria-label={t("newTab")}
        title={t("newTab")}
        className="mb-[3px] ml-1 flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground"
        onClick={() => void openTab()}
      >
        <Plus className="size-4" />
      </button>
      {state.platform === "windows" ? <WindowControls /> : null}
    </div>
  );
}

function WindowControls() {
  const t = useTranslations("desktopTabs");
  const [maximized, setMaximized] = React.useState(false);
  React.useEffect(() => {
    let disposed = false;
    const sync = () => {
      void isWindowMaximized().then((next) => {
        if (!disposed) setMaximized(next);
      });
    };
    sync();
    const unsubscribe = onWindowResized(sync);
    return () => {
      disposed = true;
      unsubscribe();
    };
  }, []);
  const buttonClass =
    "flex h-[38px] items-center justify-center text-foreground/80 transition-colors hover:bg-foreground/10 hover:text-foreground";
  return (
    <div className="absolute right-0 top-0 flex h-[38px]" data-tauri-drag-region="false">
      <button
        type="button"
        aria-label={t("minimize")}
        title={t("minimize")}
        className={buttonClass}
        style={{ width: CONTROL_WIDTH }}
        onClick={() => void minimizeWindow()}
      >
        <Minus className="size-3.5" strokeWidth={1.5} />
      </button>
      <button
        type="button"
        aria-label={maximized ? t("restore") : t("maximize")}
        title={maximized ? t("restore") : t("maximize")}
        className={buttonClass}
        style={{ width: CONTROL_WIDTH }}
        onClick={() => void toggleMaximizeWindow()}
      >
        {maximized ? <Copy className="size-3" strokeWidth={1.5} /> : <Square className="size-3" strokeWidth={1.5} />}
      </button>
      <button
        type="button"
        aria-label={t("close")}
        title={t("close")}
        className={cn(buttonClass, "hover:bg-[#e81123] hover:text-white")}
        style={{ width: CONTROL_WIDTH }}
        onClick={() => void closeWindow()}
      >
        <X className="size-4" strokeWidth={1.5} />
      </button>
    </div>
  );
}

function TabItem({
  tab,
  active,
  divider,
  label,
  closable,
  onClose,
}: {
  tab: ShellTab;
  active: boolean;
  divider: boolean;
  label: string;
  closable: boolean;
  onClose: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: tab.id });
  const Icon = tab.server === null ? Plus : tab.server.mode === "local" ? Laptop : Globe;
  return (
    <div
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      role="tab"
      tabIndex={0}
      aria-selected={active}
      title={tab.title ? `${tab.title} — ${label}` : label}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        // Only the active tab is painted in the content colour, with rounded "feet".
        "group relative flex h-[34px] min-w-0 max-w-[200px] flex-1 animate-in items-center gap-2 rounded-t-lg px-3 text-xs transition-colors fade-in-0 duration-200",
        active
          ? cn(
              "z-[1] bg-background text-foreground",
              "before:absolute before:-left-2 before:bottom-0 before:size-2 before:bg-[radial-gradient(circle_at_top_left,transparent_8px,var(--background)_8px)]",
              "after:absolute after:-right-2 after:bottom-0 after:size-2 after:bg-[radial-gradient(circle_at_top_right,transparent_8px,var(--background)_8px)]",
            )
          : "text-muted-foreground hover:bg-background/50 hover:text-foreground",
        divider && "before:absolute before:left-0 before:top-2 before:bottom-2 before:w-px before:bg-border",
        isDragging && "z-10 shadow-md",
      )}
      onPointerDown={(event) => {
        // No focus on press: moving focus mid-press would kill the drag.
        if (event.button === 0) void activateTab(tab.id, false);
        listeners?.onPointerDown?.(event);
      }}
      onPointerUp={(event) => {
        if (event.button === 0) void activateTab(tab.id, true);
      }}
      onAuxClick={(event) => {
        if (event.button === 1 && closable) {
          event.preventDefault();
          onClose();
        }
      }}
    >
      <Icon className="size-3.5 shrink-0 opacity-70" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {closable ? (
        <button
          type="button"
          aria-label="close"
          className={cn(
            "flex size-5 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-muted hover:text-foreground",
            active ? "opacity-70 hover:opacity-100" : "opacity-0 group-hover:opacity-70",
          )}
          onPointerDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.stopPropagation();
            onClose();
          }}
        >
          <X className="size-3.5" />
        </button>
      ) : null}
    </div>
  );
}

function labelFor(tab: ShellTab, t: ReturnType<typeof useTranslations<"desktopTabs">>): string {
  if (!tab.server) return t("newTab");
  if (tab.server.mode === "local") return t("local");
  try {
    return new URL(tab.server.origin).host;
  } catch {
    return tab.server.origin;
  }
}
