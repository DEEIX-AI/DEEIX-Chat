"use client";

import * as React from "react";
import { ChevronDown, Info, Star } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Unplug } from "@/components/animate-ui/icons/unplug";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { MCPToolDTO } from "@/shared/api/mcp.types";

const DEFAULT_MCP_TOOL_SELECTION_LIMIT = 32;
const MAX_MCP_TOOL_SELECTION_LIMIT = 128;

type MCPToolGroup = {
  key: string;
  serverName: string;
  tools: MCPToolDTO[];
};

type FilteredMCPToolGroup = MCPToolGroup & {
  visibleTools: MCPToolDTO[];
};

type ChatMCPProps = {
  availableTools: MCPToolDTO[];
  selectedToolIDs: number[];
  defaultToolIDs: number[];
  maxSelectedTools: number;
  disabled: boolean;
  onSelectedToolsChange: (toolIDs: number[]) => void;
  onDefaultToolsChange: (toolIDs: number[]) => void | Promise<void>;
};

type MCPToolRowActionProps = React.ComponentPropsWithoutRef<"button"> & {
  label: string;
};

function MCPToolRowAction({
  label,
  className,
  children,
  ...props
}: MCPToolRowActionProps) {
  return (
    <button
      {...props}
      type="button"
      aria-label={label}
      title={label}
      className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-md text-foreground/45 outline-none transition-[background-color,color] duration-150 hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground",
        className,
      )}
    >
      {children}
    </button>
  );
}

function resolveMCPToolLabel(tool: MCPToolDTO, fallback: string): string {
  const displayName = typeof tool.displayName === "string" ? tool.displayName.trim() : "";
  const name = typeof tool.name === "string" ? tool.name.trim() : "";
  return displayName || name || fallback;
}

function resolveMCPToolServerName(tool: MCPToolDTO): string {
  return typeof tool.serverName === "string" ? tool.serverName.trim() : "";
}

function resolveMCPToolServerKey(tool: MCPToolDTO): string {
  if (Number.isFinite(tool.serverID) && tool.serverID > 0) {
    return `server:${tool.serverID}`;
  }
  const serverName = resolveMCPToolServerName(tool);
  return serverName ? `server-name:${serverName}` : "server:unknown";
}

function buildMCPToolGroups(tools: MCPToolDTO[], fallbackServerName: string): MCPToolGroup[] {
  const groups = new Map<string, MCPToolGroup>();
  for (const tool of tools) {
    const key = resolveMCPToolServerKey(tool);
    const serverName = resolveMCPToolServerName(tool) || fallbackServerName;
    const existing = groups.get(key);
    if (existing) {
      existing.tools.push(tool);
      continue;
    }
    groups.set(key, { key, serverName, tools: [tool] });
  }
  return [...groups.values()];
}

function matchesMCPToolSearch(tool: MCPToolDTO, query: string): boolean {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) {
    return true;
  }
  return [
    resolveMCPToolLabel(tool, String(tool.id)),
    resolveMCPToolServerName(tool),
    tool.name,
    tool.description,
  ]
    .join(" ")
    .toLocaleLowerCase()
    .includes(normalizedQuery);
}

function matchesMCPServerSearch(group: MCPToolGroup, query: string): boolean {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) {
    return true;
  }
  return group.serverName.toLocaleLowerCase().includes(normalizedQuery);
}

function filterMCPToolGroups(groups: MCPToolGroup[], query: string): FilteredMCPToolGroup[] {
  const normalizedQuery = query.trim();
  if (!normalizedQuery) {
    return groups.map((group) => ({ ...group, visibleTools: group.tools }));
  }
  return groups.flatMap((group) => {
    if (matchesMCPServerSearch(group, normalizedQuery)) {
      return [{ ...group, visibleTools: group.tools }];
    }
    const visibleTools = group.tools.filter((tool) => matchesMCPToolSearch(tool, normalizedQuery));
    return visibleTools.length > 0 ? [{ ...group, visibleTools }] : [];
  });
}

function resolveToolSelectionLimit(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return DEFAULT_MCP_TOOL_SELECTION_LIMIT;
  }
  return Math.min(Math.floor(value), MAX_MCP_TOOL_SELECTION_LIMIT);
}

