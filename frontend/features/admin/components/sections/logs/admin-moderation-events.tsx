"use client";

import * as React from "react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Spinner } from "@/components/ui/spinner";
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
import {
  type ContentModerationEventDetail,
  type ModerationEvent,
  fetchContentModerationEventImage,
  getContentModerationEvent,
  listContentModerationEvents,
} from "@/features/admin/api/content-moderation";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { cn } from "@/lib/utils";

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
  }).format(date);
}

function resolveUserDisplayName(label: string | undefined, username: string | undefined, fallbackID: number): string {
  const name = (label ?? "").trim() || (username ?? "").trim();
  return name || (fallbackID > 0 ? String(fallbackID) : "-");
}

function resolveLabel(value: string, labels: Record<string, string>): string {
  return labels[value] ?? value;
}

function normalizeModerationSearchQuery(
  query: string,
  labels: Record<string, string>,
  locale: string,
): string {
  const value = query.trim();
  if (!value) return "";
  const normalized = value.toLocaleLowerCase(locale);
  return Object.entries(labels).find(
    ([, label]) => label.trim().toLocaleLowerCase(locale) === normalized,
  )?.[0] ?? value;
}

function DetailRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[88px_minmax(0,1fr)] gap-3 border-b border-border/50 py-2.5 last:border-b-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className={cn("min-w-0 break-words text-xs leading-5 text-foreground/86", mono && "font-mono")}>
        {value ?? "-"}
      </div>
    </div>
  );
}

function DetailBlock({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h4 className="px-1 text-xs font-medium text-foreground/88">{title}</h4>
      <div className="rounded-lg border border-border/60 bg-background px-3">{children}</div>
    </section>
  );
}

