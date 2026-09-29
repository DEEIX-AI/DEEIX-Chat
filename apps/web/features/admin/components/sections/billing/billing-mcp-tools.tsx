"use client";

import * as React from "react";
import { CircleDollarSign, CircleHelp, Save } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import {
  formatMCPToolPriceInput,
  mcpToolPriceInputToNanousd,
  useAdminBillingMcpTools,
} from "@/features/admin/hooks/use-admin-billing-mcp-tools";
import { SettingsSection } from "@/shared/components/settings-layout";

const MCP_PRICING_PAGE_SIZE_DEFAULT = 25;

// BulkActionControlRow shares the single-row layout of the user/model/upstream bulk menus: apply button + control.
function BulkActionControlRow({
  icon,
  label,
  disabled,
  onApply,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  disabled: boolean;
  onApply: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="flex h-7 w-full items-center gap-1.5">
      <Button
        type="button"
        variant="ghost"
        className="h-7 w-16 shrink-0 justify-start gap-2 px-2 text-[11px] text-foreground/70 shadow-none hover:bg-muted hover:text-foreground"
        onClick={onApply}
        disabled={disabled}
      >
        {icon}
        {label}
      </Button>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

export function BillingMCPToolsSection() {
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const [query, setQuery] = React.useState("");
  const [serverFilter, setServerFilter] = React.useState("");
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<Set<number>>(new Set());
  const [bulkPriceDraft, setBulkPriceDraft] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(MCP_PRICING_PAGE_SIZE_DEFAULT);
  const clearSelection = React.useCallback(() => setSelectedToolIDs(new Set()), []);
  const {
    rows,
    servers,
    priceDrafts,
    loading,
    saving,
    changedRows,
    load,
    save,
    updatePriceDraft,
    applyPriceToTools,
  } = useAdminBillingMcpTools({ onLoaded: clearSelection });

  const filteredRows = React.useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return rows.filter((row) => {
      if (serverFilter && String(row.serverID) !== serverFilter) {
        return false;
      }
      if (!normalizedQuery) {
        return true;
      }
      return (
        row.serverName.toLowerCase().includes(normalizedQuery) ||
        row.toolLabel.toLowerCase().includes(normalizedQuery) ||
        row.toolName.toLowerCase().includes(normalizedQuery)
      );
    });
  }, [query, rows, serverFilter]);

  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize));
  const pagedRows = React.useMemo(
    () => filteredRows.slice((page - 1) * pageSize, page * pageSize),
    [filteredRows, page, pageSize],
  );
  const mcpVirtualRows = useVirtualTableRows(pagedRows, {
    enabled: pagedRows.length > 100,
    estimateSize: 40,
  });
  const initialLoading = loading && pagedRows.length === 0;
  const showRows = pagedRows.length > 0;

  React.useEffect(() => {
    setPage(1);
  }, [query, serverFilter, pageSize]);

  React.useEffect(() => {
    setPage((current) => Math.min(current, pageCount));
  }, [pageCount]);

  const allPagedSelected = pagedRows.length > 0 && pagedRows.every((row) => selectedToolIDs.has(row.toolID));
  const somePagedSelected = pagedRows.some((row) => selectedToolIDs.has(row.toolID));

  const toggleSelectedPagedRows = React.useCallback((selected: boolean) => {
    setSelectedToolIDs((current) => {
      const next = new Set(current);
      for (const row of pagedRows) {
        if (selected) {
          next.add(row.toolID);
        } else {
          next.delete(row.toolID);
        }
      }
      return next;
    });
  }, [pagedRows]);

  const toggleSelectedRow = React.useCallback((toolID: number, selected: boolean) => {
    setSelectedToolIDs((current) => {
      const next = new Set(current);
      if (selected) {
        next.add(toolID);
      } else {
        next.delete(toolID);
      }
      return next;
    });
  }, []);

  const bulkPriceNanousd = mcpToolPriceInputToNanousd(bulkPriceDraft);

  // Bulk edits only update the local draft, like single-row edits; the top-right Save button persists everything.
  const applyBulkPrice = React.useCallback(() => {
    if (bulkPriceNanousd === null || bulkPriceDraft.trim() === "" || selectedToolIDs.size === 0) {
      return;
    }
    applyPriceToTools(selectedToolIDs, bulkPriceNanousd);
    setSelectedToolIDs(new Set());
  }, [applyPriceToTools, bulkPriceDraft, bulkPriceNanousd, selectedToolIDs]);

  const pricingActions = changedRows.length > 0 ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || saving}
      onClick={() => void save()}
    >
      {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;

  const sectionTitle = (
    <span className="inline-flex items-center gap-1">
      {t("toolPricing.mcpTitle")}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            className="text-muted-foreground hover:bg-transparent hover:text-foreground"
            aria-label={t("toolPricing.mcpHelpLabel")}
          >
            <CircleHelp className="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="right" className="max-w-xs text-xs leading-5">
          <div className="space-y-1">
            <p>{t("toolPricing.mcpHelpPricing")}</p>
            <p>{t("toolPricing.mcpHelpNote")}</p>
          </div>
        </TooltipContent>
      </Tooltip>
    </span>
  );

  return (
    <SettingsSection title={sectionTitle} actions={pricingActions} className="px-1">
      <div className="space-y-3">
        <TableToolbar
          query={query}
          onQueryChange={setQuery}
          queryPlaceholder={t("toolPricing.mcpSearchPlaceholder")}
          filters={[
            {
              key: "server",
              label: t("toolPricing.mcpServer"),
              value: serverFilter,
              onValueChange: setServerFilter,
              options: [
                { label: t("toolPricing.mcpServerAll"), value: "" },
                ...servers.map((server) => ({ label: server.name, value: String(server.id) })),
              ],
            },
          ]}
          selectedCount={selectedToolIDs.size}
          bulkContent={
            <div className="space-y-1">
              <BulkActionControlRow
                icon={<CircleDollarSign className="size-3 stroke-1" />}
                label={t("toolPricing.mcpBulkApply")}
                onApply={applyBulkPrice}
                disabled={loading || saving || bulkPriceNanousd === null || bulkPriceDraft.trim() === "" || selectedToolIDs.size === 0}
              >
                <Input
                  type="number"
                  min="0"
                  step="0.000001"
                  value={bulkPriceDraft}
                  placeholder={t("toolPricing.mcpBulkPrice")}
                  aria-label={t("toolPricing.mcpBulkPrice")}
                  onChange={(event) => setBulkPriceDraft(event.target.value)}
                  disabled={loading || saving || selectedToolIDs.size === 0}
                  className="h-7 px-2 text-[11px]"
                />
              </BulkActionControlRow>
            </div>
          }
          loading={loading || saving}
          onRefresh={() => void load()}
          refreshDisabled={loading || saving}
          refreshLoading={loading}
        />

        <Table
          className="min-w-[560px]"
          viewportRef={mcpVirtualRows.viewportRef}
          viewportClassName={mcpVirtualRows.viewportClassName}
          viewportStyle={mcpVirtualRows.viewportStyle}
        >
          <TableHeader>
            <TableRow>
              <TableHead className="w-[44px] py-1.5 text-center">
                <div className="flex h-7 items-center justify-center">
                  <Checkbox
                    checked={allPagedSelected ? true : somePagedSelected ? "indeterminate" : false}
                    disabled={loading || pagedRows.length === 0}
                    onCheckedChange={(checked) => toggleSelectedPagedRows(checked === true)}
                    aria-label={t("toolPricing.mcpSelectPageTools")}
                  />
                </div>
              </TableHead>
              <TableHead>{t("toolPricing.mcpServer")}</TableHead>
              <TableHead>{t("toolPricing.tool")}</TableHead>
              <TableHead className="text-right">{t("toolPricing.price")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {initialLoading ? <TableLoadingRow colSpan={4} /> : null}
            {!loading && pagedRows.length === 0 ? (
              <TableEmptyRow colSpan={4}>{t("toolPricing.mcpEmpty")}</TableEmptyRow>
            ) : null}
            {showRows ? <VirtualTablePaddingRow colSpan={4} height={mcpVirtualRows.paddingTop} /> : null}
            {showRows
              ? mcpVirtualRows.rows.map(({ item: row }) => (
                  <TableRow key={row.toolID} selected={selectedToolIDs.has(row.toolID)}>
                    <TableCell className="w-[44px] whitespace-nowrap py-1.5">
                      <div className="flex h-7 items-center justify-center">
                        <Checkbox
                          checked={selectedToolIDs.has(row.toolID)}
                          onCheckedChange={(checked) => toggleSelectedRow(row.toolID, checked === true)}
                          aria-label={t("toolPricing.mcpSelectTool", { name: row.toolLabel })}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="whitespace-nowrap py-1.5 text-xs text-muted-foreground">{row.serverName}</TableCell>
                    <TableCell className="w-full max-w-0 py-1.5 text-xs text-foreground">
                      <div className="flex min-w-0 flex-col">
                        <span className="truncate">{row.toolLabel}</span>
                        {row.toolDescription ? (
                          <span className="truncate text-[11px] text-muted-foreground" title={row.toolDescription}>
                            {row.toolDescription}
                          </span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell className="py-1.5 text-right font-mono text-xs text-muted-foreground">
                      <div className="flex items-center justify-end gap-1.5">
                        <span className="text-muted-foreground">$</span>
                        <Input
                          value={priceDrafts[row.toolID] ?? formatMCPToolPriceInput(row.priceNanousd)}
                          inputMode="decimal"
                          className="h-7 w-24 text-right font-mono text-xs"
                          disabled={loading || saving}
                          aria-label={`${row.serverName} ${row.toolLabel} ${t("toolPricing.price")}`}
                          onChange={(event) => updatePriceDraft(row.toolID, event.target.value)}
                        />
                        <span className="whitespace-nowrap text-muted-foreground">
                          / {t("toolPricing.units.call")}
                        </span>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              : null}
            {showRows ? <VirtualTablePaddingRow colSpan={4} height={mcpVirtualRows.paddingBottom} /> : null}
          </TableBody>
        </Table>

        <TablePagination
          total={filteredRows.length}
          page={page}
          pageCount={pageCount}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
          loading={loading}
        />
      </div>
    </SettingsSection>
  );
}