export function ChatMCP({
  availableTools,
  selectedToolIDs,
  defaultToolIDs,
  maxSelectedTools,
  disabled,
  onSelectedToolsChange,
  onDefaultToolsChange,
}: ChatMCPProps) {
  const tComposer = useTranslations("chat.composer");
  const [hovered, setHovered] = React.useState(false);
  const [hoveredRowKey, setHoveredRowKey] = React.useState<string | null>(null);
  const [focusedRowKey, setFocusedRowKey] = React.useState<string | null>(null);
  const [open, setOpen] = React.useState(false);
  const [search, setSearch] = React.useState("");
  const [expandedServerKeys, setExpandedServerKeys] = React.useState<Set<string>>(() => new Set());
  const selectedToolIDSet = React.useMemo(() => new Set(selectedToolIDs), [selectedToolIDs]);
  const defaultToolIDSet = React.useMemo(() => new Set(defaultToolIDs), [defaultToolIDs]);
  const selectedToolCount = selectedToolIDs.length;
  const selectionLimit = resolveToolSelectionLimit(maxSelectedTools);
  const toolGroups = React.useMemo(
    () => buildMCPToolGroups(availableTools, tComposer("mcpUnknownServer")),
    [availableTools, tComposer],
  );
  const filteredToolGroups = React.useMemo(
    () => filterMCPToolGroups(toolGroups, search),
    [toolGroups, search],
  );
  const hasSearch = search.trim().length > 0;

  const showToolLimitToast = React.useCallback(() => {
    toast.error(tComposer("mcpToolLimitTitle"), {
      description: tComposer("mcpToolLimitDescription", { limit: selectionLimit }),
    });
  }, [selectionLimit, tComposer]);

  const toggleTool = React.useCallback(
    (toolID: number, checked: boolean) => {
      if (checked) {
        if (selectedToolIDSet.has(toolID)) {
          return;
        }
        if (selectedToolIDs.length >= selectionLimit) {
          showToolLimitToast();
          return;
        }
        onSelectedToolsChange([...selectedToolIDs, toolID]);
        return;
      }
      onSelectedToolsChange(selectedToolIDs.filter((item) => item !== toolID));
    },
    [onSelectedToolsChange, selectedToolIDs, selectedToolIDSet, selectionLimit, showToolLimitToast],
  );

  const toggleToolGroup = React.useCallback(
    (tools: MCPToolDTO[], checked: boolean) => {
      const toolIDs = tools.map((tool) => tool.id);
      if (!checked) {
        const removeSet = new Set(toolIDs);
        onSelectedToolsChange(selectedToolIDs.filter((id) => !removeSet.has(id)));
        return;
      }
      const selectedSet = new Set(selectedToolIDs);
      const missingIDs = toolIDs.filter((id) => !selectedSet.has(id));
      if (selectedSet.size + missingIDs.length > selectionLimit) {
        showToolLimitToast();
        return;
      }
      onSelectedToolsChange([...selectedToolIDs, ...missingIDs]);
    },
    [onSelectedToolsChange, selectedToolIDs, selectionLimit, showToolLimitToast],
  );

  const toggleDefaultTool = React.useCallback(
    (toolID: number) => {
      if (defaultToolIDSet.has(toolID)) {
        void onDefaultToolsChange(defaultToolIDs.filter((id) => id !== toolID));
        return;
      }
      if (defaultToolIDs.length >= selectionLimit) {
        showToolLimitToast();
        return;
      }
      void onDefaultToolsChange([...defaultToolIDs, toolID]);
    },
    [defaultToolIDs, defaultToolIDSet, onDefaultToolsChange, selectionLimit, showToolLimitToast],
  );

  const toggleDefaultToolGroup = React.useCallback(
    (tools: MCPToolDTO[]) => {
      const toolIDs = tools.map((tool) => tool.id);
      if (toolIDs.length === 0) {
        return;
      }
      const allDefault = toolIDs.every((toolID) => defaultToolIDSet.has(toolID));
      if (allDefault) {
        const removeSet = new Set(toolIDs);
        void onDefaultToolsChange(defaultToolIDs.filter((id) => !removeSet.has(id)));
        return;
      }
      const missingIDs = toolIDs.filter((toolID) => !defaultToolIDSet.has(toolID));
      if (defaultToolIDs.length + missingIDs.length > selectionLimit) {
        showToolLimitToast();
        return;
      }
      void onDefaultToolsChange([...defaultToolIDs, ...missingIDs]);
    },
    [defaultToolIDs, defaultToolIDSet, onDefaultToolsChange, selectionLimit, showToolLimitToast],
  );

  const toggleServerExpanded = React.useCallback((serverKey: string) => {
    setExpandedServerKeys((current) => {
      const next = new Set(current);
      if (next.has(serverKey)) {
        next.delete(serverKey);
      } else {
        next.add(serverKey);
      }
      return next;
    });
  }, []);

  const toolSelectionState = React.useCallback(
    (tools: MCPToolDTO[]) => {
      const selectedCount = tools.filter((tool) => selectedToolIDSet.has(tool.id)).length;
      return {
        selectedCount,
        allSelected: tools.length > 0 && selectedCount === tools.length,
        partiallySelected: selectedCount > 0 && selectedCount < tools.length,
      };
    },
    [selectedToolIDSet],
  );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <InputGroupButton
          type="button"
          variant="ghost"
          size="icon-sm"
          className="relative size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8"
          disabled={disabled}
          aria-label={tComposer("mcpTools")}
          title={selectedToolCount > 0 ? tComposer("mcpToolsSelected", { count: selectedToolCount }) : tComposer("mcpTools")}
          onMouseEnter={() => setHovered(true)}
          onMouseLeave={() => setHovered(false)}
        >
          <Unplug
            size={20}
            strokeWidth={1.4}
            animate={hovered ? "default" : undefined}
          />
          {selectedToolCount > 0 ? (
            <span className="absolute -right-0.5 -top-0.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-1 text-[9px] font-medium leading-none text-primary-foreground">
              {selectedToolCount}
            </span>
          ) : null}
        </InputGroupButton>
      </PopoverTrigger>
      <PopoverContent
        side="bottom"
        align="start"
        sideOffset={8}
        data-mcp-tools-popover-content
        className="w-[22rem] p-1.5"
        onPointerDownOutside={(event) => {
          const target = event.target as HTMLElement | null;
          if (target?.closest("[data-mcp-tools-popover-content]")) {
            event.preventDefault();
          }
        }}
        onFocusOutside={(event) => {
          const target = event.target as HTMLElement | null;
          if (target?.closest("[data-mcp-tools-popover-content]")) {
            event.preventDefault();
          }
        }}
      >
        <div className="flex items-center justify-between gap-3 px-2 pb-1.5 text-[11px] font-medium text-foreground/70">
          <span>{tComposer("mcpTools")}</span>
          {selectedToolCount > 0 ? (
            <button
              type="button"
              className="text-[11px] leading-none text-foreground/55 outline-none transition-colors hover:text-foreground focus-visible:text-foreground"
              onClick={() => onSelectedToolsChange([])}
            >
              {tComposer("clear")}
            </button>
          ) : null}
        </div>
        <div
          className="px-1 py-1"
          onPointerDown={(event) => event.stopPropagation()}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={(event) => event.stopPropagation()}
        >
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            onKeyDown={(event) => event.stopPropagation()}
            className="border-border/60 bg-transparent dark:bg-transparent"
            placeholder={tComposer("searchToolsPlaceholder")}
          />
        </div>
        <div className="max-h-72 overflow-y-auto px-0.5 pt-1">
          {filteredToolGroups.map((group) => {
            const groupState = toolSelectionState(group.tools);
            const expanded = hasSearch || expandedServerKeys.has(group.key);
            const overLimit = group.tools.length > selectionLimit;
            const groupRowKey = `server:${group.key}`;
            const groupInteractive = hoveredRowKey === groupRowKey || focusedRowKey === groupRowKey;
            const defaultCount = group.tools.filter((tool) => defaultToolIDSet.has(tool.id)).length;
            const allDefault = group.tools.length > 0 && defaultCount === group.tools.length;
            const hasDefault = defaultCount > 0;
            return (
              <div key={group.key} className="mb-1">
                <div
                  data-interactive={groupInteractive}
                  data-selected={groupState.selectedCount > 0}
                  className="group/server flex h-8 items-center gap-2 rounded-md px-2 text-[11px] font-medium text-foreground/65 transition-colors data-[interactive=true]:bg-accent data-[interactive=true]:text-accent-foreground"
                >
                  <Checkbox
                    checked={groupState.allSelected ? true : groupState.partiallySelected ? "indeterminate" : false}
                    className="shrink-0"
                    aria-label={tComposer("mcpToggleServerTools", { server: group.serverName })}
                    onCheckedChange={(nextChecked) => toggleToolGroup(group.tools, nextChecked === true)}
                  />
                  <button
                    type="button"
                    className="flex h-full min-w-0 flex-1 items-center gap-2 rounded-md text-left outline-none"
                    onClick={() => toggleServerExpanded(group.key)}
                    onMouseEnter={() => setHoveredRowKey(groupRowKey)}
                    onMouseLeave={() => setHoveredRowKey((current) => (current === groupRowKey ? null : current))}
                    onFocus={() => setFocusedRowKey(groupRowKey)}
                    onBlur={() => setFocusedRowKey((current) => (current === groupRowKey ? null : current))}
                  >
                    <span className="flex min-w-0 flex-1 items-center gap-1.5">
                      <span className="min-w-0 truncate text-xs font-semibold text-current">{group.serverName}</span>
                      <span className="shrink-0 text-[10px] leading-none text-foreground/45 transition-colors group-data-[interactive=true]/server:text-accent-foreground/75">
                        |
                      </span>
                      <span className="shrink-0 text-[10px] leading-none text-foreground/45 transition-colors group-data-[interactive=true]/server:text-accent-foreground/75">
                        {tComposer("mcpServerToolCount", { selected: groupState.selectedCount, total: group.tools.length })}
                      </span>
                      {overLimit ? (
                        <span className="min-w-0 truncate text-[10px] leading-none text-amber-600 dark:text-amber-400">
                          {tComposer("mcpServerLimitHint", { limit: selectionLimit })}
                        </span>
                      ) : null}
                    </span>
                  </button>
                  <Tooltip disableHoverableContent>
                    <TooltipTrigger asChild>
                      <MCPToolRowAction
                        label={allDefault
                          ? tComposer("mcpUnsetDefaultServerTools", { server: group.serverName })
                          : tComposer("mcpSetDefaultServerTools", { server: group.serverName })}
                        className={cn("-mr-2", hasDefault && "text-amber-500 hover:text-amber-500 focus-visible:text-amber-500")}
                        onClick={(event) => {
                          event.stopPropagation();
                          toggleDefaultToolGroup(group.tools);
                        }}
                      >
                        <Star
                          className="size-3.5"
                          strokeWidth={1.8}
                          fill={allDefault ? "currentColor" : "none"}
                        />
                      </MCPToolRowAction>
                    </TooltipTrigger>
                    <TooltipContent
                      side="right"
                      align="center"
                      sideOffset={6}
                      className="text-xs data-[state=closed]:[animation-duration:60ms] data-[state=open]:[animation-duration:90ms]"
                    >
                      {allDefault
                        ? tComposer("mcpDefaultServerToolsEnabled")
                        : tComposer("mcpDefaultServerToolsDisabled")}
                    </TooltipContent>
                  </Tooltip>
                  <button
                    type="button"
                    className="-mr-2 flex size-8 shrink-0 items-center justify-center rounded-md text-foreground/45 outline-none transition-[background-color,color] duration-150 hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground"
                    aria-label={expanded ? tComposer("mcpCollapseServerTools", { server: group.serverName }) : tComposer("mcpExpandServerTools", { server: group.serverName })}
                    onClick={() => toggleServerExpanded(group.key)}
                  >
                    <ChevronDown
                      className={cn("size-3.5 shrink-0 transition-transform duration-200", expanded && "rotate-180")}
                      strokeWidth={1.7}
                    />
                  </button>
                </div>
                <AnimatePresence initial={false}>
                  {expanded ? (
                    <motion.div
                      key={`${group.key}-tools`}
                      initial={{ height: 0, opacity: 0, y: -4 }}
                      animate={{ height: "auto", opacity: 1, y: 0 }}
                      exit={{ height: 0, opacity: 0, y: -4 }}
                      transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
                      className="overflow-hidden"
                    >
                      <div className="mt-1 space-y-1 border-l border-border/60 ml-2 pl-3">
                        {group.visibleTools.map((tool) => {
                          const checked = selectedToolIDSet.has(tool.id);
                          const isDefault = defaultToolIDSet.has(tool.id);
                          const label = resolveMCPToolLabel(tool, tComposer("tool", { id: tool.id }));
                          const description = (typeof tool.description === "string" ? tool.description.trim() : "") || tComposer("noToolDescription");
                          const toolRowKey = `tool:${tool.id}`;
                          const toolInteractive = hoveredRowKey === toolRowKey || focusedRowKey === toolRowKey;
                          return (
                            <div
                              key={tool.id}
                              data-interactive={toolInteractive}
                              data-selected={checked}
                              className="group/tool flex h-8 items-center gap-2 rounded-md px-2 text-[11px] font-medium text-foreground/65 transition-colors data-[interactive=true]:bg-accent data-[interactive=true]:text-accent-foreground"
                            >
                              <Checkbox
                                checked={checked}
                                className="shrink-0"
                                aria-label={tComposer("mcpToggleTool", { tool: label })}
                                onCheckedChange={(nextChecked) => toggleTool(tool.id, nextChecked === true)}
                              />
                              <button
                                type="button"
                                className="flex h-full min-w-0 flex-1 items-center gap-1.5 rounded-md text-left outline-none"
                                onClick={() => toggleTool(tool.id, !checked)}
                                onMouseEnter={() => setHoveredRowKey(toolRowKey)}
                                onMouseLeave={() => setHoveredRowKey((current) => (current === toolRowKey ? null : current))}
                                onFocus={() => setFocusedRowKey(toolRowKey)}
                                onBlur={() => setFocusedRowKey((current) => (current === toolRowKey ? null : current))}
                              >
                                <span className="min-w-0 truncate text-xs text-current">{label}</span>
                              </button>
                              <div className="-mr-2 flex shrink-0 items-center gap-0">
                                <Tooltip disableHoverableContent>
                                  <TooltipTrigger asChild>
                                    <MCPToolRowAction
                                      label={isDefault
                                        ? tComposer("mcpUnsetDefaultTool", { tool: label })
                                        : tComposer("mcpSetDefaultTool", { tool: label })}
                                      className={cn(isDefault && "text-amber-500 hover:text-amber-500 focus-visible:text-amber-500")}
                                      onClick={(event) => {
                                        event.stopPropagation();
                                        toggleDefaultTool(tool.id);
                                      }}
                                    >
                                      <Star
                                        className="size-3.5"
                                        strokeWidth={1.8}
                                        fill={isDefault ? "currentColor" : "none"}
                                      />
                                    </MCPToolRowAction>
                                  </TooltipTrigger>
                                  <TooltipContent
                                    side="right"
                                    align="center"
                                    sideOffset={6}
                                    className="text-xs data-[state=closed]:[animation-duration:60ms] data-[state=open]:[animation-duration:90ms]"
                                  >
                                    {isDefault ? tComposer("mcpDefaultToolEnabled") : tComposer("mcpDefaultToolDisabled")}
                                  </TooltipContent>
                                </Tooltip>
                                <Tooltip disableHoverableContent>
                                  <TooltipTrigger asChild>
                                    <MCPToolRowAction label={tComposer("viewToolDescription")}>
                                      <Info className="size-3.5" strokeWidth={1.8} />
                                    </MCPToolRowAction>
                                  </TooltipTrigger>
                                  <TooltipContent
                                    side="right"
                                    align="center"
                                    sideOffset={6}
                                    className="max-w-72 whitespace-normal text-left text-xs leading-5 [text-wrap:auto] data-[state=closed]:[animation-duration:60ms] data-[state=open]:[animation-duration:90ms]"
                                  >
                                    {description}
                                  </TooltipContent>
                                </Tooltip>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </motion.div>
                  ) : null}
                </AnimatePresence>
              </div>
            );
          })}
          {filteredToolGroups.length === 0 ? (
            <div className="px-2 py-6 text-center text-xs text-muted-foreground">
              {tComposer("noMatchingTools")}
            </div>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