function ModerationEventDetailSheet({
  eventID,
  open,
  onClose,
}: {
  eventID: string;
  open: boolean;
  onClose: () => void;
}) {
  const t = useTranslations("adminLogs.moderation");
  const locale = useLocale();
  const [loading, setLoading] = React.useState(false);
  const [detail, setDetail] = React.useState<ContentModerationEventDetail | null>(null);
  const [images, setImages] = React.useState<Array<{ index: number; url: string }>>([]);
  const requestRef = React.useRef(0);
  const imagesRef = React.useRef<Array<{ index: number; url: string }>>([]);
  const snap = useDialogSnapshot(open ? { eventID } : null);

  const revokeImages = React.useCallback((items: Array<{ index: number; url: string }>) => {
    for (const item of items) URL.revokeObjectURL(item.url);
  }, []);

  const replaceImages = React.useCallback(
    (items: Array<{ index: number; url: string }>) => {
      revokeImages(imagesRef.current);
      imagesRef.current = items;
      setImages(items);
    },
    [revokeImages],
  );

  React.useEffect(() => {
    if (!open || !eventID) return;
    const requestID = ++requestRef.current;
    setLoading(true);
    setDetail(null);
    replaceImages([]);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const loaded = await getContentModerationEvent(token, eventID);
        if (requestRef.current !== requestID) return;
        setDetail(loaded);
        if (loaded.imagesAvailable && Array.isArray(loaded.images) && loaded.images.length > 0) {
          const loadedImages: Array<{ index: number; url: string }> = [];
          for (const image of loaded.images) {
            try {
              const { blob } = await fetchContentModerationEventImage(token, eventID, image.index);
              const url = URL.createObjectURL(blob);
              if (requestRef.current !== requestID) {
                URL.revokeObjectURL(url);
                revokeImages(loadedImages);
                return;
              }
              loadedImages.push({ index: image.index, url });
            } catch {
              if (requestRef.current !== requestID) {
                revokeImages(loadedImages);
                return;
              }
              toast.error(t("imageLoadFailed"));
            }
          }
          if (requestRef.current === requestID) replaceImages(loadedImages);
          else revokeImages(loadedImages);
        }
      } catch (error) {
        if (requestRef.current === requestID) {
          toast.error(t("detailFailed"), { description: resolveAdminErrorMessage(error) });
          onClose();
        }
      } finally {
        if (requestRef.current === requestID) setLoading(false);
      }
    })();
    return () => {
      requestRef.current += 1;
    };
  }, [eventID, onClose, open, replaceImages, revokeImages, t]);

  React.useEffect(() => {
    if (open) return;
    revokeImages(imagesRef.current);
    imagesRef.current = [];
  }, [open, revokeImages]);

  React.useEffect(() => {
    return () => {
      requestRef.current += 1;
      revokeImages(imagesRef.current);
      imagesRef.current = [];
    };
  }, [revokeImages]);

  const event = detail?.event;
  const resultLabels = {
    passed: t("results.passed"),
    hit: t("results.hit"),
    failed_open: t("results.failedOpen"),
  };
  const directionLabels = {
    input: t("directions.input"),
    output: t("directions.output"),
  };
  const modalityLabels = {
    text: t("modalities.text"),
    image: t("modalities.image"),
  };
  const description = snap
    ? `${event ? resolveLabel(event.result, resultLabels) : eventID} · ${formatDateTime(event?.createdAt, locale)}`
    : "";

  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent className="sm:max-w-[480px]">
        <SheetHeader>
          <SheetTitle>{t("detailTitle")}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 pb-6">
          {loading ? (
            <div className="flex h-28 items-center justify-center rounded-lg border border-border/60 bg-muted/20">
              <Spinner className="size-4 text-muted-foreground" />
            </div>
          ) : detail && event ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label={t("columns.eventId")} value={event.publicID} mono />
                <DetailRow label={t("columns.result")} value={resolveLabel(event.result, resultLabels)} />
                <DetailRow label={t("columns.direction")} value={resolveLabel(event.direction, directionLabels)} />
                <DetailRow label={t("columns.modality")} value={resolveLabel(event.modality, modalityLabels)} />
                <DetailRow label={t("columns.latency")} value={`${event.latencyMS}ms`} mono />
                <DetailRow label={t("columns.createdAt")} value={formatDateTime(event.createdAt, locale)} />
                <DetailRow label={t("columns.model")} value={event.model || "-"} mono />
                <DetailRow
                  label={t("columns.user")}
                  value={resolveUserDisplayName(
                    event.userLabel,
                    event.username,
                    event.userID,
                  )}
                />
                <DetailRow label={t("columns.userID")} value={event.userID} mono />
                <DetailRow label={t("columns.runID")} value={event.runID || "-"} mono />
              </DetailBlock>

              {detail.textAvailable && detail.decryptedText ? (
                <section className="space-y-2">
                  <h4 className="px-1 text-xs font-medium text-foreground/88">{t("detailText")}</h4>
                  <pre className="max-h-48 overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 whitespace-pre-wrap break-all text-foreground/86">
                    {detail.decryptedText}
                  </pre>
                </section>
              ) : null}

              {images.length > 0 ? (
                <section className="space-y-2">
                  <h4 className="px-1 text-xs font-medium text-foreground/88">{t("detailImages")}</h4>
                  <div className="flex flex-wrap gap-3">
                    {images.map((image) => (
                      <img
                        key={image.index}
                        src={image.url}
                        alt={t("detailImageAlt", { index: image.index })}
                        className="max-h-64 max-w-full rounded border object-contain"
                      />
                    ))}
                  </div>
                </section>
              ) : null}

              <section className="space-y-2">
                <h4 className="px-1 text-xs font-medium text-foreground/88">JSON</h4>
                <pre className="max-h-[280px] overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 whitespace-pre-wrap break-all text-foreground/86">
                  <code>
                    {JSON.stringify(
                      {
                        event: detail.event,
                        categoryScores: detail.categoryScores,
                        textAvailable: detail.textAvailable,
                        imagesAvailable: detail.imagesAvailable,
                        images: detail.images,
                      },
                      null,
                      2,
                    )}
                  </code>
                </pre>
              </section>
            </>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function ModerationEventTable() {
  const locale = useLocale();
  const t = useTranslations("adminLogs.moderation");
  const [loading, setLoading] = React.useState(true);
  const [items, setItems] = React.useState<ModerationEvent[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(20);
  const [query, setQuery] = React.useState("");
  const [resultFilter, setResultFilter] = React.useState("");
  const [directionFilter, setDirectionFilter] = React.useState("");
  const [selectedEventID, setSelectedEventID] = React.useState("");
  const requestRef = React.useRef(0);
  const resultLabels = {
    passed: t("results.passed"),
    hit: t("results.hit"),
    failed_open: t("results.failedOpen"),
  };
  const directionLabels = {
    input: t("directions.input"),
    output: t("directions.output"),
  };
  const modalityLabels = {
    text: t("modalities.text"),
    image: t("modalities.image"),
  };
  const searchQuery = normalizeModerationSearchQuery(
    query,
    { ...resultLabels, ...directionLabels, ...modalityLabels },
    locale,
  );

  const load = React.useCallback(async () => {
    const requestID = ++requestRef.current;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await listContentModerationEvents(token, {
        page,
        pageSize,
        query: searchQuery || undefined,
        result: resultFilter.trim() || undefined,
        direction: directionFilter.trim() || undefined,
      });
      if (requestRef.current !== requestID) return;
      setItems(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (error) {
      if (requestRef.current === requestID) {
        toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      if (requestRef.current === requestID) setLoading(false);
    }
  }, [directionFilter, page, pageSize, resultFilter, searchQuery, t]);

  React.useEffect(() => {
    void load();
    return () => {
      requestRef.current += 1;
    };
  }, [load]);

  const pageCount = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
  const closeDetail = React.useCallback(() => setSelectedEventID(""), []);

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
            key: "result",
            label: t("columns.result"),
            value: resultFilter,
            onValueChange: (value) => {
              setResultFilter(value);
              setPage(1);
            },
            options: [
              { label: t("filters.all"), value: "" },
              { label: t("results.passed"), value: "passed" },
              { label: t("results.hit"), value: "hit" },
              { label: t("results.failedOpen"), value: "failed_open" },
            ],
          },
          {
            key: "direction",
            label: t("columns.direction"),
            value: directionFilter,
            onValueChange: (value) => {
              setDirectionFilter(value);
              setPage(1);
            },
            options: [
              { label: t("filters.all"), value: "" },
              { label: t("directions.input"), value: "input" },
              { label: t("directions.output"), value: "output" },
            ],
          },
        ]}
        loading={loading}
        onRefresh={() => void load()}
      />

      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>{t("columns.eventId")}</TableHead>
            <TableHead>{t("columns.user")}</TableHead>
            <TableHead>{t("columns.result")}</TableHead>
            <TableHead>{t("columns.direction")}</TableHead>
            <TableHead>{t("columns.modality")}</TableHead>
            <TableHead>{t("columns.latency")}</TableHead>
            <TableHead>{t("columns.createdAt")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && items.length === 0 ? <TableLoadingRow colSpan={7} /> : null}
          {!loading && items.length === 0 ? <TableEmptyRow colSpan={7}>{t("empty")}</TableEmptyRow> : null}
          {items.map((item) => (
            <TableRow
              key={item.publicID}
              className="cursor-pointer"
              onClick={() => setSelectedEventID(item.publicID)}
            >
              <TableCell className="font-mono text-xs">{item.publicID}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
              </TableCell>
              <TableCell>{resolveLabel(item.result, resultLabels)}</TableCell>
              <TableCell>{resolveLabel(item.direction, directionLabels)}</TableCell>
              <TableCell>{resolveLabel(item.modality, modalityLabels)}</TableCell>
              <TableCell className="tabular-nums">{item.latencyMS}ms</TableCell>
              <TableCell>{formatDateTime(item.createdAt, locale)}</TableCell>
            </TableRow>
          ))}
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

      <ModerationEventDetailSheet
        eventID={selectedEventID}
        open={Boolean(selectedEventID)}
        onClose={closeDetail}
      />
    </div>
  );
}
