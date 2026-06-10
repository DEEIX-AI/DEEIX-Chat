"use client";

import * as React from "react";

import {
  readMentionFileSearchCache,
  searchMentionFiles,
} from "@/features/chat/model/mention-file-search";
import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { FileObjectDTO } from "@/shared/api/file.types";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readSessionRevision } from "@/shared/auth/session";

const MENTION_MENU_MAX_HEIGHT = 280;
const MENTION_MENU_MIN_HEIGHT = 32;
const MENTION_MENU_ROW_HEIGHT = 32;
const MENTION_MENU_ROW_GAP = 2;
const MENTION_MENU_SECTION_HEADER_HEIGHT = 26;
const MENTION_MENU_CHROME_HEIGHT = 12;
const MENTION_MENU_VIEWPORT_GUTTER = 16;
const MENTION_MENU_OFFSET = 8;
const MENTION_MENU_FILE_QUERY_DELAY_MS = 180;

export type ChatMentionMenuKind = "file" | "tool" | "model";

type ChatMentionFileMenuItem = {
  id: string;
  kind: "file";
  label: string;
  description: string;
  file: FileObjectDTO;
  selected: boolean;
};

type ChatMentionToolMenuItem = {
  id: string;
  kind: "tool";
  label: string;
  description: string;
  tool: MCPToolDTO;
  selected: boolean;
};

type ChatMentionModelMenuItem = {
  id: string;
  kind: "model";
  label: string;
  description: string;
  model: ChatModelOption;
  selected: boolean;
};

export type ChatMentionMenuItem =
  | ChatMentionFileMenuItem
  | ChatMentionToolMenuItem
  | ChatMentionModelMenuItem;

export type ChatMentionMenuSection = {
  kind: ChatMentionMenuKind;
  items: ChatMentionMenuItem[];
};

export type ChatMentionMenuLayout = {
  bottom?: number;
  height: number;
  left: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
};

type ChatMentionMenuControllerArgs = {
  availableTools: MCPToolDTO[];
  attachments: PendingAttachment[];
  defaultFileLabel: string;
  disabled: boolean;
  draft: string;
  maxSelectedTools: number;
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  selectedToolIDs: number[];
  anchorRef: React.RefObject<HTMLElement | null>;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  toolsDisabled: boolean;
  onDraftChange: (value: string) => void;
  onFileSelect: (file: FileObjectDTO) => void | Promise<void>;
  onModelChange: (platformModelName: string) => void;
  onSelectedToolsChange: (toolIDs: number[]) => void;
  onToolLimitReached?: () => void;
};

function resolveMentionQuery(value: string): string | null {
  if (!value.startsWith("@")) {
    return null;
  }
  return value.slice(1).match(/^\S*/)?.[0]?.trim().toLowerCase() ?? "";
}

function removeMentionTrigger(value: string): string {
  return value.replace(/^@\S*\s?/, "");
}

function itemMatchesQuery(values: Array<string | undefined>, query: string): boolean {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return true;
  }
  return values.join(" ").toLowerCase().includes(normalizedQuery);
}

function resolveToolLabel(tool: MCPToolDTO): string {
  const displayName = typeof tool.displayName === "string" ? tool.displayName.trim() : "";
  const name = typeof tool.name === "string" ? tool.name.trim() : "";
  return displayName || name || String(tool.id);
}

function resolveToolDescription(tool: MCPToolDTO): string {
  const serverName = tool.serverName?.trim() ?? "";
  const description = tool.description?.trim() ?? "";
  return [serverName, description].filter(Boolean).join(" - ");
}

function filterModels(modelOptions: ChatModelOption[], query: string): ChatMentionModelMenuItem[] {
  return modelOptions
    .filter((model) =>
      itemMatchesQuery([model.platformModelName, model.vendor], query),
    )
    .map((model) => ({
      id: `model:${model.platformModelName}`,
      kind: "model" as const,
      label: model.platformModelName,
      description: model.vendor,
      model,
      selected: false,
    }));
}

