"use client";

import * as React from "react";

import {
  type CommandRecentsEntry,
  type CommandRecentsKind,
  readCommandRecents,
  recordCommandRecentUsage,
} from "@/features/chat/model/command-recents-store";
import {
  readMentionFileSearchCache,
  searchMentionFiles,
} from "@/features/chat/model/mention-file-search";
import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { FileObjectDTO } from "@/shared/api/file.types";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import { listVisiblePromptPresets } from "@/shared/api/prompt-presets";
import type { PromptPresetDTO } from "@/shared/api/prompt-presets.types";
import { listVisibleSkills } from "@/shared/api/skills";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readSessionRevision } from "@/shared/auth/session";
import { resolveModelPresentationGroup } from "@/shared/lib/model-presentation";

const DEFAULT_MENTION_MENU_KINDS: readonly ChatMentionMenuKind[] = ["model", "file", "tool", "skill", "prompt"];
const MENTION_TRIGGER_KINDS: readonly ChatMentionMenuKind[] = ["model", "file", "tool"];
const PROMPT_TRIGGER_KINDS: readonly ChatMentionMenuKind[] = ["skill", "prompt"];

export type ChatMentionMenuKind = "file" | "tool" | "model" | "skill" | "prompt";
export type ChatMentionMenuTab = "all" | ChatMentionMenuKind;

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

type ChatMentionPromptMenuItem = {
  id: string;
  kind: "prompt";
  label: string;
  description: string;
  prompt: PromptPresetDTO;
  selected: boolean;
};

type ChatMentionSkillMenuItem = {
  id: string;
  kind: "skill";
  label: string;
  description: string;
  skill: SkillSummaryDTO;
  selected: boolean;
};

export type ChatMentionMenuItem =
  | ChatMentionFileMenuItem
  | ChatMentionToolMenuItem
  | ChatMentionModelMenuItem
  | ChatMentionSkillMenuItem
  | ChatMentionPromptMenuItem;

export type ChatMentionMenuRow =
  | { type: "header"; key: string; labelKey?: string; label?: string }
  | { type: "item"; key: string; item: ChatMentionMenuItem }
  | { type: "viewAll"; key: string; tab: ChatMentionMenuKind; count: number }
  | { type: "loading"; key: string };

export type ChatMentionMenuTabInfo = {
  id: ChatMentionMenuTab;
  count: number | null;
};

export type ChatMentionMenuLayout = {
  bottom?: number;
  height: number;
  left: number;
  placement: "bottom" | "top";
  top?: number;
  width: number;
};

type ChatMentionMenuPlacementPreference = "auto" | "bottom" | "top";
type ChatMentionMenuPlacementAnchor = "caret" | "container";
type ChatMentionTriggerKind = "mention" | "prompt";

type ChatMentionMenuControllerArgs = {
  availableTools: MCPToolDTO[];
  attachments: PendingAttachment[];
  defaultFileLabel: string;
  disabled: boolean;
  draft: string;
  maxSelectedTools: number;
  maxSelectedSkills: number;
  modelOptions: ChatModelOption[];
  selectedSkills?: SkillSummaryDTO[];
  selectedPlatformModelName: string;
  selectedToolIDs: number[];
  anchorRef: React.RefObject<HTMLElement | null>;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  toolsDisabled: boolean;
  onDraftChange: (value: string) => void;
  enabledKinds?: readonly ChatMentionMenuKind[];
  onFileSelect: (file: FileObjectDTO) => void | Promise<void>;
  onModelChange: (platformModelName: string) => void;
  onSelectedSkillsChange?: (skills: SkillSummaryDTO[]) => void;
  placementAnchor?: ChatMentionMenuPlacementAnchor;
  placementPreference?: ChatMentionMenuPlacementPreference;
  onModelCatalogRefresh?: () => void | Promise<void>;
  onSelectedToolsChange: (toolIDs: number[]) => void;
  onSkillLimitReached?: () => void;
  onToolLimitReached?: () => void;
};

type ChatMentionTriggerQuery = {
  kind: ChatMentionTriggerKind;
  query: string;
  range: {
    start: number;
    end: number;
  };
};

type ChatMentionMenuAnchor = {
  height: number;
  left: number;
  top: number;
  width: number;
};

type ChatMentionSelection = {
  end: number;
  start: number;
};

type CommandRecentsSnapshot = Partial<Record<CommandRecentsKind, CommandRecentsEntry[]>>;

