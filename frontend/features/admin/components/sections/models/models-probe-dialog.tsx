"use client";

import * as React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { resolveProtocolLabel } from "@/features/admin/utils/llm-display";
import type { AdminLLMModelProbeDebug, AdminLLMModelProbeResult } from "@/features/admin/api/llm.types";

type ModelProbeDialogProps = {
  open: boolean;
  loading: boolean;
  targetName: string;
  result: AdminLLMModelProbeResult | null;
  results?: AdminLLMModelProbeResult[] | null;
  onOpenChange: (open: boolean) => void;
  onDeleteRoute?: (result: AdminLLMModelProbeResult) => Promise<void>;
};

type ModelProbeState = "success" | "error" | "timeout" | "unsupported";

function formatDebugPayload(value: unknown): string {
  if (typeof value !== "string") return "";
  const trimmed = value.trim();
  if (!trimmed) return "";
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return trimmed;
  }
}

function resolveProbeState(result: AdminLLMModelProbeResult): ModelProbeState {
  if (result.success) return "success";
  if (result.status === "unsupported") return "unsupported";
  return result.errorCode === "timeout" ? "timeout" : "error";
}

function statusTextClass(state: ModelProbeState) {
  return cn(
    state === "success" && "text-emerald-700 dark:text-emerald-300",
    state === "error" && "text-destructive",
    state === "timeout" && "text-amber-700 dark:text-amber-300",
    state === "unsupported" && "text-amber-700 dark:text-amber-300",
  );
}

function StatusLine({ state }: { state: ModelProbeState }) {
  const t = useTranslations("adminModels.probe");
  return (
    <span className={cn("inline-flex items-center gap-2 text-xs font-medium", statusTextClass(state))}>
      <span
        className={cn(
          "size-1.5 rounded-full",
          state === "success" && "bg-emerald-500",
          state === "error" && "bg-destructive",
          state === "timeout" && "bg-amber-500",
          state === "unsupported" && "bg-amber-500",
        )}
        aria-hidden="true"
      />
      {t(`status.${state}`)}
    </span>
  );
}

function DetailItem({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[88px_minmax(0,1fr)] items-center gap-3 text-xs leading-5">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn("min-w-0 truncate text-foreground/85", mono && "font-mono")}>{value || "-"}</span>
    </div>
  );
}

function DetailPair({ left, right }: { left: React.ReactNode; right: React.ReactNode }) {
  return (
    <div className="grid gap-x-5 gap-y-1.5 sm:grid-cols-2">
      {left}
      {right}
    </div>
  );
}

function DebugBlock({ title, children }: { title: string; children: string }) {
  return (
    <div className="space-y-1.5">
      <div className="text-[11px] font-medium text-muted-foreground">{title}</div>
      <pre className="max-h-52 overflow-auto rounded-md border border-border/60 bg-background p-3 text-[11px] leading-5 text-foreground/75">
        {children || "-"}
      </pre>
    </div>
  );
}

