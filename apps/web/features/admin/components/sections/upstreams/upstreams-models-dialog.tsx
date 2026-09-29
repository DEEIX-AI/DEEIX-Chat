import * as React from "react";
import { toast } from "sonner";
import { Activity, Cable, Check, ChevronDownIcon, CircleOff, CloudDownload, Plus, RefreshCw, Search, Tags, ToggleLeft, Trash2 } from "lucide-react";
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
  DialogCollapsible,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogHeightTransition,
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
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { AdminBulkConfirmDialog } from "@/features/admin/components/shared/bulk-confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { ApiError } from "@/shared/api/http-client";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { isOneOf } from "@/shared/lib/type-guards";
import { cn } from "@/lib/utils";
import type {
  AdminLLMAdapter,
  AdminLLMRemoteModelItem,
  AdminLLMUpstreamView,
} from "@/features/admin/api/llm-types";
import { ModelProbeDialog } from "@/features/admin/components/shared/model-probe-dialog";
import {
  PROTOCOL_OPTIONS,
  resolveKindsDisplayForProtocols,
  resolveNextRouteProtocolSelection,
  sortProtocolsForDisplay,
} from "@/features/admin/utils/llm-display";
import { MODEL_KIND_OPTIONS, } from "@/features/admin/types/llm";
import {
  createDraftPlatformModelNameMap,
  DEFAULT_NEW_BINDING,
  summarizeImportResult,
  type NewBindingFormState,
  type RowDraft,
} from "@/features/admin/model/upstreams-models";
import { PermissionGroupSelector } from "@/features/admin/components/shared/permission-group-selector";
import { FeatureGate } from "@/shared/capabilities";
import { useAdminUpstreamsModelBinding } from "@/features/admin/hooks/use-admin-upstreams-model-binding";
import {
  ROUTE_SORT_VALUES,
  ROUTE_STATUS_FILTERS,
  routeIDsForRow,
  type RowDraftPatch,
  UPSTREAM_STATUS_FILTERS,
  useAdminUpstreamsModels,
} from "@/features/admin/hooks/use-admin-upstreams-models";
import { useAdminUpstreamsRouteProbe } from "@/features/admin/hooks/use-admin-upstreams-route-probe";
import { isAdminLLMStatus } from "@/features/admin/model/admin-unions";
import {
  isUpstreamModelSyncAbort,
  UpstreamModelBindingsApplyError,
  useAdminUpstreamsModelSync,
} from "@/features/admin/hooks/use-admin-upstreams-model-sync";

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
  const t = useTranslations("adminUpstreams");
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
            "h-8 min-w-0 w-full justify-between gap-2 border-input/40 bg-transparent px-3 py-1 text-xs font-normal text-muted-foreground shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-3",
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
  const t = useTranslations("adminUpstreams");
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
            "h-8 min-w-0 w-full justify-between gap-2 border-input/40 bg-transparent px-3 py-1 text-xs font-normal text-muted-foreground shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-3",
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

type ModelRowProps = {
  row: RowDraft;
  isSelected: boolean;
  upstreamInactive: boolean;
  onSelect: (draftKey: string, checked: boolean) => void;
  onUpdate: (draftKey: string, patch: RowDraftPatch) => void;
  onTest: (row: RowDraft, routeID: number) => void;
};

