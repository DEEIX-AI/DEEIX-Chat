import * as React from "react";
import { toast } from "sonner";
import { Activity, Cable, Check, ChevronDownIcon, CloudDownload, Plus, Tags, ToggleLeft, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableRow,
  TableSkeletonRows,
} from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  batchDeleteAdminLLMUpstreamModels,
  deleteAdminLLMUpstreamModel,
  importAdminLLMUpstreamModels,
  listAdminLLMRemoteModels,
  listAdminLLMUpstreamModels,
  testAdminLLMUpstreamModelRoute,
  upsertAdminLLMUpstreamModel,
} from "@/features/admin/api";
import { cn } from "@/lib/utils";
import type {
  AdminLLMAdapter,
  AdminLLMModelProbeResult,
  AdminLLMRemoteModelItem,
  AdminLLMUpstreamView,
  UpsertAdminLLMUpstreamModelRequest,
} from "@/features/admin/api/llm.types";
import { ModelProbeDialog } from "@/features/admin/components/sections/models/model-probe-dialog";
import {
  PROTOCOL_OPTIONS,
  resolveKindsDisplayForProtocols,
  resolveNextRouteProtocolSelection,
  sortProtocolsForDisplay,
} from "@/features/admin/utils/llm-display";
import { MODEL_KIND_OPTIONS, PAGE_SIZE_DEFAULT } from "@/features/admin/types/llm";
import {
  buildRowDrafts,
  createDraftPlatformModelNameMap,
  DEFAULT_NEW_BINDING,
  displayToKindsJson,
  summarizeBatchDeleteResult,
  summarizeImportResult,
  validateRowDrafts,
  type NewBindingFormState,
  type RowDraft,
} from "@/features/admin/model/upstream-models";

