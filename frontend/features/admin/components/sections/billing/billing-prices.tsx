"use client";

import * as React from "react";
import { ArrowDownToLine, ArrowUpFromLine, DatabaseSearch, DatabaseZap, Download, Pencil, RefreshCw, Upload } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { getAdminOpenRouterOfficialPricing, invalidateAdminReferenceDataCache, listAdminModelPricing, upsertAdminModelPricing } from "@/features/admin/api";
import type { AdminModelPricingDTO } from "@/features/admin/api/billing.types";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { PricingBillingDialog } from "@/features/admin/components/sections/billing/billing-dialogs";
import { PricingUnitCell } from "@/features/admin/components/sections/billing/billing-tables";
import {
  buildModelPricingExportObject,
  buildPricingRows,
  createFormState,
  createOptimisticModelPricing,
  DEFAULT_PAGE_SIZE,
  downloadJSONFile,
  formatDateTime,
  mergeModelPricingItem,
  normalizePricingMode,
  parseModelPricingImportJSON,
  parsePrice,
  shortListDescription,
  stringifyTieredPricing,
  type BillingModelPricingRow,
  type PricingFormState,
  type TieredPricingTierForm,
} from "@/features/admin/model/billing-settings";
import {
  applyOfficialPricingToForm,
  findOfficialPricingSuggestions,
  formatOfficialPricingValue,
  searchOfficialPricingCatalog,
  type OfficialModelPricingSuggestion,
  type OfficialPricingCatalogItem,
} from "@/features/admin/model/official-pricing";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { cn } from "@/lib/utils";
import { KNOWN_VENDOR_OPTIONS, resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";

type BillingPricesSectionProps = {
  models: AdminLLMModelDTO[];
  pricingItems: AdminModelPricingDTO[];
  setPricingItems: React.Dispatch<React.SetStateAction<AdminModelPricingDTO[]>>;
  loading: boolean;
};

const OFFICIAL_PRICING_SUGGESTION_LIMIT = 10;

function splitOfficialPricingID(id: string): { vendor: string; modelID: string } {
  const parts = id.split("/");
  if (parts.length <= 1) {
    return { vendor: "", modelID: id };
  }
  return { vendor: parts[0] ?? "", modelID: parts.slice(1).join("/") };
}

function officialPricingDisplayName(item: OfficialPricingCatalogItem): string {
  const rawName = (item.name || item.id).trim();
  const { modelID } = splitOfficialPricingID(item.id);
  let displayName = rawName;
  const colonIndex = displayName.indexOf(":");
  if (colonIndex > 0) {
    displayName = displayName.slice(colonIndex + 1).trim();
  }
  displayName = displayName.replace(/\s*\([^)]*\)\s*$/u, "").trim();
  return displayName || modelID || rawName;
}

function OfficialPricingPriceMetric({ icon, label, value }: { icon: React.ReactNode; label: string; value: number }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="inline-grid h-6 w-[5.5rem] grid-cols-[1rem_auto] items-center gap-x-2 rounded-md bg-muted/35 px-2 text-[11px] leading-none"
          aria-label={label}
        >
          <span className="inline-flex size-3.5 items-center justify-center text-muted-foreground">{icon}</span>
          <span className="text-right font-mono tabular-nums text-foreground">{`$${formatOfficialPricingValue(value)}`}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={6}>{label}</TooltipContent>
    </Tooltip>
  );
}