function canStartTrigger(value: string, triggerIndex: number, trigger: "@" | "/"): boolean {
  if (triggerIndex === 0) {
    return true;
  }

  const previous = value[triggerIndex - 1] ?? "";
  if (/\s/.test(previous) || /[\u3400-\u9fff]/.test(previous)) {
    return true;
  }
  if (/[[({<，。！？、：；,.!?;:]/.test(previous)) {
    return true;
  }
  if (trigger === "@") {
    return !/[A-Za-z0-9._-]/.test(previous);
  }
  return !/[A-Za-z0-9._~:/?#@!$&'()*+,;=%-]/.test(previous);
}

function resolveTriggerQuery(value: string, caretIndex: number): ChatMentionTriggerQuery | null {
  const end = Math.min(Math.max(caretIndex, 0), value.length);
  const prefix = value.slice(0, end);
  const mentionIndex = prefix.lastIndexOf("@");
  const promptIndex = prefix.lastIndexOf("/");
  const triggerIndex = Math.max(mentionIndex, promptIndex);
  const trigger = triggerIndex >= 0 ? prefix[triggerIndex] : "";
  if (trigger !== "@" && trigger !== "/") {
    return null;
  }
  if (!canStartTrigger(value, triggerIndex, trigger)) {
    return null;
  }

  const query = prefix.slice(triggerIndex + 1);
  if (/\s/.test(query)) {
    return null;
  }

  return {
    kind: trigger === "@" ? "mention" : "prompt",
    query: query.toLowerCase(),
    range: { start: triggerIndex, end },
  };
}

function readTextareaSelection(textarea: HTMLTextAreaElement | null, fallback: number): ChatMentionSelection {
  if (!textarea) {
    return { start: fallback, end: fallback };
  }
  return {
    start: textarea.selectionStart,
    end: textarea.selectionEnd,
  };
}

function createTextareaCaretMirror(textarea: HTMLTextAreaElement) {
  const styles = window.getComputedStyle(textarea);
  const mirror = document.createElement("div");
  mirror.style.position = "absolute";
  mirror.style.visibility = "hidden";
  mirror.style.pointerEvents = "none";
  mirror.style.whiteSpace = "pre-wrap";
  mirror.style.overflowWrap = "break-word";
  mirror.style.boxSizing = styles.boxSizing;
  mirror.style.width = styles.width;
  mirror.style.padding = styles.padding;
  mirror.style.border = styles.border;
  mirror.style.font = styles.font;
  mirror.style.fontFamily = styles.fontFamily;
  mirror.style.fontSize = styles.fontSize;
  mirror.style.fontWeight = styles.fontWeight;
  mirror.style.letterSpacing = styles.letterSpacing;
  mirror.style.lineHeight = styles.lineHeight;
  mirror.style.tabSize = styles.tabSize;
  mirror.style.textTransform = styles.textTransform;
  return mirror;
}

function resolveTextareaCaretAnchor(
  textarea: HTMLTextAreaElement | null,
  fallbackAnchor: HTMLElement,
  caretIndex: number,
): ChatMentionMenuAnchor {
  const fallbackRect = fallbackAnchor.getBoundingClientRect();
  if (!textarea || typeof document === "undefined") {
    return fallbackRect;
  }

  const textareaRect = textarea.getBoundingClientRect();
  if (textareaRect.width <= 0 || textareaRect.height <= 0) {
    return fallbackRect;
  }

  const mirror = createTextareaCaretMirror(textarea);
  const textBeforeCaret = textarea.value.slice(0, caretIndex);
  mirror.textContent = textBeforeCaret;
  const marker = document.createElement("span");
  marker.textContent = "\u200b";
  mirror.appendChild(marker);
  document.body.appendChild(mirror);

  const markerRect = marker.getBoundingClientRect();
  const styles = window.getComputedStyle(textarea);
  const borderTop = Number.parseFloat(styles.borderTopWidth) || 0;
  const mirrorRect = mirror.getBoundingClientRect();
  const markerTop = textareaRect.top + markerRect.top - mirrorRect.top - textarea.scrollTop - borderTop;
  const lineHeight = Number.parseFloat(styles.lineHeight) || textareaRect.height;
  document.body.removeChild(mirror);

  return {
    height: Math.max(1, lineHeight),
    left: fallbackRect.left,
    top: Math.min(Math.max(markerTop, textareaRect.top), textareaRect.bottom),
    width: fallbackRect.width,
  };
}

function resolveContainerAnchor(anchor: HTMLElement): ChatMentionMenuAnchor {
  const rect = anchor.getBoundingClientRect();
  return {
    height: rect.height,
    left: rect.left,
    top: rect.top,
    width: rect.width,
  };
}

function removeTriggerRange(value: string, range: ChatMentionTriggerQuery["range"]): {
  caretIndex: number;
  value: string;
} {
  const trailingSpace = value[range.end] === " " ? 1 : 0;
  return {
    caretIndex: range.start,
    value: `${value.slice(0, range.start)}${value.slice(range.end + trailingSpace)}`,
  };
}

function replaceTriggerRange(value: string, range: ChatMentionTriggerQuery["range"], content: string): {
  caretIndex: number;
  value: string;
} {
  const nextContent = content.trim();
  return {
    caretIndex: range.start + nextContent.length,
    value: `${value.slice(0, range.start)}${nextContent}${value.slice(range.end)}`,
  };
}

function insertAtCaret(value: string, caretIndex: number, content: string): {
  caretIndex: number;
  value: string;
} {
  const index = Math.min(Math.max(caretIndex, 0), value.length);
  const nextContent = content.trim();
  return {
    caretIndex: index + nextContent.length,
    value: `${value.slice(0, index)}${nextContent}${value.slice(index)}`,
  };
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

function filterModels(
  modelOptions: ChatModelOption[],
  query: string,
  selectedPlatformModelName: string,
): ChatMentionModelMenuItem[] {
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
      selected: model.platformModelName === selectedPlatformModelName,
    }));
}

function promptsToItems(prompts: PromptPresetDTO[]): ChatMentionPromptMenuItem[] {
  return prompts.map((prompt) => ({
    id: `prompt:${prompt.id}`,
    kind: "prompt" as const,
    label: prompt.trigger || prompt.title,
    description: prompt.description || prompt.content,
    prompt,
    selected: false,
  }));
}

function skillsToItems(skills: SkillSummaryDTO[], selectedSkills: SkillSummaryDTO[]): ChatMentionSkillMenuItem[] {
  const selectedIDs = new Set(selectedSkills.map((skill) => skill.id));
  return skills.map((skill) => ({
    id: `skill:${skill.id}`,
    kind: "skill" as const,
    label: skill.trigger || skill.title,
    description: skill.description,
    skill,
    selected: selectedIDs.has(skill.id),
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

function mentionItemRecentsID(item: ChatMentionMenuItem): string {
  switch (item.kind) {
    case "model":
      return item.model.platformModelName;
    case "file":
      return item.file.fileID;
    case "tool":
      return String(item.tool.id);
    case "skill":
      return String(item.skill.id);
    case "prompt":
      return String(item.prompt.id);
  }
}

function resolveRecentItems<T extends ChatMentionMenuItem>(
  items: T[],
  recents: CommandRecentsEntry[] | undefined,
  limit: number,
): Array<{ item: T; usedAt: number }> {
  if (!recents || recents.length === 0 || items.length === 0) {
    return [];
  }
  const byID = new Map(items.map((item) => [mentionItemRecentsID(item), item]));
  const resolved: Array<{ item: T; usedAt: number }> = [];
  for (const entry of recents) {
    const item = byID.get(entry.id);
    if (!item) {
      continue;
    }
    resolved.push({ item, usedAt: entry.usedAt });
    if (resolved.length >= limit) {
      break;
    }
  }
  return resolved;
}

function orderByRecents<T extends ChatMentionMenuItem>(
  items: T[],
  recents: CommandRecentsEntry[] | undefined,
): T[] {
  if (!recents || recents.length === 0 || items.length === 0) {
    return items;
  }
  const rank = new Map(recents.map((entry, index) => [entry.id, index]));
  const recentItems: T[] = [];
  const rest: T[] = [];
  for (const item of items) {
    if (rank.has(mentionItemRecentsID(item))) {
      recentItems.push(item);
    } else {
      rest.push(item);
    }
  }
  recentItems.sort(
    (a, b) => (rank.get(mentionItemRecentsID(a)) ?? 0) - (rank.get(mentionItemRecentsID(b)) ?? 0),
  );
  return [...recentItems, ...rest];
}

type ChatMentionKindData = {
  items: ChatMentionMenuItem[];
  /** Total available count; differs from items.length for server-paged kinds. */
  total: number;
  loading: boolean;
};

type BuildRowsArgs = {
  tab: ChatMentionMenuTab;
  sessionKinds: ChatMentionMenuKind[];
  kindData: Partial<Record<ChatMentionMenuKind, ChatMentionKindData>>;
  recents: CommandRecentsSnapshot;
  hasQuery: boolean;
  filesHasMore: boolean;
  filesLoadingMore: boolean;
};

function buildScopeGroupedRows(
  rows: ChatMentionMenuRow[],
  items: ChatMentionMenuItem[],
  scopeOf: (item: ChatMentionMenuItem) => "builtin" | "user",
) {
  const builtin = items.filter((item) => scopeOf(item) === "builtin");
  const mine = items.filter((item) => scopeOf(item) !== "builtin");
  if (builtin.length > 0) {
    rows.push({ type: "header", key: "header:builtin", labelKey: "mention.groups.builtin" });
    for (const item of builtin) {
      rows.push({ type: "item", key: `tab:${item.id}`, item });
    }
  }
  if (mine.length > 0) {
    rows.push({ type: "header", key: "header:mine", labelKey: "mention.groups.mine" });
    for (const item of mine) {
      rows.push({ type: "item", key: `tab:${item.id}`, item });
    }
  }
}

function buildKindTabRows(
  kind: ChatMentionMenuKind,
  data: ChatMentionKindData,
  recents: CommandRecentsSnapshot,
  filesHasMore: boolean,
  filesLoadingMore: boolean,
): ChatMentionMenuRow[] {
  const rows: ChatMentionMenuRow[] = [];

  if (kind !== "file") {
    const recentItems = resolveRecentItems(data.items, recents[kind], 5);
    if (recentItems.length > 0) {
      rows.push({ type: "header", key: "header:recent", labelKey: "mention.recent" });
      for (const { item } of recentItems) {
        rows.push({ type: "item", key: `recent:${item.id}`, item });
      }
    }
  }

  if (kind === "model") {
    const groups = new Map<string, { label: string; items: ChatMentionMenuItem[] }>();
    for (const item of data.items) {
      if (item.kind !== "model") {
        continue;
      }
      const presentation = resolveModelPresentationGroup(item.model);
      const group = groups.get(presentation.key);
      if (group) {
        group.items.push(item);
      } else {
        groups.set(presentation.key, { label: presentation.label, items: [item] });
      }
    }
    for (const [key, group] of groups) {
      rows.push({ type: "header", key: `header:${key}`, label: group.label });
      for (const item of group.items) {
        rows.push({ type: "item", key: `tab:${item.id}`, item });
      }
    }
    return rows;
  }

  if (kind === "tool") {
    const groups = new Map<string, { label: string; items: ChatMentionMenuItem[] }>();
    for (const item of data.items) {
      if (item.kind !== "tool") {
        continue;
      }
      const serverName = item.tool.serverName?.trim() ?? "";
      const key = Number.isFinite(item.tool.serverID) && item.tool.serverID > 0
        ? `server:${item.tool.serverID}`
        : `server-name:${serverName || "unknown"}`;
      const group = groups.get(key);
      if (group) {
        group.items.push(item);
      } else {
        groups.set(key, { label: serverName, items: [item] });
      }
    }
    for (const [key, group] of groups) {
      if (group.label) {
        rows.push({ type: "header", key: `header:${key}`, label: group.label });
      }
      for (const item of group.items) {
        rows.push({ type: "item", key: `tab:${item.id}`, item });
      }
    }
    return rows;
  }

  if (kind === "skill") {
    buildScopeGroupedRows(rows, data.items, (item) => (item.kind === "skill" ? item.skill.scope : "user"));
    return rows;
  }

  if (kind === "prompt") {
    buildScopeGroupedRows(rows, data.items, (item) => (item.kind === "prompt" ? item.prompt.scope : "user"));
    return rows;
  }

  for (const item of data.items) {
    rows.push({ type: "item", key: `tab:${item.id}`, item });
  }
  if (data.loading && data.items.length === 0) {
    rows.push({ type: "loading", key: "loading:file" });
  } else if (filesLoadingMore) {
    rows.push({ type: "loading", key: "loading:file-more" });
  } else if (filesHasMore) {
    // Placeholder row keeps scroll room so the load-more trigger can fire.
    rows.push({ type: "loading", key: "loading:file-pending" });
  }
  return rows;
}

function buildMenuRows({
  tab,
  sessionKinds,
  kindData,
  recents,
  hasQuery,
  filesHasMore,
  filesLoadingMore,
}: BuildRowsArgs): ChatMentionMenuRow[] {
  if (tab !== "all") {
    const data = kindData[tab];
    if (!data) {
      return [];
    }
    return buildKindTabRows(tab, data, recents, filesHasMore, filesLoadingMore);
  }

  const rows: ChatMentionMenuRow[] = [];

  const recentShownIDs = new Set<string>();
  const recentShownByKind = new Map<ChatMentionMenuKind, number>();
  if (!hasQuery) {
    const merged: Array<{ item: ChatMentionMenuItem; usedAt: number }> = [];
    for (const kind of sessionKinds) {
      const data = kindData[kind];
      if (!data) {
        continue;
      }
      merged.push(...resolveRecentItems(data.items, recents[kind], 5));
    }
    merged.sort((a, b) => b.usedAt - a.usedAt);
    const recentRows = merged.slice(0, 5);
    if (recentRows.length > 0) {
      rows.push({ type: "header", key: "header:recent", labelKey: "mention.recent" });
      for (const { item } of recentRows) {
        recentShownIDs.add(item.id);
        recentShownByKind.set(item.kind, (recentShownByKind.get(item.kind) ?? 0) + 1);
        rows.push({ type: "item", key: `recent:${item.id}`, item });
      }
    }
  }

  for (const kind of sessionKinds) {
    const data = kindData[kind];
    if (!data || (data.items.length === 0 && !data.loading)) {
      continue;
    }
    const ordered = orderByRecents(data.items, recents[kind]).filter(
      (item) => !recentShownIDs.has(item.id),
    );
    const preview = ordered.slice(0, 4);
    if (preview.length === 0 && !data.loading) {
      continue;
    }
    rows.push({ type: "header", key: `header:${kind}`, labelKey: `mention.sections.${kind}` });
    for (const item of preview) {
      rows.push({ type: "item", key: `all:${item.id}`, item });
    }
    if (data.loading && preview.length === 0) {
      rows.push({ type: "loading", key: `loading:${kind}` });
    }
    const shownCount = preview.length + (recentShownByKind.get(kind) ?? 0);
    if (data.total > shownCount) {
      rows.push({ type: "viewAll", key: `viewAll:${kind}`, tab: kind, count: data.total });
    }
  }

  return rows;
}

function resolveRowHeight(row: ChatMentionMenuRow): number {
  switch (row.type) {
    case "header":
    case "viewAll":
      return 28;
    default:
      return 32;
  }
}

function resolveMentionMenuWidth(anchorWidth: number, viewportWidth: number): number {
  const availableWidth = Math.max(0, viewportWidth - 16 * 2);
  return Math.min(anchorWidth, availableWidth);
}

function resolveMentionMenuContentHeight(rows: ChatMentionMenuRow[], showTabBar: boolean): number {
  const tabBarHeight = showTabBar ? 34 : 0;
  const maxHeight = showTabBar ? 340 : 280;
  if (rows.length === 0) {
    return Math.min(maxHeight, tabBarHeight + 96 + 12);
  }
  const rowsHeight = rows.reduce((total, row) => total + resolveRowHeight(row), 0);
  const gaps = Math.max(0, rows.length - 1) * 2;
  return Math.min(maxHeight, tabBarHeight + rowsHeight + gaps + 12);
}

function resolveMentionMenuLayout(
  anchor: ChatMentionMenuAnchor,
  desiredHeight: number,
  viewportWidth: number,
  viewportHeight: number,
  placementPreference: ChatMentionMenuPlacementPreference,
): ChatMentionMenuLayout {
  const preferredTop = anchor.top + anchor.height + 8;
  const preferredBottom = anchor.top - 8;
  const availableBelow = viewportHeight - preferredTop - 16;
  const availableAbove = preferredBottom - 16;
  const anchorInLowerHalf = anchor.top + anchor.height / 2 > viewportHeight / 2;
  const hasUsableAbove = availableAbove >= Math.min(desiredHeight, 32);
  const openBelow =
    placementPreference === "bottom" ||
    (placementPreference === "top"
      ? !hasUsableAbove
      : !anchorInLowerHalf ||
        availableBelow >= Math.min(desiredHeight, 32) ||
        availableBelow >= availableAbove);
  const availableHeight = Math.max(0, openBelow ? availableBelow : availableAbove);
  const maxHeight = Math.max(
    Math.min(32, availableHeight),
    Math.min(desiredHeight, availableHeight),
  );
  const preferredWidth = resolveMentionMenuWidth(anchor.width, viewportWidth);
  const preferredLeft = anchor.left;
  const maxLeft = Math.max(16, viewportWidth - preferredWidth - 16);
  const left = Math.min(Math.max(preferredLeft, 16), maxLeft);
  const width = Math.min(preferredWidth, Math.max(0, viewportWidth - left - 16));

  if (openBelow) {
    return { height: maxHeight, left, placement: "bottom", top: preferredTop, width };
  }

  return {
    bottom: Math.max(16, viewportHeight - preferredBottom),
    height: maxHeight,
    left,
    placement: "top",
    width,
  };
}

function mentionMenuLayoutsEqual(
  previous: ChatMentionMenuLayout | null,
  next: ChatMentionMenuLayout,
): boolean {
  return Boolean(
    previous &&
      previous.bottom === next.bottom &&
      previous.height === next.height &&
      previous.left === next.left &&
      previous.placement === next.placement &&
      previous.top === next.top &&
      previous.width === next.width,
  );
}

export function useChatMentionMenu({
  attachments,
  availableTools,
  defaultFileLabel,
  disabled,
  draft,
  maxSelectedTools,
  maxSelectedSkills,
  modelOptions,
  selectedSkills = [],
  selectedPlatformModelName,
  selectedToolIDs,
  anchorRef,
  textareaRef,
  toolsDisabled,
  onDraftChange,
  onSelectedSkillsChange,
  enabledKinds = DEFAULT_MENTION_MENU_KINDS,
  onFileSelect,
  onModelChange,
  placementAnchor = "caret",
  placementPreference = "auto",
  onModelCatalogRefresh,
  onSelectedToolsChange,
  onSkillLimitReached,
  onToolLimitReached,
}: ChatMentionMenuControllerArgs) {
  const menuRef = React.useRef<HTMLDivElement | null>(null);
  const menuID = React.useId();
  const [inputFocused, setInputFocused] = React.useState(false);
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [activeTab, setActiveTab] = React.useState<ChatMentionMenuTab>("all");
  const [browseKind, setBrowseKind] = React.useState<ChatMentionTriggerKind | null>(null);
  const [dismissedTriggerKey, setDismissedTriggerKey] = React.useState<string | null>(null);
  const [menuLayout, setMenuLayout] = React.useState<ChatMentionMenuLayout | null>(null);
  const [recentsSnapshot, setRecentsSnapshot] = React.useState<CommandRecentsSnapshot>({});
  const [files, setFiles] = React.useState<FileObjectDTO[]>([]);
  const [filesTotal, setFilesTotal] = React.useState(0);
  const [filesPage, setFilesPage] = React.useState(1);
  const [filesLoading, setFilesLoading] = React.useState(false);
  const [filesLoadingMore, setFilesLoadingMore] = React.useState(false);
  const [filesQueryKey, setFilesQueryKey] = React.useState<string | null>(null);
  const [prompts, setPrompts] = React.useState<PromptPresetDTO[]>([]);
  const [promptsTotal, setPromptsTotal] = React.useState(0);
  const [promptsLoading, setPromptsLoading] = React.useState(false);
  const [skills, setSkills] = React.useState<SkillSummaryDTO[]>([]);
  const [skillsTotal, setSkillsTotal] = React.useState(0);
  const [skillsLoading, setSkillsLoading] = React.useState(false);
  const [selection, setSelection] = React.useState<ChatMentionSelection>(() => ({
    end: draft.length,
    start: draft.length,
  }));
  const modelCatalogRefreshRequestedRef = React.useRef(false);
  const filesGenerationRef = React.useRef(0);
  const enabledKindSet = React.useMemo(() => new Set(enabledKinds), [enabledKinds]);
  const triggerQuery = selection.start === selection.end ? resolveTriggerQuery(draft, selection.start) : null;
  const sessionKind: ChatMentionTriggerKind | null = triggerQuery?.kind ?? browseKind;
  const sessionActive = sessionKind !== null;
  const query = triggerQuery ? triggerQuery.query : browseKind ? "" : null;
  const normalizedQuery = (query ?? "").trim().toLowerCase();
  const hasQuery = normalizedQuery.length > 0;
  const triggerKey = triggerQuery
    ? `${draft}:${triggerQuery.kind}:${triggerQuery.range.start}:${triggerQuery.range.end}:${triggerQuery.query}`
    : null;

  const sessionKinds = React.useMemo<ChatMentionMenuKind[]>(() => {
    if (sessionKind === "mention") {
      return MENTION_TRIGGER_KINDS.filter(
        (kind) => enabledKindSet.has(kind) && (kind !== "tool" || !toolsDisabled),
      );
    }
    if (sessionKind === "prompt") {
      return PROMPT_TRIGGER_KINDS.filter((kind) => enabledKindSet.has(kind));
    }
    return [];
  }, [enabledKindSet, sessionKind, toolsDisabled]);
  const showTabBar = sessionKinds.length > 1;
  const tabIDs = React.useMemo<ChatMentionMenuTab[]>(
    () => (showTabBar ? ["all", ...sessionKinds] : sessionKinds),
    [sessionKinds, showTabBar],
  );
  const effectiveTab: ChatMentionMenuTab = tabIDs.includes(activeTab) ? activeTab : tabIDs[0] ?? "all";

  const updateSelection = React.useCallback(() => {
    const nextSelection = readTextareaSelection(textareaRef.current, draft.length);
    setSelection((currentSelection) => (
      currentSelection.start === nextSelection.start && currentSelection.end === nextSelection.end
        ? currentSelection
        : nextSelection
    ));
  }, [draft.length, textareaRef]);

  React.useLayoutEffect(() => {
    updateSelection();
  }, [draft, updateSelection]);

  React.useEffect(() => {
    if (disabled) {
      setBrowseKind(null);
    }
  }, [disabled]);

  React.useEffect(() => {
    if (sessionKind === null) {
      return;
    }
    setActiveTab("all");
  }, [sessionKind]);

  // Freeze the recents snapshot per menu session so rows do not reorder mid-interaction.
  React.useEffect(() => {
    if (!sessionActive) {
      return;
    }
    setRecentsSnapshot({
      model: readCommandRecents("model"),
      file: readCommandRecents("file"),
      tool: readCommandRecents("tool"),
      skill: readCommandRecents("skill"),
      prompt: readCommandRecents("prompt"),
    });
  }, [sessionActive]);

  React.useEffect(() => {
    if (!inputFocused || sessionKind !== "mention" || !enabledKindSet.has("model")) {
      modelCatalogRefreshRequestedRef.current = false;
      return;
    }
    if (disabled || modelCatalogRefreshRequestedRef.current || !onModelCatalogRefresh) {
      return;
    }

    modelCatalogRefreshRequestedRef.current = true;
    void Promise.resolve(onModelCatalogRefresh()).catch((): undefined => undefined);
  }, [disabled, enabledKindSet, inputFocused, onModelCatalogRefresh, sessionKind]);

  const fileSearchKey =
    sessionKind === "mention" && enabledKindSet.has("file") && !disabled ? normalizedQuery : null;

  React.useEffect(() => {
    filesGenerationRef.current += 1;
    if (fileSearchKey === null) {
      setFiles([]);
      setFilesTotal(0);
      setFilesPage(1);
      setFilesQueryKey(null);
      setFilesLoading(false);
      setFilesLoadingMore(false);
      return;
    }

    const generation = filesGenerationRef.current;
    const sessionRevision = readSessionRevision();
    const applyPage = (page: { files: FileObjectDTO[]; total: number }) => {
      setFiles(page.files);
      setFilesTotal(page.total);
      setFilesPage(1);
      setFilesQueryKey(fileSearchKey);
      setFilesLoading(false);
      setFilesLoadingMore(false);
    };

    const cachedPage = readMentionFileSearchCache(sessionRevision, fileSearchKey, 1);
    if (cachedPage) {
      applyPage(cachedPage);
      return;
    }

    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setFilesLoading(true);
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (!token || controller.signal.aborted || filesGenerationRef.current !== generation) {
            return;
          }
          const page = await searchMentionFiles({
            accessToken: token,
            query: fileSearchKey,
            page: 1,
            sessionRevision,
            signal: controller.signal,
          });
          if (!controller.signal.aborted && filesGenerationRef.current === generation) {
            applyPage(page);
          }
        } catch {
          if (!controller.signal.aborted && filesGenerationRef.current === generation) {
            setFiles([]);
            setFilesTotal(0);
            setFilesQueryKey(fileSearchKey);
            setFilesLoading(false);
          }
        } finally {
          if (!controller.signal.aborted && filesGenerationRef.current === generation) {
            setFilesLoading(false);
          }
        }
      })();
    }, 180);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [fileSearchKey]);

  const filesHasMore = filesQueryKey !== null && files.length < filesTotal;

  const loadMoreFiles = React.useCallback(() => {
    if (fileSearchKey === null || filesQueryKey !== fileSearchKey || filesLoading || filesLoadingMore) {
      return;
    }
    if (files.length >= filesTotal) {
      return;
    }
    const generation = filesGenerationRef.current;
    const nextPage = filesPage + 1;
    const sessionRevision = readSessionRevision();
    setFilesLoadingMore(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token || filesGenerationRef.current !== generation) {
          return;
        }
        const page = await searchMentionFiles({
          accessToken: token,
          query: fileSearchKey,
          page: nextPage,
          sessionRevision,
        });
        if (filesGenerationRef.current !== generation) {
          return;
        }
        setFiles((current) => {
          const seen = new Set(current.map((file) => file.fileID));
          return [...current, ...page.files.filter((file) => !seen.has(file.fileID))];
        });
        setFilesTotal(page.total);
        setFilesPage(nextPage);
      } catch {
        // Keep already-loaded pages; a later scroll retries.
      } finally {
        if (filesGenerationRef.current === generation) {
          setFilesLoadingMore(false);
        }
      }
    })();
  }, [fileSearchKey, files.length, filesLoading, filesLoadingMore, filesPage, filesQueryKey, filesTotal]);

  const promptSearchKey =
    sessionKind === "prompt" && enabledKindSet.has("prompt") && !disabled ? normalizedQuery : null;

  React.useEffect(() => {
    if (promptSearchKey === null) {
      setPrompts([]);
      setPromptsTotal(0);
      setPromptsLoading(false);
      return;
    }

    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setPromptsLoading(true);
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (!token || controller.signal.aborted) {
            return;
          }
          const data = await listVisiblePromptPresets(token, { query: promptSearchKey, page: 1, pageSize: 50 });
          if (!controller.signal.aborted) {
            setPrompts(data.results);
            setPromptsTotal(data.total ?? data.results.length);
          }
        } catch {
          if (!controller.signal.aborted) {
            setPrompts([]);
            setPromptsTotal(0);
          }
        } finally {
          if (!controller.signal.aborted) {
            setPromptsLoading(false);
          }
        }
      })();
    }, 180);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [promptSearchKey]);

  const skillSearchKey =
    sessionKind === "prompt" && enabledKindSet.has("skill") && !disabled ? normalizedQuery : null;

  React.useEffect(() => {
    if (skillSearchKey === null) {
      setSkills([]);
      setSkillsTotal(0);
      setSkillsLoading(false);
      return;
    }

    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setSkillsLoading(true);
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (!token || controller.signal.aborted) {
            return;
          }
          const data = await listVisibleSkills(token, { query: skillSearchKey, page: 1, pageSize: 50 }, controller.signal);
          if (!controller.signal.aborted) {
            setSkills(data.results);
            setSkillsTotal(data.total ?? data.results.length);
          }
        } catch {
          if (!controller.signal.aborted) {
            setSkills([]);
            setSkillsTotal(0);
          }
        } finally {
          if (!controller.signal.aborted) {
            setSkillsLoading(false);
          }
        }
      })();
    }, 180);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [skillSearchKey]);

  const kindData = React.useMemo<Partial<Record<ChatMentionMenuKind, ChatMentionKindData>>>(() => {
    if (query === null) {
      return {};
    }
    const data: Partial<Record<ChatMentionMenuKind, ChatMentionKindData>> = {};
    for (const kind of sessionKinds) {
      if (kind === "model") {
        const items = filterModels(modelOptions, normalizedQuery, selectedPlatformModelName);
        data.model = { items, total: items.length, loading: false };
        continue;
      }
      if (kind === "tool") {
        const items = filterTools(availableTools, normalizedQuery, selectedToolIDs);
        data.tool = { items, total: items.length, loading: false };
        continue;
      }
      if (kind === "file") {
        const ready = filesQueryKey === normalizedQuery;
        const items = ready ? filesToItems(files, attachments, defaultFileLabel) : [];
        data.file = {
          items,
          total: ready ? Math.max(filesTotal, items.length) : 0,
          loading: filesLoading || (!ready && fileSearchKey !== null),
        };
        continue;
      }
      if (kind === "skill") {
        const items = skillsToItems(skills, selectedSkills);
        data.skill = { items, total: Math.max(skillsTotal, items.length), loading: skillsLoading };
        continue;
      }
      const items = promptsToItems(prompts);
      data.prompt = { items, total: Math.max(promptsTotal, items.length), loading: promptsLoading };
    }
    return data;
  }, [
    attachments,
    availableTools,
    defaultFileLabel,
    fileSearchKey,
    files,
    filesLoading,
    filesQueryKey,
    filesTotal,
    modelOptions,
    normalizedQuery,
    prompts,
    promptsLoading,
    promptsTotal,
    query,
    selectedPlatformModelName,
    selectedSkills,
    selectedToolIDs,
    sessionKinds,
    skills,
    skillsLoading,
    skillsTotal,
  ]);

  const rows = React.useMemo(
    () =>
      buildMenuRows({
        tab: effectiveTab,
        sessionKinds,
        kindData,
        recents: recentsSnapshot,
        hasQuery,
        filesHasMore,
        filesLoadingMore,
      }),
    [effectiveTab, filesHasMore, filesLoadingMore, hasQuery, kindData, recentsSnapshot, sessionKinds],
  );
  const selectableRows = React.useMemo(
    () => rows.filter((row): row is Extract<ChatMentionMenuRow, { type: "item" | "viewAll" }> =>
      row.type === "item" || row.type === "viewAll",
    ),
    [rows],
  );

  const tabs = React.useMemo<ChatMentionMenuTabInfo[]>(
    () =>
      tabIDs.map((id) => ({
        id,
        count: hasQuery && id !== "all" ? kindData[id]?.total ?? 0 : null,
      })),
    [hasQuery, kindData, tabIDs],
  );

  const hasAnyContent = React.useMemo(
    () => sessionKinds.some((kind) => {
      const data = kindData[kind];
      return Boolean(data && (data.total > 0 || data.loading));
    }),
    [kindData, sessionKinds],
  );

  const open =
    inputFocused &&
    sessionActive &&
    !disabled &&
    (triggerQuery ? dismissedTriggerKey !== triggerKey : true) &&
    hasAnyContent;
  const activeRow = open ? selectableRows[Math.min(activeIndex, selectableRows.length - 1)] ?? null : null;
  const activeRowKey = activeRow?.key ?? null;

  React.useEffect(() => {
    setActiveIndex(0);
  }, [normalizedQuery, effectiveTab]);

  React.useEffect(() => {
    setActiveIndex((current) => (selectableRows.length === 0 ? 0 : Math.min(current, selectableRows.length - 1)));
  }, [selectableRows.length]);

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

  const triggerStart = triggerQuery?.range.start ?? null;

  const updateLayout = React.useCallback(() => {
    if (!open || typeof window === "undefined") {
      return;
    }

    const anchor = anchorRef.current;
    if (!anchor) {
      return;
    }

    const menuAnchor =
      placementAnchor === "container" || triggerStart === null
        ? resolveContainerAnchor(anchor)
        : resolveTextareaCaretAnchor(textareaRef.current, anchor, triggerStart);
    const desiredHeight = resolveMentionMenuContentHeight(rows, showTabBar);
    const nextLayout = resolveMentionMenuLayout(
      menuAnchor,
      desiredHeight,
      window.innerWidth,
      window.innerHeight,
      placementPreference,
    );
    setMenuLayout((current) => (mentionMenuLayoutsEqual(current, nextLayout) ? current : nextLayout));
  }, [anchorRef, open, placementAnchor, placementPreference, rows, showTabBar, textareaRef, triggerStart]);

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

  const focusTextarea = React.useCallback((caretIndex: number) => {
    window.requestAnimationFrame(() => {
      const textarea = textareaRef.current;
      textarea?.focus();
      textarea?.setSelectionRange(caretIndex, caretIndex);
    });
  }, [textareaRef]);

  const closeSession = React.useCallback(() => {
    if (browseKind !== null) {
      setBrowseKind(null);
    }
    if (!triggerQuery) {
      return;
    }
    const nextDraft = removeTriggerRange(draft, triggerQuery.range);
    onDraftChange(nextDraft.value);
    setDismissedTriggerKey(null);
    focusTextarea(nextDraft.caretIndex);
  }, [browseKind, draft, focusTextarea, onDraftChange, triggerQuery]);

  // First multi-select pick: remove the trigger text but keep the menu open for more picks.
  const enterBrowseMode = React.useCallback(() => {
    if (!triggerQuery) {
      return;
    }
    const nextDraft = removeTriggerRange(draft, triggerQuery.range);
    setBrowseKind(triggerQuery.kind);
    onDraftChange(nextDraft.value);
    setDismissedTriggerKey(null);
    focusTextarea(nextDraft.caretIndex);
  }, [draft, focusTextarea, onDraftChange, triggerQuery]);

  const selectTab = React.useCallback((tab: ChatMentionMenuTab) => {
    setActiveTab(tab);
    setActiveIndex(0);
  }, []);

  const select = React.useCallback(
    (row: ChatMentionMenuRow) => {
      if (row.type === "viewAll") {
        selectTab(row.tab);
        return;
      }
      if (row.type !== "item") {
        return;
      }
      const item = row.item;

      if (item.kind === "model") {
        recordCommandRecentUsage("model", item.model.platformModelName);
        onModelChange(item.model.platformModelName);
        closeSession();
        return;
      }

      if (item.kind === "prompt") {
        recordCommandRecentUsage("prompt", String(item.prompt.id));
        if (triggerQuery) {
          const nextDraft = replaceTriggerRange(draft, triggerQuery.range, item.prompt.content);
          onDraftChange(nextDraft.value);
          setDismissedTriggerKey(null);
          setBrowseKind(null);
          focusTextarea(nextDraft.caretIndex);
          return;
        }
        const caret = readTextareaSelection(textareaRef.current, draft.length).start;
        const nextDraft = insertAtCaret(draft, caret, item.prompt.content);
        onDraftChange(nextDraft.value);
        setBrowseKind(null);
        focusTextarea(nextDraft.caretIndex);
        return;
      }

      if (item.kind === "skill") {
        const alreadySelected = selectedSkills.some((skill) => skill.id === item.skill.id);
        if (!alreadySelected && selectedSkills.length >= maxSelectedSkills) {
          onSkillLimitReached?.();
          return;
        }
        if (!alreadySelected) {
          recordCommandRecentUsage("skill", String(item.skill.id));
        }
        onSelectedSkillsChange?.(
          alreadySelected
            ? selectedSkills.filter((skill) => skill.id !== item.skill.id)
            : [...selectedSkills, item.skill],
        );
        if (triggerQuery) {
          enterBrowseMode();
        }
        return;
      }

      if (item.kind === "tool") {
        const alreadySelected = selectedToolIDs.includes(item.tool.id);
        if (!alreadySelected && selectedToolIDs.length >= maxSelectedTools) {
          onToolLimitReached?.();
          return;
        }
        if (!alreadySelected) {
          recordCommandRecentUsage("tool", String(item.tool.id));
        }
        onSelectedToolsChange(
          alreadySelected
            ? selectedToolIDs.filter((toolID) => toolID !== item.tool.id)
            : [...selectedToolIDs, item.tool.id],
        );
        if (triggerQuery) {
          enterBrowseMode();
        }
        return;
      }

      recordCommandRecentUsage("file", item.file.fileID);
      void onFileSelect(item.file);
      closeSession();
    },
    [
      closeSession,
      draft,
      enterBrowseMode,
      focusTextarea,
      maxSelectedSkills,
      maxSelectedTools,
      onDraftChange,
      onFileSelect,
      onModelChange,
      onSelectedSkillsChange,
      onSelectedToolsChange,
      onSkillLimitReached,
      onToolLimitReached,
      selectTab,
      selectedSkills,
      selectedToolIDs,
      textareaRef,
      triggerQuery,
    ],
  );

  const handleChange = React.useCallback(
    (value: string) => {
      if (dismissedTriggerKey !== null) {
        setDismissedTriggerKey(null);
      }
      if (browseKind !== null) {
        setBrowseKind(null);
      }
      updateSelection();
      onDraftChange(value);
    },
    [browseKind, dismissedTriggerKey, onDraftChange, updateSelection],
  );

  const handleSelectionChange = React.useCallback(() => {
    updateSelection();
  }, [updateSelection]);

  const handleListScroll = React.useCallback(
    (event: React.UIEvent<HTMLElement>) => {
      if (effectiveTab !== "file" || !filesHasMore || filesLoading || filesLoadingMore) {
        return;
      }
      const target = event.currentTarget;
      const remaining = target.scrollHeight - target.clientHeight - target.scrollTop;
      if (remaining <= 48) {
        loadMoreFiles();
      }
    },
    [effectiveTab, filesHasMore, filesLoading, filesLoadingMore, loadMoreFiles],
  );

  const handleKeyDown = React.useCallback(
    (event: React.KeyboardEvent<HTMLTextAreaElement>): boolean => {
      if (!open) {
        return false;
      }
      if (event.key === "ArrowDown") {
        event.preventDefault();
        if (selectableRows.length > 0) {
          setActiveIndex((current) => (current + 1) % selectableRows.length);
        }
        return true;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        if (selectableRows.length > 0) {
          setActiveIndex((current) => (current - 1 + selectableRows.length) % selectableRows.length);
        }
        return true;
      }
      if (event.key === "Tab" && showTabBar) {
        event.preventDefault();
        const currentIndex = tabIDs.indexOf(effectiveTab);
        const delta = event.shiftKey ? -1 : 1;
        const nextTab = tabIDs[(currentIndex + delta + tabIDs.length) % tabIDs.length];
        if (nextTab) {
          selectTab(nextTab);
        }
        return true;
      }
      if ((event.key === "Enter" || (event.key === "Tab" && !showTabBar)) && activeRow) {
        event.preventDefault();
        select(activeRow);
        return true;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        if (browseKind !== null) {
          setBrowseKind(null);
        } else {
          setDismissedTriggerKey(triggerKey);
        }
        return true;
      }
      return false;
    },
    [activeRow, browseKind, effectiveTab, open, select, selectTab, selectableRows.length, showTabBar, tabIDs, triggerKey],
  );

  return {
    activeRowKey,
    activeTab: effectiveTab,
    handleBlur: () => {
      setInputFocused(false);
      setBrowseKind(null);
    },
    handleChange,
    handleFocus: () => {
      setInputFocused(true);
      updateSelection();
    },
    handleKeyDown,
    handleListScroll,
    handleSelectionChange,
    menuID,
    menuRef,
    menuLayout,
    menuReady: open && menuLayout !== null && menuLayout.height > 0 && menuLayout.width > 0,
    open,
    rows,
    select,
    selectTab,
    showTabBar,
    tabs,
  };
}
