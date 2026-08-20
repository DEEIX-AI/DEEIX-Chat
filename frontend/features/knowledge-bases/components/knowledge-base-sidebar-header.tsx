"use client";

import {
  ArrowDownUp,
  Check,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Search,
  SquareDashed,
  SquareDashedMousePointer,
  Trash2,
} from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { KNOWLEDGE_BASE_SORT_OPTIONS } from "@/features/knowledge-bases/model/knowledge-base-options";
import type {
  KnowledgeBaseMode,
  KnowledgeBaseSortKey,
} from "@/features/knowledge-bases/types/knowledge-bases";
import { cn } from "@/lib/utils";

type KnowledgeBaseSidebarHeaderProps = {
  mode: KnowledgeBaseMode;
  collapsed: boolean;
  showCollapseButton: boolean;
  loading: boolean;
  query: string;
  searchOpen: boolean;
  selectableCount: number;
  selectedCount: number;
  sortKey: KnowledgeBaseSortKey;
  bulkDeleting: boolean;
  onToggleCollapsed: () => void;
  onToggleSearch: () => void;
  onQueryChange: (value: string) => void;
  onCreate: () => void;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onSortChange: (value: KnowledgeBaseSortKey) => void;
  onBulkDelete: () => void;
};

export function KnowledgeBaseSidebarHeader({
  mode,
  collapsed,
  showCollapseButton,
  loading,
  query,
  searchOpen,
  selectableCount,
  selectedCount,
  sortKey,
  bulkDeleting,
  onToggleCollapsed,
  onToggleSearch,
  onQueryChange,
  onCreate,
  onSelectAll,
  onClearSelection,
  onSortChange,
  onBulkDelete,
}: KnowledgeBaseSidebarHeaderProps) {
  const t = useTranslations("knowledgeBases");
  const tCommon = useTranslations("common.actions");

  if (collapsed) {
    return (
      <div className="flex flex-col items-center px-0 py-2">
        <div className="flex h-8 items-center justify-center">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={onToggleCollapsed}
            aria-label={t("expandSidebar")}
            title={t("expandSidebar")}
          >
            <PanelLeftOpen className="size-4 stroke-1" />
          </Button>
        </div>
        <div className="flex h-8 items-center justify-center">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={onToggleSearch}
            aria-label={t("search")}
            title={t("search")}
          >
            <Search className="size-4 stroke-1" />
          </Button>
        </div>
        <div className="flex h-8 items-center justify-center">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={onCreate}
            aria-label={t("create")}
            title={t("create")}
          >
            <Plus className="size-4 stroke-1" />
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-w-0 overflow-hidden pt-2">
      <div className="flex h-8 min-w-0 items-center gap-2 px-2">
        <h1 className="min-w-0 flex-1 truncate text-[15px] font-medium text-foreground">
          {t(mode === "admin" ? "adminTitle" : "title")}
        </h1>
        <div className="flex shrink-0 items-center gap-1">
          {showCollapseButton ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-6"
              onClick={onToggleCollapsed}
              aria-label={t("collapseSidebar")}
              title={t("collapseSidebar")}
            >
              <PanelLeftClose className="size-4 stroke-1" />
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={onToggleSearch}
            aria-label={t("search")}
            title={t("search")}
          >
            <Search className="size-4 stroke-1" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={onCreate}
            aria-label={t("create")}
            title={t("create")}
          >
            <Plus className="size-4 stroke-1" />
          </Button>
        </div>
      </div>

      {searchOpen ? (
        <div className="px-1 pt-2">
          <Input
            autoFocus
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder={t("searchPlaceholder")}
            className="bg-background px-2 focus-visible:ring-0"
          />
        </div>
      ) : null}

      <div className="flex min-w-0 items-center gap-0.5 overflow-hidden px-0 py-1.5">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 gap-0.5 px-1 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          disabled={loading || bulkDeleting || selectableCount === 0}
          onClick={selectedCount > 0 ? onClearSelection : onSelectAll}
        >
          {selectedCount > 0 ? (
            <SquareDashed className="size-3 stroke-1" />
          ) : (
            <SquareDashedMousePointer className="size-3 stroke-1" />
          )}
          {selectedCount > 0 ? tCommon("cancel") : t("selectAll")}
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 gap-0.5 px-1 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground"
            >
              <ArrowDownUp className="size-3 stroke-1" />
              {t("sort.action")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-36 p-1.5">
            <div className="space-y-1">
              {KNOWLEDGE_BASE_SORT_OPTIONS.map((option) => {
                const active = option === sortKey;
                return (
                  <DropdownMenuItem
                    key={option}
                    className={cn(
                      "h-6 gap-2 px-2 py-0 text-[10px]",
                      active
                        ? "bg-muted/55 text-foreground"
                        : "text-foreground/70 hover:bg-muted hover:text-foreground",
                    )}
                    onSelect={() => onSortChange(option)}
                  >
                    <ArrowDownUp className="size-3 stroke-1 text-muted-foreground" />
                    <span className="flex-1 truncate">{t(`sort.${option}`)}</span>
                    {active ? <Check className="size-3 stroke-1 text-muted-foreground" /> : null}
                  </DropdownMenuItem>
                );
              })}
            </div>
          </DropdownMenuContent>
        </DropdownMenu>

        {selectedCount > 0 ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 shrink-0 gap-0.5 px-1 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
            disabled={bulkDeleting}
            onClick={onBulkDelete}
          >
            <Trash2 className="size-3 stroke-1" />
            {tCommon("delete")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
