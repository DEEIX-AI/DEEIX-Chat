"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "motion/react";
import { Box, Check, FileText, ScrollText, Wrench } from "lucide-react";

import type {
  ChatMentionMenuItem,
  ChatMentionMenuKind,
  ChatMentionMenuLayout,
  ChatMentionMenuSection,
} from "@/features/chat/hooks/use-chat-mention-menu";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";

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
  const iconURL = React.useMemo(() => identity ? resolveLobeHubIconURL(identity.modelIcon) : "", [identity]);

  return (
    <button
      type="button"
      role="option"
      aria-selected={active}
      data-active={active}
      className="flex h-8 w-full min-w-0 items-center gap-2 rounded-md px-2 text-left text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-accent-foreground data-[active=true]:bg-accent data-[active=true]:text-accent-foreground"
      onMouseDown={(event) => {
        event.preventDefault();
        onSelect();
      }}
    >
      {item.kind === "model" ? (
        <LobeHubIcon iconUrl={iconURL} label={platformModelName} />
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

const ChatMentionMenuContent = React.memo(function ChatMentionMenuContent({
  activeIndex,
  sectionOffsets,
  sections,
  t,
  onSelect,
}: {
  activeIndex: number;
  sectionOffsets: Map<ChatMentionMenuKind, number>;
  sections: ChatMentionMenuSection[];
  t: (key: string) => string;
  onSelect: (item: ChatMentionMenuItem) => void;
}) {
  return (
    <>
      {sections.map((section) => {
        const sectionOffset = sectionOffsets.get(section.kind) ?? 0;
        return (
          <div key={section.kind} className="space-y-0.5">
            <div className="px-2 pb-1 pt-1.5 text-[11px] font-semibold text-muted-foreground">
              {t(`mention.sections.${section.kind}`)}
            </div>
            {section.items.map((item, index) => (
              <ChatMentionMenuItemButton
                key={item.id}
                item={item}
                active={sectionOffset + index === activeIndex}
                onSelect={() => onSelect(item)}
              />
            ))}
          </div>
        );
      })}
    </>
  );
});

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
  activeIndex,
  menuID,
  menuLayout,
  menuRef,
  menuReady,
  open,
  sectionOffsets,
  sections,
  t,
  onSelect,
}: {
  activeIndex: number;
  menuID: string;
  menuLayout: ChatMentionMenuLayout | null;
  menuRef: React.RefObject<HTMLDivElement | null>;
  menuReady: boolean;
  open: boolean;
  sectionOffsets: Map<ChatMentionMenuKind, number>;
  sections: ChatMentionMenuSection[];
  t: (key: string) => string;
  onSelect: (item: ChatMentionMenuItem) => void;
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
          <div data-mention-menu-scroll className="h-full overflow-y-auto overflow-x-hidden p-1.5">
            <ChatMentionMenuContent
              activeIndex={activeIndex}
              sectionOffsets={sectionOffsets}
              sections={sections}
              t={t}
              onSelect={onSelect}
            />
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>,
    document.body,
  );
}
