"use client";

import { Activity, Check, CircleHelp, CircleOff, MoreHorizontal, Plus, RefreshCw, ShieldAlert, SlidersHorizontal, X } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
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
import { TablePagination } from "@/components/ui/table-tools";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import type {
  AdminLLMAdapter,
  AdminLLMModelDTO,
  AdminLLMModelProbeResult,
  AdminLLMModelUpstreamSourceDTO,
} from "@/features/admin/api/llm-types";
import { useAdminModelsProbe } from "@/features/admin/hooks/use-admin-models-probe";
import { useAdminModelsSourceBindOptions } from "@/features/admin/hooks/use-admin-models-source-bind-options";
import { type ModelSourceRouteNumberField, useAdminModelsSources } from "@/features/admin/hooks/use-admin-models-sources";
import { isAdminLLMAdapter, isAdminLLMStatus } from "@/features/admin/model/admin-unions";
import {
  DEFAULT_MODEL_SOURCE_BIND_DRAFT,
  type ModelSourceBindDraft,
} from "@/features/admin/model/models-source-binding";
import {
  ADAPTER_LABELS,
  formatDateTime,
  resolveValue,
} from "@/features/admin/types/llm";
import { PROTOCOL_OPTIONS, sortProtocolsForDisplay } from "@/features/admin/utils/llm-display";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { ModelProbeDialog } from "../../shared/model-probe-dialog";
import {
  ModelSourceCircuitDialog,
  type ModelSourceCircuitPayload,
} from "./models-source-circuit-dialog";

type UpstreamSourcesSheetProps = {
  model: AdminLLMModelDTO | null;
  circuitBreakerEnabled: boolean;
  onClose: () => void;
  onRefreshModel: () => void;
  onSourceAvailabilityChange?: (modelID: number, previousAvailable: boolean, nextAvailable: boolean) => void;
};

type RouteNumberDraftField = ModelSourceRouteNumberField;

