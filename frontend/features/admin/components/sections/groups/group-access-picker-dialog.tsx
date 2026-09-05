"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import { TablePagination, TableToolbar, type TableToolbarFilter } from "@/components/ui/table-tools";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import type { UserIdentityProviderSummaryDTO } from "@/shared/api/auth.types";
import { cn } from "@/lib/utils";

export type GroupAccessTableItem = {
  id: number;
  label: string;
  nickname?: string;
  email?: string;
  subscriptionStatus?: string;
  identityProviders?: UserIdentityProviderSummaryDTO[];
  sourceLabels?: string[];
  vendorLabels?: string[];
  protocolLabels?: string[];
};

type GroupAccessPickerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  items: GroupAccessTableItem[];
  selectedIDs: Set<number>;
  setSelectedIDs: React.Dispatch<React.SetStateAction<Set<number>>>;
  query: string;
  onQueryChange: (value: string) => void;
  filters?: TableToolbarFilter[];
  topContent?: React.ReactNode;
  manualTitle?: string;
  page: number;
  pageSize: number;
  total: number;
  loading: boolean;
  bulkLoading: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  onRefresh: () => void;
  onSelectAllResults: () => void;
  onClearSelection: () => void;
  searchPlaceholder: string;
  itemTitle: string;
  nicknameTitle?: string;
  emailTitle?: string;
  subscriptionTitle?: string;
  identityTitle?: string;
  sourceTitle?: string;
  vendorTitle?: string;
  protocolTitle?: string;
  contentClassName?: string;
  tableViewportClassName?: string;
  emptyText: string;
};