function KindsDropdown({
  value,
  onChange,
  disabled,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  const t = useTranslations("adminChannels");
  const selectedKinds = React.useMemo(
    () => value.split(",").map((item) => item.trim()).filter(Boolean),
    [value],
  );
  const selectedKindLabel = React.useMemo(
    () =>
      selectedKinds
        .map((kind) => t(`kinds.${kind}`))
        .join(", "),
    [selectedKinds, t],
  );

  function toggle(kind: string) {
    const next = new Set(selectedKinds);
    if (next.has(kind)) next.delete(kind);
    else next.add(kind);
    if (next.size === 0) next.add("chat");
    onChange(Array.from(next).join(","));
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          role="combobox"
          disabled={disabled}
          className={cn(
            "h-7 w-full justify-between gap-2 border-input/40 bg-transparent px-2 py-0 text-[11px] font-normal text-muted-foreground shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-2",
            className,
          )}
        >
          <span className={cn("min-w-0 flex-1 truncate text-left", selectedKindLabel ? "text-foreground/75" : "")}>
            {selectedKindLabel || t("modelsDialog.selectKind")}
          </span>
          <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-48 p-1">
        {MODEL_KIND_OPTIONS.map(({ value: kind }) => (
          <button
            key={kind}
            type="button"
            onClick={() => toggle(kind)}
            className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
          >
            <span className="min-w-0 flex-1 truncate text-left">{t(`kinds.${kind}`)}</span>
            <Check
              className={cn(
                "absolute right-2 size-4 shrink-0 text-muted-foreground",
                selectedKinds.includes(kind) ? "opacity-100" : "opacity-0",
              )}
            />
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}

function ProtocolsDropdown({
  value,
  onChange,
  disabled,
  className,
}: {
  value: AdminLLMAdapter[];
  onChange: (value: AdminLLMAdapter[]) => void;
  disabled?: boolean;
  className?: string;
}) {
  const t = useTranslations("adminChannels");
  const selected = React.useMemo(() => new Set(value), [value]);
  const selectedLabel = React.useMemo(
    () =>
      PROTOCOL_OPTIONS
        .filter((item) => selected.has(item.value))
        .map((item) => item.label)
        .join(", "),
    [selected],
  );

  function toggle(protocol: AdminLLMAdapter) {
    onChange(resolveNextRouteProtocolSelection(value, protocol));
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          role="combobox"
          disabled={disabled}
          className={cn(
            "h-7 w-full justify-between gap-2 border-input/40 bg-transparent px-2 py-0 text-[11px] font-normal text-muted-foreground shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-2",
            className,
          )}
        >
          <span className={cn("min-w-0 flex-1 truncate text-left", selectedLabel ? "text-foreground/75" : "")}>
            {selectedLabel || t("modelsDialog.autoProtocol")}
          </span>
          <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72 p-1">
        <button
          type="button"
          onClick={() => onChange([])}
          className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
        >
          <span className="min-w-0 flex-1 truncate text-left">{t("modelsDialog.autoProtocol")}</span>
          <Check className={cn("absolute right-2 size-4 shrink-0 text-muted-foreground", value.length === 0 ? "opacity-100" : "opacity-0")} />
        </button>
        {PROTOCOL_OPTIONS.map((item) => (
          <button
            key={item.value}
            type="button"
            onClick={() => toggle(item.value)}
            className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
          >
            <span className="min-w-0 flex-1 truncate text-left">{item.label}</span>
            <Check
              className={cn(
                "absolute right-2 size-4 shrink-0 text-muted-foreground",
                selected.has(item.value) ? "opacity-100" : "opacity-0",
              )}
            />
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}

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

function routeIDsForRow(row: RowDraft): number[] {
  return Object.values(row.routeIDsByProtocol).filter((id) => id > 0);
}

function removeRouteIDFromRows(rows: RowDraft[], routeID: number): RowDraft[] {
  return rows.flatMap((row) => {
    if (!routeIDsForRow(row).includes(routeID)) {
      return [row];
    }
    const nextRouteIDsByProtocol = Object.fromEntries(
      Object.entries(row.routeIDsByProtocol).filter(([, id]) => id !== routeID),
    );
    const nextRouteIDs = Object.values(nextRouteIDsByProtocol).filter((id) => id > 0);
    if (nextRouteIDs.length === 0) {
      return [];
    }
    const nextProtocols = row.protocols.filter((protocol) => nextRouteIDsByProtocol[protocol] > 0);
    return [
      {
        ...row,
        protocol: nextProtocols[0] ?? row.protocol,
        protocols: nextProtocols,
        routeID: Math.min(...nextRouteIDs),
        routeIDsByProtocol: nextRouteIDsByProtocol,
      },
    ];
  });
}

function selectedProtocolsForSave(row: RowDraft): AdminLLMAdapter[] {
  const protocols = row.protocols.length > 0 ? row.protocols : [];
  return Array.from(new Set(protocols));
}

async function runOperationsInOrder(operations: Array<() => Promise<unknown>>): Promise<void> {
  for (const operation of operations) {
    await operation();
  }
}

type ModelRowProps = {
  row: RowDraft;
  isSelected: boolean;
  onSelect: (draftKey: string, checked: boolean) => void;
  onUpdate: (draftKey: string, patch: Partial<Omit<RowDraft, "draftKey" | "isDirty">>) => void;
  onTest: (row: RowDraft, routeID: number) => void;
};

const ModelRow = React.memo(function ModelRow({ row, isSelected, onSelect, onUpdate, onTest }: ModelRowProps) {
  const t = useTranslations("adminChannels");
  const modelT = useTranslations("adminModels");
  const platformModelName = row.platformModelNameDraft.trim();
  const hasBindingDraft = platformModelName.length > 0;
  const routeChecked = row.routeStatus === "active";
  const routeIDs = routeIDsForRow(row);
  const persistedRouteCount = routeIDs.length;
  const testRouteID = row.routeID || routeIDs[0] || 0;
  const testDisabled = testRouteID <= 0 || row.isDirty;
  const testTooltip = testDisabled ? modelT("probe.saveBeforeTest") : modelT("actions.test");

  const handlePlatformModelChange = (value: string) => {
    onUpdate(row.draftKey, { platformModelNameDraft: value });
  };

  return (
    <TableRow
      className={cn(
        isSelected && "bg-muted/40",
        row.isDirty && "bg-amber-50/40 dark:bg-amber-900/10",
      )}
    >
      <TableCell className="w-[44px] py-0 text-center whitespace-nowrap">
        <div className="flex h-10 items-center justify-center">
          <Checkbox
            checked={isSelected}
            disabled={persistedRouteCount === 0}
            onCheckedChange={(checked) => onSelect(row.draftKey, checked === true)}
            aria-label={t("modelsDialog.selectModel", { name: row.upstreamModelName })}
          />
        </div>
      </TableCell>
      <TableCell className="w-[56px] whitespace-nowrap">
        <Switch
          size="sm"
          checked={routeChecked}
          onCheckedChange={(checked) => onUpdate(row.draftKey, { routeStatus: checked ? "active" : "inactive" })}
          aria-label={t("modelsDialog.routeStatusFor", { name: row.upstreamModelName })}
        />
      </TableCell>
      <TableCell className="max-w-[220px] font-mono text-xs text-muted-foreground">
        <span className="block truncate" title={row.upstreamModelName}>
          {row.upstreamModelName}
        </span>
      </TableCell>
      <TableCell className="min-w-[220px]">
        <Input
          className="h-7 min-w-[220px] font-mono text-xs"
          value={row.platformModelNameDraft}
          aria-label={t("modelsDialog.platformModelName")}
          onChange={(e) => handlePlatformModelChange(e.target.value)}
        />
      </TableCell>
      <TableCell className="w-[220px] whitespace-nowrap">
        {!hasBindingDraft ? (
          <span className="text-xs text-muted-foreground">
            {t("modelsDialog.deleteAfterSave")}
          </span>
        ) : (
          <ProtocolsDropdown
            value={row.protocols}
            onChange={(protocols) =>
              onUpdate(row.draftKey, {
                protocols,
                protocol: protocols[0] ?? "",
                kindsDisplay: resolveKindsDisplayForProtocols(protocols, row.kindsDisplay),
              })
            }
          />
        )}
      </TableCell>
      <TableCell className="w-[140px]">
        <KindsDropdown
          value={row.kindsDisplay}
          onChange={(value) => onUpdate(row.draftKey, { kindsDisplay: value })}
        />
      </TableCell>
      <TableCell className="w-[48px] text-right" stickyEnd>
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex">
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground shadow-none"
                disabled={testDisabled}
                onClick={() => onTest(row, testRouteID)}
                aria-label={modelT("actions.test")}
              >
                <Activity className="size-3.5 stroke-1" />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent side="top">{testTooltip}</TooltipContent>
        </Tooltip>
      </TableCell>
    </TableRow>
  );
});

type RemoteModelsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  upstream: AdminLLMUpstreamView | null;
  onImported: () => void;
};

function remoteModelStatusKey(item: AdminLLMRemoteModelItem): "bound" | "unbound" | "unsynced" {
  if (item.alreadyBound) return "bound";
  return item.alreadySynced ? "unbound" : "unsynced";
}

function dedupeRemoteModels(items: AdminLLMRemoteModelItem[]): AdminLLMRemoteModelItem[] {
  const byName = new Map<string, AdminLLMRemoteModelItem>();
  for (const item of items) {
    const key = item.upstreamModelName.trim();
    if (!key) continue;
    const existing = byName.get(key);
    if (!existing) {
      byName.set(key, item);
      continue;
    }
    byName.set(key, {
      ...existing,
      suggestedPlatformModelName: existing.suggestedPlatformModelName || item.suggestedPlatformModelName,
      suggestedKindsJSON: existing.suggestedKindsJSON || item.suggestedKindsJSON,
      suggestedProtocol: existing.suggestedProtocol || item.suggestedProtocol,
      suggestedProtocols: Array.from(new Set([
        ...(existing.suggestedProtocols ?? []),
        ...(item.suggestedProtocols ?? []),
      ])),
      bindingCode: existing.bindingCode || item.bindingCode,
      boundPlatformModels: Array.from(new Set([...existing.boundPlatformModels, ...item.boundPlatformModels])),
      upstreamModelStatus: existing.upstreamModelStatus || item.upstreamModelStatus,
      alreadySynced: existing.alreadySynced || item.alreadySynced,
      alreadyBound: existing.alreadyBound || item.alreadyBound,
    });
  }
  return Array.from(byName.values());
}

function RemoteModelsSkeletonRows({ rowCount = 10 }: { rowCount?: number }) {
  return (
    <>
      {Array.from({ length: rowCount }).map((_, index) => (
        <TableRow key={`remote-model-skeleton-${index}`}>
          <TableCell className="w-14 px-2 text-center">
            <span className="mx-auto block size-4 animate-pulse rounded-sm bg-muted" />
          </TableCell>
          <TableCell className="min-w-0">
            <span className="block h-4 w-4/5 animate-pulse rounded-sm bg-muted" />
          </TableCell>
          <TableCell className="min-w-0">
            <span className="block h-7 w-full animate-pulse rounded-md bg-muted/80" />
          </TableCell>
          <TableCell className="w-20 text-center">
            <span className="mx-auto block h-5 w-16 animate-pulse rounded-full bg-muted/70" />
          </TableCell>
        </TableRow>
      ))}
    </>
  );
}

function RemoteModelsDialog({
  open,
  onOpenChange,
  upstream,
  onImported,
}: RemoteModelsDialogProps) {
  const t = useTranslations("adminChannels");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [loading, setLoading] = React.useState(false);
  const [importing, setImporting] = React.useState(false);
  const [remoteItems, setRemoteItems] = React.useState<AdminLLMRemoteModelItem[]>([]);
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [draftPlatformModelNames, setDraftPlatformModelNames] = React.useState<Map<string, string>>(new Map());
  const [query, setQuery] = React.useState("");

  const loadRemoteModels = React.useCallback(async () => {
    if (!upstream) return;
    setRemoteItems([]);
    setSelected(new Set());
    setDraftPlatformModelNames(new Map());
    setQuery("");
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      const data = await listAdminLLMRemoteModels(token, upstream.id);
      const syncableItems = dedupeRemoteModels(data.items.filter((i) => !i.alreadyBound));
      setRemoteItems(syncableItems);
      setSelected(new Set(syncableItems.map((i) => i.upstreamModelName)));
      setDraftPlatformModelNames(createDraftPlatformModelNameMap(syncableItems));
    } catch (err) {
      toast.error(t("modelsDialog.remoteLoadFailed"), { description: resolveErrorMessage(err) });
      onOpenChange(false);
    } finally {
      setLoading(false);
    }
  }, [onOpenChange, resolveErrorMessage, t, upstream]);

  React.useEffect(() => {
    if (!open || !upstream) return;
    void loadRemoteModels();
  }, [loadRemoteModels, open, upstream]);

  function setDraftPlatformModelName(name: string, platformModelName: string) {
    setDraftPlatformModelNames((prev) => new Map(prev).set(name, platformModelName));
  }

  function toggleOne(name: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(name);
      else next.delete(name);
      return next;
    });
  }

  function toggleAll(checked: boolean) {
    const visibleNames = filteredRemoteItems.map((i) => i.upstreamModelName);
    setSelected((prev) => {
      if (checked) {
        const next = new Set(prev);
        visibleNames.forEach((name) => next.add(name));
        return next;
      }
      const next = new Set(prev);
      visibleNames.forEach((name) => next.delete(name));
      return next;
    });
  }

  async function handleSyncBindings() {
    if (!upstream || selected.size === 0) return;
    setImporting(true);
    try {
      const token = await resolveAccessToken();
      const items = remoteItems
        .filter((i) => selected.has(i.upstreamModelName))
        .map((i) => ({
          upstreamModelName: i.upstreamModelName,
          platformModelName: (draftPlatformModelNames.get(i.upstreamModelName) || i.upstreamModelName).trim(),
          protocols: i.suggestedProtocols?.length
            ? sortProtocolsForDisplay(i.suggestedProtocols)
            : i.suggestedProtocol
              ? [i.suggestedProtocol]
              : undefined,
          kindsJSON: i.suggestedKindsJSON || undefined,
        }));
      const result = await importAdminLLMUpstreamModels(token, upstream.id, { items });
      const description = summarizeImportResult(result, {
        importSummary: (summary) => t("modelsDialog.importSummary", summary),
      });
      if (result.failedCount > 0) {
        toast.error(t("modelsDialog.importPartialFailed"), {
          description,
        });
      } else {
        toast.success(t("modelsDialog.importDone"), {
          description,
        });
      }
      onImported();
      onOpenChange(false);
    } catch (err) {
      toast.error(t("modelsDialog.importFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setImporting(false);
    }
  }

  const normalizedQuery = query.trim().toLowerCase();
  const filteredRemoteItems = React.useMemo(() => {
    if (!normalizedQuery) return remoteItems;
    return remoteItems.filter((item) => {
      return [
        item.upstreamModelName,
        item.suggestedPlatformModelName || "",
        item.suggestedProtocol || "",
        ...(item.suggestedProtocols ?? []),
        t(`modelsDialog.remoteStatus.${remoteModelStatusKey(item)}`),
      ].some((value) => value.toLowerCase().includes(normalizedQuery));
    });
  }, [normalizedQuery, remoteItems, t]);
  const allSelected = filteredRemoteItems.length > 0 && filteredRemoteItems.every((i) => selected.has(i.upstreamModelName));
  const someSelected = filteredRemoteItems.some((i) => selected.has(i.upstreamModelName));
  const hasQuery = normalizedQuery.length > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[680px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("modelsDialog.syncTitle", { name: upstream?.name ?? "" })}</DialogTitle>
          <DialogDescription>
            {t("modelsDialog.syncDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="shrink-0 px-4 pb-2">
          <TableToolbar
            query={query}
            onQueryChange={setQuery}
            queryPlaceholder={t("modelsDialog.syncSearchPlaceholder")}
            loading={loading || importing}
            refreshLoading={loading}
            refreshLabel={t("modelsDialog.reloadRemote")}
            onRefresh={() => void loadRemoteModels()}
          />
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-4 py-2">
          <Table className="min-w-0 table-auto">
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-12 px-2 py-0 text-center">
                  <div className="flex h-8 items-center justify-center">
                    <Checkbox
                      checked={allSelected ? true : someSelected ? "indeterminate" : false}
                      onCheckedChange={(v) => toggleAll(v === true)}
                      aria-label={t("table.selectAll")}
                    />
                  </div>
                </TableHead>
                <TableHead className="max-w-[220px] whitespace-nowrap">{t("modelsDialog.upstreamModelName")}</TableHead>
                <TableHead className="w-full">{t("modelsDialog.platformModelName")}</TableHead>
                <TableHead className="w-20 text-center">{t("fields.status")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? <RemoteModelsSkeletonRows rowCount={10} /> : null}
              {!loading && filteredRemoteItems.length === 0 ? (
                <TableEmptyRow colSpan={4}>
                  {hasQuery ? t("modelsDialog.noMatchedModels") : t("modelsDialog.noSyncableModels")}
                </TableEmptyRow>
              ) : null}
              {filteredRemoteItems.map((item) => (
                <TableRow key={item.upstreamModelName}>
                  <TableCell className="w-14 px-2 text-center">
                    <Checkbox
                      checked={selected.has(item.upstreamModelName)}
                      onCheckedChange={(v) => toggleOne(item.upstreamModelName, v === true)}
                      aria-label={item.upstreamModelName}
                    />
                  </TableCell>
                  <TableCell className="max-w-[220px] font-mono text-xs text-muted-foreground">
                    <span className="block truncate" title={item.upstreamModelName}>
                      {item.upstreamModelName}
                    </span>
                  </TableCell>
                  <TableCell className="min-w-0">
                    <Input
                      className="w-full min-w-0 font-mono text-xs"
                      value={draftPlatformModelNames.get(item.upstreamModelName) ?? ""}
                      onChange={(e) => setDraftPlatformModelName(item.upstreamModelName, e.target.value)}
                    />
                  </TableCell>
                  <TableCell className="w-20 text-center">
                    <Badge variant={item.alreadyBound ? "secondary" : "outline"}>
                      {t(`modelsDialog.remoteStatus.${remoteModelStatusKey(item)}`)}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        <div className="flex shrink-0 items-center justify-between gap-3 px-4 py-3">
          <span className="text-xs text-muted-foreground">
            {t("modelsDialog.syncSummary", {
              total: remoteItems.length,
              shown: filteredRemoteItems.length,
              selected: selected.size,
              hasQuery: hasQuery ? "true" : "false",
              hasSelected: selected.size > 0 ? "true" : "false",
            })}
          </span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)} disabled={importing}>
              {commonT("actions.cancel")}
            </Button>
            <Button
              size="sm"
              onClick={handleSyncBindings}
              disabled={importing || selected.size === 0}
            >
              {importing ? <SpinnerLabel>{t("modelsDialog.syncing")}</SpinnerLabel> : t("sync")}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

type NewBindingDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  upstreamId: number;
  onCreated: () => void;
};

function NewBindingDialog({
  open,
  onOpenChange,
  upstreamId,
  onCreated,
}: NewBindingDialogProps) {
  const t = useTranslations("adminChannels");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [form, setForm] = React.useState<NewBindingFormState>(DEFAULT_NEW_BINDING);
  const [saving, setSaving] = React.useState(false);

  function setField<K extends keyof NewBindingFormState>(
    key: K,
    value: NewBindingFormState[K],
  ) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    if (!form.upstreamModelName.trim() || !form.platformModelName.trim()) {
      toast.error(t("modelsDialog.bindingNamesRequired"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      const payload: UpsertAdminLLMUpstreamModelRequest = {
        upstreamModelName: form.upstreamModelName.trim(),
        platformModelName: form.platformModelName.trim(),
        kindsJSON: displayToKindsJson(form.kindsDisplay),
        status: form.status,
        priority: 1,
        weight: 1,
      };
      const protocols = form.protocols.length > 0 ? form.protocols : [undefined];
      for (const protocol of protocols) {
        await upsertAdminLLMUpstreamModel(token, upstreamId, {
          ...payload,
          protocol,
        });
      }
      toast.success(t("modelsDialog.bindingCreated"));
      setForm(DEFAULT_NEW_BINDING);
      onOpenChange(false);
      onCreated();
    } catch (err) {
      toast.error(t("toast.createFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{t("modelsDialog.createBindingTitle")}</DialogTitle>
        <DialogDescription>{t("modelsDialog.createBindingDescription")}</DialogDescription>
        </DialogHeader>

        <div className="grid gap-4 py-2">
          <div className="grid gap-1.5">
            <Label>{t("modelsDialog.upstreamModelName")}</Label>
            <Input
              placeholder="gpt-5.5"
              value={form.upstreamModelName}
              onChange={(e) => setField("upstreamModelName", e.target.value)}
            />
          </div>

          <div className="grid gap-1.5">
            <Label>{t("modelsDialog.platformModelName")}</Label>
            <Input
              placeholder="claude-sonnet-4.5"
              value={form.platformModelName}
              onChange={(e) => setField("platformModelName", e.target.value)}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label>{t("modelsDialog.protocol")}</Label>
              <ProtocolsDropdown
                value={form.protocols}
                onChange={(protocols) =>
                  setForm((prev) => ({
                    ...prev,
                    protocols,
                    kindsDisplay: resolveKindsDisplayForProtocols(protocols, prev.kindsDisplay),
                  }))
                }
                className="h-9 text-sm"
              />
            </div>

            <div className="grid gap-1.5">
              <Label>{t("modelsDialog.kind")}</Label>
              <KindsDropdown
                value={form.kindsDisplay}
                onChange={(v) => setField("kindsDisplay", v)}
                className="w-full"
              />
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label>{t("fields.status")}</Label>
            <Switch
              size="sm"
              checked={form.status === "active"}
              onCheckedChange={(checked) => setField("status", checked ? "active" : "inactive")}
              aria-label={t("modelsDialog.routeStatus")}
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={saving}
          >
            {commonT("actions.cancel")}
          </Button>
          <Button size="sm" onClick={handleSave} disabled={saving}>
            {saving ? <SpinnerLabel>{t("sheet.saving")}</SpinnerLabel> : commonT("actions.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type UpstreamModelsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  upstream: AdminLLMUpstreamView | null;
  openRemoteOnOpen?: boolean;
  onUpstreamUpdated: (updated: AdminLLMUpstreamView) => void;
  onRemoteOpenHandled?: () => void;
};

type RouteStatusFilter = "bound" | "active" | "inactive";
type UpstreamStatusFilter = "all" | "active" | "inactive";
type RouteSortValue = "upstream_asc" | "upstream_desc" | "platform_asc" | "platform_desc" | "status_asc" | "protocol_asc";

type RouteListParams = {
  upstreamID: number | null;
  page: number;
  pageSize: number;
  query: string;
  routeStatusFilter: RouteStatusFilter;
  upstreamStatusFilter: UpstreamStatusFilter;
  protocolFilter: string;
  sortValue: RouteSortValue;
};

type BulkPatchConfirm = {
  patch: Partial<Omit<RowDraft, "draftKey" | "isDirty">>;
};

const DEFAULT_ROUTE_LIST_PARAMS: RouteListParams = {
  upstreamID: null,
  page: 1,
  pageSize: PAGE_SIZE_DEFAULT,
  query: "",
  routeStatusFilter: "bound",
  upstreamStatusFilter: "all",
  protocolFilter: "",
  sortValue: "upstream_asc",
};

export function UpstreamModelsDialog({
  open,
  onOpenChange,
  upstream,
  openRemoteOnOpen = false,
  onUpstreamUpdated,
  onRemoteOpenHandled,
}: UpstreamModelsDialogProps) {
  const t = useTranslations("adminChannels");
  const modelT = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [rows, setRows] = React.useState<RowDraft[]>([]);
  const [loadedUpstreamID, setLoadedUpstreamID] = React.useState<number | null>(null);
  const [loadingList, setLoadingList] = React.useState(false);
  const [remoteModelsOpen, setRemoteModelsOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [newBindingOpen, setNewBindingOpen] = React.useState(false);
  const [bulkRouteStatus, setBulkRouteStatus] = React.useState<"active" | "inactive">("active");
  const [bulkProtocols, setBulkProtocols] = React.useState<AdminLLMAdapter[]>(["openai_responses"]);
  const [bulkKindsDisplay, setBulkKindsDisplay] = React.useState("chat");
  const [bulkPatchConfirm, setBulkPatchConfirm] = React.useState<BulkPatchConfirm | null>(null);
  const [query, setQuery] = React.useState("");
  const [listParams, setListParams] = React.useState<RouteListParams>(DEFAULT_ROUTE_LIST_PARAMS);
  const [total, setTotal] = React.useState(0);
  const [probeOpen, setProbeOpen] = React.useState(false);
  const [probeLoading, setProbeLoading] = React.useState(false);
  const [probeTargetName, setProbeTargetName] = React.useState("");
  const [probeResults, setProbeResults] = React.useState<AdminLLMModelProbeResult[]>([]);
  const requestSeqRef = React.useRef(0);
  const upstreamID = upstream?.id ?? null;

  const loadBindings = React.useCallback(async (params: RouteListParams = listParams) => {
    if (!upstreamID || params.upstreamID !== upstreamID) return;
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoadingList(true);
    try {
      const token = await resolveAccessToken();
      const result = await listAdminLLMUpstreamModels(token, upstreamID, {
        page: params.page,
        pageSize: params.pageSize,
        query: params.query,
        routeStatus: params.routeStatusFilter,
        upstreamStatus: params.upstreamStatusFilter === "all" ? "" : params.upstreamStatusFilter,
        protocol: params.protocolFilter,
        sort: params.sortValue,
      });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      setRows(buildRowDrafts(result.results));
      setTotal(result.total);
      setLoadedUpstreamID(upstreamID);
      setSelected(new Set());
    } catch (err) {
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      setRows([]);
      setTotal(0);
      setLoadedUpstreamID(upstreamID);
      toast.error(t("modelsDialog.loadFailed"), { description: resolveErrorMessage(err) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoadingList(false);
      }
    }
  }, [listParams, resolveErrorMessage, t, upstreamID]);

  React.useEffect(() => {
    if (!open || !upstreamID) {
      setRows([]);
      setTotal(0);
      setLoadedUpstreamID(null);
      setSelected(new Set());
      return;
    }
    setSelected(new Set());
    setQuery("");
    setListParams({ ...DEFAULT_ROUTE_LIST_PARAMS, upstreamID });
  }, [open, upstreamID]);

  React.useEffect(() => {
    if (!open || !upstreamID || listParams.upstreamID !== upstreamID) {
      return;
    }
    void loadBindings(listParams);
  }, [listParams, loadBindings, open, upstreamID]);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      const nextQuery = query.trim();
      setListParams((prev) => {
        if (!open || !upstreamID || prev.upstreamID !== upstreamID) {
          return prev;
        }
        if (prev.query === nextQuery && prev.page === 1) {
          return prev;
        }
        return { ...prev, query: nextQuery, page: 1 };
      });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [open, query, upstreamID]);

  React.useEffect(() => {
    if (!open || !upstream || !openRemoteOnOpen) return;
    setRemoteModelsOpen(true);
    onRemoteOpenHandled?.();
  }, [onRemoteOpenHandled, open, openRemoteOnOpen, upstream]);

  const tableReady = upstream ? loadedUpstreamID === upstream.id && !loadingList : false;
  const visibleRows = React.useMemo(() => {
    if (!tableReady) {
      return [];
    }
    return rows;
  }, [rows, tableReady]);
  const {
    page,
    pageSize,
    routeStatusFilter,
    upstreamStatusFilter,
    protocolFilter,
    sortValue,
  } = listParams;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const hasActiveListQuery =
    listParams.query !== "" ||
    routeStatusFilter !== "bound" ||
    upstreamStatusFilter !== "all" ||
    protocolFilter !== "";

  const updateListParams = React.useCallback((patch: Partial<RouteListParams>) => {
    setListParams((prev) => ({ ...prev, ...patch, page: patch.page ?? 1 }));
  }, []);

  const selectableRows = React.useMemo(
    () => visibleRows.filter((row) => routeIDsForRow(row).length > 0),
    [visibleRows],
  );
  const allSelected = selectableRows.length > 0 && selectableRows.every((row) => selected.has(row.draftKey));
  const someSelected = selectableRows.some((row) => selected.has(row.draftKey));

  function handleSelectAll(checked: boolean) {
    setSelected((current) => {
      const next = new Set(current);
      for (const row of selectableRows) {
        if (checked) {
          next.add(row.draftKey);
        } else {
          next.delete(row.draftKey);
        }
      }
      return next;
    });
  }

  const handleSelectOne = React.useCallback((draftKey: string, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(draftKey);
      else next.delete(draftKey);
      return next;
    });
  }, []);

  const handleTestRoute = React.useCallback(
    async (row: RowDraft, routeID: number) => {
      if (!upstreamID || routeID <= 0) return;
      setProbeTargetName(`${row.platformModelNameDraft || row.platformModelName} / ${row.upstreamModelName}`);
      setProbeResults([]);
      setProbeOpen(true);
      setProbeLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(modelT("toast.sessionExpired"), { description: modelT("toast.signInAgain") });
          setProbeOpen(false);
          return;
        }
        setProbeResults([await testAdminLLMUpstreamModelRoute(token, upstreamID, routeID)]);
      } catch (error) {
        toast.error(t("toast.operationFailed"), { description: resolveErrorMessage(error) });
        setProbeOpen(false);
      } finally {
        setProbeLoading(false);
      }
    },
    [modelT, resolveErrorMessage, t, upstreamID],
  );

  const handleDeleteProbeRoute = React.useCallback(
    async (result: AdminLLMModelProbeResult) => {
      if (!upstream) {
        return;
      }
      try {
        const token = await resolveAccessToken();
        await deleteAdminLLMUpstreamModel(token, result.upstreamID, result.routeID);
        const nextResults = probeResults.filter((item) => item.routeID !== result.routeID);
        setRows((prev) => removeRouteIDFromRows(prev, result.routeID));
        setProbeResults(nextResults);
        if (nextResults.length === 0) {
          setProbeOpen(false);
        }
        setSelected((prev) => {
          const next = new Set(prev);
          rows.forEach((row) => {
            if (routeIDsForRow(row).includes(result.routeID)) {
              next.delete(row.draftKey);
            }
          });
          return next;
        });
        toast.success(modelT("toast.sourceDeleted"));
        void loadBindings();
        onUpstreamUpdated({ ...upstream });
      } catch (error) {
        toast.error(modelT("toast.sourceDeleteFailed"), { description: resolveErrorMessage(error) });
        throw error;
      }
    },
    [loadBindings, modelT, onUpstreamUpdated, probeResults, resolveErrorMessage, rows, upstream],
  );

  const updateRow = React.useCallback((
    draftKey: string,
    patch: Partial<Omit<RowDraft, "draftKey" | "isDirty">>,
  ) => {
    setRows((prev) =>
      prev.map((r) =>
        r.draftKey === draftKey ? { ...r, ...patch, isDirty: true } : r,
      ),
    );
  }, []);

  const applyBulkPatch = React.useCallback((patch: Partial<Omit<RowDraft, "draftKey" | "isDirty">>) => {
    if (selected.size === 0) return;
    setRows((prev) =>
      prev.map((row) =>
        routeIDsForRow(row).length > 0 && selected.has(row.draftKey)
          ? { ...row, ...patch, isDirty: true }
          : row,
      ),
    );
  }, [selected]);

  async function handleDeleteSelected() {
    if (!upstream || selected.size === 0) return;
    const routeIDs = rows
      .filter((row) => selected.has(row.draftKey))
      .flatMap(routeIDsForRow);
    if (routeIDs.length === 0) return;
    setDeleting(true);
    try {
      const token = await resolveAccessToken();
      const result = await batchDeleteAdminLLMUpstreamModels(token, upstream.id, {
        ids: routeIDs,
      });
      const deletedIDs = new Set(
        result.results
          .filter((item) => item.status === "deleted" || item.status === "not_found")
          .map((item) => item.id),
      );
      setRows((prev) =>
        prev.filter((row) => routeIDsForRow(row).some((routeID) => !deletedIDs.has(routeID))),
      );
      setSelected(new Set());
      if (result.failedCount > 0) {
        toast.error(t("modelsDialog.batchDeletePartialFailed"), {
          description: summarizeBatchDeleteResult(result, {
            batchDeleteSummary: (successCount, notFoundCount, failedCount) =>
              t("modelsDialog.batchDeleteSummary", { successCount, notFoundCount, failedCount }),
          }),
        });
      } else {
        toast.success(t("modelsDialog.batchDeleteDone"), {
          description: summarizeBatchDeleteResult(result, {
            batchDeleteSummary: (successCount, notFoundCount, failedCount) =>
              t("modelsDialog.batchDeleteSummary", { successCount, notFoundCount, failedCount }),
          }),
        });
      }
      void loadBindings();
      onUpstreamUpdated({ ...upstream });
    } catch (err) {
      toast.error(t("toast.deleteFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setDeleting(false);
      setDeleteConfirmOpen(false);
    }
  }

  async function handleSave() {
    if (!upstream) return;
    const dirty = rows.filter((r) => r.isDirty);
    if (dirty.length === 0) {
      toast.info(t("modelsDialog.noPendingChanges"));
      return;
    }
    const validationError = validateRowDrafts(rows, {
      upstreamModelRequired: t("modelsDialog.upstreamModelRequired"),
      activeRouteRequiresPlatformModel: t("modelsDialog.activeRouteRequiresPlatformModel"),
      duplicateBinding: (upstreamModelName, platformModelName) =>
        t("modelsDialog.duplicateBinding", { upstreamModelName, platformModelName }),
    });
    if (validationError) {
      toast.error(validationError);
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      const deleteOperations: Array<() => Promise<unknown>> = [];
      const upsertOperations: Array<() => Promise<unknown>> = [];
      let savedCount = 0;
      let deletedCount = 0;

      for (const row of dirty) {
        const platformModelName = row.platformModelNameDraft.trim();
        const existingRouteIDs = routeIDsForRow(row);
        const shouldDeleteRoute =
          existingRouteIDs.length > 0 &&
          row.routeStatus === "inactive" &&
          platformModelName.length === 0;

        if (shouldDeleteRoute) {
          for (const routeID of existingRouteIDs) {
            deleteOperations.push(() => deleteAdminLLMUpstreamModel(token, upstream.id, routeID));
            deletedCount += 1;
          }
          continue;
        }
        if (!platformModelName) {
          continue;
        }

        const basePayload: UpsertAdminLLMUpstreamModelRequest = {
          platformModelName,
          upstreamModelName: row.upstreamModelName.trim(),
          kindsJSON: displayToKindsJson(row.kindsDisplay),
          status: row.routeStatus || "active",
          priority: row.priority || 1,
          weight: row.weight || 1,
        };
        const desiredProtocols = selectedProtocolsForSave(row);
        if (desiredProtocols.length === 0) {
          const keepRouteID = row.routeID || existingRouteIDs[0];
          upsertOperations.push(() =>
            upsertAdminLLMUpstreamModel(token, upstream.id, {
              ...basePayload,
              routeID: keepRouteID || undefined,
            }),
          );
          savedCount += 1;
          for (const routeID of existingRouteIDs) {
            if (routeID === keepRouteID) continue;
            deleteOperations.push(() => deleteAdminLLMUpstreamModel(token, upstream.id, routeID));
            deletedCount += 1;
          }
          continue;
        }

        const desiredSet = new Set(desiredProtocols);
        const reusableRouteIDs = Object.entries(row.routeIDsByProtocol)
          .filter(([protocol]) => !desiredSet.has(protocol as AdminLLMAdapter))
          .map(([, routeID]) => routeID)
          .filter((routeID) => routeID > 0);
        const reusedRouteIDs = new Set<number>();
        for (const protocol of desiredProtocols) {
          const existingRouteID = row.routeIDsByProtocol[protocol];
          const routeID = existingRouteID || reusableRouteIDs.shift();
          if (routeID) {
            reusedRouteIDs.add(routeID);
          }
          upsertOperations.push(() =>
            upsertAdminLLMUpstreamModel(token, upstream.id, {
              ...basePayload,
              routeID,
              protocol,
            }),
          );
          savedCount += 1;
        }
        for (const [protocol, routeID] of Object.entries(row.routeIDsByProtocol)) {
          if (desiredSet.has(protocol as AdminLLMAdapter) || reusedRouteIDs.has(routeID)) continue;
          deleteOperations.push(() => deleteAdminLLMUpstreamModel(token, upstream.id, routeID));
          deletedCount += 1;
        }
      }

      if (deleteOperations.length === 0 && upsertOperations.length === 0) {
        toast.info(t("modelsDialog.noSavableChanges"));
        await loadBindings();
        return;
      }

      await runOperationsInOrder(upsertOperations);
      await runOperationsInOrder(deleteOperations);
      if (savedCount > 0 && deletedCount > 0) {
        toast.success(t("modelsDialog.savedAndDeleted", { savedCount, deletedCount }));
      } else if (deletedCount > 0) {
        toast.success(t("modelsDialog.deletedBindings", { deletedCount }), {
          description: t("modelsDialog.deleteBindingDescription"),
        });
      } else {
        toast.success(t("modelsDialog.savedChanges", { savedCount }));
      }
      await loadBindings();
      onUpstreamUpdated({ ...upstream });
    } catch (err) {
      toast.error(t("toast.updateFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setSaving(false);
    }
  }

  const dirtyCount = rows.filter((r) => r.isDirty).length;
  const selectedCount = selected.size;
  const selectedRouteCount = React.useMemo(
    () =>
      rows
        .filter((row) => selected.has(row.draftKey))
        .reduce((count, row) => count + routeIDsForRow(row).length, 0),
    [rows, selected],
  );

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent
          className="flex max-h-[min(90vh,800px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 md:w-[calc(100vw-8rem)] sm:max-w-[860px]"
        >
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{t("modelsDialog.manageTitle")}</DialogTitle>
            <DialogDescription>
              {t("modelsDialog.manageDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="shrink-0 px-4 pb-3">
            <TableToolbar
              query={query}
              onQueryChange={setQuery}
              queryPlaceholder={t("modelsDialog.manageSearchPlaceholder")}
              loading={loadingList}
              selectedCount={selectedCount}
              onRefresh={() => void loadBindings()}
              refreshLoading={loadingList}
              refreshDisabled={!upstream || loadingList}
              refreshLabel={t("modelsDialog.refreshBindings")}
              filters={[
                {
                  key: "route-status",
                  label: t("modelsDialog.routeStatus"),
                  value: routeStatusFilter === "bound" ? "" : routeStatusFilter,
                  onValueChange: (value) => updateListParams({ routeStatusFilter: (value || "bound") as RouteStatusFilter }),
                  options: [
                    { label: t("modelsDialog.allRoutes"), value: "" },
                    { label: t("status.active"), value: "active" },
                    { label: t("status.inactive"), value: "inactive" },
                  ],
                },
                {
                  key: "upstream-status",
                  label: t("modelsDialog.upstreamStatus"),
                  value: upstreamStatusFilter === "all" ? "" : upstreamStatusFilter,
                  onValueChange: (value) => updateListParams({ upstreamStatusFilter: (value || "all") as UpstreamStatusFilter }),
                  options: [
                    { label: t("modelsDialog.allUpstreams"), value: "" },
                    { label: t("modelsDialog.upstreamActive"), value: "active" },
                    { label: t("modelsDialog.upstreamInactive"), value: "inactive" },
                  ],
                },
                {
                  key: "protocol",
                  label: t("modelsDialog.protocol"),
                  value: protocolFilter,
                  onValueChange: (value) => updateListParams({ protocolFilter: value }),
                  options: [
                    { label: t("modelsDialog.allProtocols"), value: "" },
                    ...PROTOCOL_OPTIONS.map((item) => ({ label: item.label, value: item.value })),
                  ],
                },
              ]}
              sort={{
                value: sortValue,
                onValueChange: (value) => updateListParams({ sortValue: value as RouteSortValue }),
                options: [
                  { label: t("modelsDialog.sort.upstreamAsc"), value: "upstream_asc" },
                  { label: t("modelsDialog.sort.upstreamDesc"), value: "upstream_desc" },
                  { label: t("modelsDialog.sort.platformAsc"), value: "platform_asc" },
                  { label: t("modelsDialog.sort.platformDesc"), value: "platform_desc" },
                  { label: t("modelsDialog.sort.statusAsc"), value: "status_asc" },
                  { label: t("modelsDialog.sort.protocolAsc"), value: "protocol_asc" },
                ],
              }}
              bulkContent={
                <div className="space-y-1">
                  <BulkActionControlRow
                    icon={<ToggleLeft className="size-3 stroke-1" />}
                    label={t("actions.apply")}
                    onApply={() => setBulkPatchConfirm({ patch: { routeStatus: bulkRouteStatus } })}
                    disabled={selectedCount === 0}
                  >
                    <Select
                      value={bulkRouteStatus}
                      onValueChange={(value) => {
                        setBulkRouteStatus(value as "active" | "inactive");
                      }}
                      disabled={selectedCount === 0}
                    >
                      <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent position="popper" align="start" className="z-[100]">
                        <SelectItem value="active" className="text-[11px]">{t("status.active")}</SelectItem>
                        <SelectItem value="inactive" className="text-[11px]">{t("status.inactive")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </BulkActionControlRow>

                  <BulkActionControlRow
                    icon={<Cable className="size-3 stroke-1" />}
                    label={t("actions.apply")}
                    onApply={() =>
                      setBulkPatchConfirm({
                        patch: {
                          protocols: bulkProtocols,
                          protocol: bulkProtocols[0] ?? "",
                          kindsDisplay: resolveKindsDisplayForProtocols(bulkProtocols, bulkKindsDisplay),
                        },
                      })
                    }
                    disabled={selectedCount === 0}
                  >
                    <ProtocolsDropdown
                      value={bulkProtocols}
                      onChange={setBulkProtocols}
                      disabled={selectedCount === 0}
                      className="h-7 w-full px-2 text-[11px]"
                    />
                  </BulkActionControlRow>

                  <BulkActionControlRow
                    icon={<Tags className="size-3 stroke-1" />}
                    label={t("actions.apply")}
                    onApply={() => setBulkPatchConfirm({ patch: { kindsDisplay: bulkKindsDisplay } })}
                    disabled={selectedCount === 0 || !bulkKindsDisplay}
                  >
                    <KindsDropdown
                      value={bulkKindsDisplay}
                      onChange={setBulkKindsDisplay}
                      disabled={selectedCount === 0}
                      className="h-7 w-full px-2 text-[11px]"
                    />
                  </BulkActionControlRow>
                </div>
              }
              bulkActions={[
                {
                  key: "delete-bindings",
                  label: t("modelsDialog.deleteBindings"),
                  icon: <Trash2 />,
                  onClick: () => setDeleteConfirmOpen(true),
                  disabled: deleting,
                },
              ]}
            >
              <Button size="sm" onClick={() => setRemoteModelsOpen(true)} disabled={!upstream}>
                <CloudDownload className="size-3" />{t("sync")}
              </Button>
              <Button size="sm" onClick={() => setNewBindingOpen(true)} disabled={!upstream}>
                <Plus className="size-3" />{commonT("actions.create")}
              </Button>
            </TableToolbar>
          </div>

          <div className="min-h-0 flex-1 overflow-auto px-4 py-2">
            <Table className="min-w-[800px]">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-[44px] py-0 text-center">
                      <div className="flex h-8 items-center justify-center">
                        <Checkbox
                          checked={allSelected ? true : someSelected ? "indeterminate" : false}
                          onCheckedChange={(checked) => handleSelectAll(checked === true)}
                          aria-label={t("table.selectAll")}
                        />
                      </div>
                    </TableHead>
                    <TableHead className="w-[56px]">{t("modelsDialog.routeStatus")}</TableHead>
                    <TableHead>{t("modelsDialog.upstreamModelName")}</TableHead>
                    <TableHead className="min-w-[220px]">{t("modelsDialog.platformModel")}</TableHead>
                    <TableHead className="w-[220px]">{t("modelsDialog.protocol")}</TableHead>
                    <TableHead className="w-[140px]">{t("modelsDialog.kind")}</TableHead>
                    <TableHead className="w-[48px]" stickyEnd />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {!tableReady ? <TableSkeletonRows colSpan={7} rowCount={10} /> : null}
                  {tableReady && rows.length === 0 ? (
                    <TableEmptyRow colSpan={7}>
                      {hasActiveListQuery ? t("modelsDialog.noMatchedBindings") : t("modelsDialog.noBindings")}
                    </TableEmptyRow>
                  ) : null}
                  {visibleRows.map((row) => (
                    <ModelRow
                      key={row.draftKey}
                      row={row}
                      isSelected={selected.has(row.draftKey)}
                      onSelect={handleSelectOne}
                      onUpdate={updateRow}
                      onTest={handleTestRoute}
                    />
                  ))}
                </TableBody>
            </Table>
          </div>

          <TablePagination
            total={total}
            page={page}
            pageCount={pageCount}
            pageSize={pageSize}
            onPageChange={(nextPage) => updateListParams({ page: nextPage })}
            onPageSizeChange={(nextPageSize) => updateListParams({ pageSize: nextPageSize })}
            loading={loadingList}
            className="shrink-0 px-4 py-3"
          />

          <div className="flex shrink-0 flex-row justify-end gap-2 p-4">
            <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)} disabled={saving}>
              {commonT("actions.close")}
            </Button>
            <Button size="sm" onClick={handleSave} disabled={saving || dirtyCount === 0}>
              {saving ? <SpinnerLabel>{t("sheet.saving")}</SpinnerLabel> : commonT("actions.save")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {upstream && (
        <RemoteModelsDialog
          open={remoteModelsOpen}
          onOpenChange={setRemoteModelsOpen}
          upstream={upstream}
          onImported={() => {
            void loadBindings();
            onUpstreamUpdated({ ...upstream });
          }}
        />
      )}

      {upstream && (
        <NewBindingDialog
          open={newBindingOpen}
          onOpenChange={setNewBindingOpen}
          upstreamId={upstream.id}
          onCreated={() => {
            void loadBindings();
            onUpstreamUpdated({ ...upstream });
          }}
        />
      )}

      <ModelProbeDialog
        open={probeOpen}
        loading={probeLoading}
        targetName={probeTargetName}
        result={null}
        results={probeResults}
        onDeleteRoute={handleDeleteProbeRoute}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !probeLoading) {
            setProbeOpen(false);
          }
        }}
      />

      <AlertDialog
        open={deleteConfirmOpen}
        onOpenChange={(nextOpen) => !deleting && setDeleteConfirmOpen(nextOpen)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("modelsDialog.batchDeleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("modelsDialog.batchDeleteDescription", { count: selectedCount })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel
              disabled={deleting}
              onClick={() => setDeleteConfirmOpen(false)}
            >
              {commonT("actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleting || selectedRouteCount === 0}
              onClick={(event) => {
                event.preventDefault();
                void handleDeleteSelected();
              }}
            >
              {deleting ? <SpinnerLabel>{t("modelsDialog.deleting")}</SpinnerLabel> : t("modelsDialog.confirmDelete", { count: selectedCount })}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AdminBulkConfirmDialog
        open={bulkPatchConfirm !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            setBulkPatchConfirm(null);
          }
        }}
        pending={false}
        title={t("modelsDialog.bulkConfirmTitle")}
        description={t("modelsDialog.bulkConfirmDescription", { count: selectedCount })}
        confirmLabel={t("modelsDialog.bulkConfirmApply")}
        pendingLabel={t("modelsDialog.bulkConfirmApplying")}
        onConfirm={() => {
          if (bulkPatchConfirm) {
            applyBulkPatch(bulkPatchConfirm.patch);
          }
          setBulkPatchConfirm(null);
        }}
      />
    </>
  );
}
