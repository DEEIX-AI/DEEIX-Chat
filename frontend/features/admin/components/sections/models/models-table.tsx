"use client";

import * as React from "react";
import { useLocale, useTranslations } from "next-intl";
import {
  Activity,
  CheckCircle2,
  CircleOff,
  CircleX,
  List,
  MoreHorizontal,
  Pencil,
  RotateCcw,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";

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
import { Switch } from "@/components/ui/switch";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useProgressiveRows } from "@/hooks/use-progressive-rows";
import {
  deleteAdminLLMUpstreamModel,
  listAdminLLMModelUpstreamSources,
  openAdminLLMUpstreamModelCircuit,
  resetAdminLLMUpstreamModelCircuit,
  updateAdminLLMModelUpstreamSource,
} from "@/features/admin/api";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";
import type {
  AdminLLMModelAccessScope,
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
} from "@/features/admin/api/llm.types";
import {
  ADAPTER_LABELS,
  formatDateTime,
  resolveErrorMessage,
  resolveValue,
} from "@/features/admin/types/llm";
import { sortProtocolsForDisplay } from "@/features/admin/utils/llm-display";
import { parseKindsJSON } from "@/shared/model/llm-schema";

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
          {["chat", "audio", "image_gen", "image_edit", "video_gen"].includes(kind)
            ? t(`kinds.${kind}`)
            : kind}
        </Badge>
      ))}
    </div>
  );
}

function SourceStatusText({
  status,
  circuitOpen,
  circuitUntil,
}: {
  status: AdminLLMStatus;
  circuitOpen: boolean;
  circuitUntil: string;
}) {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  if (circuitOpen) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-default text-xs text-destructive">{t("status.circuitOpen")}</span>
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs">
          {t("sources.circuitUntil", { time: formatCircuitUntil(circuitUntil, locale) })}
        </TooltipContent>
      </Tooltip>
    );
  }
  if (status === "inactive") {
    return (
      <CircleX
        className="size-4 text-muted-foreground"
        aria-label={t("status.inactive")}
      />
    );
  }
  return (
    <CheckCircle2
      className="size-4 text-emerald-600 dark:text-emerald-400"
      aria-label={t("status.active")}
    />
  );
}

type InlineSourceEntry = {
  items: AdminLLMModelUpstreamSourceDTO[];
  loading: boolean;
};

type InlineSourceDeleteTarget = {
  modelId: number;
  source: AdminLLMModelUpstreamSourceDTO;
};

type ModelsTableProps = {
  items: AdminLLMModelDTO[];
  loading: boolean;
  selectedModelIDs: Set<number>;
  onSelectedModelIDsChange: React.Dispatch<React.SetStateAction<Set<number>>>;
  onEdit: (item: AdminLLMModelDTO) => void;
  onViewSources: (item: AdminLLMModelDTO) => void;
  onToggleStatus: (item: AdminLLMModelDTO, status: AdminLLMStatus) => void;
  onToggleAccessScope: (item: AdminLLMModelDTO, scope: AdminLLMModelAccessScope) => void;
  onDelete: (item: AdminLLMModelDTO) => void;
  onTestModel?: (item: AdminLLMModelDTO) => void;
  onTestSource?: (source: AdminLLMModelUpstreamSourceDTO) => void;
  onSourceStatusChange?: (modelID: number, previous: AdminLLMStatus, next: AdminLLMStatus) => void;
  onSourceDeleteChange?: (modelID: number, source: AdminLLMModelUpstreamSourceDTO, deleted: boolean) => void;
};

type ModelTableRowProps = {
  item: AdminLLMModelDTO;
  selected: boolean;
  expanded: boolean;
  opening: boolean;
  collapsing: boolean;
  inlineData: InlineSourceEntry | undefined;
  onSelectModel: (id: number, checked: boolean) => void;
  onToggleRow: (item: AdminLLMModelDTO) => void;
  onEdit: (item: AdminLLMModelDTO) => void;
  onViewSources: (item: AdminLLMModelDTO) => void;
  onToggleStatus: (item: AdminLLMModelDTO, status: AdminLLMStatus) => void;
  onToggleAccessScope: (item: AdminLLMModelDTO, scope: AdminLLMModelAccessScope) => void;
  onDelete: (item: AdminLLMModelDTO) => void;
  onTestModel?: (item: AdminLLMModelDTO) => void;
  onTestSource?: (source: AdminLLMModelUpstreamSourceDTO) => void;
  onInlineStatusToggle: (source: AdminLLMModelUpstreamSourceDTO, modelId: number) => void;
  onInlineCircuit: (source: AdminLLMModelUpstreamSourceDTO, modelId: number, action: "open" | "reset") => void;
  onInlineSourceDeleteRequest: (target: InlineSourceDeleteTarget) => void;
};

