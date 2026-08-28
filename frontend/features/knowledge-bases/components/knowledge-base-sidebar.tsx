"use client";

import { BookOpen, Ellipsis, PencilLine, SquareCheckBig, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { KnowledgeBaseSidebarHeader } from "@/features/knowledge-bases/components/knowledge-base-sidebar-header";
import type {
  KnowledgeBaseMode,
  KnowledgeBaseMobileView,
  KnowledgeBaseSortKey,
} from "@/features/knowledge-bases/types/knowledge-bases";
import { cn } from "@/lib/utils";
import type { KnowledgeBaseDTO } from "@/shared/api/knowledge-bases.types";

type KnowledgeBaseSidebarProps = {
  mode: KnowledgeBaseMode;
  items: KnowledgeBaseDTO[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  mobileView: KnowledgeBaseMobileView;
  collapsed: boolean;
  showCollapseButton: boolean;
  selectedID: string;
  selectedIDs: string[];
  sortKey: KnowledgeBaseSortKey;
  query: string;
  searchOpen: boolean;
  bulkDeleting: boolean;
  onToggleCollapsed: () => void;
  onLoadMore: () => void;
  onToggleSearch: () => void;
  onQueryChange: (value: string) => void;
  onCreate: () => void;
  onSelect: (publicID: string) => void;
  onToggleSelection: (publicID: string, checked: boolean) => void;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onSortChange: (value: KnowledgeBaseSortKey) => void;
  onBulkDelete: () => void;
  onEdit: (item: KnowledgeBaseDTO) => void;
  onDelete: (item: KnowledgeBaseDTO) => void;
};

export function KnowledgeBaseSidebar({
  mode,
  items,
  loading,
  loadingMore,
  hasMore,
  mobileView,
  collapsed,
  showCollapseButton,
  selectedID,
  selectedIDs,
  sortKey,
  query,
  searchOpen,
  bulkDeleting,
  onToggleCollapsed,
  onLoadMore,
  onToggleSearch,
  onQueryChange,
  onCreate,
  onSelect,
  onToggleSelection,
  onSelectAll,
  onClearSelection,
  onSortChange,
  onBulkDelete,
  onEdit,
  onDelete,
}: KnowledgeBaseSidebarProps) {
  const t = useTranslations("knowledgeBases");
  const selectedIDSet = React.useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const selectableCount = React.useMemo(
    () => items.filter((item) => mode === "admin" || item.scope === "user").length,
    [items, mode],
  );

  return (
    <aside
      className={cn(
        "h-full min-h-0 min-w-0 shrink-0 overflow-hidden border-border/45 bg-transparent transition-[width,max-width,flex-basis] duration-200",
        "w-full border-r-0 md:border-r",
        collapsed
          ? "md:w-12 md:basis-12 md:max-w-12"
          : "md:w-64 md:basis-64 md:max-w-64 lg:w-72 lg:basis-72 lg:max-w-72",
        mobileView === "list" ? "flex" : "hidden md:flex",
      )}
    >
      <div className={cn(
        "flex min-h-0 min-w-0 flex-1 flex-col px-3 md:px-2",
        collapsed && "md:px-0",
      )}>
        <KnowledgeBaseSidebarHeader
          mode={mode}
          collapsed={collapsed}
          showCollapseButton={showCollapseButton}
          loading={loading}
          query={query}
          searchOpen={searchOpen}
          selectableCount={selectableCount}
          selectedCount={selectedIDs.length}
          sortKey={sortKey}
          bulkDeleting={bulkDeleting}
          onToggleCollapsed={onToggleCollapsed}
          onToggleSearch={onToggleSearch}
          onQueryChange={onQueryChange}
          onCreate={onCreate}
          onSelectAll={onSelectAll}
          onClearSelection={onClearSelection}
          onSortChange={onSortChange}
          onBulkDelete={onBulkDelete}
        />

        {!collapsed && loading && items.length === 0 ? (
          <div className="flex min-h-0 flex-1 items-center justify-center pr-2 text-muted-foreground">
            <Spinner className="size-4" />
          </div>
        ) : !collapsed && items.length > 0 ? (
          <div className="min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden pr-2">
            <div className="w-full max-w-full min-w-0 space-y-1 px-1.5 py-2.5 pb-4">
              {items.map((item) => (
                <KnowledgeBaseSidebarItem
                  key={item.publicID}
                  item={item}
                  mode={mode}
                  selected={selectedID === item.publicID}
                  checked={selectedIDSet.has(item.publicID)}
                  onSelect={onSelect}
                  onToggleSelection={onToggleSelection}
                  onEdit={onEdit}
                  onDelete={onDelete}
                />
              ))}
              {hasMore ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 w-full text-[11px] text-muted-foreground"
                  disabled={loadingMore}
                  onClick={onLoadMore}
                >
                  {loadingMore ? <Spinner className="size-3" /> : t("loadMore")}
                </Button>
              ) : null}
            </div>
          </div>
        ) : !collapsed ? (
          <CenteredEmptyState
            className="min-w-0 flex-1"
            title={query.trim() ? t("searchEmpty") : t("empty")}
            description={query.trim() ? t("searchEmptyDescription") : t("emptyDescription")}
          />
        ) : null}
      </div>
    </aside>
  );
}

type KnowledgeBaseSidebarItemProps = {
  item: KnowledgeBaseDTO;
  mode: KnowledgeBaseMode;
  selected: boolean;
  checked: boolean;
  onSelect: (publicID: string) => void;
  onToggleSelection: (publicID: string, checked: boolean) => void;
  onEdit: (item: KnowledgeBaseDTO) => void;
  onDelete: (item: KnowledgeBaseDTO) => void;
};

function KnowledgeBaseSidebarItem({
  item,
  mode,
  selected,
  checked,
  onSelect,
  onToggleSelection,
  onEdit,
  onDelete,
}: KnowledgeBaseSidebarItemProps) {
  const t = useTranslations("knowledgeBases");
  const editable = mode === "admin" || item.scope === "user";

  return (
    <div className="group relative h-11 w-full min-w-0 overflow-hidden rounded-md">
      <Checkbox
        checked={editable && checked}
        disabled={!editable}
        className="absolute left-1.5 top-1/2 z-20 size-3 -translate-y-1/2"
        aria-label={t("selectKnowledgeBase")}
        onClick={(event) => event.stopPropagation()}
        onCheckedChange={(nextChecked) => {
          if (editable) onToggleSelection(item.publicID, nextChecked === true);
        }}
      />
      <button
        type="button"
        className={cn(
          "flex h-11 w-full min-w-0 items-center gap-2 overflow-hidden rounded-md text-left transition-colors",
          editable ? "pl-7 pr-12" : "pl-7 pr-1.5",
          selected
            ? "bg-accent text-accent-foreground hover:bg-accent"
            : "text-foreground hover:bg-accent/65",
        )}
        onClick={() => onSelect(item.publicID)}
      >
        <span className="flex size-4 shrink-0 items-center justify-center">
          <BookOpen className="size-3.5 text-muted-foreground" strokeWidth={1.5} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-1.5">
            <span className="truncate text-xs font-medium">{item.name}</span>
            {item.scope === "builtin" ? (
              <Badge variant="secondary" className="h-4 rounded-md px-1 text-[9px] font-normal">
                {t("builtin")}
              </Badge>
            ) : null}
            {mode === "admin" && !item.enabled ? (
              <Badge
                variant="outline"
                className="h-4 rounded-md px-1 text-[9px] font-normal text-muted-foreground"
              >
                {t("disabled")}
              </Badge>
            ) : null}
          </span>
          <span className="mt-0.5 block truncate text-[10px] leading-3.5 text-muted-foreground">
            {item.fileCount === 0
              ? t("healthEmpty")
              : item.readyFileCount === item.fileCount
                ? t("healthReady", { count: item.readyFileCount })
                : item.readyFileCount > 0
                  ? t("healthPartial", { ready: item.readyFileCount, total: item.fileCount })
                  : t("healthPending", { count: item.fileCount })}
          </span>
        </span>
      </button>

      {editable ? (
        <div
          className={cn(
            "absolute inset-y-0 right-1 z-20 flex items-center gap-0.5 transition-opacity duration-150",
            selected
              ? "pointer-events-auto opacity-100"
              : "pointer-events-none opacity-0 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100",
          )}
        >
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-5 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
                aria-label={t("moreActions")}
                title={t("moreActions")}
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                }}
              >
                <Ellipsis className="size-3" strokeWidth={1} />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-32">
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault();
                  onToggleSelection(item.publicID, !checked);
                }}
              >
                <DropdownMenuItemIcon icon={SquareCheckBig} />
                {checked ? t("cancelSelect") : t("select")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault();
                  onEdit(item);
                }}
              >
                <DropdownMenuItemIcon icon={PencilLine} />
                {t("editTitle")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-5 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label={t("delete")}
            title={t("delete")}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onDelete(item);
            }}
          >
            <Trash2 className="size-3" strokeWidth={1} />
          </Button>
        </div>
      ) : null}
    </div>
  );
}