export function GroupAccessPickerDialog({
  open,
  onOpenChange,
  title,
  description,
  items,
  selectedIDs,
  setSelectedIDs,
  query,
  onQueryChange,
  filters,
  topContent,
  manualTitle,
  page,
  pageSize,
  total,
  loading,
  bulkLoading,
  onPageChange,
  onPageSizeChange,
  onRefresh,
  onSelectAllResults,
  onClearSelection,
  searchPlaceholder,
  itemTitle,
  nicknameTitle,
  emailTitle,
  subscriptionTitle,
  identityTitle,
  sourceTitle,
  vendorTitle,
  protocolTitle,
  contentClassName,
  tableViewportClassName,
  emptyText,
}: GroupAccessPickerDialogProps) {
  const t = useTranslations("adminGroups");
  const accessTable = (
    <GroupAccessTable
      items={items}
      selectedIDs={selectedIDs}
      setSelectedIDs={setSelectedIDs}
      query={query}
      onQueryChange={onQueryChange}
      filters={filters}
      page={page}
      pageSize={pageSize}
      total={total}
      loading={loading}
      bulkLoading={bulkLoading}
      onPageChange={onPageChange}
      onPageSizeChange={onPageSizeChange}
      onRefresh={onRefresh}
      onSelectAllResults={onSelectAllResults}
      onClearSelection={onClearSelection}
      searchPlaceholder={searchPlaceholder}
      itemTitle={itemTitle}
      nicknameTitle={nicknameTitle}
      emailTitle={emailTitle}
      subscriptionTitle={subscriptionTitle}
      identityTitle={identityTitle}
      sourceTitle={sourceTitle}
      vendorTitle={vendorTitle}
      protocolTitle={protocolTitle}
      tableViewportClassName={tableViewportClassName}
      emptyText={emptyText}
    />
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          "flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0",
          contentClassName ?? "sm:max-w-[720px]",
        )}
      >
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="sr-only">{description}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-hidden px-4 py-2">
          {topContent ? <div className="pb-3">{topContent}</div> : null}
          {manualTitle ? (
            <div className="space-y-2 rounded-md bg-muted/30 px-3 py-2.5">
              <p className="text-xs font-medium text-foreground">{manualTitle}</p>
              {accessTable}
            </div>
          ) : (
            accessTable
          )}
        </div>

        <DialogFooter className="shrink-0 px-4 py-3">
          <Button type="button" onClick={() => onOpenChange(false)}>
            {t("done")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function BadgeList({
  labels,
  maxVisible = 2,
}: {
  labels?: string[];
  maxVisible?: number;
}) {
  const values = (labels ?? []).map((label) => label.trim()).filter(Boolean);
  if (values.length === 0) {
    return <span className="text-xs text-muted-foreground">-</span>;
  }

  const visible = values.slice(0, maxVisible);
  const hiddenCount = Math.max(0, values.length - visible.length);
  const title = values.join(", ");

  return (
    <div className="flex min-w-0 items-center gap-1 overflow-hidden" title={title}>
      {visible.map((label) => (
        <Badge
          key={label}
          variant="secondary"
          className="min-w-0 max-w-full shrink justify-start rounded-sm px-1.5 py-0 text-[10px] font-normal leading-4"
        >
          <span className="min-w-0 truncate">{label}</span>
        </Badge>
      ))}
      {hiddenCount > 0 ? (
        <Badge
          variant="outline"
          className="shrink-0 rounded-sm px-1.5 py-0 text-[10px] font-normal leading-4 text-muted-foreground"
        >
          +{hiddenCount}
        </Badge>
      ) : null}
    </div>
  );
}

function IdentityProviderIconList({
  providers,
}: {
  providers?: UserIdentityProviderSummaryDTO[];
}) {
  const values = (providers ?? []).filter((provider) => provider.slug || provider.name);
  if (values.length === 0) {
    return <span className="text-xs text-muted-foreground">-</span>;
  }

  const visible = values.slice(0, 4);
  const hiddenCount = Math.max(0, values.length - visible.length);
  const title = values.map((provider) => provider.name || provider.slug).join(", ");

  return (
    <div className="flex min-w-0 items-center gap-1" title={title}>
      {visible.map((provider) => (
        <IdentityProviderIcon
          key={`${provider.id}:${provider.slug}`}
          name={provider.name}
          slug={provider.slug}
          logoURL={provider.logoURL}
          className="size-4"
          iconClassName="size-4"
          fallbackClassName="text-[10px]"
        />
      ))}
      {hiddenCount > 0 ? (
        <span className="text-[10px] text-muted-foreground">+{hiddenCount}</span>
      ) : null}
    </div>
  );
}

type GroupAccessTableProps = {
  items: GroupAccessTableItem[];
  selectedIDs: Set<number>;
  setSelectedIDs: React.Dispatch<React.SetStateAction<Set<number>>>;
  query: string;
  onQueryChange: (value: string) => void;
  filters?: TableToolbarFilter[];
  page: number;
  pageSize: number;
  total: number;
  loading: boolean;
  bulkLoading: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  onRefresh: () => void;
  onSelectAllResults: () => void;
  onClearSelection: () => void;
  searchPlaceholder: string;
  itemTitle: string;
  nicknameTitle?: string;
  emailTitle?: string;
  subscriptionTitle?: string;
  identityTitle?: string;
  sourceTitle?: string;
  vendorTitle?: string;
  protocolTitle?: string;
  tableViewportClassName?: string;
  emptyText: string;
};

function GroupAccessTable({
  items,
  selectedIDs,
  setSelectedIDs,
  query,
  onQueryChange,
  filters,
  page,
  pageSize,
  total,
  loading,
  bulkLoading,
  onPageChange,
  onPageSizeChange,
  onRefresh,
  onSelectAllResults,
  onClearSelection,
  searchPlaceholder,
  itemTitle,
  nicknameTitle,
  emailTitle,
  subscriptionTitle,
  identityTitle,
  sourceTitle,
  vendorTitle,
  protocolTitle,
  tableViewportClassName = "max-h-[300px]",
  emptyText,
}: GroupAccessTableProps) {
  const t = useTranslations("adminGroups");
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const isPageEmpty = items.length === 0;
  const busy = loading || bulkLoading;
  const showSourceColumn = Boolean(sourceTitle);
  const showVendorColumn = Boolean(vendorTitle);
  const showProtocolColumn = Boolean(protocolTitle);
  const showNicknameColumn = Boolean(nicknameTitle);
  const showEmailColumn = Boolean(emailTitle);
  const showSubscriptionColumn = Boolean(subscriptionTitle);
  const showIdentityColumn = Boolean(identityTitle);
  const tableColumnCount =
    2 +
    (showNicknameColumn ? 1 : 0) +
    (showEmailColumn ? 1 : 0) +
    (showSubscriptionColumn ? 1 : 0) +
    (showIdentityColumn ? 1 : 0) +
    (showSourceColumn ? 1 : 0) +
    (showVendorColumn ? 1 : 0) +
    (showProtocolColumn ? 1 : 0);
  const pageSelectedCount = React.useMemo(
    () => items.reduce((count, item) => count + (selectedIDs.has(item.id) ? 1 : 0), 0),
    [items, selectedIDs],
  );
  const pageSelectionState = isPageEmpty
    ? false
    : pageSelectedCount === items.length
      ? true
      : pageSelectedCount > 0
        ? "indeterminate"
        : false;

  const toggle = React.useCallback(
    (id: number) => {
      setSelectedIDs((prev) => {
        const next = new Set(prev);
        if (next.has(id)) {
          next.delete(id);
        } else {
          next.add(id);
        }
        return next;
      });
    },
    [setSelectedIDs],
  );

  const setPageSelected = React.useCallback((checked: boolean) => {
    setSelectedIDs((prev) => {
      const next = new Set(prev);
      items.forEach((item) => {
        if (checked) {
          next.add(item.id);
        } else {
          next.delete(item.id);
        }
      });
      return next;
    });
  }, [items, setSelectedIDs]);

  return (
    <div className="flex min-h-0 flex-col gap-2.5">
      <TableToolbar
        query={query}
        onQueryChange={onQueryChange}
        queryPlaceholder={searchPlaceholder}
        filters={filters}
        loading={busy}
        onRefresh={onRefresh}
        refreshDisabled={busy}
        refreshLoading={loading}
        className="min-h-9 py-0"
      >
        <Button
          type="button"
          size="sm"
          variant="secondary"
          className="h-7 px-2 text-xs shadow-none"
          disabled={busy || total === 0}
          onClick={onSelectAllResults}
        >
          {bulkLoading ? <SpinnerLabel>{t("selecting")}</SpinnerLabel> : t("selectAllResults")}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="secondary"
          className="h-7 px-2 text-xs shadow-none"
          disabled={busy || selectedIDs.size === 0}
          onClick={onClearSelection}
        >
          {t("clearSelection")}
        </Button>
      </TableToolbar>

      <Table
        shellClassName="rounded-md"
        className={cn(
          (showSourceColumn || showNicknameColumn || showEmailColumn || showSubscriptionColumn || showIdentityColumn) &&
            "min-w-[820px] table-fixed",
        )}
        viewportClassName={cn(
          tableViewportClassName,
          "[&_thead]:sticky [&_thead]:top-0 [&_thead]:z-20",
        )}
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-10 px-2 text-center">
              <div className="flex h-6 items-center justify-center">
                <Checkbox
                  checked={pageSelectionState}
                  onCheckedChange={(value) => setPageSelected(value === true)}
                  disabled={busy || isPageEmpty}
                  aria-label={t("selectPage")}
                />
              </div>
            </TableHead>
            <TableHead>{itemTitle}</TableHead>
            {showNicknameColumn ? (
              <TableHead className="w-[150px]">{nicknameTitle}</TableHead>
            ) : null}
            {showEmailColumn ? (
              <TableHead className="w-[220px]">{emailTitle}</TableHead>
            ) : null}
            {showSubscriptionColumn ? (
              <TableHead className="w-[160px]">{subscriptionTitle}</TableHead>
            ) : null}
            {showIdentityColumn ? (
              <TableHead className="w-[110px]">{identityTitle}</TableHead>
            ) : null}
            {showSourceColumn ? (
              <TableHead className="w-[160px]">{sourceTitle}</TableHead>
            ) : null}
            {showVendorColumn ? (
              <TableHead className="w-[120px]">{vendorTitle}</TableHead>
            ) : null}
            {showProtocolColumn ? (
              <TableHead className="w-[200px]">{protocolTitle}</TableHead>
            ) : null}
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? <TableLoadingRow colSpan={tableColumnCount} /> : null}
          {!loading && isPageEmpty ? (
            <TableEmptyRow colSpan={tableColumnCount}>{emptyText}</TableEmptyRow>
          ) : null}
          {!loading
            ? items.map((item) => {
                const checked = selectedIDs.has(item.id);
                return (
                  <TableRow
                    key={item.id}
                    interactive
                    selected={checked}
                    className={cn("cursor-pointer", busy && "pointer-events-none opacity-60")}
                    onClick={() => toggle(item.id)}
                  >
                    <TableCell
                      className="w-10 px-2 py-1.5 text-center"
                      onClick={(event) => event.stopPropagation()}
                    >
                      <div className="flex h-6 items-center justify-center">
                        <Checkbox
                          checked={checked}
                          onCheckedChange={() => toggle(item.id)}
                          disabled={busy}
                          aria-label={item.label}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="min-w-0 py-1.5">
                      <span className="block max-w-full truncate text-xs text-foreground">
                        {item.label}
                      </span>
                    </TableCell>
                    {showNicknameColumn ? (
                      <TableCell className="w-[150px] py-1.5 text-xs text-muted-foreground">
                        <span className="block max-w-full truncate" title={item.nickname}>
                          {item.nickname || "-"}
                        </span>
                      </TableCell>
                    ) : null}
                    {showEmailColumn ? (
                      <TableCell className="w-[220px] py-1.5 text-xs text-muted-foreground">
                        <span className="block max-w-full truncate" title={item.email}>
                          {item.email || "-"}
                        </span>
                      </TableCell>
                    ) : null}
                    {showSubscriptionColumn ? (
                      <TableCell className="w-[160px] py-1.5 text-xs text-muted-foreground">
                        <span className="block max-w-full truncate" title={item.subscriptionStatus}>
                          {item.subscriptionStatus || "-"}
                        </span>
                      </TableCell>
                    ) : null}
                    {showIdentityColumn ? (
                      <TableCell className="w-[110px] py-1.5 text-xs text-muted-foreground">
                        <IdentityProviderIconList providers={item.identityProviders} />
                      </TableCell>
                    ) : null}
                    {showSourceColumn ? (
                      <TableCell className="w-[160px] py-1.5 text-xs text-muted-foreground">
                        <BadgeList labels={item.sourceLabels} />
                      </TableCell>
                    ) : null}
                    {showVendorColumn ? (
                      <TableCell className="w-[120px] py-1.5 text-xs text-muted-foreground">
                        <BadgeList labels={item.vendorLabels} maxVisible={1} />
                      </TableCell>
                    ) : null}
                    {showProtocolColumn ? (
                      <TableCell className="w-[200px] py-1.5 text-xs text-muted-foreground">
                        <BadgeList labels={item.protocolLabels} maxVisible={1} />
                      </TableCell>
                    ) : null}
                  </TableRow>
                );
              })
            : null}
        </TableBody>
      </Table>

      <TablePagination
        total={total}
        page={page}
        pageCount={totalPages}
        pageSize={pageSize}
        summary={
          selectedIDs.size > 0
            ? t("selectionPagination", {
                selected: selectedIDs.size,
                total,
                page: Math.min(page, totalPages),
                totalPages,
              })
            : t("paginationSummary", {
                total,
                page: Math.min(page, totalPages),
                totalPages,
              })
        }
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
        loading={busy}
      />
    </div>
  );
}
