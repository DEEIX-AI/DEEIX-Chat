"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "motion/react";
import { Box, Check, ChevronRight, FileText, LoaderCircle, ScrollText, Wrench } from "lucide-react";

import type {
  ChatMentionMenuItem,
  ChatMentionMenuLayout,
  ChatMentionMenuRow,
  ChatMentionMenuTab,
  ChatMentionMenuTabInfo,
} from "@/features/chat/hooks/use-chat-mention-menu";
import { cn } from "@/lib/utils";
import { ModelIcon } from "@/shared/components/model-icon";
import { resolveModelIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";

type ChatMentionMenuTranslator = (key: string, values?: Record<string, string | number>) => string;

function ChatMentionMenuItemButton({
  item,
  active,
  onSelect,
}: {
  item: ChatMentionMenuItem;
  active: boolean;
  onSelect: () => void;
}) {
  const platformModelName = item.kind === "model" ? item.model.platformModelName.trim() : "";
  const identity = React.useMemo(() => {
    if (item.kind !== "model") {
      return null;
    }
    return resolveModelIdentity({
      code: item.model.platformModelName,
      vendor: item.model.vendor,
      icon: item.model.icon,
    });
  }, [item]);
  const iconURL = React.useMemo(() => identity ? resolveModelIconURL(identity.modelIcon) : "", [identity]);

  return (
    <button
      type="button"
      role="option"
      aria-selected={active}
      data-active={active}
      className="flex h-8 w-full min-w-0 scroll-mt-7 items-center gap-2 rounded-md px-2 text-left text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-accent-foreground data-[active=true]:bg-accent data-[active=true]:text-accent-foreground"
      onMouseDown={(event) => {
        event.preventDefault();
        onSelect();
      }}
    >
      {item.kind === "model" ? (
        <ModelIcon iconUrl={iconURL} label={platformModelName} />
      ) : item.kind === "file" ? (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground">
          <FileText className="size-3.5" strokeWidth={1.7} />
        </span>
      ) : item.kind === "tool" ? (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground">
          <Wrench className="size-3.5" strokeWidth={1.7} />
        </span>
      ) : item.kind === "skill" ? (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground">
          <Box className="size-3.5" strokeWidth={1.7} />
        </span>
      ) : (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground">
          <ScrollText className="size-3.5" strokeWidth={1.7} />
        </span>
      )}
      <span className="flex min-w-0 flex-1 items-baseline gap-2 overflow-hidden">
        <span className="shrink-0 whitespace-nowrap text-foreground/90">{item.label}</span>
        {item.description ? (
          <span className="min-w-0 flex-1 truncate font-normal text-muted-foreground/80">{item.description}</span>
        ) : null}
      </span>
      <span className="flex size-3.5 shrink-0 items-center justify-center">
        {item.selected ? <Check className="size-3.5 text-current" strokeWidth={1.8} /> : null}
      </span>
    </button>
  );
}

type ChatMentionMenuSectionGroup = {
  key: string;
  header: Extract<ChatMentionMenuRow, { type: "header" }> | null;
  rows: Exclude<ChatMentionMenuRow, { type: "header" }>[];
};

function groupRowsIntoSections(rows: ChatMentionMenuRow[]): ChatMentionMenuSectionGroup[] {
  const sections: ChatMentionMenuSectionGroup[] = [];
  let current: ChatMentionMenuSectionGroup | null = null;
  for (const row of rows) {
    if (row.type === "header") {
      current = { key: row.key, header: row, rows: [] };
      sections.push(current);
      continue;
    }
    if (!current) {
      current = { key: `section:${row.key}`, header: null, rows: [] };
      sections.push(current);
    }
    current.rows.push(row);
  }
  return sections;
}

const ChatMentionMenuContent = React.memo(function ChatMentionMenuContent({
  activeRowKey,
  rows,
  t,
  onSelect,
}: {
  activeRowKey: string | null;
  rows: ChatMentionMenuRow[];
  t: ChatMentionMenuTranslator;
  onSelect: (row: ChatMentionMenuRow) => void;
}) {
  const sections = React.useMemo(() => groupRowsIntoSections(rows), [rows]);

  return (
    <>
      {sections.map((section) => (
        <div key={section.key} className="space-y-0.5">
          {section.header ? (
            <div className="sticky top-0 z-[1] bg-pure px-2 pb-1 pt-1.5 text-[11px] font-semibold text-muted-foreground">
              {section.header.labelKey ? t(section.header.labelKey) : section.header.label}
            </div>
          ) : null}
          {section.rows.map((row) => {
            if (row.type === "item") {
              return (
                <ChatMentionMenuItemButton
                  key={row.key}
                  item={row.item}
                  active={row.key === activeRowKey}
                  onSelect={() => onSelect(row)}
                />
              );
            }
            if (row.type === "viewAll") {
              return (
                <button
                  key={row.key}
                  type="button"
                  role="option"
                  aria-selected={row.key === activeRowKey}
                  data-active={row.key === activeRowKey}
                  className="flex h-7 w-full scroll-mt-7 items-center gap-2 rounded-md px-2 text-left text-[11px] font-medium text-muted-foreground/85 outline-none transition-colors hover:bg-accent hover:text-accent-foreground data-[active=true]:bg-accent data-[active=true]:text-accent-foreground"
                  onMouseDown={(event) => {
                    event.preventDefault();
                    onSelect(row);
                  }}
                >
                  <span className="min-w-0 flex-1 truncate">
                    {t("mention.viewAll", { count: row.count })}
                  </span>
                  <ChevronRight className="size-3.5 shrink-0" strokeWidth={1.7} />
                </button>
              );
            }
            return (
              <div key={row.key} className="flex h-8 items-center justify-center" aria-hidden>
                <LoaderCircle className="size-3.5 animate-spin text-muted-foreground/70" strokeWidth={1.8} />
              </div>
            );
          })}
        </div>
      ))}
    </>
  );
});

function ChatMentionMenuTabBar({
  activeTab,
  tabs,
  t,
  onSelectTab,
}: {
  activeTab: ChatMentionMenuTab;
  tabs: ChatMentionMenuTabInfo[];
  t: ChatMentionMenuTranslator;
  onSelectTab: (tab: ChatMentionMenuTab) => void;
}) {
  return (
    <div
      role="tablist"
      className="flex shrink-0 items-center gap-1 overflow-x-auto px-1.5 pb-1 pt-1.5 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
    >
      {tabs.map((tab) => {
        const active = tab.id === activeTab;
        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={active}
            className={cn(
              "flex h-6 shrink-0 items-center gap-1 rounded-md px-2 text-[11px] font-medium outline-none transition-colors",
              active
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
            )}
            onMouseDown={(event) => {
              event.preventDefault();
              onSelectTab(tab.id);
            }}
          >
            <span>{t(`mention.tabs.${tab.id}`)}</span>
            {tab.count !== null ? (
              <span
                className={cn(
                  "text-[10px] leading-none tabular-nums",
                  active ? "text-accent-foreground/70" : "text-muted-foreground/70",
                )}
              >
                {tab.count}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

export function resolveMentionMenuMotionStyle(layout: ChatMentionMenuLayout | null): React.CSSProperties | undefined {
  if (!layout) {
    return undefined;
  }
  return {
    bottom: layout.bottom,
    left: layout.left,
    top: layout.top,
    width: layout.width,
    contain: "layout paint",
    transformOrigin: layout.placement === "bottom" ? "top center" : "bottom center",
    willChange: "height, opacity, transform",
  };
}

export function ChatMentionMenuPortal({
  activeRowKey,
  activeTab,
  menuID,
  menuLayout,
  menuRef,
  menuReady,
  open,
  rows,
  showTabBar,
  tabs,
  t,
  onListScroll,
  onSelect,
  onSelectTab,
}: {
  activeRowKey: string | null;
  activeTab: ChatMentionMenuTab;
  menuID: string;
  menuLayout: ChatMentionMenuLayout | null;
  menuRef: React.RefObject<HTMLDivElement | null>;
  menuReady: boolean;
  open: boolean;
  rows: ChatMentionMenuRow[];
  showTabBar: boolean;
  tabs: ChatMentionMenuTabInfo[];
  t: ChatMentionMenuTranslator;
  onListScroll?: (event: React.UIEvent<HTMLElement>) => void;
  onSelect: (row: ChatMentionMenuRow) => void;
  onSelectTab: (tab: ChatMentionMenuTab) => void;
}) {
  const menuMotionStyle = React.useMemo(
    () => resolveMentionMenuMotionStyle(menuLayout),
    [menuLayout],
  );
  const menuHeight = menuLayout?.height ?? 0;
  const shouldRender = open && menuReady && menuMotionStyle !== undefined;

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <AnimatePresence initial={false}>
      {shouldRender ? (
        <motion.div
          ref={menuRef}
          id={menuID}
          key="chat-mention-menu"
          role="listbox"
          className="bg-pure fixed z-[60] overflow-hidden rounded-xl border-[0.5px] border-border/70 text-popover-foreground shadow-xs"
          style={menuMotionStyle}
          onMouseDown={(event) => {
            // Keep focus in the textarea for any interaction inside the menu
            // (tab bar, scrollbar, empty areas), so the menu does not close.
            event.preventDefault();
          }}
          initial={{
            height: Math.min(menuHeight, 12),
            opacity: 0,
            scale: 0.99,
            y: menuLayout?.placement === "top" ? 4 : -4,
          }}
          animate={{ height: menuHeight, opacity: 1, scale: 1, y: 0 }}
          exit={{
            height: Math.min(menuHeight, 12),
            opacity: 0,
            scale: 0.99,
            y: menuLayout?.placement === "top" ? 4 : -4,
          }}
          transition={{
            height: { type: "spring", stiffness: 520, damping: 42, mass: 0.75 },
            opacity: { duration: 0.1, ease: "easeOut" },
            scale: { duration: 0.12, ease: "easeOut" },
            y: { duration: 0.12, ease: "easeOut" },
          }}
        >
          <div className="flex h-full flex-col">
            {showTabBar ? (
              <ChatMentionMenuTabBar
                activeTab={activeTab}
                tabs={tabs}
                t={t}
                onSelectTab={onSelectTab}
              />
            ) : null}
            <div
              data-mention-menu-scroll
              className={cn(
                "min-h-0 flex-1 overflow-y-auto overflow-x-hidden px-1.5 pb-1.5",
                showTabBar ? "pt-0" : "pt-1.5",
              )}
              onScroll={onListScroll}
            >
              {rows.length === 0 ? (
                <div className="flex h-full min-h-16 items-center justify-center px-4 text-center text-xs text-muted-foreground">
                  {t(`mention.empty.${activeTab}`)}
                </div>
              ) : (
                <ChatMentionMenuContent
                  activeRowKey={activeRowKey}
                  rows={rows}
                  t={t}
                  onSelect={onSelect}
                />
              )}
            </div>
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>,
    document.body,
  );
}
