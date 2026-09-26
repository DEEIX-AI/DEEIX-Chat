"use client";

import * as React from "react";
import {
  ArrowDownUp,
  Check,
  ChevronLeft,
  ChevronRight,
  Funnel,
  ListChecks,
  RefreshCw,
  Search,
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
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

export type TableToolbarOption = {
  label: string;
  value: string;
};

type TableToolbarSelectFilter = {
  key: string;
  label: string;
  value: string;
  options: TableToolbarOption[];
  onValueChange: (value: string) => void;
};

type TableToolbarCustomFilter = {
  key: string;
  label: string;
  active: boolean;
  content: React.ReactNode;
};

export type TableToolbarFilter = TableToolbarSelectFilter | TableToolbarCustomFilter;

const ALL_FILTER_VALUE = "__table_toolbar_all__";
const DEFAULT_QUERY_DEBOUNCE_MS = 250;

export type TableToolbarSort = {
  value: string;
  options: TableToolbarOption[];
  onValueChange: (value: string) => void;
};

export type TableToolbarProps = {
  query: string;
  onQueryChange: (value: string) => void;
  queryDebounceMs?: number;
  queryPlaceholder?: string;
  filters?: TableToolbarFilter[];
  sort?: TableToolbarSort;
  selectedCount?: number;
  bulkContent?: React.ReactNode;
  bulkActions?: Array<{
    key: string;
    label: string;
    icon?: React.ReactNode;
    onClick: () => void;
    disabled?: boolean;
  }>;
  loading?: boolean;
  onRefresh: () => void;
  refreshDisabled?: boolean;
  refreshLoading?: boolean;
  refreshLabel?: string;
  children?: React.ReactNode;
  className?: string;
};

function PopoverField({
  label,
  children,
  className,
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <p className="text-[11px] text-muted-foreground">{label}</p>
      {children}
    </div>
  );
}

function MenuOption({
  label,
  active,
  icon,
  onSelect,
}: {
  label: string;
  active: boolean;
  icon?: React.ReactNode;
  onSelect: () => void;
}) {
  return (
    <DropdownMenuItem
      onSelect={onSelect}
      className={cn(
        "h-6 gap-2 px-2 py-0 text-[10px]",
        active
          ? "bg-muted/55 text-foreground focus:bg-muted/55"
          : "text-foreground/70 focus:bg-muted focus:text-foreground",
      )}
    >
      {icon}
      <span className="flex-1 truncate">{label}</span>
      {active ? <Check className="size-3 stroke-1 text-muted-foreground" /> : null}
    </DropdownMenuItem>
  );
}

function isCustomFilter(filter: TableToolbarFilter): filter is TableToolbarCustomFilter {
  return "content" in filter;
}

function useCommittedQueryDraft({
  value,
  onValueChange,
  debounceMs,
}: {
  value: string;
  onValueChange: (value: string) => void;
  debounceMs: number;
}) {
  const [draft, setDraft] = React.useState(value);
  const commitTimerRef = React.useRef<number | null>(null);

  const clearCommitTimer = React.useCallback(() => {
    if (commitTimerRef.current === null) {
      return;
    }

    window.clearTimeout(commitTimerRef.current);
    commitTimerRef.current = null;
  }, []);

  React.useEffect(() => {
    clearCommitTimer();
    setDraft(value);
  }, [clearCommitTimer, value]);

  React.useEffect(() => {
    return () => {
      clearCommitTimer();
    };
  }, [clearCommitTimer]);

  const commit = React.useCallback(
    (nextValue: string, immediate = false) => {
      clearCommitTimer();

      if (immediate || debounceMs <= 0) {
        onValueChange(nextValue);
        return;
      }

      commitTimerRef.current = window.setTimeout(() => {
        commitTimerRef.current = null;
        onValueChange(nextValue);
      }, debounceMs);
    },
    [clearCommitTimer, debounceMs, onValueChange],
  );

  const update = React.useCallback(
    (nextValue: string) => {
      setDraft(nextValue);
      commit(nextValue);
    },
    [commit],
  );

  const clear = React.useCallback(() => {
    setDraft("");
    commit("", true);
  }, [commit]);

  return { clear, draft, update };
}

function SelectFilterControl({
  filter,
  disabled,
}: {
  filter: TableToolbarSelectFilter;
  disabled: boolean;
}) {
  return (
    <Select
      value={filter.value.trim() || ALL_FILTER_VALUE}
      disabled={disabled}
      onValueChange={(value) => filter.onValueChange(value === ALL_FILTER_VALUE ? "" : value)}
    >
      <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
        <SelectValue />
      </SelectTrigger>
      <SelectContent
        position="popper"
        align="start"
        className="z-[100]"
        viewportClassName="max-h-[220px]"
      >
        {filter.options.map((option) => (
          <SelectItem
            key={`${filter.key}:${option.value || ALL_FILTER_VALUE}`}
            value={option.value || ALL_FILTER_VALUE}
            className="text-[11px]"
          >
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function ToolbarButton({
  className,
  ...props
}: React.ComponentProps<typeof Button>) {
  return (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      className={cn(
        "h-8 gap-1.5 px-2 text-xs text-muted-foreground shadow-none hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground sm:px-2.5",
        className,
      )}
      {...props}
    />
  );
}

export function TableToolbar({
  query,
  onQueryChange,
  queryDebounceMs = DEFAULT_QUERY_DEBOUNCE_MS,
  queryPlaceholder,
  filters = [],
  sort,
  selectedCount = 0,
  bulkContent,
  bulkActions = [],
  loading = false,
  onRefresh,
  refreshDisabled,
  refreshLoading,
  refreshLabel,
  children,
  className,
}: TableToolbarProps) {
  const t = useTranslations("common.table");
  const queryDraft = useCommittedQueryDraft({
    value: query,
    onValueChange: onQueryChange,
    debounceMs: queryDebounceMs,
  });
  const hasSelection = selectedCount > 0;
  const hasQuery = queryDraft.draft.trim() !== "";
  const activeFilterCount = filters.filter((filter) => (isCustomFilter(filter) ? filter.active : filter.value.trim() !== "")).length;

  return (
    <section
      className={cn(
        "flex min-h-10 flex-nowrap items-center gap-1.5 overflow-x-auto overscroll-x-contain px-0.5 py-1 [scrollbar-width:none] [-ms-overflow-style:none] md:gap-3 [&::-webkit-scrollbar]:hidden",
        className,
      )}
    >
      <div className="flex min-w-max flex-nowrap items-center gap-1.5 md:min-w-0 md:flex-1">
        <div className="md:hidden">
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarButton aria-label={t("search")}>
                <Search className="size-3.5 stroke-1" />
                <span className="hidden sm:inline">{t("search")}</span>
                {hasQuery ? <span className="text-[10px] tabular-nums">1</span> : null}
              </ToolbarButton>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-[min(320px,calc(100vw-32px))] p-3">
              <div className="space-y-2">
                <PopoverField label={t("search")}>
                  <div className="relative">
                    <Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 stroke-1 text-muted-foreground" />
                    <Input
                      value={queryDraft.draft}
                      placeholder={queryPlaceholder ?? t("searchPlaceholder")}
                      onChange={(event) => queryDraft.update(event.target.value)}
                      className="bg-background pl-8"
                    />
                  </div>
                </PopoverField>
                {hasQuery ? (
                  <div className="flex justify-end border-t border-border/60 pt-2">
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      className="h-7 px-2 text-xs shadow-none hover:bg-muted"
                      onClick={queryDraft.clear}
                    >
                      {t("clearSearch")}
                    </Button>
                  </div>
                ) : null}
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <div className="relative hidden min-w-[220px] flex-1 md:block md:max-w-[320px]">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 stroke-1 text-muted-foreground" />
          <Input
            value={queryDraft.draft}
            placeholder={queryPlaceholder ?? t("searchPlaceholder")}
            onChange={(event) => queryDraft.update(event.target.value)}
            className="bg-background pl-8"
          />
        </div>

        {filters.length ? (
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarButton disabled={loading} aria-label={t("filter")}>
                <Funnel className="size-3.5 stroke-1" />
                <span className="hidden sm:inline">{t("filter")}</span>
                {activeFilterCount ? <span className="text-[10px] tabular-nums">{activeFilterCount}</span> : null}
              </ToolbarButton>
            </PopoverTrigger>
            <PopoverContent align="start" className="max-h-[min(420px,calc(100vh-96px))] w-[240px] overflow-y-auto p-2">
              <div className="space-y-2">
                {filters.map((filter) => (
                  <div key={filter.key} className="space-y-1">
                    <p className="px-1 text-[10px] text-muted-foreground">{filter.label}</p>

                    {isCustomFilter(filter) ? (
                      filter.content
                    ) : (
                      <SelectFilterControl filter={filter} disabled={loading} />
                    )}
                  </div>
                ))}
              </div>
            </PopoverContent>
          </Popover>
        ) : null}

        {sort ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <ToolbarButton disabled={loading} aria-label={t("sort")}>
                <ArrowDownUp className="size-3.5 stroke-1" />
                <span className="hidden sm:inline">{t("sort")}</span>
              </ToolbarButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-40 p-1.5">
              <div className="space-y-1">
                {sort.options.map((option) => (
                  <MenuOption
                    key={option.value}
                    label={option.label}
                    active={sort.value === option.value}
                    icon={<ArrowDownUp className="size-3 stroke-1 text-muted-foreground" />}
                    onSelect={() => sort.onValueChange(option.value)}
                  />
                ))}
              </div>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null}

        {(bulkContent || bulkActions.length > 0) && (
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarButton
                disabled={loading || !hasSelection}
                aria-label={t("bulk")}
              >
                <ListChecks className="size-3.5 stroke-1" />
                <span className="hidden sm:inline">{t("bulk")}</span>
              </ToolbarButton>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-[240px] p-2">
              <div className="space-y-1">
                <p className="flex h-7 items-center px-2 text-[11px] text-muted-foreground">{t("selectedItems", { count: selectedCount })}</p>

                {bulkContent}

                {bulkActions.length ? (
                  <div className="space-y-1">
                    {bulkActions.map((action) => (
                      <button
                        key={action.key}
                        type="button"
                        onClick={action.onClick}
                        disabled={loading || !hasSelection || action.disabled}
                        className="group flex h-7 w-full items-center gap-2 rounded-md px-2 text-left text-[11px] text-destructive transition-colors hover:bg-destructive/10 disabled:pointer-events-none disabled:opacity-50"
                      >
                        {action.icon ? (
                          <span className="flex w-4 shrink-0 justify-center text-muted-foreground group-hover:text-current [&_svg]:size-3 [&_svg]:stroke-1">
                            {action.icon}
                          </span>
                        ) : null}
                        <span className="flex-1 truncate">{action.label}</span>
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>
            </PopoverContent>
          </Popover>
        )}
      </div>

      <div className="ml-auto flex min-h-8 shrink-0 items-center gap-1 md:gap-1">
        <ToolbarButton
          onClick={onRefresh}
          disabled={refreshDisabled ?? loading}
          aria-label={refreshLabel ?? t("refresh")}
          title={refreshLabel ?? t("refresh")}
        >
          <RefreshCw className={cn("size-3.5 stroke-1", (refreshLoading ?? loading) && "animate-spin")} />
        </ToolbarButton>
        {children}
      </div>
    </section>
  );
}

export type TablePaginationProps = {
  total: number;
  page: number;
  pageCount: number;
  pageSize: number;
  pageSizeOptions?: readonly number[];
  showPageSize?: boolean;
  summary?: React.ReactNode;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  loading?: boolean;
  className?: string;
};

export function TablePagination({
  total,
  page,
  pageCount,
  pageSize,
  pageSizeOptions = [25, 50, 100, 200, 500, 1000],
  showPageSize = true,
  summary,
  onPageChange,
  onPageSizeChange,
  loading = false,
  className,
}: TablePaginationProps) {
  const t = useTranslations("common.table");
  const normalizedPageCount = Math.max(1, pageCount);

  return (
    <div className={cn("flex flex-wrap items-center justify-between gap-1.5 px-0.5 text-xs font-normal text-muted-foreground", className)}>
      <p>
        {summary ?? t("pagination", { total, page: Math.min(page, normalizedPageCount), pageCount: normalizedPageCount })}
      </p>

      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          className="h-5 w-5 text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          onClick={() => onPageChange(Math.max(1, page - 1))}
          disabled={loading || page <= 1}
          aria-label={t("previousPage")}
          title={t("previousPage")}
        >
          <ChevronLeft className="size-4 stroke-1" />
        </Button>

        {showPageSize ? (
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="h-5 px-1.5 text-xs font-normal text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
                disabled={loading}
              >
                <span>{t("pageSize", { size: pageSize })}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-[88px] min-w-[88px] space-y-0.5 p-1">
              {pageSizeOptions.map((value) => (
                <DropdownMenuItem
                  key={value}
                  disabled={loading}
                  onSelect={() => onPageSizeChange(value)}
                  className={cn("h-6 justify-end px-2 py-0 text-xs font-normal text-muted-foreground focus:bg-muted focus:text-foreground", value === pageSize && "bg-muted/60 text-foreground")}
                >
                  <span className="inline-flex w-full items-center justify-end tabular-nums">
                    <span>{t("pageSize", { size: value })}</span>
                  </span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null}

        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          className="h-5 w-5 text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          onClick={() => onPageChange(Math.min(normalizedPageCount, page + 1))}
          disabled={loading || page >= normalizedPageCount}
          aria-label={t("nextPage")}
          title={t("nextPage")}
        >
          <ChevronRight className="size-4 stroke-1" />
        </Button>
      </div>
    </div>
  );
}