function filterTools(
  availableTools: MCPToolDTO[],
  query: string,
  selectedToolIDs: number[],
): ChatMentionToolMenuItem[] {
  const selectedIDs = new Set(selectedToolIDs);
  return availableTools
    .filter((tool) =>
      itemMatchesQuery([resolveToolLabel(tool), tool.name, tool.serverName, tool.description], query),
    )
    .map((tool) => ({
      id: `tool:${tool.id}`,
      kind: "tool" as const,
      label: resolveToolLabel(tool),
      description: resolveToolDescription(tool),
      tool,
      selected: selectedIDs.has(tool.id),
    }));
}

function filesToItems(
  files: FileObjectDTO[],
  attachments: PendingAttachment[],
  defaultFileLabel: string,
): ChatMentionFileMenuItem[] {
  const attachedIDs = new Set(attachments.map((item) => item.fileID));
  return files.map((file) => ({
    id: `file:${file.fileID}`,
    kind: "file" as const,
    label: file.fileName || defaultFileLabel,
    description: file.mimeType || file.fileCategory || "",
    file,
    selected: attachedIDs.has(file.fileID),
  }));
}

function buildSections({
  attachments,
  availableTools,
  defaultFileLabel,
  files,
  filesQuery,
  fileLoading,
  modelOptions,
  query,
  selectedPlatformModelName,
  selectedToolIDs,
  toolsDisabled,
}: {
  attachments: PendingAttachment[];
  availableTools: MCPToolDTO[];
  defaultFileLabel: string;
  files: FileObjectDTO[];
  filesQuery: string;
  fileLoading: boolean;
  modelOptions: ChatModelOption[];
  query: string | null;
  selectedPlatformModelName: string;
  selectedToolIDs: number[];
  toolsDisabled: boolean;
}): ChatMentionMenuSection[] {
  if (query === null) {
    return [];
  }

  const normalizedQuery = query.trim().toLowerCase();
  const fileItems = fileLoading || filesQuery !== normalizedQuery ? [] : filesToItems(files, attachments, defaultFileLabel);
  const toolItems = toolsDisabled ? [] : filterTools(availableTools, query, selectedToolIDs);
  const modelItems = filterModels(modelOptions, query).map((item) => ({
    ...item,
    selected: item.model.platformModelName === selectedPlatformModelName,
  }));

  return [
    { kind: "model" as const, items: modelItems },
    { kind: "file" as const, items: fileItems },
    { kind: "tool" as const, items: toolItems },
  ].filter((section) => section.items.length > 0);
}

function flattenSections(sections: ChatMentionMenuSection[]): ChatMentionMenuItem[] {
  return sections.flatMap((section) => section.items);
}

function resolveMentionMenuWidth(anchorWidth: number, viewportWidth: number): number {
  const availableWidth = Math.max(0, viewportWidth - MENTION_MENU_VIEWPORT_GUTTER * 2);
  return Math.min(anchorWidth, availableWidth);
}

function resolveMentionMenuContentHeight(sections: ChatMentionMenuSection[]): number {
  const itemCount = sections.reduce((total, section) => total + section.items.length, 0);
  if (itemCount === 0) {
    return MENTION_MENU_MIN_HEIGHT;
  }
  const sectionChrome = sections.length * MENTION_MENU_SECTION_HEADER_HEIGHT;
  return Math.min(
    MENTION_MENU_MAX_HEIGHT,
    itemCount * MENTION_MENU_ROW_HEIGHT
      + Math.max(0, itemCount - 1) * MENTION_MENU_ROW_GAP
      + sectionChrome
      + MENTION_MENU_CHROME_HEIGHT,
  );
}