function resolveModelProtocols(item: AdminLLMModelDTO): string[] {
  try {
    return item.protocolsJSON ? sortProtocolsForDisplay(JSON.parse(item.protocolsJSON) as string[]) : [];
  } catch {
    return [];
  }
}

const ModelTableRow = React.memo(function ModelTableRow({
  item,
  selected,
  expanded,
  opening,
  collapsing,
  inlineData,
  onSelectModel,
  onToggleRow,
  onEdit,
  onViewSources,
  onToggleStatus,
  onToggleAccessScope,
  onDelete,
  onTestModel,
  onTestSource,
  onInlineStatusToggle,
  onInlineCircuit,
  onInlineSourceDeleteRequest,
}: ModelTableRowProps) {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  const identity = resolveModelIdentity({
    code: item.platformModelName,
    vendor: item.vendor,
    icon: item.icon,
  });
  const iconURL = resolveLobeHubIconURL(identity.modelIcon);
  const vendorIconURL = resolveLobeHubIconURL(identity.vendorIcon);
  const titleText = item.platformModelName.trim();
  const protocols = resolveModelProtocols(item);

  return (
    <React.Fragment>
      <TableRow
        className="cursor-pointer"
        selected={selected}
        aria-expanded={expanded && !collapsing}
        onClick={() => onToggleRow(item)}
      >
        <TableCell className="w-[44px] py-0 whitespace-nowrap">
          <div className="flex h-8 items-center justify-center" onClick={(event) => event.stopPropagation()}>
            <Checkbox
              checked={selected}
              onCheckedChange={(checked) => onSelectModel(item.id, checked === true)}
              aria-label={t("table.selectModel", { name: item.platformModelName })}
            />
          </div>
        </TableCell>

        <TableCell className="py-1">
          <div className="flex min-w-0 items-center gap-2">
            <LobeHubIcon iconUrl={iconURL} label={titleText} />
            <div className="flex min-w-0 flex-1">
              <span className="truncate text-xs font-medium leading-5 text-foreground">
                {titleText}
              </span>
            </div>
          </div>
        </TableCell>

        <TableCell className="py-1">
          <KindsBadges kindsJson={item.kindsJSON} />
        </TableCell>

        <TableCell className="py-1">
          <ProtocolBadges protocols={protocols} />
        </TableCell>

        <TableCell className="w-[120px] py-1">
          {identity.vendorKey !== "unknown" ? (
            <div className="flex min-w-0 items-center gap-1.5">
              {vendorIconURL ? <LobeHubIcon iconUrl={vendorIconURL} label={identity.vendorLabel} size={14} /> : null}
              <span className="block max-w-[92px] truncate text-xs text-muted-foreground">
                {identity.vendorLabel}
              </span>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground">-</span>
          )}
        </TableCell>

        <TableCell className="whitespace-nowrap py-1 text-center">
          <span className="text-xs text-muted-foreground">
            {item.activeSourceCount}/{item.sourceCount}
          </span>
        </TableCell>

        <TableCell className="w-[72px] whitespace-nowrap py-1" onClick={(event) => event.stopPropagation()}>
          <div className="flex h-8 items-center justify-center">
            <Switch
              size="sm"
              checked={item.status === "active"}
              onCheckedChange={(checked) => onToggleStatus(item, checked ? "active" : "inactive")}
              aria-label={t("table.modelStatusAria", { name: item.platformModelName })}
            />
          </div>
        </TableCell>

        <TableCell className="w-[112px] whitespace-nowrap py-1" onClick={(event) => event.stopPropagation()}>
          <Select
            value={item.accessScope === "internal" ? "internal" : "public"}
            onValueChange={(value) => onToggleAccessScope(item, value as AdminLLMModelAccessScope)}
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
        </TableCell>

        <TableCell className="whitespace-nowrap py-1 text-muted-foreground">
          {formatDateTime(item.updatedAt, locale)}
        </TableCell>

        <TableCell
          className="w-[56px] whitespace-nowrap py-1"
          stickyEnd
          onClick={(event) => event.stopPropagation()}
        >
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
                <DropdownMenuItem onSelect={() => onToggleStatus(item, "inactive")}>
                  <CircleOff className="size-3.5 stroke-1" />
                  {t("table.disableModel")}
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem onSelect={() => onToggleStatus(item, "active")}>
                  <RotateCcw className="size-3.5 stroke-1" />
                  {t("table.enableModel")}
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() => onDelete(item)}
                className="text-destructive focus:text-destructive"
              >
                <Trash2 className="size-3.5 stroke-1" />
                {t("table.deleteModel")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      {expanded ? (
        <>
          {inlineData?.loading ? (
            <TableRow tone="muted">
              <CollapsibleTableCell
                colSpan={9}
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
              const sourceVendorIconURL = resolveLobeHubIconURL(sourceIdentity.vendorIcon);

              return (
                <TableRow key={source.id} tone="muted">
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[44px] py-0 whitespace-nowrap"
                  >
                    <div className="flex h-8 items-center justify-center">
                      <span className="size-1.5 rounded-full bg-muted-foreground/40" />
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="py-1">
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
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="py-1">
                    <KindsBadges kindsJson={source.upstreamModelKindsJSON} />
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="py-1">
                    <SingleProtocolText protocol={source.protocol} />
                  </CollapsibleTableCell>
                  <CollapsibleTableCell opening={opening} closing={collapsing} className="w-[120px] py-1">
                    {sourceIdentity.vendorKey !== "unknown" ? (
                      <div className="flex min-w-0 items-center gap-1.5">
                        {sourceVendorIconURL ? <LobeHubIcon iconUrl={sourceVendorIconURL} label={sourceIdentity.vendorLabel} size={14} /> : null}
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
                    className="py-1 text-center text-[11px] leading-4 text-muted-foreground"
                  >
                    <div className="max-w-[12rem] truncate" title={resolveValue(source.upstreamName)}>
                      {resolveValue(source.upstreamName)}
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[72px] whitespace-nowrap py-1"
                  >
                    <div className="flex h-8 items-center justify-center">
                      <SourceStatusText
                        status={source.status}
                        circuitOpen={source.circuitOpen}
                        circuitUntil={source.circuitUntil}
                      />
                    </div>
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="whitespace-nowrap py-1 text-[11px] leading-4 text-muted-foreground"
                  >
                    {formatDateTime(source.updatedAt, locale)}
                  </CollapsibleTableCell>
                  <CollapsibleTableCell
                    opening={opening}
                    closing={collapsing}
                    className="w-[56px] whitespace-nowrap py-1"
                    stickyEnd
                  >
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
                          <>
                            <DropdownMenuItem onSelect={() => onTestSource(source)}>
                              <Activity className="size-3.5 stroke-1" />
                              {t("actions.test")}
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                          </>
                        ) : null}
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
                        <DropdownMenuSeparator />
                        {source.circuitOpen ? (
                          <DropdownMenuItem onSelect={() => onInlineCircuit(source, item.id, "reset")}>
                            <RotateCcw className="size-3.5 stroke-1" />
                            {t("sources.resetCircuit")}
                          </DropdownMenuItem>
                        ) : (
                          <DropdownMenuItem onSelect={() => onInlineCircuit(source, item.id, "open")}>
                            <CircleOff className="size-3.5 stroke-1" />
                            {t("sources.openCircuit")}
                          </DropdownMenuItem>
                        )}
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
                  </CollapsibleTableCell>
                </TableRow>
              );
            })
          ) : (
            <TableRow tone="muted">
              <CollapsibleTableCell
                colSpan={9}
                opening={opening}
                closing={collapsing}
                className="py-3 pl-16 text-xs text-muted-foreground"
              >
                {t("sources.empty")}
              </CollapsibleTableCell>
            </TableRow>
          )}
        </>
      ) : null}
    </React.Fragment>
  );
});

