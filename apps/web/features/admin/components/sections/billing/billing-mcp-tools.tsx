"use client";

import * as React from "react";
import { CircleDollarSign, CircleHelp, Save } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { listAdminMCPServers, listAdminMCPServerTools, updateAdminMCPTool } from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { SettingsSection } from "@/shared/components/settings-layout";

type MCPToolPricingRow = {
  toolID: number;
  serverID: number;
  serverName: string;
  toolLabel: string;
  toolName: string;
  toolDescription: string;
  priceNanousd: number;
};

type MCPServerOption = {
  id: number;
  name: string;
};

const MCP_PRICING_PAGE_SIZES = [25, 50, 100] as const;

function formatMCPToolPriceInput(priceNanousd: number): string {
  if (!Number.isFinite(priceNanousd) || priceNanousd <= 0) {
    return "0";
  }
  return String(priceNanousd / 1_000_000_000);
}

function mcpToolPriceInputToNanousd(value: string): number | null {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return Math.round(parsed * 1_000_000_000);
}

function mcpToolPriceDraftsFrom(rows: MCPToolPricingRow[]): Record<number, string> {
  return Object.fromEntries(rows.map((row) => [row.toolID, formatMCPToolPriceInput(row.priceNanousd)]));
}

// BulkActionControlRow 与用户/模型/上游管理的批量菜单保持同一行布局：应用按钮 + 控件。
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
  const [rows, setRows] = React.useState<MCPToolPricingRow[]>([]);
  const [servers, setServers] = React.useState<MCPServerOption[]>([]);
  const [savedPrices, setSavedPrices] = React.useState<Record<number, number>>({});
  const [priceDrafts, setPriceDrafts] = React.useState<Record<number, string>>({});
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [serverFilter, setServerFilter] = React.useState("");
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<Set<number>>(new Set());
  const [bulkPriceDraft, setBulkPriceDraft] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState<number>(MCP_PRICING_PAGE_SIZES[0]);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const serverItems = await listAdminMCPServers(token);
      const toolLists = await Promise.all(serverItems.map((server) => listAdminMCPServerTools(token, server.id)));
      const nextRows = serverItems.flatMap((server, index) => toolLists[index].map((tool) => ({
        toolID: tool.id,
        serverID: server.id,
        serverName: server.name,
        toolLabel: tool.displayName?.trim() || tool.name,
        toolName: tool.name,
        toolDescription: tool.description?.trim() ?? "",
        priceNanousd: tool.priceNanousd,
      })));
      setServers(serverItems.map((server) => ({ id: server.id, name: server.name })));
      setRows(nextRows);
      setSavedPrices(Object.fromEntries(nextRows.map((row) => [row.toolID, row.priceNanousd])));
      setPriceDrafts(mcpToolPriceDraftsFrom(nextRows));
      setSelectedToolIDs(new Set());
    } catch (error) {
      toast.error(t("toast.mcpToolsLoadFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void load();
  }, [load]);

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

  // 批量设置只更新本地草稿，与单行编辑一致，统一由右上角保存按钮持久化。
  const applyBulkPrice = React.useCallback(() => {
    if (bulkPriceNanousd === null || bulkPriceDraft.trim() === "" || selectedToolIDs.size === 0) {
      return;
    }
    setRows((current) => current.map((row) => (
      selectedToolIDs.has(row.toolID) ? { ...row, priceNanousd: bulkPriceNanousd } : row
    )));
    setPriceDrafts((current) => {
      const next = { ...current };
      for (const toolID of selectedToolIDs) {
        next[toolID] = formatMCPToolPriceInput(bulkPriceNanousd);
      }
      return next;
    });
    setSelectedToolIDs(new Set());
  }, [bulkPriceDraft, bulkPriceNanousd, selectedToolIDs]);

  const changedRows = React.useMemo(
    () => rows.filter((row) => row.priceNanousd !== (savedPrices[row.toolID] ?? 0)),
    [rows, savedPrices],
  );
  const pricingActions = changedRows.length > 0 ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || saving}
      onClick={() => void handleSave()}
    >
      {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;

  async function handleSave() {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const savedTools = await Promise.all(changedRows.map((row) => (
        updateAdminMCPTool(token, row.toolID, { priceNanousd: row.priceNanousd })
      )));
      const savedPriceByID = new Map(savedTools.map((tool) => [tool.id, tool.priceNanousd]));
      setRows((current) => current.map((row) => (
        savedPriceByID.has(row.toolID) ? { ...row, priceNanousd: savedPriceByID.get(row.toolID) ?? row.priceNanousd } : row
      )));
      setSavedPrices((current) => {
        const next = { ...current };
        for (const [toolID, priceNanousd] of savedPriceByID) {
          next[toolID] = priceNanousd;
        }
        return next;
      });
      setPriceDrafts((current) => {
        const next = { ...current };
        for (const [toolID, priceNanousd] of savedPriceByID) {
          next[toolID] = formatMCPToolPriceInput(priceNanousd);
        }
        return next;
      });
      toast.success(t("toast.mcpToolPricingSaved"));
    } catch (error) {
      toast.error(t("toast.mcpToolPricingSaveFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setSaving(false);
    }
  }

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
                          onChange={(event) => {
                            const nextDraft = event.target.value;
                            const nextNanousd = mcpToolPriceInputToNanousd(nextDraft);
                            setPriceDrafts((current) => ({
                              ...current,
                              [row.toolID]: nextDraft,
                            }));
                            if (nextNanousd === null) {
                              return;
                            }
                            setRows((current) => current.map((item) => (
                              item.toolID === row.toolID ? { ...item, priceNanousd: nextNanousd } : item
                            )));
                          }}
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
          pageSizeOptions={MCP_PRICING_PAGE_SIZES}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
          loading={loading}
        />
      </div>
    </SettingsSection>
  );
}