function resolveMentionMenuLayout(
  anchor: HTMLElement,
  sections: ChatMentionMenuSection[],
  viewportWidth: number,
  viewportHeight: number,
): ChatMentionMenuLayout {
  const anchorRect = anchor.getBoundingClientRect();
  const preferredTop = anchorRect.bottom + MENTION_MENU_OFFSET;
  const preferredBottom = anchorRect.top - MENTION_MENU_OFFSET;
  const desiredHeight = resolveMentionMenuContentHeight(sections);
  const availableBelow = viewportHeight - preferredTop - MENTION_MENU_VIEWPORT_GUTTER;
  const availableAbove = preferredBottom - MENTION_MENU_VIEWPORT_GUTTER;
  const anchorInLowerHalf = anchorRect.top + anchorRect.height / 2 > viewportHeight / 2;
  const openBelow =
    !anchorInLowerHalf &&
    (availableBelow >= Math.min(desiredHeight, MENTION_MENU_MIN_HEIGHT) ||
      availableBelow >= availableAbove);
  const availableHeight = Math.max(0, openBelow ? availableBelow : availableAbove);
  const maxHeight = Math.max(
    Math.min(MENTION_MENU_MIN_HEIGHT, availableHeight),
    Math.min(desiredHeight, availableHeight),
  );
  const preferredWidth = resolveMentionMenuWidth(anchorRect.width, viewportWidth);
  const preferredLeft = anchorRect.left;
  const maxLeft = Math.max(
    MENTION_MENU_VIEWPORT_GUTTER,
    viewportWidth - preferredWidth - MENTION_MENU_VIEWPORT_GUTTER,
  );
  const left = Math.min(Math.max(preferredLeft, MENTION_MENU_VIEWPORT_GUTTER), maxLeft);
  const width = Math.min(
    preferredWidth,
    Math.max(0, viewportWidth - left - MENTION_MENU_VIEWPORT_GUTTER),
  );

  if (openBelow) {
    return { height: maxHeight, left, placement: "bottom", top: preferredTop, width };
  }

  return {
    bottom: Math.max(MENTION_MENU_VIEWPORT_GUTTER, viewportHeight - preferredBottom),
    height: maxHeight,
    left,
    placement: "top",
    width,
  };
}

