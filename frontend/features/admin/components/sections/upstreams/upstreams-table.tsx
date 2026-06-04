import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { useLocale, useTranslations } from "next-intl";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import type { AdminLLMUpstreamView } from "@/features/admin/api/llm.types";
import { resolveCompatibleLabel, resolveProtocolLabel } from "@/features/admin/utils/llm-display";
import { CircleOff, CloudDownload, MoreHorizontal, Pencil, RotateCcw, Settings2, Trash2, Zap } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useProgressiveRows } from "@/hooks/use-progressive-rows";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PROTOCOL_DEFAULT_KIND_ORDER = [
  "chat",
  "audio",
  "image_gen",
  "image_edit",
  "video_gen",
];
const PROTOCOL_DEFAULT_KINDS = new Set(PROTOCOL_DEFAULT_KIND_ORDER);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function msToS(ms: number): string {
  if (!ms) return "-";
  return String(Math.round(ms / 1000));
}

function formatDateTime(value: string, locale: string): string {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

function formatCircuitUntil(until: string, locale: string, unknown: string): string {
  if (!until) return unknown;
  const ts = Number(until);
  const d = Number.isFinite(ts) ? new Date(ts * 1000) : new Date(until);
  if (Number.isNaN(d.getTime())) return until;
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

function parseProtocolDefaults(raw: string): Array<{ kind: string; protocol: string }> {
  if (!raw.trim()) return [];
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
      return [];
    }
    return protocolDefaultEntries(parsed as Record<string, unknown>)
      .map(([kind, protocol]) => ({ kind, protocol: String(protocol) }));
  } catch {
    return [];
  }
}