function ResultDebugContent({
  debug,
  state,
  errorCode,
  errorMessage,
}: {
  debug?: AdminLLMModelProbeDebug;
  state: ModelProbeState;
  errorCode?: string;
  errorMessage?: string;
}) {
  const t = useTranslations("adminModels.probe");
  const hasError = Boolean(errorCode || errorMessage);
  if (!debug) {
    return (
      <div className="space-y-3 border-t border-border/60 pt-3">
        <div className="flex items-center justify-between gap-4">
          <StatusLine state={state} />
          <span className="text-xs text-muted-foreground">{t("noDebug")}</span>
        </div>
        {hasError ? (
          <div className="text-xs leading-5">
            <div className="font-mono text-[11px] text-foreground/80">{errorCode || t("unknownError")}</div>
            <div className="mt-1 text-muted-foreground">{errorMessage || t("unknownError")}</div>
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <Tabs defaultValue="request" className="gap-3 border-t border-border/60 pt-3">
      <div className="flex items-center justify-between gap-4">
        <StatusLine state={state} />
        <TabsList>
          <TabsTrigger value="request">{t("debug.request")}</TabsTrigger>
          <TabsTrigger value="response">{t("debug.response")}</TabsTrigger>
        </TabsList>
      </div>
      {hasError ? (
        <div className="text-xs leading-5">
          <div className="font-mono text-[11px] text-foreground/80">{errorCode || t("unknownError")}</div>
          <div className="mt-1 text-muted-foreground">{errorMessage || t("unknownError")}</div>
        </div>
      ) : null}
      <TabsContent value="request" className="space-y-3">
        <DebugBlock title={t("debug.requestHeaders")}>
          {Object.keys(debug.request.headers ?? {}).length > 0
            ? JSON.stringify(debug.request.headers, null, 2)
            : ""}
        </DebugBlock>
        <DebugBlock title={t("debug.body")}>{formatDebugPayload(debug.request.body)}</DebugBlock>
      </TabsContent>
      <TabsContent value="response" className="space-y-3">
        <DebugBlock title={t("debug.responseHeaders")}>
          {Object.keys(debug.response.headers ?? {}).length > 0
            ? JSON.stringify(debug.response.headers, null, 2)
            : ""}
        </DebugBlock>
        <DebugBlock title={t("debug.body")}>{formatDebugPayload(debug.response.body)}</DebugBlock>
      </TabsContent>
    </Tabs>
  );
}

export function ModelProbeDialog({
  open,
  loading,
  targetName,
  result,
  results,
  onOpenChange,
  onDeleteRoute,
}: ModelProbeDialogProps) {
  const t = useTranslations("adminModels.probe");
  const commonT = useTranslations("common");
  const normalizedResults = React.useMemo(
    () => (results && results.length > 0 ? results : result ? [result] : []),
    [result, results],
  );
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const activeResult = normalizedResults[activeIndex] ?? null;
  const resultState = activeResult ? resolveProbeState(activeResult) : null;
  const upstreamStatusCode = activeResult?.upstreamStatusCode || activeResult?.debug?.response.statusCode || "-";
  const method = activeResult?.debug?.request.method || "-";
  const path = activeResult?.debug?.request.path || "-";
  const canNavigate = normalizedResults.length > 1;
  const canDeleteRoute = Boolean(onDeleteRoute && activeResult?.upstreamID && activeResult.routeID);
  const description = activeResult
    ? `${activeResult.platformModelName || "-"} / ${activeResult.upstreamModelName || "-"}`
    : targetName || t("description");

  React.useEffect(() => {
    if (open) {
      setActiveIndex(0);
      setDeleteOpen(false);
      setDeleting(false);
    }
  }, [open, targetName]);

  React.useEffect(() => {
    setActiveIndex((current) => Math.min(current, Math.max(normalizedResults.length - 1, 0)));
  }, [normalizedResults.length]);

  async function handleConfirmDelete() {
    if (!activeResult || !onDeleteRoute || deleting) {
      return;
    }
    setDeleting(true);
    try {
      await onDeleteRoute(activeResult);
      setDeleteOpen(false);
    } catch {
      // The caller owns context-specific toast messaging; keep the dialog open for retry.
    } finally {
      setDeleting(false);
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="flex max-h-[min(86vh,720px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]">
          <DialogHeader className="shrink-0 px-5 pb-4 pt-5">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0 space-y-1.5">
                <DialogTitle>{t("title")}</DialogTitle>
                <DialogDescription className="truncate">{description}</DialogDescription>
              </div>
              {canNavigate ? (
                <div className="flex shrink-0 items-center gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="text-muted-foreground"
                    aria-label={t("previous")}
                    onClick={() => setActiveIndex((current) => Math.max(current - 1, 0))}
                    disabled={activeIndex === 0 || loading}
                  >
                    <ChevronLeft className="size-3.5 stroke-1" />
                  </Button>
                  <span className="min-w-10 text-center text-[11px] text-muted-foreground">
                    {t("resultCount", { current: activeIndex + 1, total: normalizedResults.length })}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="text-muted-foreground"
                    aria-label={t("next")}
                    onClick={() => setActiveIndex((current) => Math.min(current + 1, normalizedResults.length - 1))}
                    disabled={activeIndex >= normalizedResults.length - 1 || loading}
                  >
                    <ChevronRight className="size-3.5 stroke-1" />
                  </Button>
                </div>
              ) : null}
            </div>
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4">
            {loading ? (
              <div className="flex h-36 items-center justify-center">
                <div className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                  <Spinner className="size-4" />
                  {t("running")}
                </div>
              </div>
            ) : activeResult ? (
              <div className="space-y-4">
                <div className="space-y-2">
                  <DetailPair
                    left={<DetailItem label={t("fields.upstream")} value={activeResult.upstreamName} />}
                    right={<DetailItem label={t("fields.endpoint")} value={resolveProtocolLabel(activeResult.protocol)} />}
                  />
                  <DetailPair
                    left={<DetailItem label={t("debug.method")} value={method} mono />}
                    right={<DetailItem label={t("debug.path")} value={path} mono />}
                  />
                  <DetailPair
                    left={<DetailItem label={t("fields.statusCode")} value={upstreamStatusCode} mono />}
                    right={<DetailItem label={t("fields.latency")} value={t("latency", { value: activeResult.latencyMS || 0 })} mono />}
                  />
                </div>

                <ResultDebugContent
                  debug={activeResult.debug}
                  state={resultState ?? "error"}
                  errorCode={activeResult.errorCode}
                  errorMessage={activeResult.errorMessage}
                />
              </div>
            ) : (
              <div className="border-y border-border/60 py-3 text-xs text-muted-foreground">
                {t("empty")}
              </div>
            )}
          </div>

          <DialogFooter className="shrink-0 px-5 py-3">
            {canDeleteRoute && (
              <Button
                type="button"
                variant="ghost"
                className="text-destructive hover:text-destructive"
                onClick={() => setDeleteOpen(true)}
                disabled={loading || deleting}
              >
                {t("deleteSource")}
              </Button>
            )}
            <Button onClick={() => onOpenChange(false)} disabled={loading || deleting}>
              {commonT("actions.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteOpen} onOpenChange={(nextOpen) => !deleting && setDeleteOpen(nextOpen)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteConfirmDescription", {
                upstream: activeResult?.upstreamName || "-",
                model: activeResult?.upstreamModelName || "-",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={(event) => {
                event.preventDefault();
                void handleConfirmDelete();
              }}
              disabled={deleting}
            >
              {deleting ? t("deletingSource") : commonT("actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