const ModelRow = React.memo(function ModelRow({ row, isSelected, upstreamInactive, onSelect, onUpdate, onTest }: ModelRowProps) {
  const t = useTranslations("adminUpstreams");
  const modelT = useTranslations("adminModels");
  const platformModelName = row.platformModelNameDraft.trim();
  const hasBindingDraft = platformModelName.length > 0;
  const upstreamModelInactive = row.upstreamModelStatus === "inactive";
  const unavailable = upstreamInactive || upstreamModelInactive;
  const routeChecked = !unavailable && row.routeStatus === "active";
  const routeIDs = routeIDsForRow(row);
  const persistedRouteCount = routeIDs.length;
  const testRouteID = row.routeID || routeIDs[0] || 0;
  const testDisabled = unavailable || testRouteID <= 0 || row.isDirty;
  const unavailableReason = upstreamModelInactive
    ? t("modelsDialog.upstreamModelInactiveHint")
    : upstreamInactive
      ? t("modelsDialog.upstreamInactive")
      : null;
  const testTooltip = unavailableReason ?? (testDisabled ? modelT("probe.saveBeforeTest") : modelT("actions.test"));

  const handlePlatformModelChange = (value: string) => {
    onUpdate(row.draftKey, { platformModelNameDraft: value });
  };

  return (
    <TableRow
      selected={isSelected}
      tone={row.isDirty ? "warning" : undefined}
    >
      <TableCell className="w-[44px] py-1.5 text-center whitespace-nowrap">
        <div className="flex h-7 items-center justify-center">
          <Checkbox
            checked={isSelected}
            disabled={persistedRouteCount === 0}
            onCheckedChange={(checked) => onSelect(row.draftKey, checked === true)}
            aria-label={t("modelsDialog.selectModel", { name: row.upstreamModelName })}
          />
        </div>
      </TableCell>
      <TableCell className="w-[56px] py-1.5 whitespace-nowrap">
        <div className="flex h-7 items-center">
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex">
                <Switch
                  size="sm"
                  checked={routeChecked}
                  disabled={unavailable}
                  onCheckedChange={(checked) => onUpdate(row.draftKey, { routeStatus: checked ? "active" : "inactive" })}
                  aria-label={t("modelsDialog.routeStatusFor", { name: row.upstreamModelName })}
                />
              </span>
            </TooltipTrigger>
            {unavailable ? (
              <TooltipContent side="top" className="max-w-[280px] text-xs">
                {unavailableReason}
              </TooltipContent>
            ) : null}
          </Tooltip>
        </div>
      </TableCell>
      <TableCell className="max-w-[220px] py-1.5 font-mono text-xs text-muted-foreground">
        <div className="flex h-7 min-w-0 items-center gap-1">
          {upstreamModelInactive ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  className="inline-flex shrink-0 items-center text-muted-foreground/70"
                  role="img"
                  aria-label={t("modelsDialog.upstreamModelInactive")}
                >
                  <CircleOff className="size-3 stroke-[1.5]" />
                </span>
              </TooltipTrigger>
              <TooltipContent side="top" className="max-w-[280px] text-xs">
                {t("modelsDialog.upstreamModelInactiveHint")}
              </TooltipContent>
            </Tooltip>
          ) : null}
          <span
            className={cn("min-w-0 truncate", upstreamModelInactive && "line-through opacity-60")}
            title={row.upstreamModelName}
          >
            {row.upstreamModelName}
          </span>
        </div>
      </TableCell>
      <TableCell className="min-w-[220px] py-1.5">
        <Input
          className="h-7 min-w-[220px] font-mono text-xs"
          value={row.platformModelNameDraft}
          aria-label={t("modelsDialog.platformModelName")}
          onChange={(e) => handlePlatformModelChange(e.target.value)}
        />
      </TableCell>
      <TableCell className="w-[220px] py-1.5 whitespace-nowrap">
        {!hasBindingDraft ? (
          <span className="flex h-7 items-center text-xs text-muted-foreground">
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
            className="h-7 px-2 py-0 text-[11px] has-[>svg]:px-2"
          />
        )}
      </TableCell>
      <TableCell className="w-[140px] py-1.5">
        <KindsDropdown
          value={row.kindsDisplay}
          onChange={(value) => onUpdate(row.draftKey, { kindsDisplay: value })}
          className="h-7 px-2 py-0 text-[11px] has-[>svg]:px-2"
        />
      </TableCell>
      <TableCell className="w-[48px] py-1.5 text-right" stickyEnd>
        <div className="flex h-7 items-center justify-end">
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
        </div>
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

type SyncPlanStatusKey = "added" | "updated" | "reactivated" | "inactivated" | "unchanged" | "protected";

// Models that left the remote catalog only exist in the sync plan, so the table
// carries them as a separate row kind instead of forging remote items for them.
type CatalogRow =
  | { kind: "remote"; name: string; item: AdminLLMRemoteModelItem }
  | { kind: "inactivated"; name: string };

const syncPlanChipClassName =
  "inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-2 text-xs text-muted-foreground outline-none transition-colors hover:bg-muted/60 focus-visible:bg-muted/60";
const syncPlanChipActiveClassName = "bg-muted font-medium text-foreground hover:bg-muted";

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

function RemoteModelsDialog({
  open,
  onOpenChange,
  upstream,
  onImported,
}: RemoteModelsDialogProps) {
  const t = useTranslations("adminUpstreams");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [importing, setImporting] = React.useState(false);
  const [remoteItems, setRemoteItems] = React.useState<AdminLLMRemoteModelItem[]>([]);
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [draftPlatformModelNames, setDraftPlatformModelNames] = React.useState<Map<string, string>>(new Map());
  const [query, setQuery] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState<SyncPlanStatusKey | null>(null);
  const [permissionGroupIDs, setPermissionGroupIDs] = React.useState<number[]>([]);
  const [syncConfirmationOpen, setSyncConfirmationOpen] = React.useState(false);
  const [tooltipPortalContainer, setTooltipPortalContainer] = React.useState<HTMLDivElement | null>(null);
  const {
    catalog,
    catalogError,
    catalogLoading: loading,
    permissionGroups,
    permissionGroupsError,
    permissionGroupsLoading,
    reloadCatalog: loadRemoteModels,
    applySync,
  } = useAdminUpstreamsModelSync(open, upstream?.id ?? null);
  const remoteTotal = catalog?.total ?? null;
  const remoteSnapshotID = catalog?.snapshotID ?? "";
  const syncPlan = catalog?.syncPlan ?? null;

  React.useEffect(() => {
    setRemoteItems([]);
    setSelected(new Set());
    setDraftPlatformModelNames(new Map());
    setQuery("");
    setStatusFilter(null);
    if (!catalog) return;
    const items = dedupeRemoteModels(catalog.items);
    const syncableItems = items.filter((item) => !item.alreadyBound);
    setRemoteItems([...syncableItems, ...items.filter((item) => item.alreadyBound)]);
    setSelected(new Set(syncableItems.map((item) => item.upstreamModelName)));
    setDraftPlatformModelNames(createDraftPlatformModelNameMap(syncableItems));
  }, [catalog]);

  React.useEffect(() => {
    if (!catalogError) return;
    toast.error(t("modelsDialog.remoteLoadFailed"), { description: resolveErrorMessage(catalogError) });
    onOpenChange(false);
  }, [catalogError, onOpenChange, resolveErrorMessage, t]);

  React.useEffect(() => {
    if (!permissionGroupsError) return;
    toast.error(t("modelsDialog.permissionGroupsLoadFailed"), { description: resolveErrorMessage(permissionGroupsError) });
  }, [permissionGroupsError, resolveErrorMessage, t]);

  React.useEffect(() => {
    if (!open) {
      setPermissionGroupIDs([]);
      setSyncConfirmationOpen(false);
      return;
    }
    const defaultGroup = permissionGroups.find((group) => group.isDefault);
    setPermissionGroupIDs(defaultGroup ? [defaultGroup.id] : []);
  }, [open, permissionGroups]);

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
    const visibleNames = visibleSyncableItems.map((i) => i.upstreamModelName);
    setSelected((prev) => {
      if (checked) {
        const next = new Set(prev);
        visibleNames.forEach((name) => {
          next.add(name);
        });
        return next;
      }
      const next = new Set(prev);
      visibleNames.forEach((name) => {
        next.delete(name);
      });
      return next;
    });
  }

  const syncPlanStatuses = React.useMemo(
    (): { key: SyncPlanStatusKey; label: string; models: string[] }[] =>
      syncPlan
        ? [
            { key: "added", label: t("modelsDialog.syncPlanAddedLabel"), models: syncPlan.addedModels },
            { key: "updated", label: t("modelsDialog.syncPlanUpdatedLabel"), models: syncPlan.updatedModels },
            { key: "reactivated", label: t("modelsDialog.syncPlanReactivatedLabel"), models: syncPlan.reactivatedModels },
            { key: "inactivated", label: t("modelsDialog.syncPlanInactivatedLabel"), models: syncPlan.inactivatedModels },
            { key: "unchanged", label: t("modelsDialog.syncPlanUnchangedLabel"), models: syncPlan.unchangedModels },
            { key: "protected", label: t("modelsDialog.syncPlanProtectedLabel"), models: syncPlan.protectedModels },
          ]
        : [],
    [syncPlan, t],
  );
  const statusFilterNames = React.useMemo(() => {
    if (!statusFilter) return null;
    const status = syncPlanStatuses.find((item) => item.key === statusFilter);
    return new Set((status?.models ?? []).map((name) => name.trim()));
  }, [statusFilter, syncPlanStatuses]);
  const normalizedQuery = query.trim().toLowerCase();
  const catalogRows = React.useMemo((): CatalogRow[] => {
    const remoteNames = new Set(remoteItems.map((item) => item.upstreamModelName.trim()));
    const inactivatedRows = (syncPlan?.inactivatedModels ?? [])
      .filter((name) => !remoteNames.has(name.trim()))
      .map((name): CatalogRow => ({ kind: "inactivated", name }));
    return [
      ...remoteItems.map((item): CatalogRow => ({ kind: "remote", name: item.upstreamModelName, item })),
      ...inactivatedRows,
    ];
  }, [remoteItems, syncPlan]);
  const filteredRows = React.useMemo(() => {
    return catalogRows.filter((row) => {
      if (statusFilterNames && !statusFilterNames.has(row.name.trim())) return false;
      if (!normalizedQuery) return true;
      const values = row.kind === "remote"
        ? [
            row.item.upstreamModelName,
            row.item.suggestedPlatformModelName || "",
            row.item.suggestedProtocol || "",
            ...(row.item.suggestedProtocols ?? []),
            t(`modelsDialog.remoteStatus.${remoteModelStatusKey(row.item)}`),
          ]
        : [row.name, t("modelsDialog.syncPlanInactivatedLabel")];
      return values.some((value) => value.toLowerCase().includes(normalizedQuery));
    });
  }, [catalogRows, normalizedQuery, statusFilterNames, t]);
  const syncableItems = React.useMemo(() => remoteItems.filter((item) => !item.alreadyBound), [remoteItems]);
  const visibleSyncableItems = React.useMemo(
    () => filteredRows.flatMap((row) => (row.kind === "remote" && !row.item.alreadyBound ? [row.item] : [])),
    [filteredRows],
  );
  const selectedRemoteItems = React.useMemo(
    () => syncableItems.filter((item) => selected.has(item.upstreamModelName)),
    [syncableItems, selected],
  );
  const allSelected = visibleSyncableItems.length > 0 && visibleSyncableItems.every((i) => selected.has(i.upstreamModelName));
  const someSelected = visibleSyncableItems.some((i) => selected.has(i.upstreamModelName));
  const hasQuery = normalizedQuery.length > 0;
  const catalogChangeCount = syncPlan
    ? syncPlan.addedModels.length
      + syncPlan.updatedModels.length
      + syncPlan.reactivatedModels.length
      + syncPlan.inactivatedModels.length
    : 0;
  const hasCatalogChanges = catalogChangeCount > 0;
  const hasSyncWork = hasCatalogChanges || selectedRemoteItems.length > 0;

  function toggleStatusFilter(key: SyncPlanStatusKey) {
    setStatusFilter((prev) => (prev === key ? null : key));
  }

  function formatCatalogSummary(result: Awaited<ReturnType<typeof applySync>>["catalog"]) {
    return t("modelsDialog.catalogSyncSummary", {
      createdUpstreamModels: result.createdUpstreamModels,
      updatedUpstreamModels: result.updatedUpstreamModels,
      reactivatedModels: result.reactivatedModels,
      inactivatedModels: result.inactivatedModels,
      unchangedUpstreamModels: result.unchangedUpstreamModels,
      protectedUpstreamModels: result.protectedUpstreamModels,
    });
  }

  async function executeSyncBindings(allowEmpty: boolean) {
    if (!upstream) return;
    setImporting(true);
    try {
      const items = selectedRemoteItems.map((item) => ({
        upstreamModelName: item.upstreamModelName,
        platformModelName: (draftPlatformModelNames.get(item.upstreamModelName) || item.upstreamModelName).trim(),
        protocols: item.suggestedProtocols?.length
          ? sortProtocolsForDisplay(item.suggestedProtocols)
          : item.suggestedProtocol
            ? [item.suggestedProtocol]
            : undefined,
        kindsJSON: item.suggestedKindsJSON || undefined,
      }));
      const result = await applySync({
        allowEmpty,
        expectedSnapshot: remoteSnapshotID,
        items,
        permissionGroupIDs: permissionGroupIDs.length > 0 ? permissionGroupIDs : undefined,
      });
      const catalogSummary = formatCatalogSummary(result.catalog);
      const summaries = [catalogSummary];

      if (result.bindings) {
        summaries.push(summarizeImportResult(result.bindings, {
          importSummary: (summary) => t("modelsDialog.importSummary", summary),
        }));
        if (result.bindings.failedCount > 0) {
          toast.error(t("modelsDialog.importPartialFailed"), {
            description: summaries.join(" · "),
          });
        } else {
          toast.success(t("modelsDialog.importDone"), {
            description: summaries.join(" · "),
          });
        }
      } else {
        toast.success(t("modelsDialog.importDone"), {
          description: summaries.join(" · "),
        });
      }
      onImported();
      onOpenChange(false);
    } catch (err) {
      if (isUpstreamModelSyncAbort(err)) return;
      const catalogSummary = err instanceof UpstreamModelBindingsApplyError
        ? formatCatalogSummary(err.catalog)
        : "";
      const reportedError = err instanceof UpstreamModelBindingsApplyError ? err.originalError : err;
      toast.error(t(catalogSummary ? "modelsDialog.importAfterSyncFailed" : "modelsDialog.importFailed"), {
        description: [catalogSummary, resolveErrorMessage(reportedError)].filter(Boolean).join(" · "),
      });
      if (catalogSummary) {
        onImported();
      } else if (err instanceof ApiError && err.errorCode === "llm.remote_models_snapshot_changed") {
        await loadRemoteModels();
      }
    } finally {
      setImporting(false);
    }
  }

  function handleSyncBindings() {
    if ((syncPlan?.inactivatedModels.length ?? 0) > 0) {
      setSyncConfirmationOpen(true);
      return;
    }
    void executeSyncBindings(remoteTotal === 0);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={setTooltipPortalContainer}
        className="w-[calc(100vw-2rem)] gap-0 overflow-visible p-0 sm:max-w-[680px]"
      >
        <DialogHeightTransition contentClassName="max-h-[min(92svh,840px)]">
          <DialogHeader className="shrink-0 px-5 pt-5 pb-3">
            <DialogTitle>{t("modelsDialog.syncTitle", { name: upstream?.name ?? "" })}</DialogTitle>
            <DialogDescription>
              {t("modelsDialog.syncDescription")}
            </DialogDescription>
          </DialogHeader>

          <div className="shrink-0 px-5 pb-2">
            <div className="flex h-8 items-center gap-3">
              <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                  {loading && !syncPlan ? (
                    <span className="shrink-0 text-[11px] text-muted-foreground">
                      {t("modelsDialog.syncPlanLoading")}
                    </span>
                  ) : (
                    <>
                      <button
                        type="button"
                        aria-pressed={statusFilter === null}
                        onClick={() => setStatusFilter(null)}
                        className={cn(syncPlanChipClassName, statusFilter === null && syncPlanChipActiveClassName)}
                      >
                        <span>{t("modelsDialog.syncPlanAllLabel")}</span>
                        <span className="font-mono tabular-nums">{catalogRows.length}</span>
                      </button>
                      {syncPlanStatuses
                        .filter((status) => status.models.length > 0)
                        .map((status) => {
                          const active = statusFilter === status.key;
                          return (
                            <Tooltip key={status.key}>
                              <TooltipTrigger
                                type="button"
                                aria-label={`${status.label} ${status.models.length}`}
                                aria-pressed={active}
                                onClick={() => toggleStatusFilter(status.key)}
                                className={cn(
                                  syncPlanChipClassName,
                                  status.key === "inactivated" && "text-destructive",
                                  active && syncPlanChipActiveClassName,
                                )}
                              >
                                <span>{status.label}</span>
                                <span className="font-mono tabular-nums">{status.models.length}</span>
                              </TooltipTrigger>
                              <TooltipContent
                                portalContainer={tooltipPortalContainer}
                                side="bottom"
                                sideOffset={6}
                                className="w-72 px-3 py-2.5"
                              >
                                <p className="mb-1.5 font-medium">
                                  {status.label} · {status.models.length}
                                </p>
                                <div className="max-h-48 space-y-0.5 overflow-y-auto overscroll-contain pr-1">
                                  {status.models.map((modelName) => (
                                    <div key={modelName} className="break-all font-mono text-[11px] leading-5 text-background/80">
                                      {modelName}
                                    </div>
                                  ))}
                                </div>
                              </TooltipContent>
                            </Tooltip>
                          );
                        })}
                    </>
                  )}
                </div>
                <Button
                  type="button"
                  size="icon-sm"
                  variant="ghost"
                  className="size-7 shrink-0 text-muted-foreground shadow-none"
                  onClick={() => void loadRemoteModels()}
                  disabled={loading || importing}
                  aria-label={t("modelsDialog.reloadRemote")}
                  title={t("modelsDialog.reloadRemote")}
                >
                  <RefreshCw className={cn("size-3.5 stroke-1", loading && "animate-spin")} />
                </Button>
            </div>
          </div>

          <DialogCollapsible open={catalogRows.length > 0} className="shrink-0">
            <div>
              <div className="px-5 pb-2">
                <div className="relative">
                  <Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 stroke-1 text-muted-foreground" />
                  <Input
                    value={query}
                    placeholder={t("modelsDialog.syncSearchPlaceholder")}
                    onChange={(event) => setQuery(event.target.value)}
                    disabled={loading || importing}
                    className="bg-background pl-8"
                  />
                </div>
              </div>
              <div className="min-h-0 overflow-hidden px-5 py-2">
                <Table
                  className="min-w-full table-fixed"
                  shellClassName="w-full"
                  viewportClassName="[&_thead]:sticky [&_thead]:top-0 [&_thead]:z-20"
                  viewportStyle={{ maxHeight: "min(27rem, calc(92svh - 15rem))" }}
                >
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead className="w-12 px-2 py-1.5 text-center">
                        <div className="flex h-7 items-center justify-center">
                          <Checkbox
                            checked={allSelected ? true : someSelected ? "indeterminate" : false}
                            onCheckedChange={(v) => toggleAll(v === true)}
                            disabled={visibleSyncableItems.length === 0}
                            aria-label={t("table.selectAll")}
                          />
                        </div>
                      </TableHead>
                      <TableHead className="w-[36%] whitespace-nowrap">{t("modelsDialog.upstreamModelName")}</TableHead>
                      <TableHead>{t("modelsDialog.platformModelName")}</TableHead>
                      <TableHead className="w-20 text-center">{t("fields.status")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {!loading && filteredRows.length === 0 ? (
                      <TableEmptyRow colSpan={4}>
                        {hasQuery
                          ? t("modelsDialog.noMatchedModels")
                          : statusFilter
                            ? t("modelsDialog.syncPlanNoModels")
                            : t("modelsDialog.noSyncableModels")}
                      </TableEmptyRow>
                    ) : null}
                    {filteredRows.map((row) => (
                      <TableRow
                        key={row.name}
                        selected={row.kind === "remote" && selected.has(row.name)}
                      >
                        <TableCell className="w-14 px-2 py-1.5 text-center">
                          <div className="flex h-7 items-center justify-center">
                            {row.kind === "remote" && !row.item.alreadyBound ? (
                              <Checkbox
                                checked={selected.has(row.name)}
                                onCheckedChange={(v) => toggleOne(row.name, v === true)}
                                aria-label={row.name}
                              />
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell className="py-1.5 font-mono text-xs text-muted-foreground">
                          <span className="flex h-7 items-center truncate" title={row.name}>
                            {row.name}
                          </span>
                        </TableCell>
                        <TableCell className="min-w-0 py-1.5">
                          <div className="flex h-7 items-center">
                            {row.kind === "inactivated" ? (
                              <span className="text-xs text-muted-foreground">—</span>
                            ) : row.item.alreadyBound ? (
                              <span
                                className="truncate font-mono text-xs text-muted-foreground"
                                title={row.item.boundPlatformModels.join(", ")}
                              >
                                {row.item.boundPlatformModels.join(", ")}
                              </span>
                            ) : (
                              <Input
                                className="w-full min-w-0 font-mono text-xs"
                                value={draftPlatformModelNames.get(row.name) ?? ""}
                                onChange={(e) => setDraftPlatformModelName(row.name, e.target.value)}
                              />
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="w-20 py-1.5 text-center">
                          <div className="flex h-7 items-center justify-center">
                            {row.kind === "inactivated" ? (
                              <Badge variant="secondary" className="text-destructive">
                                {t("modelsDialog.syncPlanInactivatedLabel")}
                              </Badge>
                            ) : (
                              <Badge variant="secondary" className={cn(!row.item.alreadyBound && "text-muted-foreground")}>
                                {t(`modelsDialog.remoteStatus.${remoteModelStatusKey(row.item)}`)}
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          </DialogCollapsible>

          <DialogCollapsible open={catalogRows.length === 0} className="shrink-0">
            <div className="px-5 py-2">
              <div className="flex h-20 items-center justify-center text-xs text-muted-foreground">
                {loading || catalogRows.length > 0 ? (
                  <SpinnerLabel>{t("modelsDialog.loadingRemote")}</SpinnerLabel>
                ) : (
                  t("modelsDialog.noSyncableModels")
                )}
              </div>
            </div>
          </DialogCollapsible>

          <DialogFooter className="shrink-0 items-center justify-between px-5 py-3">
            <div className="flex min-w-0 items-center text-xs text-muted-foreground">
              {catalogRows.length === 0 && !loading ? (
                <span>{t("modelsDialog.remoteCatalogSummary", { total: remoteTotal ?? 0 })}</span>
              ) : selectedRemoteItems.length > 0 ? (
                <FeatureGate feature="multiUser">
                  <PermissionGroupSelector
                    variant="inline"
                    groups={permissionGroups}
                    selectedIDs={permissionGroupIDs}
                    disabled={loading || importing}
                    loading={permissionGroupsLoading}
                    triggerPrefix={t("modelsDialog.importPermissionGroups")}
                    placeholder={t("modelsDialog.permissionGroupsPlaceholder")}
                    emptyLabel={t("modelsDialog.permissionGroupsEmpty")}
                    autoBadgeLabel={t("modelsDialog.permissionGroupsAutoBadge")}
                    onSelectedIDsChange={setPermissionGroupIDs}
                  />
                </FeatureGate>
              ) : null}
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={importing}>
                {commonT("actions.cancel")}
              </Button>
              <Button
                onClick={handleSyncBindings}
                disabled={loading || importing || remoteTotal === null || !remoteSnapshotID || !syncPlan || !hasSyncWork}
              >
                {importing
                  ? <SpinnerLabel>{t("modelsDialog.syncing")}</SpinnerLabel>
                  : hasSyncWork
                    ? t("modelsDialog.applySync")
                    : t("modelsDialog.syncPlanCurrent")}
              </Button>
            </div>
          </DialogFooter>
        </DialogHeightTransition>
      </DialogContent>
      <AlertDialog open={syncConfirmationOpen} onOpenChange={setSyncConfirmationOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("modelsDialog.inactivateSyncTitle")}</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2">
                <p>
                  {t("modelsDialog.inactivateSyncSummary", {
                    count: syncPlan?.inactivatedModels.length ?? 0,
                  })}
                </p>
                <p>{t("modelsDialog.inactivateSyncImpact")}</p>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setSyncConfirmationOpen(false);
                void executeSyncBindings(remoteTotal === 0);
              }}
            >
              {t("modelsDialog.confirmApplySync")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
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
  const t = useTranslations("adminUpstreams");
  const commonT = useTranslations("common");
  const [form, setForm] = React.useState<NewBindingFormState>(DEFAULT_NEW_BINDING);
  const { saving, createBinding } = useAdminUpstreamsModelBinding();

  React.useEffect(() => {
    if (!open) return;
    setForm(DEFAULT_NEW_BINDING);
  }, [open]);

  function setField<K extends keyof NewBindingFormState>(
    key: K,
    value: NewBindingFormState[K],
  ) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSave() {
    await createBinding(upstreamId, form, () => {
      setForm(DEFAULT_NEW_BINDING);
      onOpenChange(false);
      onCreated();
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-[520px]">
        <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{t("modelsDialog.createBindingTitle")}</DialogTitle>
            <DialogDescription>{t("modelsDialog.createBindingDescription")}</DialogDescription>
          </DialogHeader>
  
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
            <div className="grid gap-4">
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
  
              <div className="grid min-w-0 gap-4 sm:grid-cols-2">
                <div className="grid min-w-0 gap-1.5">
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
                  />
                </div>
  
                <div className="grid min-w-0 gap-1.5">
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
          </div>
  
          <DialogFooter className="shrink-0 px-4 py-3">
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
        </DialogHeightTransition>
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

type BulkPatchConfirm = {
  patch: RowDraftPatch;
};

const isRouteStatusFilter = isOneOf(ROUTE_STATUS_FILTERS);
const isUpstreamStatusFilter = isOneOf(UPSTREAM_STATUS_FILTERS);
const isRouteSortValue = isOneOf(ROUTE_SORT_VALUES);

export function UpstreamModelsDialog({
  open,
  onOpenChange,
  upstream,
  openRemoteOnOpen = false,
  onUpstreamUpdated,
  onRemoteOpenHandled,
}: UpstreamModelsDialogProps) {
  const t = useTranslations("adminUpstreams");
  const commonT = useTranslations("common");
  const [remoteModelsOpen, setRemoteModelsOpen] = React.useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = React.useState(false);
  const [newBindingOpen, setNewBindingOpen] = React.useState(false);
  const [bulkRouteStatus, setBulkRouteStatus] = React.useState<"active" | "inactive">("active");
  const [bulkProtocols, setBulkProtocols] = React.useState<AdminLLMAdapter[]>([]);
  const [bulkKindsDisplay, setBulkKindsDisplay] = React.useState("chat");
  const [bulkPatchConfirm, setBulkPatchConfirm] = React.useState<BulkPatchConfirm | null>(null);
  const stableUpstream = useDialogSnapshot(upstream);
  const upstreamID = stableUpstream?.id ?? null;
  const {
    rows,
    loadedUpstreamID,
    loadingList,
    saving,
    deleting,
    selected,
    setSelected,
    query,
    setQuery,
    listParams,
    total,
    loadBindings,
    updateListParams,
    updateRow,
    applyBulkPatch,
    removeRouteLocally,
    deleteSelected,
    save: handleSave,
  } = useAdminUpstreamsModels({ open, upstream: stableUpstream, onUpstreamUpdated });
  const {
    probeOpen,
    setProbeOpen,
    probeLoading,
    probeTargetName,
    probeResults,
    testRoute: handleTestRoute,
    deleteProbeRoute: handleDeleteProbeRoute,
  } = useAdminUpstreamsRouteProbe({
    upstream: stableUpstream,
    onRouteDeleted: (result, probedUpstream) => {
      removeRouteLocally(result.routeID);
      void loadBindings();
      onUpstreamUpdated({ ...probedUpstream });
    },
  });

  React.useEffect(() => {
    setBulkProtocols([]);
  }, [upstreamID]);

  React.useEffect(() => {
    if (!open || !stableUpstream || !openRemoteOnOpen) return;
    setRemoteModelsOpen(true);
    onRemoteOpenHandled?.();
  }, [onRemoteOpenHandled, open, openRemoteOnOpen, stableUpstream]);

  const tableReady = stableUpstream ? loadedUpstreamID === stableUpstream.id : false;
  const visibleRows = React.useMemo(() => {
    if (!tableReady) {
      return [];
    }
    return rows;
  }, [rows, tableReady]);
  const virtualRows = useVirtualTableRows(visibleRows, {
    enabled: visibleRows.length > 100,
    estimateSize: 40,
  });
  const initialTableLoading = !tableReady || (loadingList && rows.length === 0);
  const showRows = visibleRows.length > 0;
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
  }, [setSelected]);

  async function handleDeleteSelected() {
    if (await deleteSelected()) {
      setDeleteConfirmOpen(false);
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
  const upstreamInactive = stableUpstream?.status === "inactive";

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent
          className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 md:w-[calc(100vw-8rem)] sm:max-w-[860px]"
        >
          <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
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
                refreshDisabled={!stableUpstream || loadingList}
                refreshLabel={t("modelsDialog.refreshBindings")}
                filters={[
                  {
                    key: "route-status",
                    label: t("modelsDialog.routeStatus"),
                    value: routeStatusFilter === "bound" ? "" : routeStatusFilter,
                    onValueChange: (value) => {
                      const routeStatusFilter = value || "bound";
                      if (isRouteStatusFilter(routeStatusFilter)) updateListParams({ routeStatusFilter });
                    },
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
                    onValueChange: (value) => {
                      const upstreamStatusFilter = value || "all";
                      if (isUpstreamStatusFilter(upstreamStatusFilter)) updateListParams({ upstreamStatusFilter });
                    },
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
                  onValueChange: (value) => {
                    if (isRouteSortValue(value)) updateListParams({ sortValue: value });
                  },
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
                          if (isAdminLLMStatus(value)) setBulkRouteStatus(value);
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
                <Button size="sm" onClick={() => setRemoteModelsOpen(true)} disabled={!stableUpstream}>
                  <CloudDownload className="size-3" />{t("sync")}
                </Button>
                <Button size="sm" onClick={() => setNewBindingOpen(true)} disabled={!stableUpstream}>
                  <Plus className="size-3" />{commonT("actions.create")}
                </Button>
              </TableToolbar>
            </div>
  
            <div className="min-h-0 overflow-hidden px-4 py-2">
              <Table
                className="min-w-[800px]"
                shellClassName="min-h-0"
                viewportRef={virtualRows.viewportRef}
                viewportClassName={cn(virtualRows.viewportClassName, "overscroll-contain")}
                viewportStyle={{
                  ...virtualRows.viewportStyle,
                  maxHeight: "min(480px, calc(86vh - 260px))",
                }}
              >
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead className="w-[44px] py-1.5 text-center">
                        <div className="flex h-7 items-center justify-center">
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
                    {initialTableLoading ? (
                      <TableLoadingRow colSpan={7} />
                    ) : null}
                    {tableReady && !loadingList && rows.length === 0 ? (
                      <TableEmptyRow colSpan={7}>
                        {hasActiveListQuery ? t("modelsDialog.noMatchedBindings") : t("modelsDialog.noBindings")}
                      </TableEmptyRow>
                    ) : null}
                    {showRows ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingTop} /> : null}
                    {showRows
                      ? virtualRows.rows.map(({ item: row }) => (
                          <ModelRow
                            key={row.draftKey}
                            row={row}
                            isSelected={selected.has(row.draftKey)}
                            upstreamInactive={upstreamInactive}
                            onSelect={handleSelectOne}
                            onUpdate={updateRow}
                            onTest={handleTestRoute}
                          />
                        ))
                      : null}
                    {showRows ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingBottom} /> : null}
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
  
            <DialogFooter className="shrink-0 px-4 py-3">
              <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
                {commonT("actions.close")}
              </Button>
              <Button onClick={handleSave} disabled={saving || dirtyCount === 0}>
                {saving ? <SpinnerLabel>{t("sheet.saving")}</SpinnerLabel> : commonT("actions.save")}
              </Button>
            </DialogFooter>
          </DialogHeightTransition>
        </DialogContent>
      </Dialog>

      {stableUpstream && (
        <RemoteModelsDialog
          open={remoteModelsOpen}
          onOpenChange={setRemoteModelsOpen}
          upstream={stableUpstream}
          onImported={() => {
            void loadBindings();
            onUpstreamUpdated({ ...stableUpstream });
          }}
        />
      )}

      {stableUpstream && (
        <NewBindingDialog
          open={newBindingOpen}
          onOpenChange={setNewBindingOpen}
          upstreamId={stableUpstream.id}
          onCreated={() => {
            void loadBindings();
            onUpstreamUpdated({ ...stableUpstream });
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
              {t("modelsDialog.batchDeleteDescription")}
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
              {deleting ? <SpinnerLabel>{t("modelsDialog.deleting")}</SpinnerLabel> : t("modelsDialog.confirmDelete")}
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