function protocolDefaultEntries(parsed: Record<string, unknown>): Array<[string, string]> {
  return Object.entries(parsed)
    .filter(([kind, value]) => PROTOCOL_DEFAULT_KINDS.has(kind) && typeof value === "string" && value.trim())
    .sort(
      ([left], [right]) =>
        PROTOCOL_DEFAULT_KIND_ORDER.indexOf(left) - PROTOCOL_DEFAULT_KIND_ORDER.indexOf(right),
    ) as Array<[string, string]>;
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

type UpstreamsTableProps = {
  items: AdminLLMUpstreamView[];
  loading: boolean;
  selected: Set<number>;
  togglingStatusIDs: Set<number>;
  onSelectAll: (checked: boolean) => void;
  onSelectOne: (id: number, checked: boolean) => void;
  onEdit: (item: AdminLLMUpstreamView) => void;
  onManageModels: (item: AdminLLMUpstreamView) => void;
  onSyncModels: (item: AdminLLMUpstreamView) => void;
  onCircuitAction: (
    item: AdminLLMUpstreamView,
    action: "open" | "reset",
  ) => void;
  onToggleStatus: (item: AdminLLMUpstreamView) => void;
  onDelete: (item: AdminLLMUpstreamView) => void;
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function UpstreamsTable({
  items,
  loading,
  selected,
  togglingStatusIDs,
  onSelectAll,
  onSelectOne,
  onEdit,
  onManageModels,
  onSyncModels,
  onCircuitAction,
  onToggleStatus,
  onDelete,
}: UpstreamsTableProps) {
  const locale = useLocale();
  const t = useTranslations("adminChannels");
  const allSelected = items.length > 0 && items.every((item) => selected.has(item.id));
  const someSelected = items.some((item) => selected.has(item.id));
  const { visibleRows: renderedItems } = useProgressiveRows(items, {
    initialCount: 12,
    step: 16,
    disabled: loading,
  });

  return (
    <Table>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="w-[44px] py-1.5 text-center">
            <div className="flex h-7 items-center justify-center">
              <Checkbox
                checked={allSelected ? true : someSelected ? "indeterminate" : false}
                onCheckedChange={(checked) => onSelectAll(checked === true)}
                aria-label={t("table.selectAll")}
              />
            </div>
          </TableHead>
          <TableHead>{t("table.id")}</TableHead>
          <TableHead>{t("table.name")}</TableHead>
          <TableHead>{t("table.url")}</TableHead>
          <TableHead>{t("table.compatibilityProtocol")}</TableHead>
          <TableHead className="text-center">{t("fields.status")}</TableHead>
          <TableHead>{t("table.modelCount")}</TableHead>
          <TableHead>{t("table.timeouts")}</TableHead>
          <TableHead>{t("table.updatedAt")}</TableHead>
          <TableHead className="w-[56px]" stickyEnd />
        </TableRow>
      </TableHeader>
      <TableBody>
        {loading && items.length === 0 ? (
          <TableSkeletonRows colSpan={10} rowCount={10} />
        ) : null}

        {items.length === 0 && !loading ? (
          <TableEmptyRow colSpan={10}>{t("table.empty")}</TableEmptyRow>
        ) : (
          renderedItems.map((item) => {
            const protocolDefaults = parseProtocolDefaults(item.protocolDefaultsJSON);

            return (
              <TableRow
                key={item.id}
                selected={selected.has(item.id)}
              >
                <TableCell className="w-[44px] py-1.5 whitespace-nowrap text-center">
                  <div className="flex h-7 items-center justify-center">
                    <Checkbox
                      checked={selected.has(item.id)}
                      onCheckedChange={(checked) =>
                        onSelectOne(item.id, checked === true)
                      }
                      aria-label={t("table.selectRow", { name: item.name })}
                    />
                  </div>
                </TableCell>

                <TableCell className="py-1.5 whitespace-nowrap font-mono text-xs text-muted-foreground">
                  <span className="flex h-7 items-center">{item.id}</span>
                </TableCell>

                <TableCell>
                  <div className="max-w-[18rem] truncate whitespace-nowrap">
                    <span className="font-medium">{item.name}</span>
                  </div>
                </TableCell>

                <TableCell>
                  <div
                    className="max-w-[12rem] truncate text-xs text-muted-foreground"
                    title={item.baseURL}
                  >
                    {item.baseURL}
                  </div>
                </TableCell>

                <TableCell className="whitespace-nowrap">
                  <div className="flex items-center gap-1.5">
                    <Badge variant="secondary">
                      {item.compatible === "custom" ? t("compatible.custom") : resolveCompatibleLabel(item.compatible)}
                    </Badge>
                    {protocolDefaults.length > 0 ? (
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Badge
                            variant="secondary"
                            className="max-w-32 cursor-default truncate text-muted-foreground"
                          >
                            {t("table.protocolDefaultsCount", { count: protocolDefaults.length })}
                          </Badge>
                        </TooltipTrigger>
                        <TooltipContent side="top">
                          <div className="space-y-1.5">
                            {protocolDefaults.map((entry) => (
                              <div key={`${entry.kind}:${entry.protocol}`} className="grid grid-cols-[4rem_minmax(0,1fr)] items-center gap-3">
                                <span className="text-xs">
                                  {t(`kinds.${entry.kind}`)}
                                </span>
                                <span className="truncate text-xs">
                                  {resolveProtocolLabel(entry.protocol)}
                                </span>
                              </div>
                            ))}
                          </div>
                        </TooltipContent>
                      </Tooltip>
                    ) : (
                      <span className="text-xs text-muted-foreground">{t("table.noDefaults")}</span>
                    )}
                  </div>
                </TableCell>

                <TableCell className="py-1.5 whitespace-nowrap">
                  <div className="flex h-7 items-center justify-center gap-2">
                    <Switch
                      size="sm"
                      checked={item.status === "active"}
                      disabled={togglingStatusIDs.has(item.id)}
                      onCheckedChange={() => onToggleStatus(item)}
                      aria-label={t("table.toggleStatus", {
                        action: item.status === "active" ? t("actions.disable") : t("actions.enable"),
                        name: item.name,
                      })}
                    />
                    {item.circuitOpen ? (
                      <Badge
                        variant="destructive"
                        title={t("table.circuitUntil", {
                          time: formatCircuitUntil(item.circuitUntil, locale, t("table.unknown")),
                        })}
                      >
                        {t("status.circuitOpen")}
                      </Badge>
                    ) : null}
                  </div>
                </TableCell>

                <TableCell>
                  <div className="max-w-[10rem] truncate text-xs text-muted-foreground">
                    {t("table.modelCountSummary", { active: item.activeModelsCount, total: item.modelsCount })}
                  </div>
                </TableCell>

                <TableCell>
                  <div className="max-w-[15rem] truncate font-mono text-xs text-muted-foreground">
                    C {msToS(item.connectTimeoutMS)}s / R {msToS(item.readTimeoutMS)}s / S {msToS(item.streamIdleTimeoutMS)}s
                  </div>
                </TableCell>

                <TableCell className="whitespace-nowrap text-muted-foreground">
                  {formatDateTime(item.updatedAt, locale)}
                </TableCell>

                <TableCell className="w-[56px] py-1.5 whitespace-nowrap" stickyEnd>
                  <div className="flex h-7 items-center justify-end gap-1">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon-xs" className="text-muted-foreground shadow-none">
                          <MoreHorizontal className="size-3.5 stroke-1" />
                          <span className="sr-only">{t("table.actions")}</span>
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => onEdit(item)}>
                          <Pencil className="size-3.5 stroke-1" />
                          {t("actions.edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => onManageModels(item)}>
                          <Settings2 className="size-3.5 stroke-1" />
                          {t("actions.manageModels")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => onSyncModels(item)}>
                          <CloudDownload className="size-3.5 stroke-1" />
                          {t("actions.syncRemoteModels")}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        {item.circuitOpen ? (
                          <DropdownMenuItem
                            onClick={() => onCircuitAction(item, "reset")}
                          >
                            <RotateCcw className="size-3.5 stroke-1" />
                            {t("actions.resetCircuit")}
                          </DropdownMenuItem>
                        ) : (
                          <DropdownMenuItem
                            onClick={() => onCircuitAction(item, "open")}
                          >
                            <CircleOff className="size-3.5 stroke-1" />
                            {t("actions.openCircuit")}
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem onClick={() => onToggleStatus(item)}>
                          <Zap className="size-3.5 stroke-1" />
                          {item.status === "active" ? t("actions.disableUpstream") : t("actions.enableUpstream")}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          className="text-destructive"
                          onClick={() => onDelete(item)}
                        >
                          <Trash2 className="size-3.5 stroke-1" />
                          {t("actions.delete")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </TableCell>
              </TableRow>
            );
          })
        )}
      </TableBody>
    </Table>
  );
}