export function useChatMentionMenu({
  attachments,
  availableTools,
  defaultFileLabel,
  disabled,
  draft,
  maxSelectedTools,
  modelOptions,
  selectedPlatformModelName,
  selectedToolIDs,
  anchorRef,
  textareaRef,
  toolsDisabled,
  onDraftChange,
  onFileSelect,
  onModelChange,
  onSelectedToolsChange,
  onToolLimitReached,
}: ChatMentionMenuControllerArgs) {
  const menuRef = React.useRef<HTMLDivElement | null>(null);
  const menuID = React.useId();
  const [inputFocused, setInputFocused] = React.useState(false);
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [dismissedDraft, setDismissedDraft] = React.useState<string | null>(null);
  const [menuLayout, setMenuLayout] = React.useState<ChatMentionMenuLayout | null>(null);
  const [files, setFiles] = React.useState<FileObjectDTO[]>([]);
  const [filesLoading, setFilesLoading] = React.useState(false);
  const [filesQuery, setFilesQuery] = React.useState("");
  const query = resolveMentionQuery(draft);

  React.useEffect(() => {
    if (query === null || disabled) {
      setFiles([]);
      setFilesQuery("");
      setFilesLoading(false);
      return;
    }

    const sessionRevision = readSessionRevision();
    const cachedFiles = readMentionFileSearchCache(sessionRevision, query);
    if (cachedFiles) {
      setFiles(cachedFiles);
      setFilesQuery(query);
      setFilesLoading(false);
      return;
    }

    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setFilesLoading(true);
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (!token || controller.signal.aborted) {
            return;
          }
          const results = await searchMentionFiles({
            accessToken: token,
            query,
            sessionRevision,
          });
          if (!controller.signal.aborted) {
            setFiles(results);
            setFilesQuery(query);
          }
        } catch {
          if (!controller.signal.aborted) {
            setFiles([]);
          }
        } finally {
          if (!controller.signal.aborted) {
            setFilesLoading(false);
          }
        }
      })();
    }, MENTION_MENU_FILE_QUERY_DELAY_MS);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [disabled, query]);

  const sections = React.useMemo(
    () =>
      buildSections({
        attachments,
        availableTools,
        defaultFileLabel,
        files,
        filesQuery,
        fileLoading: filesLoading,
        modelOptions,
        query,
        selectedPlatformModelName,
        selectedToolIDs,
        toolsDisabled,
      }),
    [
      attachments,
      availableTools,
      defaultFileLabel,
      files,
      filesQuery,
      filesLoading,
      modelOptions,
      query,
      selectedPlatformModelName,
      selectedToolIDs,
      toolsDisabled,
    ],
  );
  const items = React.useMemo(() => flattenSections(sections), [sections]);
  const open = inputFocused && query !== null && dismissedDraft !== draft && !disabled && items.length > 0;
  const activeItem = open ? items[Math.min(activeIndex, items.length - 1)] : null;

  React.useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  React.useEffect(() => {
    setActiveIndex((current) => (items.length === 0 ? 0 : Math.min(current, items.length - 1)));
  }, [items.length]);

  React.useEffect(() => {
    if (!open) {
      return;
    }
    const frameID = window.requestAnimationFrame(() => {
      const scrollContainer = menuRef.current?.querySelector<HTMLElement>("[data-mention-menu-scroll]");
      if (activeIndex === 0) {
        if (scrollContainer) {
          scrollContainer.scrollTop = 0;
        }
        return;
      }
      const activeElement = menuRef.current?.querySelector<HTMLElement>('[data-active="true"]');
      activeElement?.scrollIntoView({ block: "nearest" });
    });
    return () => window.cancelAnimationFrame(frameID);
  }, [activeIndex, open]);

  const updateLayout = React.useCallback(() => {
    if (!open || typeof window === "undefined") {
      return;
    }

    const anchor = anchorRef.current;
    if (!anchor) {
      return;
    }

    setMenuLayout(resolveMentionMenuLayout(anchor, sections, window.innerWidth, window.innerHeight));
  }, [anchorRef, open, sections]);

  React.useLayoutEffect(() => {
    if (!open) {
      setMenuLayout(null);
      return;
    }
    updateLayout();
    let frameID = window.requestAnimationFrame(updateLayout);
    const update = () => {
      window.cancelAnimationFrame(frameID);
      frameID = window.requestAnimationFrame(updateLayout);
    };
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    return () => {
      window.cancelAnimationFrame(frameID);
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
    };
  }, [open, updateLayout]);

  const finishSelection = React.useCallback(() => {
    onDraftChange(removeMentionTrigger(draft));
    setDismissedDraft(null);
    window.requestAnimationFrame(() => {
      textareaRef.current?.focus();
    });
  }, [draft, onDraftChange, textareaRef]);

  const select = React.useCallback(
    (item: ChatMentionMenuItem) => {
      if (item.kind === "model") {
        onModelChange(item.model.platformModelName);
        finishSelection();
        return;
      }

      if (item.kind === "tool") {
        const alreadySelected = selectedToolIDs.includes(item.tool.id);
        if (!alreadySelected && selectedToolIDs.length >= maxSelectedTools) {
          onToolLimitReached?.();
          return;
        }
        onSelectedToolsChange(
          alreadySelected
            ? selectedToolIDs.filter((toolID) => toolID !== item.tool.id)
            : [...selectedToolIDs, item.tool.id],
        );
        finishSelection();
        return;
      }

      void onFileSelect(item.file);
      finishSelection();
    },
    [
      finishSelection,
      maxSelectedTools,
      onFileSelect,
      onModelChange,
      onSelectedToolsChange,
      onToolLimitReached,
      selectedToolIDs,
    ],
  );

  const handleChange = React.useCallback(
    (value: string) => {
      if (dismissedDraft !== null && value !== dismissedDraft) {
        setDismissedDraft(null);
      }
      onDraftChange(value);
    },
    [dismissedDraft, onDraftChange],
  );

  const handleKeyDown = React.useCallback(
    (event: React.KeyboardEvent<HTMLTextAreaElement>): boolean => {
      if (!open) {
        return false;
      }
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setActiveIndex((current) => (current + 1) % items.length);
        return true;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        setActiveIndex((current) => (current - 1 + items.length) % items.length);
        return true;
      }
      if ((event.key === "Enter" || event.key === "Tab") && activeItem) {
        event.preventDefault();
        select(activeItem);
        return true;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        setDismissedDraft(draft);
        return true;
      }
      return false;
    },
    [activeItem, draft, items.length, open, select],
  );

  return {
    activeIndex,
    filesLoading,
    handleBlur: () => setInputFocused(false),
    handleChange,
    handleFocus: () => setInputFocused(true),
    handleKeyDown,
    menuID,
    menuRef,
    menuLayout,
    menuReady: open && menuLayout !== null && menuLayout.height > 0 && menuLayout.width > 0,
    open,
    sections,
    select,
  };
}
