"use client";

import * as React from "react";
import { useLocale, useTranslations } from "next-intl";

import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
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
import type { AdminErrorLogDTO } from "@/features/admin/api/admin-types";
import type { AdminErrorLogStatusClass } from "@/features/admin/api/audit";
import { useAdminLogsErrors } from "@/features/admin/hooks/use-admin-logs-errors";
import { ADMIN_ERROR_LOG_STATUS_CLASSES } from "@/features/admin/model/admin-unions";
import { cn } from "@/lib/utils";
import { CopyActionButton } from "@/shared/components/copy-action";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { isOneOf } from "@/shared/lib/type-guards";

const isErrorLogStatusClass = isOneOf(ADMIN_ERROR_LOG_STATUS_CLASSES);

// Mirrors the backend marker written when a 5xx left no cause, so it can be shown as a localized hint.
const NO_CAUSE_RECORDED = "<no cause recorded>";

function formatDateTime(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function resolveUserName(item: AdminErrorLogDTO): string {
  const name = item.userLabel?.trim() || item.userDisplayName?.trim() || item.userUsername?.trim();
  if (name) return name;
  return item.userID > 0 ? String(item.userID) : "-";
}

// Stream failures end with a terminal error event after a 2xx response header.
function isStreamFailure(item: AdminErrorLogDTO): boolean {
  return item.statusCode < 400;
}

function DetailRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[96px_minmax(0,1fr)] gap-3 border-b border-border/50 py-2.5 last:border-b-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className={cn("min-w-0 break-words text-xs leading-5 text-foreground/86", mono && "font-mono")}>
        {value ?? "-"}
      </div>
    </div>
  );
}

function DetailBlock({
  title,
  action,
  children,
}: {
  title: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-2">
      <div className="flex items-center justify-between gap-2 px-1">
        <h4 className="text-xs font-medium text-foreground/88">{title}</h4>
        {action}
      </div>
      {children}
    </section>
  );
}

// Upstream bodies may be JSON, plain text or stream fragments; only parseable JSON gets a formatted view.
function formatUpstreamDetail(detail: string): string | null {
  const trimmed = detail.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return null;
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return null;
  }
}