function parseOfficialPricingMultiplier(value: string): number {
  const parsed = Number(value.trim());
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function scaleOfficialPricingValue(value: number, multiplier: number): number {
  return Number((value * multiplier).toFixed(6));
}

function scaleOfficialPricingPayload(
  payload: OfficialModelPricingSuggestion["payload"],
  multiplier: number,
): OfficialModelPricingSuggestion["payload"] {
  return {
    ...payload,
    inputUSDPerMTokens: scaleOfficialPricingValue(payload.inputUSDPerMTokens, multiplier),
    outputUSDPerMTokens: scaleOfficialPricingValue(payload.outputUSDPerMTokens, multiplier),
    cacheReadUSDPerMTokens: scaleOfficialPricingValue(payload.cacheReadUSDPerMTokens, multiplier),
    cacheWriteUSDPerMTokens: scaleOfficialPricingValue(payload.cacheWriteUSDPerMTokens, multiplier),
  };
}

function modelPricingExportFilename(): string {
  const date = new Date().toISOString().slice(0, 10);
  return `deeix-chat-model-pricing-${date}.json`;
}

export function BillingPricesSection({ models, pricingItems, setPricingItems, loading }: BillingPricesSectionProps) {
  const locale = useLocale();
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const importPricingInputRef = React.useRef<HTMLInputElement | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [modelPricingRefreshing, setModelPricingRefreshing] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [freeFilter, setFreeFilter] = React.useState("");
  const [pricingModeFilter, setPricingModeFilter] = React.useState("");
  const [vendorFilter, setVendorFilter] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(DEFAULT_PAGE_SIZE);
  const [editRow, setEditRow] = React.useState<BillingModelPricingRow | null>(null);
  const [form, setForm] = React.useState<PricingFormState | null>(null);
  const [officialPricingSearch, setOfficialPricingSearch] = React.useState("");
  const [officialPricingMultiplier, setOfficialPricingMultiplier] = React.useState("1");
  const [officialPricingImportSuggestion, setOfficialPricingImportSuggestion] = React.useState<OfficialModelPricingSuggestion | null>(null);
  const [officialPricingCatalog, setOfficialPricingCatalog] = React.useState<OfficialPricingCatalogItem[]>([]);
  const [officialPricingCatalogStale, setOfficialPricingCatalogStale] = React.useState(true);
  const [officialPricingCatalogLoading, setOfficialPricingCatalogLoading] = React.useState(false);
  const [officialPricingCatalogHasError, setOfficialPricingCatalogHasError] = React.useState(false);
  const [officialPricingSingleDialogOpen, setOfficialPricingSingleDialogOpen] = React.useState(false);
  const [freeSwitchPendingModel, setFreeSwitchPendingModel] = React.useState("");
  const stableEditRow = useDialogSnapshot(editRow);
  const stableForm = useDialogSnapshot(form);
  const stableOfficialPricingImportSuggestion = useDialogSnapshot(officialPricingImportSuggestion);

  const rows = React.useMemo(() => buildPricingRows(models, pricingItems), [models, pricingItems]);
  const vendorFilterOptions = React.useMemo(() => {
    const options = new Map(KNOWN_VENDOR_OPTIONS.map((item) => [item.value, item.label]));
    for (const row of rows) {
      const value = row.vendor.trim();
      if (!value || options.has(value)) {
        continue;
      }
      const identity = resolveModelIdentity({
        code: row.platformModelName,
        vendor: value,
        icon: row.icon,
      });
      options.set(value, identity.vendorLabel);
    }
    return Array.from(options.entries()).map(([value, label]) => ({ value, label }));
  }, [rows]);
  const filteredRows = React.useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return rows.filter((row) => {
      const matchesQuery =
        !keyword ||
        row.platformModelName.toLowerCase().includes(keyword) ||
        row.vendor.toLowerCase().includes(keyword);
      const matchesStatus =
        statusFilter === "" ||
        (statusFilter === "configured" && row.pricing) ||
        (statusFilter === "unconfigured" && !row.pricing);
      const matchesFree =
        freeFilter === "" ||
        (freeFilter === "free" && row.isFree) ||
        (freeFilter === "not_free" && !row.isFree);
      const matchesPricingMode =
        pricingModeFilter === "" ||
        Boolean(row.pricing && normalizePricingMode(row.pricing.pricingMode) === pricingModeFilter);
      const matchesVendor = vendorFilter === "" || row.vendor.trim() === vendorFilter;
      return matchesQuery && matchesStatus && matchesFree && matchesPricingMode && matchesVendor;
    });
  }, [freeFilter, pricingModeFilter, query, rows, statusFilter, vendorFilter]);
  const editOfficialPricingSuggestions = React.useMemo(() => {
    if (!editRow) {
      return [];
    }
    return officialPricingSearch.trim()
      ? searchOfficialPricingCatalog(officialPricingSearch, editRow, officialPricingCatalog, OFFICIAL_PRICING_SUGGESTION_LIMIT)
      : findOfficialPricingSuggestions(editRow, officialPricingCatalog, OFFICIAL_PRICING_SUGGESTION_LIMIT);
  }, [editRow, officialPricingCatalog, officialPricingSearch]);
  const officialPricingMultiplierValue = React.useMemo(() => parseOfficialPricingMultiplier(officialPricingMultiplier), [officialPricingMultiplier]);
  const officialPricingMultiplierValid = React.useMemo(() => {
    const parsed = Number(officialPricingMultiplier.trim());
    return Number.isFinite(parsed) && parsed > 0;
  }, [officialPricingMultiplier]);

  React.useEffect(() => {
    setPage(1);
  }, [freeFilter, pricingModeFilter, query, statusFilter, vendorFilter]);

  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize));
  const pageRows = React.useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredRows.slice(start, start + pageSize);
  }, [filteredRows, page, pageSize]);
  const modelPricingVirtualRows = useVirtualTableRows(pageRows, {
    enabled: pageRows.length > 100,
    estimateSize: 40,
  });
  const modelPricingInitialLoading = loading && pageRows.length === 0;
  const showModelPricingRows = pageRows.length > 0;

  function openEdit(row: BillingModelPricingRow) {
    setEditRow(row);
    setForm(createFormState(row));
    setOfficialPricingSearch("");
  }

  async function loadModelPricing() {
    setModelPricingRefreshing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const items = await listAllAdminPages((options) => listAdminModelPricing(token, options));
      setPricingItems(items);
      invalidateAdminReferenceDataCache();
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setModelPricingRefreshing(false);
    }
  }

  async function refreshOfficialPricingCatalog(options: { quiet?: boolean; refresh?: boolean } = {}) {
    setOfficialPricingCatalogLoading(true);
    setOfficialPricingCatalogHasError(false);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const data = await getAdminOpenRouterOfficialPricing(token, { refresh: options.refresh });
      const catalog = data.items ?? [];
      if (catalog.length === 0) {
        throw new Error(t("toast.officialPricingRemoteEmpty"));
      }
      setOfficialPricingCatalog(catalog);
      setOfficialPricingCatalogStale(Boolean(data.stale));
      if (data.stale) {
        setOfficialPricingCatalogHasError(true);
        if (!options.quiet) {
          toast.error(t("toast.officialPricingRemoteFailed"));
        }
      }
      if (!options.quiet && !data.stale) {
        toast.success(t("toast.officialPricingRemoteLoaded", { count: catalog.length }));
      }
    } catch (error) {
      setOfficialPricingCatalogStale(true);
      setOfficialPricingCatalogHasError(true);
      if (!options.quiet) {
        toast.error(t("toast.officialPricingRemoteFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      setOfficialPricingCatalogLoading(false);
    }
  }

  function openOfficialPricingForEdit() {
    if (!editRow) {
      return;
    }
    setOfficialPricingSearch("");
    setOfficialPricingSingleDialogOpen(true);
    if ((officialPricingCatalog.length === 0 || officialPricingCatalogStale) && !officialPricingCatalogLoading) {
      void refreshOfficialPricingCatalog({ quiet: true });
    }
  }

  function openOfficialPricingImportDialog(suggestion: OfficialModelPricingSuggestion) {
    setOfficialPricingMultiplier("1");
    setOfficialPricingImportSuggestion(suggestion);
  }

  function confirmOfficialPricingImport() {
    if (!officialPricingImportSuggestion || !officialPricingMultiplierValid) {
      return;
    }
    const payload = scaleOfficialPricingPayload(officialPricingImportSuggestion.payload, officialPricingMultiplierValue);
    setForm((current) => current ? applyOfficialPricingToForm(current, payload) : current);
    setOfficialPricingImportSuggestion(null);
    setOfficialPricingSingleDialogOpen(false);
    setOfficialPricingSearch("");
    toast.success(t("toast.officialPricingApplied"));
  }

  function updateTieredTier(index: number, patch: Partial<TieredPricingTierForm>) {
    setForm((current) => {
      if (!current) return current;
      return {
        ...current,
        tieredTiers: current.tieredTiers.map((tier, tierIndex) =>
          tierIndex === index ? { ...tier, ...patch } : tier,
        ),
      };
    });
  }

  function addTieredTier() {
    setForm((current) => {
      if (!current) return current;
      return {
        ...current,
        tieredTiers: [
          ...current.tieredTiers,
          {
            id: `new-${Date.now()}-${current.tieredTiers.length}`,
            upToKTokens: "0",
            input: "0",
            cacheRead: "0",
            cacheWrite: "0",
            output: "0",
          },
        ],
      };
    });
  }

  function removeTieredTier(index: number) {
    setForm((current) => {
      if (!current || current.tieredTiers.length <= 1) return current;
      return {
        ...current,
        tieredTiers: current.tieredTiers.filter((_, tierIndex) => tierIndex !== index),
      };
    });
  }

  async function savePricing(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!form) return;
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const payload = {
        platformModelName: form.platformModelName,
        currency: "USD",
        pricingMode: form.pricingMode,
        inputUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.input) : 0,
        cacheReadUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.cacheRead) : 0,
        cacheWriteUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.cacheWrite) : 0,
        outputUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.output) : 0,
        callUSDPerCall: form.pricingMode === "call" ? parsePrice(form.call) : 0,
        durationUSDPerSecond: form.pricingMode === "duration" ? parsePrice(form.duration) : 0,
        tieredPricingJSON: form.pricingMode === "tiered" ? stringifyTieredPricing(form.tieredTiers) : undefined,
        isFree: form.isFree,
      };
      const data = await upsertAdminModelPricing(token, payload);
      setPricingItems((current) => mergeModelPricingItem(current, data.modelPricing));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.pricingSaved"));
      setEditRow(null);
      setForm(null);
    } catch (error) {
      toast.error(t("toast.pricingSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  function exportModelPricing() {
    const payload = buildModelPricingExportObject(pricingItems);
    const count = Object.keys(payload).length;
    if (count === 0) {
      toast.error(t("toast.exportEmpty"));
      return;
    }
    downloadJSONFile(modelPricingExportFilename(), payload);
    toast.success(t("toast.exported", { count }));
  }

  async function importModelPricingFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    event.target.value = "";
    if (!file) {
      return;
    }

    setSaving(true);
    try {
      const raw = await file.text();
      const validNames = new Set(rows.map((row) => row.platformModelName));
      const parsed = parseModelPricingImportJSON(raw, validNames, {
        invalidJSON: t("importErrors.invalidJSON"),
        rootObject: t("importErrors.rootObject"),
        emptyModelName: t("importErrors.emptyModelName"),
        duplicateModel: (model) => t("importErrors.duplicateModel", { model }),
        pricingObject: (model) => t("importErrors.pricingObject", { model }),
        invalidPricingMode: (model) => t("importErrors.invalidPricingMode", { model }),
        invalidNumber: (model, field) => t("importErrors.invalidNumber", { model, field }),
        invalidTieredPricing: (model, field) => t("importErrors.invalidTieredPricing", { model, field }),
        invalidTieredPricingJSON: (model) => t("importErrors.invalidTieredPricingJSON", { model }),
      });
      if (parsed.unknownModelNames.length > 0) {
        toast.error(t("toast.importUnknownModels"), {
          description: shortListDescription(parsed.unknownModelNames, "", t("toast.moreItems")),
        });
        return;
      }
      if (parsed.errors.length > 0) {
        toast.error(t("toast.importInvalidJSON"), {
          description: shortListDescription(parsed.errors, "", t("toast.moreItems")),
        });
        return;
      }
      if (parsed.items.length === 0) {
        toast.error(t("toast.importEmpty"));
        return;
      }

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const savedItems: AdminModelPricingDTO[] = [];
      for (const item of parsed.items) {
        const data = await upsertAdminModelPricing(token, item);
        savedItems.push(data.modelPricing);
      }
      setPricingItems((current) => savedItems.reduce((items, item) => mergeModelPricingItem(items, item), current));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.imported", { count: parsed.items.length }));
    } catch (error) {
      toast.error(t("toast.importFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function toggleModelFree(row: BillingModelPricingRow, checked: boolean) {
    if (freeSwitchPendingModel) {
      return;
    }
    const previousPricingItems = pricingItems;
    setFreeSwitchPendingModel(row.platformModelName);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const pricingMode = normalizePricingMode(row.pricing?.pricingMode);
      const payload = {
        platformModelName: row.platformModelName,
        currency: row.pricing?.currency || "USD",
        pricingMode,
        inputUSDPerMTokens: pricingMode === "token" ? row.pricing?.inputUSDPerMTokens ?? 0 : 0,
        cacheReadUSDPerMTokens: pricingMode === "token" ? row.pricing?.cacheReadUSDPerMTokens ?? 0 : 0,
        cacheWriteUSDPerMTokens: pricingMode === "token" ? row.pricing?.cacheWriteUSDPerMTokens ?? 0 : 0,
        outputUSDPerMTokens: pricingMode === "token" ? row.pricing?.outputUSDPerMTokens ?? 0 : 0,
        callUSDPerCall: pricingMode === "call" ? row.pricing?.callUSDPerCall ?? 0 : 0,
        durationUSDPerSecond: pricingMode === "duration" ? row.pricing?.durationUSDPerSecond ?? 0 : 0,
        tieredPricingJSON: pricingMode === "tiered" ? row.pricing?.tieredPricingJSON || stringifyTieredPricing(createFormState(row).tieredTiers) : undefined,
        isFree: checked,
      };
      setPricingItems((current) => mergeModelPricingItem(current, createOptimisticModelPricing(row, payload)));
      const data = await upsertAdminModelPricing(token, payload);
      setPricingItems((current) => mergeModelPricingItem(current, data.modelPricing));
      invalidateAdminReferenceDataCache();
      toast.success(checked ? t("toast.freeEnabled") : t("toast.freeDisabled"));
    } catch (error) {
      setPricingItems(previousPricingItems);
      toast.error(t("toast.freeSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setFreeSwitchPendingModel("");
    }
  }

  return (
    <section className="space-y-6 px-1">
      <div className="flex h-10 items-center">
        <h3 className="text-sm font-semibold">{t("modelPricing.title")}</h3>
      </div>
      <div className="space-y-3">
        <TableToolbar
          query={query}
          onQueryChange={setQuery}
          queryPlaceholder={t("modelPricing.searchPlaceholder")}
          filters={[
            {
              key: "status",
              label: t("modelPricing.filterLabel"),
              value: statusFilter,
              onValueChange: setStatusFilter,
              options: [
                { label: t("modelPricing.all"), value: "" },
                { label: t("modelPricing.configured"), value: "configured" },
                { label: t("modelPricing.unconfigured"), value: "unconfigured" },
              ],
            },
            {
              key: "free",
              label: t("modelPricing.freeFilterLabel"),
              value: freeFilter,
              onValueChange: setFreeFilter,
              options: [
                { label: t("modelPricing.allFreeStatus"), value: "" },
                { label: t("modelPricing.freeOnly"), value: "free" },
                { label: t("modelPricing.notFree"), value: "not_free" },
              ],
            },
            {
              key: "pricingMode",
              label: t("modelPricing.pricingMode"),
              value: pricingModeFilter,
              onValueChange: setPricingModeFilter,
              options: [
                { label: t("modelPricing.allPricingModes"), value: "" },
                { label: t("pricingModes.token"), value: "token" },
                { label: t("pricingModes.call"), value: "call" },
                { label: t("pricingModes.duration"), value: "duration" },
                { label: t("pricingModes.tiered"), value: "tiered" },
              ],
            },
            {
              key: "vendor",
              label: t("modelPricing.vendor"),
              value: vendorFilter,
              onValueChange: setVendorFilter,
              options: [
                { label: t("modelPricing.allVendors"), value: "" },
                ...vendorFilterOptions,
              ],
            },
          ]}
          loading={loading}
          onRefresh={() => void loadModelPricing()}
          refreshDisabled={loading || saving || modelPricingRefreshing}
          refreshLoading={modelPricingRefreshing}
        >
          <input
            ref={importPricingInputRef}
            type="file"
            accept="application/json,.json"
            className="hidden"
            onChange={(event) => void importModelPricingFile(event)}
          />
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            className="size-8 text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
            disabled={loading || saving}
            onClick={exportModelPricing}
            aria-label={t("actions.exportPricing")}
            title={t("actions.exportPricing")}
          >
            <Download className="size-3.5 stroke-1" />
          </Button>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            className="size-8 text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
            disabled={loading || saving}
            onClick={() => importPricingInputRef.current?.click()}
            aria-label={t("actions.importPricing")}
            title={t("actions.importPricing")}
          >
            <Upload className="size-3.5 stroke-1" />
          </Button>
        </TableToolbar>

        <Table
          viewportRef={modelPricingVirtualRows.viewportRef}
          viewportClassName={modelPricingVirtualRows.viewportClassName}
          viewportStyle={modelPricingVirtualRows.viewportStyle}
        >
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[210px]">{t("modelPricing.platformModel")}</TableHead>
              <TableHead>{t("modelPricing.free")}</TableHead>
              <TableHead>{t("modelPricing.pricingMode")}</TableHead>
              <TableHead className="min-w-[260px]">{t("modelPricing.basePrice")}</TableHead>
              <TableHead>{t("modelPricing.updatedAt")}</TableHead>
              <TableHead stickyEnd className="w-[56px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {modelPricingInitialLoading ? <TableLoadingRow colSpan={6} /> : null}
            {!loading && pageRows.length === 0 ? <TableEmptyRow colSpan={6}>{t("modelPricing.empty")}</TableEmptyRow> : null}
            {showModelPricingRows ? <VirtualTablePaddingRow colSpan={6} height={modelPricingVirtualRows.paddingTop} /> : null}
            {showModelPricingRows
              ? modelPricingVirtualRows.rows.map(({ item: row }) => {
                  const identity = resolveModelIdentity({
                    code: row.platformModelName,
                    vendor: row.vendor,
                    icon: row.icon,
                  });
                  const iconURL = resolveLobeHubIconURL(identity.modelIcon);

                  return (
                    <TableRow key={row.platformModelName}>
                      <TableCell className="py-1.5">
                        <div className="flex h-7 min-w-0 items-center gap-2">
                          <LobeHubIcon iconUrl={iconURL} label={row.platformModelName} />
                          <div className="flex min-w-0 flex-1">
                            <span className="truncate text-xs font-medium leading-5 text-foreground">
                              {row.platformModelName}
                            </span>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="py-1.5">
                        <div className="flex h-7 items-center">
                          <Switch
                            size="sm"
                            checked={row.isFree}
                            disabled={loading || saving || Boolean(freeSwitchPendingModel)}
                            onCheckedChange={(checked) => void toggleModelFree(row, checked)}
                            aria-label={`${row.platformModelName} ${t("modelPricing.freeModel")}`}
                          />
                        </div>
                      </TableCell>
                      <TableCell className="py-1.5">
                        {row.pricing ? t(`pricingModes.${normalizePricingMode(row.pricing.pricingMode)}`) : <span className="text-muted-foreground">-</span>}
                      </TableCell>
                      <TableCell className="py-1.5">
                        <PricingUnitCell pricing={row.pricing} />
                      </TableCell>
                      <TableCell className="py-1.5 text-muted-foreground">
                        {formatDateTime(row.pricing?.updatedAt ?? "", locale)}
                      </TableCell>
                      <TableCell stickyEnd className="w-[56px] py-1.5 text-right">
                        <div className="flex h-7 items-center justify-end">
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-xs"
                            className="h-7 w-7 text-muted-foreground shadow-none"
                            onClick={() => openEdit(row)}
                            aria-label={t("actions.editPricing")}
                          >
                            <Pencil className="size-3.5 stroke-1" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })
              : null}
            {showModelPricingRows ? <VirtualTablePaddingRow colSpan={6} height={modelPricingVirtualRows.paddingBottom} /> : null}
          </TableBody>
        </Table>

        <TablePagination
          total={filteredRows.length}
          page={page}
          pageCount={pageCount}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={(next) => {
            setPageSize(next);
            setPage(1);
          }}
          loading={loading}
        />
      </div>

      <PricingBillingDialog
        open={!!editRow && !!form}
        saving={saving}
        form={stableForm}
        setForm={setForm}
        onOpenChange={(open) => {
          if (!open && !saving) {
            setEditRow(null);
            setForm(null);
            setOfficialPricingSearch("");
            setOfficialPricingSingleDialogOpen(false);
          }
        }}
        onCancel={() => {
          setEditRow(null);
          setForm(null);
          setOfficialPricingSearch("");
          setOfficialPricingSingleDialogOpen(false);
        }}
        onSubmit={savePricing}
        onAddTier={addTieredTier}
        onRemoveTier={removeTieredTier}
        onUpdateTier={updateTieredTier}
        onOpenOfficialPricing={openOfficialPricingForEdit}
      />

      <Dialog
        open={officialPricingSingleDialogOpen}
        onOpenChange={(open) => {
          if (!open && !saving) {
            setOfficialPricingSingleDialogOpen(false);
            setOfficialPricingSearch("");
            setOfficialPricingImportSuggestion(null);
          }
        }}
      >
        <DialogContent className="flex max-h-[min(82vh,560px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[760px]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{t("modelPricing.officialPricingSingleTitle")}</DialogTitle>
            <DialogDescription>{t("modelPricing.officialPricingSingleDescription")}</DialogDescription>
          </DialogHeader>

          <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden px-4 py-2">
            {stableEditRow ? (
              <>
                <div className="shrink-0 space-y-2">
                  <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                    <Input
                      value={officialPricingSearch}
                      placeholder={t("modelPricing.officialPricingSearchPlaceholder")}
                      disabled={saving}
                      onChange={(event) => setOfficialPricingSearch(event.target.value)}
                    />
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      className="h-8 shrink-0 px-2.5 text-xs shadow-none"
                      disabled={officialPricingCatalogLoading || saving}
                      onClick={() => void refreshOfficialPricingCatalog({ refresh: true })}
                      aria-label={t("modelPricing.officialPricingSync")}
                      title={t("modelPricing.officialPricingSync")}
                    >
                      <RefreshCw className={cn("size-3.5 stroke-1", officialPricingCatalogLoading && "animate-spin")} />
                      {t("modelPricing.officialPricingSync")}
                    </Button>
                  </div>
                  {officialPricingCatalogHasError ? (
                    <p className="px-1 text-[11px] leading-5 text-muted-foreground">{t("modelPricing.officialPricingRemoteError")}</p>
                  ) : null}
                </div>
                <Table
                  shellClassName="min-h-0"
                  viewportClassName="max-h-[min(42vh,360px)] overflow-y-auto"
                >
                  <TableHeader className="sticky top-0 z-20">
                    <TableRow>
                      <TableHead>{t("modelPricing.modelInfo")}</TableHead>
                      <TableHead className="w-[240px]">
                        <span>{t("modelPricing.basePrice")}</span>
                        <span className="ml-1 font-mono text-[10px] text-muted-foreground">USD / 1M</span>
                      </TableHead>
                      <TableHead className="w-[84px] text-right">{t("modelPricing.similarity")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {officialPricingCatalogLoading && editOfficialPricingSuggestions.length === 0 ? (
                      <TableLoadingRow colSpan={3} />
                    ) : null}
                    {!officialPricingCatalogLoading && editOfficialPricingSuggestions.length === 0 ? (
                      <TableEmptyRow colSpan={3}>{t("modelPricing.officialPricingEmpty")}</TableEmptyRow>
                    ) : null}
                    {editOfficialPricingSuggestions.map((suggestion) => {
                      const { vendor, modelID } = splitOfficialPricingID(suggestion.item.id);
                      const identity = resolveModelIdentity({ code: suggestion.item.id, vendor });
                      const iconURL = resolveLobeHubIconURL(identity.modelIcon || identity.vendorIcon);
                      const displayName = officialPricingDisplayName(suggestion.item);
                      const fullName = suggestion.item.name || suggestion.item.id;

                      return (
                        <TableRow
                          key={suggestion.item.id}
                          role="button"
                          tabIndex={0}
                          aria-label={`${displayName} ${t("modelPricing.officialPricingImport")}`}
                          className="cursor-pointer transition-colors hover:bg-muted/70 focus-visible:bg-muted/70 focus-visible:outline-none"
                          onClick={() => openOfficialPricingImportDialog(suggestion)}
                          onKeyDown={(event) => {
                            if (event.key === "Enter" || event.key === " ") {
                              event.preventDefault();
                              openOfficialPricingImportDialog(suggestion);
                            }
                          }}
                        >
                          <TableCell className="w-[240px]">
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <div className="flex min-w-0 items-center gap-4">
                                  <LobeHubIcon
                                    iconUrl={iconURL}
                                    label={fullName}
                                    size={18}
                                  />
                                  <div className="flex min-w-0 flex-1 flex-col">
                                    <span className="truncate text-xs font-medium text-foreground">
                                      {displayName}
                                    </span>
                                    <span className="truncate font-mono text-[11px] text-muted-foreground">{modelID}</span>
                                  </div>
                                </div>
                              </TooltipTrigger>
                              <TooltipContent align="center" className="max-w-[240px]" side="right" sideOffset={8}>
                                <div className="flex min-w-0 flex-col gap-1">
                                  <span className="truncate text-xs font-medium">{fullName}</span>
                                  <span className="truncate font-mono text-[11px] text-muted-foreground">{suggestion.item.id}</span>
                                </div>
                              </TooltipContent>
                            </Tooltip>
                          </TableCell>
                          <TableCell className="w-[240px] py-1.5">
                            <div className="grid grid-cols-[5.5rem_5.5rem] gap-x-4 gap-y-1 leading-5">
                              <OfficialPricingPriceMetric
                                icon={<ArrowUpFromLine className="size-3" strokeWidth={1.4} />}
                                label={t("modelPricing.priceInput")}
                                value={suggestion.payload.inputUSDPerMTokens}
                              />
                              <OfficialPricingPriceMetric
                                icon={<ArrowDownToLine className="size-3" strokeWidth={1.4} />}
                                label={t("modelPricing.priceOutput")}
                                value={suggestion.payload.outputUSDPerMTokens}
                              />
                              <OfficialPricingPriceMetric
                                icon={<DatabaseSearch className="size-3" strokeWidth={1.4} />}
                                label={t("modelPricing.priceCacheRead")}
                                value={suggestion.payload.cacheReadUSDPerMTokens}
                              />
                              <OfficialPricingPriceMetric
                                icon={<DatabaseZap className="size-3" strokeWidth={1.4} />}
                                label={t("modelPricing.priceCacheWrite")}
                                value={suggestion.payload.cacheWriteUSDPerMTokens}
                              />
                            </div>
                          </TableCell>
                          <TableCell className="w-[84px] py-1.5 text-right">
                            <span className="font-mono text-[11px] text-muted-foreground">{suggestion.score}%</span>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </>
            ) : null}
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!officialPricingImportSuggestion}
        onOpenChange={(open) => {
          if (!open) {
            setOfficialPricingImportSuggestion(null);
          }
        }}
      >
        <DialogContent className="flex flex-col gap-0 overflow-hidden p-0 sm:max-w-[360px]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{t("modelPricing.officialPricingImportTitle")}</DialogTitle>
            <DialogDescription>{t("modelPricing.officialPricingImportDescription")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-1 px-4 py-2">
            <p className="text-xs text-muted-foreground">{t("modelPricing.officialPricingMultiplier")}</p>
            <div className="relative">
              <Input
                value={officialPricingMultiplier}
                autoFocus
                inputMode="decimal"
                className="h-7 pr-7 text-left font-mono"
                placeholder="1"
                aria-invalid={!officialPricingMultiplierValid}
                onChange={(event) => setOfficialPricingMultiplier(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    confirmOfficialPricingImport();
                  }
                }}
              />
              <span className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-[11px] text-muted-foreground">X</span>
            </div>
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button
              type="button"
              variant="ghost"
              onClick={() => setOfficialPricingImportSuggestion(null)}
            >
              {tActions("cancel")}
            </Button>
            <Button
              type="button"
              disabled={!officialPricingMultiplierValid || !stableOfficialPricingImportSuggestion}
              onClick={confirmOfficialPricingImport}
            >
              {t("modelPricing.officialPricingImport")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
