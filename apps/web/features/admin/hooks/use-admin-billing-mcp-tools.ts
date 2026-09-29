"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminMCPServers, listAdminMCPServerTools, updateAdminMCPTool } from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type MCPToolPricingRow = {
  toolID: number;
  serverID: number;
  serverName: string;
  toolLabel: string;
  toolName: string;
  toolDescription: string;
  priceNanousd: number;
};

export type MCPServerOption = {
  id: number;
  name: string;
};

export function formatMCPToolPriceInput(priceNanousd: number): string {
  if (!Number.isFinite(priceNanousd) || priceNanousd <= 0) {
    return "0";
  }
  return String(priceNanousd / 1_000_000_000);
}

export function mcpToolPriceInputToNanousd(value: string): number | null {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return Math.round(parsed * 1_000_000_000);
}

function mcpToolPriceDraftsFrom(rows: MCPToolPricingRow[]): Record<number, string> {
  return Object.fromEntries(rows.map((row) => [row.toolID, formatMCPToolPriceInput(row.priceNanousd)]));
}

type UseAdminBillingMcpToolsOptions = {
  /** Called after every successful reload; the section clears its row selection here. */
  onLoaded: () => void;
};

export function useAdminBillingMcpTools({ onLoaded }: UseAdminBillingMcpToolsOptions) {
  const t = useTranslations("adminBilling");
  const [rows, setRows] = React.useState<MCPToolPricingRow[]>([]);
  const [servers, setServers] = React.useState<MCPServerOption[]>([]);
  const [savedPrices, setSavedPrices] = React.useState<Record<number, number>>({});
  const [priceDrafts, setPriceDrafts] = React.useState<Record<number, string>>({});
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  // A ref keeps `load` stable so the mount effect does not re-run when the
  // caller passes a fresh callback on every render.
  const onLoadedRef = React.useRef(onLoaded);

  React.useEffect(() => {
    onLoadedRef.current = onLoaded;
  }, [onLoaded]);

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
      onLoadedRef.current();
    } catch (error) {
      toast.error(t("toast.mcpToolsLoadFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const updatePriceDraft = React.useCallback((toolID: number, nextDraft: string) => {
    const nextNanousd = mcpToolPriceInputToNanousd(nextDraft);
    setPriceDrafts((current) => ({
      ...current,
      [toolID]: nextDraft,
    }));
    if (nextNanousd === null) {
      return;
    }
    setRows((current) => current.map((item) => (
      item.toolID === toolID ? { ...item, priceNanousd: nextNanousd } : item
    )));
  }, []);

  // Bulk edits only update the local draft, like single-row edits; the Save action persists everything.
  const applyPriceToTools = React.useCallback((toolIDs: ReadonlySet<number>, priceNanousd: number) => {
    setRows((current) => current.map((row) => (
      toolIDs.has(row.toolID) ? { ...row, priceNanousd } : row
    )));
    setPriceDrafts((current) => {
      const next = { ...current };
      for (const toolID of toolIDs) {
        next[toolID] = formatMCPToolPriceInput(priceNanousd);
      }
      return next;
    });
  }, []);

  const changedRows = React.useMemo(
    () => rows.filter((row) => row.priceNanousd !== (savedPrices[row.toolID] ?? 0)),
    [rows, savedPrices],
  );

  async function save() {
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

  return {
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
  };
}
