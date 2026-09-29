"use client";

import {
  Activity,
  CheckCircle2,
  CircleOff,
  CircleX,
  List,
  MoreHorizontal,
  Pencil,
  RotateCcw,
  ShieldAlert,
  SlidersHorizontal,
  Trash2,
} from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import type {
  AdminLLMModelAccessScope,
  AdminLLMModelCbPolicyMode,
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
} from "@/features/admin/api/llm-types";
import {
  ADAPTER_LABELS,
  formatDateTime,
  resolveValue,
} from "@/features/admin/types/llm";
import {
  type InlineSourceEntry,
  type InlineSourceTarget,
  useAdminModelsInlineSources,
} from "@/features/admin/hooks/use-admin-models-inline-sources";
import { isAdminLLMModelAccessScope } from "@/features/admin/model/admin-unions";
import { sortProtocolsForDisplay } from "@/features/admin/utils/llm-display";
import { cn } from "@/lib/utils";
import { ModelIcon, parseKindsJSON, resolveModelIconURL, resolveModelIdentity } from "@/entities/model";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { isString, parseJSON } from "@/shared/lib/type-guards";
import {
  ModelSourceCircuitDialog,
  type ModelSourceCircuitPayload,
} from "./models-source-circuit-dialog";

const EXPANDED_ROW_ANIMATION_MS = 220;

type CollapsibleTableCellProps = React.ComponentProps<typeof TableCell> & {
  closing: boolean;
  opening: boolean;
  innerClassName?: string;
};

function CollapsibleTableCell({
  closing,
  opening,
  className,
  innerClassName,
  children,
  ...props
}: CollapsibleTableCellProps) {
  const closed = closing || opening;

  return (
    <TableCell
      className={cn(
        className,
        "transition-[padding] duration-200 ease-in-out motion-reduce:transition-none",
        closed && "py-0",
      )}
      {...props}
    >
      <div
        className={cn(
          "grid transition-[grid-template-rows,opacity,transform] duration-200 ease-in-out motion-reduce:transition-none",
          closed
            ? "grid-rows-[0fr] -translate-y-1 opacity-0"
            : "grid-rows-[1fr] translate-y-0 opacity-100",
        )}
      >
        <div className={cn("min-h-0 overflow-hidden", innerClassName)}>{children}</div>
      </div>
    </TableCell>
  );
}