function UpstreamDetailBlock({ detail }: { detail: string }) {
  const t = useTranslations("adminLogs.errors.detail");
  const [raw, setRaw] = React.useState(false);
  const formatted = React.useMemo(() => formatUpstreamDetail(detail), [detail]);
  const showRaw = raw || formatted === null;
  const body = showRaw ? detail : (formatted ?? detail);

  return (
    <DetailBlock
      title={t("upstream")}
      action={
        <div className="flex items-center gap-1">
          {formatted !== null ? (
            <div className="flex items-center rounded-md bg-foreground/[0.05] p-0.5">
              {[
                { key: false, label: t("viewFormatted") },
                { key: true, label: t("viewRaw") },
              ].map((option) => (
                <button
                  key={option.label}
                  type="button"
                  onClick={() => setRaw(option.key)}
                  className={cn(
                    "rounded px-1.5 py-0.5 text-[11px] transition-colors",
                    showRaw === option.key
                      ? "bg-background text-foreground shadow-xs"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>
          ) : null}
          <CopyActionButton
            type="button"
            variant="ghost"
            size="icon"
            className="size-6 text-muted-foreground"
            value={detail}
            aria-label={t("copyUpstream")}
            messages={{ copied: t("copied"), failed: t("copyFailed") }}
          />
        </div>
      }
    >
      <pre className="max-h-72 overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 font-mono text-[11.5px] leading-5 whitespace-pre-wrap break-all text-foreground/86">
        {body}
      </pre>
    </DetailBlock>
  );
}

function ErrorLogDetailSheet({ item, onClose }: { item: AdminErrorLogDTO | null; onClose: () => void }) {
  const t = useTranslations("adminLogs.errors");
  const locale = useLocale();
  // Keep rendering the last item while the sheet animates closed.
  const snapshot = useDialogSnapshot(item);
  const cause = snapshot?.cause?.trim() ?? "";
  const missingCause = !cause || cause === NO_CAUSE_RECORDED;

  return (
    <Sheet open={Boolean(item)} onOpenChange={(next) => !next && onClose()}>
      <SheetContent className="w-full sm:max-w-[560px]">
        <SheetHeader>
          <SheetTitle>{t("detail.title")}</SheetTitle>
          <SheetDescription className="font-mono">{snapshot?.requestID ?? ""}</SheetDescription>
        </SheetHeader>
        {snapshot ? (
          <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 pb-6">
            <DetailBlock
              title={t("detail.cause")}
              action={
                missingCause ? null : (
                  <CopyActionButton
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-6 text-muted-foreground"
                    value={cause}
                    aria-label={t("detail.copyCause")}
                    messages={{ copied: t("detail.copied"), failed: t("detail.copyFailed") }}
                  />
                )
              }
            >
              {missingCause ? (
                <p className="rounded-lg border border-border/60 bg-muted/25 px-3 py-2.5 text-xs text-muted-foreground">
                  {t("noCause")}
                </p>
              ) : (
                <pre className="max-h-72 overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 font-mono text-xs leading-5 whitespace-pre-wrap break-words text-foreground/86">
                  {cause}
                </pre>
              )}
            </DetailBlock>

            <DetailBlock title={t("detail.request")}>
              <div className="rounded-lg border border-border/60 bg-background px-3">
                <DetailRow label={t("detail.requestID")} value={snapshot.requestID} mono />
                <DetailRow label={t("detail.traceID")} value={snapshot.traceID || "-"} mono />
                <DetailRow label={t("detail.method")} value={snapshot.method || "-"} mono />
                <DetailRow label={t("detail.route")} value={snapshot.route || "-"} mono />
                <DetailRow label={t("detail.path")} value={snapshot.path || "-"} mono />
                <DetailRow
                  label={t("detail.status")}
                  value={
                    isStreamFailure(snapshot)
                      ? `${snapshot.statusCode} · ${t("statusClass.stream")}`
                      : String(snapshot.statusCode)
                  }
                  mono
                />
                <DetailRow label={t("detail.errorCode")} value={snapshot.errorCode || "-"} mono />
                <DetailRow label={t("detail.latency")} value={`${snapshot.latencyMS}ms`} mono />
                <DetailRow label={t("detail.time")} value={formatDateTime(snapshot.createdAt, locale)} />
                <DetailRow label={t("detail.user")} value={resolveUserName(snapshot)} />
              </div>
            </DetailBlock>

            {snapshot.detail?.trim() ? <UpstreamDetailBlock detail={snapshot.detail} /> : null}

            <DetailBlock title={t("detail.client")}>
              <div className="rounded-lg border border-border/60 bg-background px-3">
                <DetailRow label={t("detail.clientIP")} value={snapshot.clientIP || "-"} mono />
                <DetailRow label={t("detail.userAgent")} value={snapshot.userAgent || "-"} />
              </div>
            </DetailBlock>

            <p className="px-1 text-xs leading-5 text-muted-foreground">{t("detail.relatedHint")}</p>
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

export function ErrorLogTable() {
  const locale = useLocale();
  const t = useTranslations("adminLogs.errors");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(25);
  const [query, setQuery] = React.useState("");
  const [statusClass, setStatusClass] = React.useState<AdminErrorLogStatusClass | "">("");
  const [selected, setSelected] = React.useState<AdminErrorLogDTO | null>(null);
  const { loading, items, total, load } = useAdminLogsErrors({ page, pageSize, query, statusClass });
  const pageCount = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
  const closeDetail = React.useCallback(() => setSelected(null), []);

  return (
    <div className="space-y-3">
      <TableToolbar
        query={query}
        onQueryChange={(value) => {
          setQuery(value);
          setPage(1);
        }}
        queryPlaceholder={t("searchPlaceholder")}
        filters={[
          {
            key: "statusClass",
            label: t("statusClass.label"),
            value: statusClass,
            onValueChange: (value) => {
              setStatusClass(isErrorLogStatusClass(value) ? value : "");
              setPage(1);
            },
            options: [
              { label: t("statusClass.all"), value: "" },
              { label: t("statusClass.server"), value: "5xx" },
              { label: t("statusClass.stream"), value: "stream" },
            ],
          },
        ]}
        loading={loading}
        onRefresh={() => void load()}
      />

      {/* The shell clips overflow and the viewport is display:table, so the columns must fit the
          container: cap wide cells with an inner max-w + truncate instead of fixed header widths. */}
      <Table shellClassName="w-full max-w-full">
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="whitespace-nowrap">{t("columns.requestID")}</TableHead>
            <TableHead className="whitespace-nowrap">{t("columns.user")}</TableHead>
            <TableHead className="whitespace-nowrap">{t("columns.route")}</TableHead>
            <TableHead className="whitespace-nowrap">{t("columns.status")}</TableHead>
            <TableHead className="whitespace-nowrap">{t("columns.cause")}</TableHead>
            <TableHead className="whitespace-nowrap">{t("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && items.length === 0 ? <TableLoadingRow colSpan={6} /> : null}
          {!loading && items.length === 0 ? <TableEmptyRow colSpan={6}>{t("empty")}</TableEmptyRow> : null}
          {items.map((item) => {
            const cause = item.cause?.trim() ?? "";
            return (
              <TableRow key={item.id} className="cursor-pointer" onClick={() => setSelected(item)}>
                <TableCell className="font-mono text-xs text-foreground">
                  <div className="max-w-[13rem] truncate" title={item.requestID}>{item.requestID}</div>
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">
                  <div className="max-w-[7rem] truncate" title={resolveUserName(item)}>{resolveUserName(item)}</div>
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  <div className="max-w-[12rem] truncate" title={`${item.method} ${item.route}`}>
                    <span className="text-foreground/70">{item.method}</span> {item.route || "-"}
                  </div>
                </TableCell>
                {/* Status plus error code stays within ~10rem so the widest row cannot stretch the table. */}
                <TableCell className="whitespace-nowrap text-xs">
                  <span className={cn("font-mono tabular-nums", isStreamFailure(item) ? "text-muted-foreground" : "text-destructive")}>
                    {isStreamFailure(item) ? t("statusClass.stream") : item.statusCode}
                  </span>
                  {item.errorCode ? (
                    <span className="ml-1.5 inline-block max-w-[6.5rem] truncate align-bottom font-mono text-muted-foreground" title={item.errorCode}>
                      {item.errorCode}
                    </span>
                  ) : null}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  <div className="max-w-[16rem] truncate" title={cause}>
                    {!cause || cause === NO_CAUSE_RECORDED ? t("noCause") : cause}
                  </div>
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>

      <TablePagination
        page={page}
        pageCount={pageCount}
        pageSize={pageSize}
        total={total}
        loading={loading}
        onPageChange={setPage}
        onPageSizeChange={(next) => {
          setPageSize(next);
          setPage(1);
        }}
      />

      <ErrorLogDetailSheet item={selected} onClose={closeDetail} />
    </div>
  );
}