export function ModelsTable({
  items,
  loading,
  selectedModelIDs,
  onSelectedModelIDsChange,
  onEdit,
  onViewSources,
  onToggleStatus,
  onToggleAccessScope,
  onDelete,
  onTestModel,
  onTestSource,
  onSourceStatusChange,
  onSourceDeleteChange,
}: ModelsTableProps) {
  const t = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const [expandedRows, setExpandedRows] = React.useState<Set<number>>(new Set());
  const [openingRows, setOpeningRows] = React.useState<Set<number>>(new Set());
  const [collapsingRows, setCollapsingRows] = React.useState<Set<number>>(new Set());
  const [inlineSources, setInlineSources] = React.useState<Record<number, InlineSourceEntry>>({});
  const [deleteSourceTarget, setDeleteSourceTarget] = React.useState<InlineSourceDeleteTarget | null>(null);
  const [deleteSourcePending, setDeleteSourcePending] = React.useState(false);
  const inlineSourcesRef = React.useRef(inlineSources);
  const collapseTimersRef = React.useRef<Record<number, number>>({});
  const openFramesRef = React.useRef<Record<number, number>>({});
  const { visibleRows: renderedItems } = useProgressiveRows(items, {
    initialCount: 12,
    step: 14,
    disabled: loading,
  });

  const allModelsSelected = items.length > 0 && items.every((item) => selectedModelIDs.has(item.id));
  const someModelsSelected = items.some((item) => selectedModelIDs.has(item.id));

  React.useEffect(() => {
    inlineSourcesRef.current = inlineSources;
  }, [inlineSources]);

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
      Object.values(timers).forEach((timer) => window.clearTimeout(timer));
      Object.values(frames).forEach((frame) => window.cancelAnimationFrame(frame));
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

  const refreshInlineSources = React.useCallback(async (modelId: number) => {
    const token = await resolveAccessToken();
    if (!token) return;
    const data = await listAdminLLMModelUpstreamSources(token, modelId, {
      page: 1,
      pageSize: 100,
    });
    const nextEntry = { items: data.results, loading: false };
    inlineSourcesRef.current = {
      ...inlineSourcesRef.current,
      [modelId]: nextEntry,
    };
    setInlineSources((prev) => ({
      ...prev,
      [modelId]: nextEntry,
    }));
  }, []);

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

      if (!inlineSourcesRef.current[item.id]) {
        const loadingEntry = { items: [], loading: true };
        inlineSourcesRef.current = {
          ...inlineSourcesRef.current,
          [item.id]: loadingEntry,
        };
        setInlineSources((prev) => ({
          ...prev,
          [item.id]: loadingEntry,
        }));
        try {
          await refreshInlineSources(item.id);
        } catch {
          const failedEntry = { items: [], loading: false };
          inlineSourcesRef.current = {
            ...inlineSourcesRef.current,
            [item.id]: failedEntry,
          };
          setInlineSources((prev) => ({
            ...prev,
            [item.id]: failedEntry,
          }));
        }
      }
    },
    [clearCollapseTimer, clearOpenFrame, expandedRows, refreshInlineSources],
  );

  const handleInlineCircuit = React.useCallback(
    async (
      source: AdminLLMModelUpstreamSourceDTO,
      modelId: number,
      action: "open" | "reset",
    ) => {
      const token = await resolveAccessToken();
      if (!token) return;
      const nextSource =
        action === "open"
          ? {
              ...source,
              circuitOpen: true,
              circuitUntil: String(Math.floor(Date.now() / 1000) + 24 * 60 * 60),
            }
          : { ...source, circuitOpen: false, circuitUntil: "" };
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: {
          ...(prev[modelId] ?? { items: [], loading: false }),
          items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? nextSource : item)),
        },
      }));
      try {
        if (action === "open") {
          await openAdminLLMUpstreamModelCircuit(token, source.upstreamID, source.id);
          toast.success(t("toast.circuitOpened"));
        } else {
          await resetAdminLLMUpstreamModelCircuit(token, source.upstreamID, source.id);
          toast.success(t("toast.circuitReset"));
        }
      } catch (error) {
        setInlineSources((prev) => ({
          ...prev,
          [modelId]: {
            ...(prev[modelId] ?? { items: [], loading: false }),
            items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? source : item)),
          },
        }));
        toast.error(t("toast.operationFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [t],
  );

  const handleInlineStatusToggle = React.useCallback(
    async (source: AdminLLMModelUpstreamSourceDTO, modelId: number) => {
      const token = await resolveAccessToken();
      if (!token) return;

      const nextStatus = source.status === "active" ? "inactive" : "active";
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: {
          ...(prev[modelId] ?? { items: [], loading: false }),
          items: (prev[modelId]?.items ?? []).map((item) =>
            item.id === source.id ? { ...item, status: nextStatus } : item,
          ),
        },
      }));
      onSourceStatusChange?.(modelId, source.status, nextStatus);
      try {
        const data = await updateAdminLLMModelUpstreamSource(token, modelId, source.id, {
          status: nextStatus,
        });
        setInlineSources((prev) => ({
          ...prev,
          [modelId]: {
            ...(prev[modelId] ?? { items: [], loading: false }),
            items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? data.source : item)),
          },
        }));
        toast.success(nextStatus === "inactive" ? t("toast.sourceDisabled") : t("toast.sourceEnabled"));
      } catch (error) {
        setInlineSources((prev) => ({
          ...prev,
          [modelId]: {
            ...(prev[modelId] ?? { items: [], loading: false }),
            items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? source : item)),
          },
        }));
        onSourceStatusChange?.(modelId, nextStatus, source.status);
        toast.error(t("toast.operationFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [onSourceStatusChange, t],
  );

  const handleInlineSourceDelete = React.useCallback(async () => {
    if (!deleteSourceTarget || deleteSourcePending) {
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return;
    }

    const { modelId, source } = deleteSourceTarget;
    const previousEntry = inlineSourcesRef.current[modelId] ?? { items: [], loading: false };
    setDeleteSourcePending(true);
    setInlineSources((prev) => ({
      ...prev,
      [modelId]: {
        ...(prev[modelId] ?? { items: [], loading: false }),
        items: (prev[modelId]?.items ?? []).filter((item) => item.id !== source.id),
      },
    }));
    onSourceDeleteChange?.(modelId, source, true);

    try {
      await deleteAdminLLMUpstreamModel(token, source.upstreamID, source.id);
      toast.success(t("toast.sourceDeleted"));
      setDeleteSourceTarget(null);
    } catch (error) {
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: previousEntry,
      }));
      onSourceDeleteChange?.(modelId, source, false);
      toast.error(t("toast.sourceDeleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setDeleteSourcePending(false);
    }
  }, [deleteSourcePending, deleteSourceTarget, onSourceDeleteChange, t]);

  return (
    <>
    <Table>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="w-[44px] py-0 text-center">
            <div className="flex h-8 items-center justify-center">
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
        {loading && items.length === 0 ? (
          <TableSkeletonRows colSpan={10} rowCount={10} />
        ) : null}

        {items.length === 0 && !loading ? (
          <TableEmptyRow colSpan={10}>{t("table.empty")}</TableEmptyRow>
        ) : null}

        {renderedItems.map((item) => (
          <ModelTableRow
            key={item.id}
            item={item}
            selected={selectedModelIDs.has(item.id)}
            expanded={expandedRows.has(item.id) || collapsingRows.has(item.id)}
            opening={openingRows.has(item.id)}
            collapsing={collapsingRows.has(item.id)}
            inlineData={inlineSources[item.id]}
            onSelectModel={handleSelectModel}
            onToggleRow={handleToggleRow}
            onEdit={onEdit}
            onViewSources={onViewSources}
            onToggleStatus={onToggleStatus}
            onToggleAccessScope={onToggleAccessScope}
            onDelete={onDelete}
            onTestModel={onTestModel}
            onTestSource={onTestSource}
            onInlineStatusToggle={handleInlineStatusToggle}
            onInlineCircuit={handleInlineCircuit}
            onInlineSourceDeleteRequest={setDeleteSourceTarget}
          />
        ))}
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
              name: deleteSourceTarget?.source.upstreamModelName ?? "",
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
    </>
  );
}