function formatCircuitUntil(until: string, locale: string): string {
  if (!until) return "-";
  const ts = Number(until);
  const d = Number.isFinite(ts) && ts > 0 ? new Date(ts * 1000) : new Date(until);
  if (Number.isNaN(d.getTime())) return until;
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

function ProtocolBadges({ protocols }: { protocols: string[] }) {
  const sortedProtocols = sortProtocolsForDisplay(protocols);
  if (sortedProtocols.length === 0) return <span className="text-muted-foreground">-</span>;
  return (
    <div className="flex min-w-0 flex-nowrap items-center gap-1">
      {sortedProtocols.map((item) => (
        <Badge key={item} variant="secondary" className="whitespace-nowrap">
          {ADAPTER_LABELS[item] ?? item}
        </Badge>
      ))}
    </div>
  );
}

function SingleProtocolText({ protocol }: { protocol: string }) {
  return <Badge variant="secondary" className="whitespace-nowrap">{ADAPTER_LABELS[protocol] ?? protocol}</Badge>;
}

function KindsBadges({ kindsJson }: { kindsJson: string | null | undefined }) {
  const t = useTranslations("adminModels");
  const kinds = parseKindsJSON(kindsJson);
  if (kinds.length === 0) return <span className="text-muted-foreground">-</span>;
  return (
    <div className="flex min-w-0 flex-nowrap items-center justify-start gap-1 overflow-hidden">
      {kinds.map((kind) => (
        <Badge key={kind} variant="secondary">
          {["chat", "audio", "image_gen", "image_edit", "video_gen", "video_extension"].includes(kind)
            ? t(`kinds.${kind}`)
            : kind}
        </Badge>
      ))}
    </div>
  );
}

type ModelAvailability = "available" | "notEnabled" | "noSource";

function resolveModelAvailability(item: AdminLLMModelDTO): ModelAvailability {
  if (item.sourceCount <= 0) {
    return "noSource";
  }
  if (item.status !== "active") {
    return "notEnabled";
  }
  return item.activeSourceCount > 0 ? "available" : "notEnabled";
}

function ModelAvailabilityBadge({ availability }: { availability: ModelAvailability }) {
  const t = useTranslations("adminModels");
  if (availability === "available") {
    return null;
  }
  return (
    <Badge
      variant="outline"
      className={cn(
        "h-5 rounded-md px-1.5 py-0 text-[10px]",
        "shrink-0",
        availability === "noSource" && "border-border/70 text-muted-foreground",
        availability === "notEnabled" && "border-border/50 text-muted-foreground/80",
      )}
    >
      {availability === "noSource" ? t("availability.noSource") : t("availability.notEnabled")}
    </Badge>
  );
}

function SourceStatusText({
  modelStatus,
  status,
  upstreamStatus,
  upstreamModelStatus,
  circuitOpen,
  circuitUntil,
  circuitScope,
}: {
  modelStatus: AdminLLMStatus;
  status: AdminLLMStatus;
  upstreamStatus: AdminLLMStatus;
  upstreamModelStatus: AdminLLMStatus;
  circuitOpen: boolean;
  circuitUntil: string;
  circuitScope: "upstream" | "source" | "";
}) {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  const inactiveReason =
    modelStatus === "inactive"
      ? t("sources.platformModelInactive")
      : upstreamStatus === "inactive"
      ? t("sources.upstreamInactive")
      : upstreamModelStatus === "inactive"
        ? t("sources.upstreamModelInactive")
        : t("status.inactive");
  if (circuitOpen) {
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
  if (modelStatus === "inactive" || status === "inactive" || upstreamStatus === "inactive" || upstreamModelStatus === "inactive") {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <CircleX
            className="size-4 text-muted-foreground"
            aria-label={inactiveReason}
          />
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs">
          {inactiveReason}
        </TooltipContent>
      </Tooltip>
    );
  }
  return (
    <CheckCircle2
      className="size-4 text-emerald-600 dark:text-emerald-400"
      aria-label={t("status.active")}
    />
  );
}

type InlineSourceDeleteTarget = InlineSourceTarget;

type InlineSourceCircuitTarget = {
  modelId: number;
  policyMode: AdminLLMModelCbPolicyMode;
  source: AdminLLMModelUpstreamSourceDTO;
};

type ModelsTableProps = {
  items: AdminLLMModelDTO[];
  loading: boolean;
  batchApplying: boolean;
  circuitBreakerEnabled: boolean;
  selectedModelIDs: Set<number>;
  onSelectedModelIDsChange: React.Dispatch<React.SetStateAction<Set<number>>>;
  onEdit: (item: AdminLLMModelDTO) => void;
  onViewSources: (item: AdminLLMModelDTO) => void;
  pendingModelIDs: ReadonlySet<number>;
  onToggleStatus: (item: AdminLLMModelDTO, status: AdminLLMStatus) => void;
  onToggleAccessScope: (item: AdminLLMModelDTO, scope: AdminLLMModelAccessScope) => void;
  onDelete: (item: AdminLLMModelDTO) => void;
  onTestModel?: (item: AdminLLMModelDTO) => void;
  onTestSource?: (source: AdminLLMModelUpstreamSourceDTO) => void;
  onRefreshModels?: () => void;
  onSourceAvailabilityChange?: (modelID: number, previousAvailable: boolean, nextAvailable: boolean) => void;
  onSourceDeleteChange?: (modelID: number, source: AdminLLMModelUpstreamSourceDTO, deleted: boolean) => void;
};

type ModelTableRowProps = {
  item: AdminLLMModelDTO;
  circuitBreakerEnabled: boolean;
  selected: boolean;
  expanded: boolean;
  opening: boolean;
  collapsing: boolean;
  inlineData: InlineSourceEntry | undefined;
  onSelectModel: (id: number, checked: boolean) => void;
  onToggleRow: (item: AdminLLMModelDTO) => void;
  onEdit: (item: AdminLLMModelDTO) => void;
  onViewSources: (item: AdminLLMModelDTO) => void;
  updatePending: boolean;
  onToggleStatus: (item: AdminLLMModelDTO, status: AdminLLMStatus) => void;
  onToggleAccessScope: (item: AdminLLMModelDTO, scope: AdminLLMModelAccessScope) => void;
  onDelete: (item: AdminLLMModelDTO) => void;
  onTestModel?: (item: AdminLLMModelDTO) => void;
  onTestSource?: (source: AdminLLMModelUpstreamSourceDTO) => void;
  onInlineStatusToggle: (source: AdminLLMModelUpstreamSourceDTO, modelId: number) => void;
  onInlineCircuit: (source: AdminLLMModelUpstreamSourceDTO, modelId: number, action: "open" | "reset") => void;
  onInlineCircuitSettings: (target: InlineSourceCircuitTarget) => void;
  onInlineSourceDeleteRequest: (target: InlineSourceDeleteTarget) => void;
};

// Malformed or non-array JSON yields no protocols rather than breaking the row.
function resolveModelProtocols(item: AdminLLMModelDTO): string[] {
  const parsed = item.protocolsJSON ? parseJSON(item.protocolsJSON) : undefined;
  return Array.isArray(parsed) ? sortProtocolsForDisplay(parsed.filter(isString)) : [];
}

const ModelTableRow = React.memo(function ModelTableRow({
  item,
  circuitBreakerEnabled,
  selected,
  expanded,
  opening,
  collapsing,
  inlineData,
  onSelectModel,
  onToggleRow,
  onEdit,
  onViewSources,
  updatePending,
  onToggleStatus,
  onToggleAccessScope,
  onDelete,
  onTestModel,
  onTestSource,
  onInlineStatusToggle,
  onInlineCircuit,
  onInlineCircuitSettings,
  onInlineSourceDeleteRequest,
}: ModelTableRowProps) {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  const identity = resolveModelIdentity({
    code: item.platformModelName,
    vendor: item.vendor,
    icon: item.icon,
  });
  const iconURL = resolveModelIconURL(identity.modelIcon);
  const vendorLabel = item.vendorName.trim() || item.vendor.trim();
  const vendorIconURL = resolveModelIconURL(item.vendorIcon);
  const showVendor = item.vendor.trim().toLowerCase() !== "unknown" && vendorLabel;
  const titleText = item.platformModelName.trim();
  const protocols = resolveModelProtocols(item);
  const availability = resolveModelAvailability(item);
  const muted = availability !== "available";

  return (
    <React.Fragment>
      <TableRow
        className={cn("cursor-pointer", muted && "text-muted-foreground")}
        tone={muted ? "muted" : undefined}
        selected={selected}
        aria-expanded={expanded && !collapsing}
        onClick={() => onToggleRow(item)}
      >
        <TableCell className="w-[44px] py-1.5 whitespace-nowrap">
          <div className="flex h-7 items-center justify-center">
            <Checkbox
              checked={selected}
              disabled={updatePending}
              onClick={(event) => event.stopPropagation()}
              onCheckedChange={(checked) => onSelectModel(item.id, checked === true)}
              aria-label={t("table.selectModel", { name: item.platformModelName })}
            />
          </div>
        </TableCell>

        <TableCell className="py-1.5">
          <div className="flex min-w-0 items-center gap-2">
            <ModelAvailabilityBadge availability={availability} />
            <ModelIcon iconUrl={iconURL} label={titleText} />
            <span className={cn("min-w-0 flex-1 truncate text-xs font-medium leading-5", muted ? "text-muted-foreground" : "text-foreground")}>
              {titleText}
            </span>
          </div>
        </TableCell>

        <TableCell className="py-1.5">
          <KindsBadges kindsJson={item.kindsJSON} />
        </TableCell>

        <TableCell className="py-1.5">
          <ProtocolBadges protocols={protocols} />
        </TableCell>

        <TableCell className="w-[120px] py-1.5">
          {showVendor ? (
            <div className="flex min-w-0 items-center gap-1.5">
              {vendorIconURL ? <ModelIcon iconUrl={vendorIconURL} label={vendorLabel} size={14} /> : null}
              <span className="block max-w-[92px] truncate text-xs text-muted-foreground">
                {vendorLabel}
              </span>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground">-</span>
          )}
        </TableCell>

        <TableCell className="whitespace-nowrap py-1.5 text-center">
          <span className={cn(
            "text-xs",
            item.activeSourceCount > 0 ? "text-muted-foreground" : "text-muted-foreground/75",
          )}>
            {item.activeSourceCount}/{item.sourceCount}
          </span>
        </TableCell>

        <TableCell className="w-[72px] whitespace-nowrap py-1.5" onClick={(event) => event.stopPropagation()}>
          <div className="flex h-7 items-center justify-center">
            <Switch
              size="sm"
              checked={item.status === "active"}
              disabled={updatePending}
              onCheckedChange={(checked) => onToggleStatus(item, checked ? "active" : "inactive")}
              aria-label={t("table.modelStatusAria", { name: item.platformModelName })}
            />
          </div>
        </TableCell>

        <TableCell className="w-[112px] whitespace-nowrap py-1.5" onClick={(event) => event.stopPropagation()}>
          <div className="flex h-7 items-center">
            <Select
              value={item.accessScope === "internal" ? "internal" : "public"}
              disabled={updatePending}
              onValueChange={(value) => {
                if (isAdminLLMModelAccessScope(value)) onToggleAccessScope(item, value);
              }}
            >
              <SelectTrigger
                size="sm"
                className="h-7 w-[96px] border-input/40 bg-transparent px-2 text-xs shadow-none"
                aria-label={t("table.modelAccessScopeAria", { name: item.platformModelName })}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="public" className="text-xs">{t("accessScope.public")}</SelectItem>
                <SelectItem value="internal" className="text-xs">{t("accessScope.internal")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </TableCell>

        <TableCell className="whitespace-nowrap py-1.5 text-muted-foreground">
          {formatDateTime(item.updatedAt, locale)}
        </TableCell>

        <TableCell
          className="w-[56px] whitespace-nowrap py-1.5"
          stickyEnd
          onClick={(event) => event.stopPropagation()}
        >
          <div className="flex h-7 items-center justify-end">
            <DropdownMenu modal={false}>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  size="icon-sm"
                  variant="ghost"
                  className="text-muted-foreground shadow-none"
                  aria-label={t("table.modelActions")}
                >
                  <MoreHorizontal className="size-3.5 stroke-1" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => onEdit(item)}>
                  <Pencil className="size-3.5 stroke-1" />
                  {t("table.editModel")}
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => onViewSources(item)}>
                  <List className="size-3.5 stroke-1" />
                  {t("table.viewSources")}
                </DropdownMenuItem>
                {onTestModel ? (
                  <DropdownMenuItem onSelect={() => onTestModel(item)}>
                    <Activity className="size-3.5 stroke-1" />
                    {t("actions.testAll")}
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuSeparator />
                {item.status === "active" ? (
                  <DropdownMenuItem disabled={updatePending} onSelect={() => onToggleStatus(item, "inactive")}>
                    <CircleOff className="size-3.5 stroke-1" />
                    {t("table.disableModel")}
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem disabled={updatePending} onSelect={() => onToggleStatus(item, "active")}>
                    <RotateCcw className="size-3.5 stroke-1" />
                    {t("table.enableModel")}
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  disabled={updatePending}
                  onSelect={() => onDelete(item)}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 className="size-3.5 stroke-1" />
                  {t("table.deleteModel")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </TableCell>
      </TableRow>

      {expanded ? (
          inlineData?.loading ? (
            <TableRow tone="muted">
              <CollapsibleTableCell
                colSpan={10}
                opening={opening}
                closing={collapsing}
                className="py-3 pl-16 text-xs text-muted-foreground"
              >
                <div className="h-3 w-24 animate-pulse rounded-sm bg-muted/70" aria-hidden="true" />
              </CollapsibleTableCell>
            </TableRow>
          ) : inlineData && inlineData.items.length > 0 ? (
            inlineData.items.map((source) => {
              const sourceIdentity = resolveModelIdentity({
                code: source.upstreamModelName,
                vendor: source.upstreamModelVendor,
                icon: source.upstreamModelIcon,
              });
              const sourceVendorIconURL = resolveModelIconURL(sourceIdentity.vendorIcon);

              return (
                <TableRow key={source.id} tone="muted">
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[44px] whitespace-nowrap py-1.5"
                  >
                    <div className="flex h-7 items-center justify-center">
                      <span className="size-1.5 rounded-full bg-muted-foreground/40" />
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="py-1.5">
                    <div className="flex min-w-0 items-baseline gap-1.5">
                      <span className="shrink-0 text-[11px] leading-4 text-muted-foreground">{t("upstreamModel")}</span>
                      <span
                        className="truncate font-mono text-[11px] font-medium leading-4 text-foreground"
                        title={resolveValue(source.upstreamModelName)}
                      >
                        {resolveValue(source.upstreamModelName)}
                      </span>
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="py-1.5">
                    <KindsBadges kindsJson={source.upstreamModelKindsJSON} />
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="py-1.5">
                    <SingleProtocolText protocol={source.protocol} />
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="w-[120px] py-1.5">
                    {sourceIdentity.vendorKey !== "unknown" ? (
                      <div className="flex min-w-0 items-center gap-1.5">
                        {sourceVendorIconURL ? <ModelIcon iconUrl={sourceVendorIconURL} label={sourceIdentity.vendorLabel} size={14} /> : null}
                        <span className="block max-w-[92px] truncate text-[11px] leading-4 text-muted-foreground">
                          {sourceIdentity.vendorLabel}
                        </span>
                      </div>
                    ) : (
                      <span className="text-[11px] leading-4 text-muted-foreground">-</span>
                    )}
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="py-1.5 text-center text-[11px] leading-4 text-muted-foreground"
                  >
                    <div className="max-w-[12rem] truncate" title={resolveValue(source.upstreamName)}>
                      {resolveValue(source.upstreamName)}
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[72px] whitespace-nowrap py-1.5"
                  >
                    <div className="flex h-7 items-center justify-center">
                      <SourceStatusText
                        modelStatus={item.status}
                        status={source.status}
                        upstreamStatus={source.upstreamStatus}
                        upstreamModelStatus={source.upstreamModelStatus}
                        circuitOpen={circuitBreakerEnabled && source.circuitOpen}
                        circuitUntil={source.circuitUntil}
                        circuitScope={source.circuitScope}
                      />
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[112px] whitespace-nowrap py-1.5 text-[11px] leading-4 text-muted-foreground"
                  >
                    -
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="whitespace-nowrap py-1.5 text-[11px] leading-4 text-muted-foreground"
                  >
                    {formatDateTime(source.updatedAt, locale)}
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[56px] whitespace-nowrap py-1.5"
                    stickyEnd
                  >
                    <div className="flex h-7 items-center justify-end">
                      <DropdownMenu modal={false}>
                        <DropdownMenuTrigger asChild>
                          <Button
                            type="button"
                            size="icon-sm"
                            variant="ghost"
                            className="text-muted-foreground shadow-none"
                            aria-label={t("sources.sourceActions")}
                          >
                            <MoreHorizontal className="size-3.5 stroke-1" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          {onTestSource ? (
                            <DropdownMenuItem onSelect={() => onTestSource(source)}>
                              <Activity className="size-3.5 stroke-1" />
                              {t("actions.test")}
                            </DropdownMenuItem>
                          ) : null}
                          <DropdownMenuItem
                            onSelect={() => onInlineCircuitSettings({
                              modelId: item.id,
                              policyMode: item.cbPolicyMode,
                              source,
                            })}
                          >
                            <SlidersHorizontal className="size-3.5 stroke-1" />
                            {t("sources.circuitSettings")}
                          </DropdownMenuItem>
                          {source.status === "active" ? (
                            <DropdownMenuItem onSelect={() => onInlineStatusToggle(source, item.id)}>
                              <CircleOff className="size-3.5 stroke-1" />
                              {t("sources.disableSource")}
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem onSelect={() => onInlineStatusToggle(source, item.id)}>
                              <RotateCcw className="size-3.5 stroke-1" />
                              {t("sources.enableSource")}
                            </DropdownMenuItem>
                          )}
                          {circuitBreakerEnabled ? (
                            source.circuitOpen ? (
                              <DropdownMenuItem onSelect={() => onInlineCircuit(source, item.id, "reset")}>
                                <RotateCcw className="size-3.5 stroke-1" />
                                {t("sources.resetCircuit")}
                              </DropdownMenuItem>
                            ) : (
                              <DropdownMenuItem onSelect={() => onInlineCircuit(source, item.id, "open")}>
                                <CircleOff className="size-3.5 stroke-1" />
                                {t("sources.openCircuit")}
                              </DropdownMenuItem>
                            )
                          ) : null}
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            variant="destructive"
                            onSelect={() => onInlineSourceDeleteRequest({ modelId: item.id, source })}
                          >
                            <Trash2 className="size-3.5 stroke-1" />
                            {t("sources.deleteSource")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </CollapsibleTableCell>
                </TableRow>
              );
            })
          ) : (
            <TableRow tone="muted">
              <CollapsibleTableCell
                colSpan={10}
                opening={opening}
                closing={collapsing}
                className="py-3 pl-16 text-xs text-muted-foreground"
              >
                {t("sources.empty")}
              </CollapsibleTableCell>
            </TableRow>
          )
      ) : null}
    </React.Fragment>
  );
});

export function ModelsTable({
  items,
  loading,
  batchApplying,
  circuitBreakerEnabled,
  selectedModelIDs,
  onSelectedModelIDsChange,
  onEdit,
  onViewSources,
  pendingModelIDs,
  onToggleStatus,
  onToggleAccessScope,
  onDelete,
  onTestModel,
  onTestSource,
  onRefreshModels,
  onSourceAvailabilityChange,
  onSourceDeleteChange,
}: ModelsTableProps) {
  const t = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const [expandedRows, setExpandedRows] = React.useState<Set<number>>(new Set());
  const [openingRows, setOpeningRows] = React.useState<Set<number>>(new Set());
  const [collapsingRows, setCollapsingRows] = React.useState<Set<number>>(new Set());
  const [deleteSourceTarget, setDeleteSourceTarget] = React.useState<InlineSourceDeleteTarget | null>(null);
  const stableDeleteSourceTarget = useDialogSnapshot(deleteSourceTarget);
  const [circuitTarget, setCircuitTarget] = React.useState<InlineSourceCircuitTarget | null>(null);
  const {
    inlineSources,
    deleteSourcePending,
    circuitPending,
    ensureInlineSources,
    handleInlineCircuit,
    handleInlineStatusToggle,
    saveCircuitSettings,
    deleteInlineSource,
  } = useAdminModelsInlineSources({
    items,
    circuitBreakerEnabled,
    onRefreshModels,
    onSourceAvailabilityChange,
    onSourceDeleteChange,
  });
  const collapseTimersRef = React.useRef<Record<number, number>>({});
  const openFramesRef = React.useRef<Record<number, number>>({});
  const virtualRows = useVirtualTableRows(items, {
    enabled: items.length > 100,
    estimateSize: 40,
  });
  const initialLoading = loading && items.length === 0;
  const showRows = items.length > 0;

  const allModelsSelected = items.length > 0 && items.every((item) => selectedModelIDs.has(item.id));
  const someModelsSelected = items.some((item) => selectedModelIDs.has(item.id));

  const clearCollapseTimer = React.useCallback((id: number) => {
    const timer = collapseTimersRef.current[id];
    if (!timer) return;
    window.clearTimeout(timer);
    delete collapseTimersRef.current[id];
  }, []);

  const clearOpenFrame = React.useCallback((id: number) => {
    const frame = openFramesRef.current[id];
    if (!frame) return;
    window.cancelAnimationFrame(frame);
    delete openFramesRef.current[id];
  }, []);

  React.useEffect(() => {
    const timers = collapseTimersRef.current;
    const frames = openFramesRef.current;
    return () => {
      Object.values(timers).forEach((timer) => {
        window.clearTimeout(timer);
      });
      Object.values(frames).forEach((frame) => {
        window.cancelAnimationFrame(frame);
      });
    };
  }, []);

  const handleSelectAllModels = React.useCallback((checked: boolean) => {
    onSelectedModelIDsChange(checked ? new Set(items.map((item) => item.id)) : new Set());
  }, [items, onSelectedModelIDsChange]);

  const handleSelectModel = React.useCallback((id: number, checked: boolean) => {
    onSelectedModelIDsChange((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }, [onSelectedModelIDsChange]);

  const handleToggleRow = React.useCallback(
    async (item: AdminLLMModelDTO) => {
      if (expandedRows.has(item.id)) {
        clearCollapseTimer(item.id);
        clearOpenFrame(item.id);
        setOpeningRows((prev) => {
          if (!prev.has(item.id)) return prev;
          const next = new Set(prev);
          next.delete(item.id);
          return next;
        });
        setExpandedRows((prev) => {
          if (!prev.has(item.id)) return prev;
          const next = new Set(prev);
          next.delete(item.id);
          return next;
        });
        setCollapsingRows((prev) => {
          const next = new Set(prev);
          next.add(item.id);
          return next;
        });
        collapseTimersRef.current[item.id] = window.setTimeout(() => {
          setCollapsingRows((prev) => {
            if (!prev.has(item.id)) return prev;
            const next = new Set(prev);
            next.delete(item.id);
            return next;
          });
          delete collapseTimersRef.current[item.id];
        }, EXPANDED_ROW_ANIMATION_MS);
        return;
      }

      clearCollapseTimer(item.id);
      clearOpenFrame(item.id);
      setOpeningRows((prev) => {
        const next = new Set(prev);
        next.add(item.id);
        return next;
      });
      setCollapsingRows((prev) => {
        if (!prev.has(item.id)) return prev;
        const next = new Set(prev);
        next.delete(item.id);
        return next;
      });
      setExpandedRows((prev) => {
        if (prev.has(item.id)) return prev;
        const next = new Set(prev);
        next.add(item.id);
        return next;
      });
      openFramesRef.current[item.id] = window.requestAnimationFrame(() => {
        openFramesRef.current[item.id] = window.requestAnimationFrame(() => {
          setOpeningRows((prev) => {
            if (!prev.has(item.id)) return prev;
            const next = new Set(prev);
            next.delete(item.id);
            return next;
          });
          delete openFramesRef.current[item.id];
        });
      });

      await ensureInlineSources(item.id);
    },
    [clearCollapseTimer, clearOpenFrame, ensureInlineSources, expandedRows],
  );

  const handleInlineCircuitSettingsSave = React.useCallback(async (payload: ModelSourceCircuitPayload) => {
    if (!circuitTarget) {
      return;
    }
    if (await saveCircuitSettings(circuitTarget, payload)) {
      setCircuitTarget(null);
    }
  }, [circuitTarget, saveCircuitSettings]);

  const handleInlineSourceDelete = React.useCallback(async () => {
    if (!deleteSourceTarget) {
      return;
    }
    if (await deleteInlineSource(deleteSourceTarget)) {
      setDeleteSourceTarget(null);
    }
  }, [deleteInlineSource, deleteSourceTarget]);

  return (
    <>
    <Table
      viewportRef={virtualRows.viewportRef}
      viewportClassName={virtualRows.viewportClassName}
      viewportStyle={virtualRows.viewportStyle}
    >
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="w-[44px] py-1.5 text-center">
            <div className="flex h-7 items-center justify-center">
              <Checkbox
                checked={allModelsSelected ? true : someModelsSelected ? "indeterminate" : false}
                onCheckedChange={(checked) => handleSelectAllModels(checked === true)}
                aria-label={t("table.selectAllModels")}
              />
            </div>
          </TableHead>
          <TableHead>{t("platformModel")}</TableHead>
          <TableHead>{t("table.kind")}</TableHead>
          <TableHead>{t("sources.protocol")}</TableHead>
          <TableHead className="w-[120px]">{t("table.vendor")}</TableHead>
          <TableHead className="w-[96px] text-center">{t("table.sources")}</TableHead>
          <TableHead className="w-[72px] text-center">{t("fields.status")}</TableHead>
          <TableHead className="w-[112px]">{t("table.accessScope")}</TableHead>
          <TableHead className="w-[140px]">{t("sources.updatedAt")}</TableHead>
          <TableHead className="w-[56px]" stickyEnd />
        </TableRow>
      </TableHeader>

      <TableBody>
        {initialLoading ? (
          <TableLoadingRow colSpan={10} />
        ) : null}

        {items.length === 0 && !loading ? (
          <TableEmptyRow colSpan={10}>{t("table.empty")}</TableEmptyRow>
        ) : null}

        {showRows ? <VirtualTablePaddingRow colSpan={10} height={virtualRows.paddingTop} /> : null}
        {showRows
          ? virtualRows.rows.map(({ item }) => (
              <ModelTableRow
                key={item.id}
                item={item}
                circuitBreakerEnabled={circuitBreakerEnabled}
                selected={selectedModelIDs.has(item.id)}
                expanded={expandedRows.has(item.id) || collapsingRows.has(item.id)}
                opening={openingRows.has(item.id)}
                collapsing={collapsingRows.has(item.id)}
                inlineData={inlineSources[item.id]}
                onSelectModel={handleSelectModel}
                onToggleRow={handleToggleRow}
                onEdit={onEdit}
                onViewSources={onViewSources}
                updatePending={batchApplying || pendingModelIDs.has(item.id)}
                onToggleStatus={onToggleStatus}
                onToggleAccessScope={onToggleAccessScope}
                onDelete={onDelete}
                onTestModel={onTestModel}
                onTestSource={onTestSource}
                onInlineStatusToggle={handleInlineStatusToggle}
                onInlineCircuit={handleInlineCircuit}
                onInlineCircuitSettings={setCircuitTarget}
                onInlineSourceDeleteRequest={setDeleteSourceTarget}
              />
            ))
          : null}
        {showRows ? <VirtualTablePaddingRow colSpan={10} height={virtualRows.paddingBottom} /> : null}
      </TableBody>
    </Table>
    <AlertDialog
      open={deleteSourceTarget !== null}
      onOpenChange={(open) => {
        if (!open && !deleteSourcePending) {
          setDeleteSourceTarget(null);
        }
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("sources.deleteTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("sources.deleteDescription", {
              name: stableDeleteSourceTarget?.source.upstreamModelName ?? "",
            })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleteSourcePending}>
            {commonT("actions.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            onClick={(event) => {
              event.preventDefault();
              void handleInlineSourceDelete();
            }}
            disabled={deleteSourcePending}
          >
            {deleteSourcePending ? t("sources.deletingSource") : t("sources.confirmDeleteSource")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    <ModelSourceCircuitDialog
      source={circuitTarget?.source ?? null}
      policyMode={circuitTarget?.policyMode}
      pending={circuitPending}
      onClose={() => setCircuitTarget(null)}
      onSave={handleInlineCircuitSettingsSave}
    />
    </>
  );
}
