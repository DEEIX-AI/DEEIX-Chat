"use client";

import * as React from "react";
import { ArrowDownUp, Check, Funnel, PanelLeftClose, PanelLeftOpen, Plus, Search, SquareDashed, SquareDashedMousePointer, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";

import type { FileFilterValue, FileSortKey } from "@/features/files/types/files";
import { FILE_FILTER_OPTIONS, FILE_SORT_OPTIONS } from "@/features/files/utils/file-display";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { UserStorageQuotaDTO } from "@/shared/api/file.types";

type SidebarHeaderProps = {
  collapsed: boolean;
  showCollapseButton?: boolean;
  total: number;
  quota: UserStorageQuotaDTO | null;
  query: string;
  searchOpen: boolean;
  filterKeys: FileFilterValue[];
  sortKey: FileSortKey;
  uploading: boolean;
  selectedCount: number;
  selectAllDisabled: boolean;
  bulkDeleteDisabled: boolean;
  onToggleCollapsed: () => void;
  onToggleSearch: () => void;
  onQueryChange: (value: string) => void;
  onFilterToggle: (value: FileFilterValue | "all") => void;
  onSortChange: (value: FileSortKey) => void;
  onSelectLoaded: () => void;
  onClearSelection: () => void;
  onBulkDeleteRequest: () => void;
  onUpload: () => void;
};

export function SidebarHeader({
  collapsed,
  total,
  quota,
  query,
  searchOpen,
  filterKeys,
  sortKey,
  uploading,
  selectedCount,
  selectAllDisabled,
  bulkDeleteDisabled,
  showCollapseButton = true,
  onToggleCollapsed,
  onToggleSearch,
  onQueryChange,
  onFilterToggle,
  onSortChange,
  onSelectLoaded,
  onClearSelection,
  onBulkDeleteRequest,
  onUpload,
}: SidebarHeaderProps) {
  const tCommon = useTranslations("common.actions");
  const t = useTranslations("files");
  const activeFilterSet = React.useMemo(() => new Set(filterKeys), [filterKeys]);
  const hasActiveFilters = filterKeys.length > 0;
  const allFilterOption = FILE_FILTER_OPTIONS[0];
  const AllFilterIcon = allFilterOption.icon;

  if (collapsed) {
    return (
      <div className="flex flex-col items-center px-0 py-2">
        <div className="flex h-8 items-center justify-center">
          <Button type="button" variant="ghost" size="icon" className="size-6" onClick={onToggleCollapsed} aria-label={t("actions.expandSidebar")} title={t("actions.expandSidebar")}>
            <PanelLeftOpen className="size-4 stroke-1" />
          </Button>
        </div>
        <div className="flex h-8 items-center justify-center">
          <Button type="button" variant="ghost" size="icon" className="size-6" onClick={onToggleSearch} aria-label={t("actions.search")} title={t("actions.search")}>
            <Search className="size-4 stroke-1" />
          </Button>
        </div>
        <div className="flex h-8 items-center justify-center">
          <Button type="button" variant="ghost" size="icon" className="size-6" onClick={onUpload} disabled={uploading} aria-label={t("upload")} title={t("upload")}>
            <Plus className="size-4 stroke-1" />
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-w-0 overflow-hidden pt-2">
      <div className="flex min-w-0 h-8 items-center gap-2 px-2">
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-[15px] font-medium text-foreground">{t("title")}</h1>
        </div>

        <div className="flex shrink-0 items-center gap-1">
          {showCollapseButton ? (
            <Button type="button" variant="ghost" size="icon" className="size-6" onClick={onToggleCollapsed} aria-label={t("actions.collapseSidebar")} title={t("actions.collapseSidebar")}>
              <PanelLeftClose className="size-4 stroke-1 " />
            </Button>
          ) : null}
          <Button type="button" variant="ghost" size="icon" className="size-6" onClick={onToggleSearch} aria-label={t("actions.search")} title={t("actions.search")}>
            <Search className="size-4 stroke-1" />
          </Button>
          <Button type="button" variant="ghost" size="icon" className="size-6" onClick={onUpload} disabled={uploading} aria-label={t("upload")} title={t("upload")}>
            <Plus className="size-4 stroke-1" />
          </Button>
        </div>
      </div>

      {searchOpen ? (
        <div className="pt-2 px-1">
          <Input
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder={t("searchPlaceholder")}
            className="bg-background px-2 focus-visible:ring-0"
          />
        </div>
      ) : null}

      <div className="flex min-w-0 items-center gap-0.5 overflow-hidden px-0 pt-1.5 pb-1.5">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 gap-0.5 px-1 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          onClick={selectedCount > 0 ? onClearSelection : onSelectLoaded}
          disabled={selectedCount > 0 ? bulkDeleteDisabled : selectAllDisabled}
        >
          {selectedCount > 0 ? <SquareDashed className="size-3 stroke-1" /> : <SquareDashedMousePointer className="size-3 stroke-1" />}
          {selectedCount > 0 ? tCommon("cancel") : t("actions.selectAll")}
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className={cn(
                "h-7 gap-0.5 px-1 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground",
                hasActiveFilters && "bg-muted text-foreground",
              )}
            >
              <Funnel className="size-3 stroke-1" />
              {t("actions.filter")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-40 p-1.5">
            <div className="space-y-1">
              <button
                type="button"
                className={cn(
                  "flex h-6 w-full items-center gap-2 rounded-md px-2 text-left text-[10px] transition-colors",
                  !hasActiveFilters ? "bg-muted/55 text-foreground" : "text-foreground/70 hover:bg-muted hover:text-foreground",
                )}
                onClick={() => onFilterToggle("all")}
              >
                <AllFilterIcon className="size-3 stroke-1 text-muted-foreground" />
                <span className="flex-1 truncate">{t("filters.all")}</span>
                {!hasActiveFilters ? <Check className="size-3 stroke-1 text-muted-foreground" /> : null}
              </button>

              <DropdownMenuSeparator className="mx-0 my-1" />

              {FILE_FILTER_OPTIONS.filter((item) => item.value !== "all").map((item) => {
                const value = item.value as FileFilterValue;
                const active = activeFilterSet.has(value);
                return (
                  <button
                    key={value}
                    type="button"
                    className={cn(
                      "flex h-6 w-full items-center gap-2 rounded-md px-2 text-left text-[10px] transition-colors",
                      active ? "bg-muted/55 text-foreground" : "text-foreground/70 hover:bg-muted hover:text-foreground",
                    )}
                    onClick={() => onFilterToggle(value)}
                  >
                    <item.icon className="size-3 stroke-1 text-muted-foreground" />
                    <span className="flex-1 truncate">{t(`filters.${item.value}`)}</span>
                    {active ? <Check className="size-3 stroke-1 text-muted-foreground" /> : null}
                  </button>
                );
              })}
            </div>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 gap-0.5 px-1 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground"
            >
              <ArrowDownUp className="size-3 stroke-1" />
              {t("actions.sort")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-36 p-1.5">
            <div className="space-y-1">
              {FILE_SORT_OPTIONS.map((item) => {
                const active = item.value === sortKey;
                return (
                  <button
                    key={item.value}
                    type="button"
                    className={cn(
                      "flex h-6 w-full items-center gap-2 rounded-md px-2 text-left text-[10px] transition-colors",
                      active ? "bg-muted/55 text-foreground" : "text-foreground/70 hover:bg-muted hover:text-foreground",
                    )}
                    onClick={() => onSortChange(item.value)}
                  >
                    <ArrowDownUp className="size-3 stroke-1 text-muted-foreground" />
                    <span className="flex-1 truncate">{t(item.value === "last_used" ? "sort.lastUsed" : `sort.${item.value}`)}</span>
                    {active ? <Check className="size-3 stroke-1 text-muted-foreground" /> : null}
                  </button>
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
            onClick={onBulkDeleteRequest}
            disabled={bulkDeleteDisabled}
          >
            <Trash2 className="size-3 stroke-1" />
            {t("actions.delete")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