function formatCircuitUntil(until: string, locale: string): string {
  const raw = until.trim();
  if (!raw) {
    return "-";
  }
  const timestamp = Number(raw);
  const date = Number.isFinite(timestamp) && timestamp > 0 ? new Date(timestamp * 1000) : new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function SourceCircuitStatus({
  circuitUntil,
  circuitScope,
}: {
  circuitUntil: string;
  circuitScope: "upstream" | "source" | "";
}) {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <ShieldAlert
          className="size-4 text-destructive"
          aria-label={t("status.circuitOpen")}
        />
      </TooltipTrigger>
      <TooltipContent side="top" className="text-xs">
        <div className="space-y-1">
          <div>{circuitScope === "upstream" ? t("sources.circuitScopeUpstream") : t("sources.circuitScopeSource")}</div>
          <div>{t("sources.circuitUntil", { time: formatCircuitUntil(circuitUntil, locale) })}</div>
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

function SourceInactiveStatus({
  reason,
}: {
  reason: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <X
          className="size-4 text-muted-foreground"
          aria-label={reason}
        />
      </TooltipTrigger>
      <TooltipContent side="top" className="text-xs">
        {reason}
      </TooltipContent>
    </Tooltip>
  );
}

export function UpstreamSourcesSheet({
  model,
  circuitBreakerEnabled,
  onClose,
  onRefreshModel,
  onSourceAvailabilityChange,
}: UpstreamSourcesSheetProps) {
  const t = useTranslations("adminModels.sources");
  const probeT = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const locale = useLocale();
  const [circuitSource, setCircuitSource] = React.useState<AdminLLMModelUpstreamSourceDTO | null>(null);
  const [bindOpen, setBindOpen] = React.useState(false);
  const [bindForm, setBindForm] = React.useState<ModelSourceBindDraft>(DEFAULT_MODEL_SOURCE_BIND_DRAFT);
  const {
    sources,
    loading,
    total,
    page,
    pageSize,
    actionSourceID,
    routeDrafts,
    bindPending,
    loadSources,
    setRouteDraft,
    handleRouteValueCommit,
    handleProtocolChange,
    handleToggleStatus,
    handleCircuitAction,
    saveCircuitSettings,
    removeSource,
    submitBind,
  } = useAdminModelsSources({ model, circuitBreakerEnabled, onRefreshModel, onSourceAvailabilityChange });
  const handleProbeRouteDeleted = React.useCallback((result: AdminLLMModelProbeResult) => {
    removeSource(result.routeID);
    onRefreshModel();
  }, [onRefreshModel, removeSource]);
  const {
    probeOpen,
    setProbeOpen,
    probeLoading,
    probeTargetName,
    probeResults,
    testSource: handleTestSource,
    deleteProbeRoute: handleDeleteProbeRoute,
    clearProbeResults,
  } = useAdminModelsProbe({ onRouteDeleted: handleProbeRouteDeleted });
  const {
    upstreams,
    upstreamsLoading,
    upstreamModels,
    upstreamModelsLoading,
    clearUpstreamModels,
  } = useAdminModelsSourceBindOptions({ bindOpen, upstreamID: bindForm.upstreamID });
  const stableModel = useDialogSnapshot(model);

  // Data resets live in useAdminModelsSources; this only resets the sheet's own UI.
  React.useEffect(() => {
    if (model) {
      clearProbeResults();
      setBindOpen(false);
      setBindForm(DEFAULT_MODEL_SOURCE_BIND_DRAFT);
      clearUpstreamModels();
      setCircuitSource(null);
      return;
    }

    setBindOpen(false);
    setCircuitSource(null);
  }, [clearProbeResults, clearUpstreamModels, model]);

  function setBindField<K extends keyof ModelSourceBindDraft>(key: K, value: ModelSourceBindDraft[K]) {
    setBindForm((current) => ({ ...current, [key]: value }));
  }

  function handleBindUpstreamChange(upstreamID: string) {
    setBindForm({
      ...DEFAULT_MODEL_SOURCE_BIND_DRAFT,
      upstreamID,
    });
  }

  function handleBindUpstreamModelChange(upstreamModelID: string) {
    const selected = upstreamModels.find((item) => String(item.id) === upstreamModelID);
    setBindForm((current) => ({
      ...current,
      upstreamModelID,
      protocol: selected?.suggestedProtocol ?? "",
    }));
  }

  const handleRouteInputKeyDown = React.useCallback(
    (
      event: React.KeyboardEvent<HTMLInputElement>,
      source: AdminLLMModelUpstreamSourceDTO,
      field: RouteNumberDraftField,
    ) => {
      if (event.key === "Enter") {
        event.preventDefault();
        event.currentTarget.blur();
      }
      if (event.key === "Escape") {
        setRouteDraft(source.id, field, String(source[field]));
        event.currentTarget.blur();
      }
    },
    [setRouteDraft],
  );

  const openCircuitSettings = React.useCallback((source: AdminLLMModelUpstreamSourceDTO) => {
    setCircuitSource(source);
  }, []);

  const handleSaveCircuitSettings = React.useCallback(async (payload: ModelSourceCircuitPayload) => {
    if (!circuitSource) return;
    if (await saveCircuitSettings(circuitSource, payload)) {
      setCircuitSource(null);
    }
  }, [circuitSource, saveCircuitSettings]);

  const selectedUpstreamModel = upstreamModels.find((item) => String(item.id) === bindForm.upstreamModelID);
  const protocolOptions = React.useMemo(() => {
    const values = new Set<AdminLLMAdapter>(PROTOCOL_OPTIONS.map((item) => item.value));
    if (selectedUpstreamModel?.suggestedProtocol) {
      values.add(selectedUpstreamModel.suggestedProtocol);
    }
    if (selectedUpstreamModel?.protocol) {
      values.add(selectedUpstreamModel.protocol);
    }
    return sortProtocolsForDisplay(Array.from(values));
  }, [selectedUpstreamModel?.protocol, selectedUpstreamModel?.suggestedProtocol]);

  const handleBindSubmit = React.useCallback(async () => {
    await submitBind(bindForm, () => {
      setBindForm(DEFAULT_MODEL_SOURCE_BIND_DRAFT);
      setBindOpen(false);
    });
  }, [bindForm, submitBind]);

  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const virtualRows = useVirtualTableRows(sources, {
    enabled: sources.length > 100,
    estimateSize: 40,
  });
  const initialLoading = loading && sources.length === 0;
  const showRows = sources.length > 0;

  return (
    <>
      <Sheet open={!!model} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="flex flex-col sm:max-w-[720px]" showCloseButton={false}>
          <SheetHeader className="px-4 pb-4">
            <div className="flex items-center justify-between gap-3">
              <SheetTitle>{t("title")}</SheetTitle>
              <Button
                type="button"
                size="sm"
                variant={bindOpen ? "secondary" : "outline"}
                onClick={() => setBindOpen((current) => !current)}
              >
                <Plus className="size-3.5 stroke-1" />
                {t("bindSource")}
              </Button>
            </div>
          </SheetHeader>

          <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-4">
            <Table
              className="min-w-[760px]"
              viewportRef={virtualRows.viewportRef}
              viewportClassName={virtualRows.viewportClassName}
              viewportStyle={virtualRows.viewportStyle}
            >
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("upstream")}</TableHead>
                  <TableHead>{t("upstreamModel")}</TableHead>
                  <TableHead>{t("protocol")}</TableHead>
                  <TableHead className="w-[150px] text-center">
                    <div className="flex items-center justify-center gap-1">
                      <span>{t("priorityWeight")}</span>
                      <Popover>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <PopoverTrigger asChild>
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon-xs"
                                className="text-muted-foreground hover:bg-transparent hover:text-foreground"
                                aria-label={`${t("priorityDesc")} ${t("weightDesc")}`}
                              >
                                <CircleHelp className="size-3" />
                              </Button>
                            </PopoverTrigger>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="max-w-xs text-xs">
                            <div className="space-y-1">
                              <p>{t("priorityDesc")}</p>
                              <p>{t("weightDesc")}</p>
                            </div>
                          </TooltipContent>
                        </Tooltip>
                        <PopoverContent className="w-72 max-w-[calc(100vw-2rem)] p-3 text-xs leading-5">
                          <div className="space-y-1">
                            <p>{t("priorityDesc")}</p>
                            <p>{t("weightDesc")}</p>
                          </div>
                        </PopoverContent>
                      </Popover>
                    </div>
                  </TableHead>
                  <TableHead className="w-[72px] text-center">{t("status")}</TableHead>
                  <TableHead className="w-[140px]">{t("updatedAt")}</TableHead>
                  <TableHead className="w-[56px]" stickyEnd />
                </TableRow>
              </TableHeader>
              <TableBody>
                {bindOpen ? (
                  <TableRow interactive={false} tone="muted">
                    <TableCell className="py-1.5">
                      <Select
                        value={bindForm.upstreamID}
                        onValueChange={handleBindUpstreamChange}
                        disabled={bindPending || upstreamsLoading}
                      >
                        <SelectTrigger className="h-7 min-w-[140px] bg-background text-xs">
                          <SelectValue placeholder={upstreamsLoading ? t("loadingUpstreams") : t("selectUpstream")} />
                        </SelectTrigger>
                        <SelectContent>
                          {upstreams.map((item) => (
                            <SelectItem key={item.id} value={String(item.id)}>
                              {item.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell className="py-1.5">
                      <Select
                        value={bindForm.upstreamModelID}
                        onValueChange={handleBindUpstreamModelChange}
                        disabled={bindPending || !bindForm.upstreamID || upstreamModelsLoading}
                      >
                        <SelectTrigger className="h-7 min-w-[180px] bg-background font-mono text-xs">
                          <SelectValue placeholder={upstreamModelsLoading ? t("loadingUpstreamModels") : t("selectUpstreamModel")} />
                        </SelectTrigger>
                        <SelectContent>
                          {upstreamModels.map((item) => (
                            <SelectItem key={item.id} value={String(item.id)}>
                              {item.upstreamModelName}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell className="py-1.5">
                      <Select
                        value={bindForm.protocol}
                        onValueChange={(value) => {
                          if (isAdminLLMAdapter(value)) setBindField("protocol", value);
                        }}
                        disabled={bindPending || !bindForm.upstreamModelID}
                      >
                        <SelectTrigger className="h-7 min-w-[180px] bg-background text-xs">
                          <SelectValue placeholder={t("selectProtocol")} />
                        </SelectTrigger>
                        <SelectContent>
                          {protocolOptions.map((protocol) => (
                            <SelectItem key={protocol} value={protocol}>
                              {ADAPTER_LABELS[protocol] ?? protocol}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell className="whitespace-nowrap py-1.5">
                      <div className="flex h-7 items-center justify-center gap-1">
                        <Input
                          value={bindForm.priority}
                          inputMode="numeric"
                          disabled={bindPending}
                          onChange={(event) => setBindField("priority", event.target.value)}
                          className="h-7 w-[58px] bg-background px-2 text-center font-mono text-xs tabular-nums"
                          aria-label={t("priority")}
                        />
                        <span className="text-xs text-muted-foreground">/</span>
                        <Input
                          value={bindForm.weight}
                          inputMode="numeric"
                          disabled={bindPending}
                          onChange={(event) => setBindField("weight", event.target.value)}
                          className="h-7 w-[58px] bg-background px-2 text-center font-mono text-xs tabular-nums"
                          aria-label={t("weight")}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="w-[72px] py-1.5">
                      <Select
                        value={bindForm.status}
                        onValueChange={(value) => {
                          if (isAdminLLMStatus(value)) setBindField("status", value);
                        }}
                        disabled={bindPending}
                      >
                        <SelectTrigger className="h-7 w-[72px] bg-background px-2 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="active">{probeT("status.active")}</SelectItem>
                          <SelectItem value="inactive">{probeT("status.inactive")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell className="py-1.5 text-muted-foreground">-</TableCell>
                    <TableCell className="w-[56px] whitespace-nowrap py-1.5" stickyEnd>
                      <div className="flex h-7 items-center justify-end gap-1">
                        <Button
                          type="button"
                          size="icon-sm"
                          variant="ghost"
                          className="text-muted-foreground shadow-none"
                          disabled={bindPending}
                          onClick={() => {
                            setBindOpen(false);
                            setBindForm(DEFAULT_MODEL_SOURCE_BIND_DRAFT);
                          }}
                          aria-label={commonT("actions.cancel")}
                        >
                          <X className="size-3.5 stroke-1" />
                        </Button>
                        <Button
                          type="button"
                          size="icon-sm"
                          disabled={bindPending}
                          onClick={() => void handleBindSubmit()}
                          aria-label={t("bindConfirm")}
                        >
                          {bindPending ? <Spinner className="size-3.5" /> : <Check className="size-3.5 stroke-1" />}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : null}
                {initialLoading ? <TableLoadingRow colSpan={7} /> : null}
                {showRows ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingTop} /> : null}
                {showRows
                  ? virtualRows.rows.map(({ item: source }) => {
                      const actionPending = actionSourceID === source.id;

                      return (
                        <TableRow key={source.id}>
                          <TableCell className="py-1.5">
                            <div className="whitespace-nowrap">
                              <span className="font-medium">{resolveValue(source.upstreamName)}</span>
                            </div>
                          </TableCell>
                          <TableCell className="py-1.5 font-mono text-xs">
                            {resolveValue(source.upstreamModelName)}
                          </TableCell>
                          <TableCell className="whitespace-nowrap py-1.5">
                            <Select
                              value={routeDrafts[source.id]?.protocol || source.protocol}
                              onValueChange={(value) => {
                                if (isAdminLLMAdapter(value)) void handleProtocolChange(source, value);
                              }}
                              disabled={actionPending}
                            >
                              <SelectTrigger className="h-7 min-w-[180px] bg-background text-xs">
                                <SelectValue placeholder={t("selectProtocol")} />
                              </SelectTrigger>
                              <SelectContent>
                                {PROTOCOL_OPTIONS.map(({ value: protocol }) => (
                                  <SelectItem key={protocol} value={protocol}>
                                    {ADAPTER_LABELS[protocol] ?? protocol}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </TableCell>
                          <TableCell className="whitespace-nowrap py-1.5">
                            <div className="flex h-7 items-center justify-center gap-1">
                              <Input
                                type="text"
                                inputMode="numeric"
                                value={routeDrafts[source.id]?.priority ?? String(source.priority)}
                                disabled={actionPending}
                                onChange={(event) => setRouteDraft(source.id, "priority", event.target.value)}
                                onBlur={() => void handleRouteValueCommit(source, "priority")}
                                onKeyDown={(event) => handleRouteInputKeyDown(event, source, "priority")}
                                aria-label={t("priorityAria", { name: source.upstreamModelName })}
                                className="h-7 w-[58px] px-2 text-center font-mono tabular-nums"
                              />
                              <span className="text-xs text-muted-foreground">/</span>
                              <Input
                                type="text"
                                inputMode="numeric"
                                value={routeDrafts[source.id]?.weight ?? String(source.weight)}
                                disabled={actionPending}
                                onChange={(event) => setRouteDraft(source.id, "weight", event.target.value)}
                                onBlur={() => void handleRouteValueCommit(source, "weight")}
                                onKeyDown={(event) => handleRouteInputKeyDown(event, source, "weight")}
                                aria-label={t("weightAria", { name: source.upstreamModelName })}
                                className="h-7 w-[58px] px-2 text-center font-mono tabular-nums"
                              />
                            </div>
                          </TableCell>
                          <TableCell className="w-[72px] whitespace-nowrap py-1.5">
                            <div className="flex h-7 items-center justify-center">
                              {circuitBreakerEnabled && source.circuitOpen ? (
                                <SourceCircuitStatus
                                  circuitUntil={source.circuitUntil}
                                  circuitScope={source.circuitScope}
                                />
                              ) : model?.status === "inactive" || source.upstreamStatus === "inactive" || source.upstreamModelStatus === "inactive" ? (
                                <SourceInactiveStatus
                                  reason={
                                    model?.status === "inactive"
                                      ? t("platformModelInactive")
                                      : source.upstreamStatus === "inactive"
                                      ? t("upstreamInactive")
                                      : t("upstreamModelInactive")
                                  }
                                />
                              ) : (
                                <Switch
                                  size="sm"
                                  checked={source.status === "active"}
                                  disabled={actionPending}
                                  onCheckedChange={(checked) => void handleToggleStatus(source, checked ? "active" : "inactive")}
                                  aria-label={t("sourceStatusAria", { name: source.upstreamModelName })}
                                />
                              )}
                            </div>
                          </TableCell>
                          <TableCell className="whitespace-nowrap py-1.5 text-muted-foreground">
                            {formatDateTime(source.updatedAt, locale)}
                          </TableCell>
                          <TableCell className="w-[56px] whitespace-nowrap py-1.5" stickyEnd>
                            <div className="flex h-7 items-center justify-end">
                              <DropdownMenu modal={false}>
                                <DropdownMenuTrigger asChild>
                                  <Button
                                    type="button"
                                    size="icon-sm"
                                    variant="ghost"
                                    className="text-muted-foreground shadow-none"
                                    aria-label={t("sourceActions")}
                                    disabled={actionPending}
                                  >
                                    {actionPending ? (
                                      <Spinner className="size-3.5" />
                                    ) : (
                                      <MoreHorizontal className="size-3.5 stroke-1" />
                                    )}
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                  <DropdownMenuItem onSelect={() => void handleTestSource(source)}>
                                    <Activity className="size-3.5 stroke-1" />
                                    {probeT("actions.test")}
                                  </DropdownMenuItem>
                                  <DropdownMenuItem onSelect={() => openCircuitSettings(source)}>
                                    <SlidersHorizontal className="size-3.5 stroke-1" />
                                    {t("circuitSettings")}
                                  </DropdownMenuItem>
                                  {circuitBreakerEnabled ? (
                                    source.circuitOpen ? (
                                      <DropdownMenuItem onSelect={() => void handleCircuitAction(source, "reset")}>
                                        <RefreshCw className="size-3.5 stroke-1" />
                                        {t("resetCircuit")}
                                      </DropdownMenuItem>
                                    ) : (
                                      <DropdownMenuItem onSelect={() => void handleCircuitAction(source, "open")}>
                                        <CircleOff className="size-3.5 stroke-1" />
                                        {t("openCircuit")}
                                      </DropdownMenuItem>
                                    )
                                  ) : null}
                                </DropdownMenuContent>
                              </DropdownMenu>
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                  })
                  : null}
                {showRows ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingBottom} /> : null}

                {!loading && sources.length === 0 ? (
                  <TableEmptyRow colSpan={7}>{t("empty")}</TableEmptyRow>
                ) : null}
              </TableBody>
            </Table>

            <div className="mt-4">
              <TablePagination
                total={total}
                page={page}
                pageCount={pageCount}
                pageSize={pageSize}
                onPageChange={(nextPage) => {
                  if (model) {
                    void loadSources(model.id, nextPage, pageSize);
                  }
                }}
                onPageSizeChange={(nextPageSize) => {
                  if (model) {
                    void loadSources(model.id, 1, nextPageSize);
                  }
                }}
                loading={loading}
              />
            </div>
          </div>

          <SheetFooter className="flex flex-row justify-end gap-2 px-4 py-3">
            <Button type="button" variant="ghost" onClick={onClose}>
              {commonT("actions.close")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ModelProbeDialog
        open={probeOpen}
        loading={probeLoading}
        targetName={probeTargetName}
        result={null}
        results={probeResults}
        onDeleteRoute={handleDeleteProbeRoute}
        onOpenChange={(open) => {
          if (!open && !probeLoading) {
            setProbeOpen(false);
          }
        }}
      />
      <ModelSourceCircuitDialog
        source={circuitSource}
        policyMode={stableModel?.cbPolicyMode}
        pending={actionSourceID === circuitSource?.id}
        onClose={() => setCircuitSource(null)}
        onSave={handleSaveCircuitSettings}
      />
    </>
  );
}
