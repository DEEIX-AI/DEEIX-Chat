"use client";

import * as React from "react";

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

const MENTION_MENU_MAX_HEIGHT = 280;
const MENTION_MENU_MIN_HEIGHT = 32;
const MENTION_MENU_ROW_HEIGHT = 32;
const MENTION_MENU_ROW_GAP = 2;
const MENTION_MENU_SECTION_HEADER_HEIGHT = 28;
const MENTION_MENU_SECTION_GAP = 2;
const MENTION_MENU_CHROME_HEIGHT = 12;
const MENTION_MENU_VIEWPORT_GUTTER = 16;
const MENTION_MENU_OFFSET = 8;
const MENTION_MENU_FILE_QUERY_DELAY_MS = 180;
const MENTION_MENU_PROMPT_QUERY_DELAY_MS = 180;
const DEFAULT_MENTION_MENU_KINDS: readonly ChatMentionMenuKind[] = ["model", "file", "tool", "skill", "prompt"];

export type ChatMentionMenuKind = "file" | "tool" | "model" | "skill" | "prompt";

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

type ChatMentionMenuPlacementPreference = "auto" | "bottom" | "top";
type ChatMentionMenuPlacementAnchor = "caret" | "container";

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
  kind: "mention" | "prompt";
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

function resolveTriggerQuery(value: string, caretIndex: number): ChatMentionTriggerQuery | null {
  const end = Math.min(Math.max(caretIndex, 0), value.length);
  let start = end;
  while (start > 0 && !/\s/.test(value[start - 1] ?? "")) {
    start -= 1;
  }

  const token = value.slice(start, end);
  const trigger = token[0];
  if (trigger !== "@" && trigger !== "/") {
    return null;
  }

  return {
    kind: trigger === "@" ? "mention" : "prompt",
    query: token.slice(1).toLowerCase(),
    range: { start, end },
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

function buildSections({
  attachments,
  availableTools,
  defaultFileLabel,
  files,
  filesQuery,
  fileLoading,
  promptLoading,
  skillLoading,
  modelOptions,
  prompts,
  skills,
  query,
  queryKind,
  selectedPlatformModelName,
  selectedSkills = [],
  selectedToolIDs,
  toolsDisabled,
  enabledKinds,
}: {
  attachments: PendingAttachment[];
  availableTools: MCPToolDTO[];
  defaultFileLabel: string;
  files: FileObjectDTO[];
  filesQuery: string;
  fileLoading: boolean;
  modelOptions: ChatModelOption[];
  prompts: PromptPresetDTO[];
  promptLoading: boolean;
  skills: SkillSummaryDTO[];
  skillLoading: boolean;
  query: string | null;
  queryKind: "mention" | "prompt" | null;
  selectedPlatformModelName: string;
  selectedSkills: SkillSummaryDTO[];
  selectedToolIDs: number[];
  toolsDisabled: boolean;
  enabledKinds: ReadonlySet<ChatMentionMenuKind>;
}): ChatMentionMenuSection[] {
  if (query === null) {
    return [];
  }

  const normalizedQuery = query.trim().toLowerCase();
  if (queryKind === "prompt") {
    const sections: ChatMentionMenuSection[] = [];
    if (enabledKinds.has("skill")) {
      const skillItems = skillLoading ? [] : skillsToItems(skills, selectedSkills);
      if (skillItems.length > 0) {
        sections.push({ kind: "skill" as const, items: skillItems });
      }
    }
    if (enabledKinds.has("prompt")) {
      const promptItems = promptLoading ? [] : promptsToItems(prompts);
      if (promptItems.length > 0) {
        sections.push({ kind: "prompt" as const, items: promptItems });
      }
    }
    if (sections.length === 0) {
      return [];
    }
    return sections;
  }

  const fileItems = enabledKinds.has("file") && !fileLoading && filesQuery === normalizedQuery
    ? filesToItems(files, attachments, defaultFileLabel)
    : [];
  const toolItems = enabledKinds.has("tool") && !toolsDisabled
    ? filterTools(availableTools, query, selectedToolIDs)
    : [];
  const modelItems = enabledKinds.has("model")
    ? filterModels(modelOptions, query).map((item) => ({
        ...item,
        selected: item.model.platformModelName === selectedPlatformModelName,
      }))
    : [];

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
  const sectionGaps = sections.length * MENTION_MENU_SECTION_GAP;
  return Math.min(
    MENTION_MENU_MAX_HEIGHT,
    itemCount * MENTION_MENU_ROW_HEIGHT
      + Math.max(0, itemCount - 1) * MENTION_MENU_ROW_GAP
      + sectionChrome
      + sectionGaps
      + MENTION_MENU_CHROME_HEIGHT,
  );
}

function resolveMentionMenuLayout(
  anchor: ChatMentionMenuAnchor,
  sections: ChatMentionMenuSection[],
  viewportWidth: number,
  viewportHeight: number,
  placementPreference: ChatMentionMenuPlacementPreference,
): ChatMentionMenuLayout {
  const preferredTop = anchor.top + anchor.height + MENTION_MENU_OFFSET;
  const preferredBottom = anchor.top - MENTION_MENU_OFFSET;
  const desiredHeight = resolveMentionMenuContentHeight(sections);
  const availableBelow = viewportHeight - preferredTop - MENTION_MENU_VIEWPORT_GUTTER;
  const availableAbove = preferredBottom - MENTION_MENU_VIEWPORT_GUTTER;
  const anchorInLowerHalf = anchor.top + anchor.height / 2 > viewportHeight / 2;
  const hasUsableAbove = availableAbove >= Math.min(desiredHeight, MENTION_MENU_MIN_HEIGHT);
  const openBelow =
    placementPreference === "bottom" ||
    (placementPreference === "top"
      ? !hasUsableAbove
      : !anchorInLowerHalf ||
        availableBelow >= Math.min(desiredHeight, MENTION_MENU_MIN_HEIGHT) ||
        availableBelow >= availableAbove);
  const availableHeight = Math.max(0, openBelow ? availableBelow : availableAbove);
  const maxHeight = Math.max(
    Math.min(MENTION_MENU_MIN_HEIGHT, availableHeight),
    Math.min(desiredHeight, availableHeight),
  );
  const preferredWidth = resolveMentionMenuWidth(anchor.width, viewportWidth);
  const preferredLeft = anchor.left;
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
  const [dismissedDraft, setDismissedDraft] = React.useState<string | null>(null);
  const [menuLayout, setMenuLayout] = React.useState<ChatMentionMenuLayout | null>(null);
  const [files, setFiles] = React.useState<FileObjectDTO[]>([]);
  const [filesLoading, setFilesLoading] = React.useState(false);
  const [filesQuery, setFilesQuery] = React.useState("");
  const [prompts, setPrompts] = React.useState<PromptPresetDTO[]>([]);
  const [promptsLoading, setPromptsLoading] = React.useState(false);
  const [skills, setSkills] = React.useState<SkillSummaryDTO[]>([]);
  const [skillsLoading, setSkillsLoading] = React.useState(false);
  const modelCatalogRefreshRequestedRef = React.useRef(false);
  const enabledKindSet = React.useMemo(() => new Set(enabledKinds), [enabledKinds]);
  const triggerQuery = resolveTriggerQuery(draft, textareaRef.current?.selectionStart ?? draft.length);
  const mentionQuery = triggerQuery?.kind === "mention" ? triggerQuery.query : null;
  const promptQuery = triggerQuery?.kind === "prompt" ? triggerQuery.query : null;
  const query = mentionQuery ?? promptQuery;
  const queryKind = mentionQuery !== null ? "mention" : promptQuery !== null ? "prompt" : null;

  React.useEffect(() => {
    if (!inputFocused || mentionQuery === null || !enabledKindSet.has("model")) {
      modelCatalogRefreshRequestedRef.current = false;
      return;
    }
    if (disabled || modelCatalogRefreshRequestedRef.current || !onModelCatalogRefresh) {
      return;
    }

    modelCatalogRefreshRequestedRef.current = true;
    void Promise.resolve(onModelCatalogRefresh()).catch(() => undefined);
  }, [disabled, enabledKindSet, inputFocused, mentionQuery, onModelCatalogRefresh]);

  React.useEffect(() => {
    if (mentionQuery === null || disabled || !enabledKindSet.has("file")) {
      setFiles([]);
      setFilesQuery("");
      setFilesLoading(false);
      return;
    }

    const sessionRevision = readSessionRevision();
    const cachedFiles = readMentionFileSearchCache(sessionRevision, mentionQuery);
    if (cachedFiles) {
      setFiles(cachedFiles);
      setFilesQuery(mentionQuery);
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
            query: mentionQuery,
            sessionRevision,
          });
          if (!controller.signal.aborted) {
            setFiles(results);
            setFilesQuery(mentionQuery);
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
  }, [disabled, enabledKindSet, mentionQuery]);

  React.useEffect(() => {
    if (promptQuery === null || disabled || !enabledKindSet.has("prompt")) {
      setPrompts([]);
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
          const data = await listVisiblePromptPresets(token, { query: promptQuery, page: 1, pageSize: 50 });
          if (!controller.signal.aborted) {
            setPrompts(data.results);
          }
        } catch {
          if (!controller.signal.aborted) {
            setPrompts([]);
          }
        } finally {
          if (!controller.signal.aborted) {
            setPromptsLoading(false);
          }
        }
      })();
    }, MENTION_MENU_PROMPT_QUERY_DELAY_MS);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [disabled, enabledKindSet, promptQuery]);

  React.useEffect(() => {
    if (promptQuery === null || disabled || !enabledKindSet.has("skill")) {
      setSkills([]);
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
          const data = await listVisibleSkills(token, { query: promptQuery, page: 1, pageSize: 50 });
          if (!controller.signal.aborted) {
            setSkills(data.results);
          }
        } catch {
          if (!controller.signal.aborted) {
            setSkills([]);
          }
        } finally {
          if (!controller.signal.aborted) {
            setSkillsLoading(false);
          }
        }
      })();
    }, MENTION_MENU_PROMPT_QUERY_DELAY_MS);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [disabled, enabledKindSet, promptQuery]);

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
        prompts,
        promptLoading: promptsLoading,
        skills,
        skillLoading: skillsLoading,
        query,
        queryKind,
        selectedPlatformModelName,
        selectedSkills,
        selectedToolIDs,
        toolsDisabled,
        enabledKinds: enabledKindSet,
      }),
    [
      attachments,
      availableTools,
      defaultFileLabel,
      files,
      filesQuery,
      filesLoading,
      modelOptions,
      prompts,
      promptsLoading,
      skills,
      skillsLoading,
      query,
      queryKind,
      selectedPlatformModelName,
      selectedSkills,
      selectedToolIDs,
      toolsDisabled,
      enabledKindSet,
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

    const menuAnchor =
      placementAnchor === "container"
        ? resolveContainerAnchor(anchor)
        : resolveTextareaCaretAnchor(textareaRef.current, anchor, triggerQuery?.range.start ?? draft.length);
    const nextLayout = resolveMentionMenuLayout(menuAnchor, sections, window.innerWidth, window.innerHeight, placementPreference);
    setMenuLayout((current) => (mentionMenuLayoutsEqual(current, nextLayout) ? current : nextLayout));
  }, [anchorRef, draft.length, open, placementAnchor, placementPreference, sections, textareaRef, triggerQuery?.range.start]);

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

  const finishSelection = React.useCallback(() => {
    if (!triggerQuery) {
      return;
    }
    const nextDraft = removeTriggerRange(draft, triggerQuery.range);
    onDraftChange(nextDraft.value);
    setDismissedDraft(null);
    focusTextarea(nextDraft.caretIndex);
  }, [draft, focusTextarea, onDraftChange, triggerQuery]);

  const select = React.useCallback(
    (item: ChatMentionMenuItem) => {
      if (item.kind === "model") {
        onModelChange(item.model.platformModelName);
        finishSelection();
        return;
      }

      if (item.kind === "prompt") {
        if (!triggerQuery) {
          return;
        }
        const nextDraft = replaceTriggerRange(draft, triggerQuery.range, item.prompt.content);
        onDraftChange(nextDraft.value);
        setDismissedDraft(null);
        focusTextarea(nextDraft.caretIndex);
        return;
      }

      if (item.kind === "skill") {
        const alreadySelected = selectedSkills.some((skill) => skill.id === item.skill.id);
        if (!alreadySelected && selectedSkills.length >= maxSelectedSkills) {
          onSkillLimitReached?.();
          return;
        }
        onSelectedSkillsChange?.(
          alreadySelected
            ? selectedSkills.filter((skill) => skill.id !== item.skill.id)
            : [...selectedSkills, item.skill],
        );
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
      maxSelectedSkills,
      maxSelectedTools,
      onFileSelect,
      onDraftChange,
      onModelChange,
      onSelectedSkillsChange,
      onSkillLimitReached,
      onSelectedToolsChange,
      onToolLimitReached,
      selectedSkills,
      selectedToolIDs,
      draft,
      focusTextarea,
      triggerQuery,
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
