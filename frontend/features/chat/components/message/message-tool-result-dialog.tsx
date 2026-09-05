"use client";

import { Maximize2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

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
import { getConversationToolCallDetail } from "@/shared/api/conversation";
import type { ConversationToolCallDetailDTO } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { CopyActionButton } from "@/shared/components/copy-action";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import { formatBytes } from "@/shared/lib/file-display";

type ToolResultSection = {
  key: "output" | "error";
  label: string;
  value: string;
  editorValue: string | null;
  largeJSON: boolean;
  omitted: boolean;
  omittedSize: number;
};

const INLINE_JSON_FORMAT_LIMIT = 256 * 1024;

function firstNonWhitespaceCharacter(value: string): string {
  return /\S/.exec(value)?.[0] ?? "";
}

function prepareJSONResult(value: string): { value: string; large: boolean } | null {
  if (value.length > INLINE_JSON_FORMAT_LIMIT) {
    const firstCharacter = firstNonWhitespaceCharacter(value);
    return firstCharacter === "{" || firstCharacter === "["
      ? { value, large: true }
      : null;
  }
  try {
    return { value: JSON.stringify(JSON.parse(value), null, 2), large: false };
  } catch {
    return null;
  }
}

function resultViewerHeight(value: string, multiple: boolean): number {
  if (value.length > INLINE_JSON_FORMAT_LIMIT) {
    return multiple ? 280 : 480;
  }
  let lineCount = 1;
  for (const character of value) {
    if (character === "\n") {
      lineCount += 1;
    }
  }
  return Math.min(multiple ? 280 : 480, Math.max(160, lineCount * 20 + 40));
}

export function MessageToolResultDialog({
  runID,
  toolCallID,
  toolName,
}: {
  runID: string;
  toolCallID: string;
  toolName: string;
}) {
  const t = useTranslations("chat.processTrace.tool.detail");
  const [open, setOpen] = React.useState(false);
  const [detail, setDetail] = React.useState<ConversationToolCallDetailDTO | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [loadFailed, setLoadFailed] = React.useState(false);

  React.useEffect(() => {
    setDetail(null);
    setLoadFailed(false);
  }, [runID, toolCallID]);

  React.useEffect(() => {
    if (!open || detail) {
      return;
    }

    const controller = new AbortController();
    setLoading(true);
    setLoadFailed(false);
    void (async () => {
      try {
        const accessToken = await resolveAccessToken();
        if (!accessToken) {
          throw new Error("missing access token");
        }
        const result = await getConversationToolCallDetail(
          accessToken,
          runID,
          toolCallID,
          controller.signal,
        );
        if (!controller.signal.aborted) {
          setDetail(result);
        }
      } catch {
        if (!controller.signal.aborted) {
          setLoadFailed(true);
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    })();

    return () => controller.abort();
  }, [detail, open, runID, toolCallID]);

  const sections = React.useMemo<ToolResultSection[]>(() => {
    if (!detail) {
      return [];
    }
    const output = detail.outputJSON;
    const error = detail.errorJSON;
    const preparedOutput = prepareJSONResult(output);
    const preparedError = prepareJSONResult(error);
    return [
      {
        key: "output" as const,
        label: t("response"),
        value: output,
        editorValue: preparedOutput?.value ?? null,
        largeJSON: preparedOutput?.large ?? false,
        omitted: detail.outputOmitted,
        omittedSize: detail.outputSizeBytes,
      },
      {
        key: "error" as const,
        label: t("error"),
        value: error,
        editorValue: preparedError?.value ?? null,
        largeJSON: preparedError?.large ?? false,
        omitted: detail.errorOmitted,
        omittedSize: detail.errorSizeBytes,
      },
    ].filter((section) => section.value || section.omitted);
  }, [detail, t]);
  const copyValue = React.useMemo(() => {
    const available = sections.filter((section) => section.value);
    if (available.length === 1) {
      return available[0].value;
    }
    return available.map((section) => `${section.label}\n${section.value}`).join("\n\n");
  }, [sections]);
  const waitingForDetail = open && !detail && !loadFailed;

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 shrink-0 gap-1 px-1.5 text-[11px] font-normal text-muted-foreground shadow-none"
        data-screenshot-exclude="true"
        onClick={() => setOpen(true)}
      >
        <Maximize2 className="size-3" />
        {t("viewFullResult")}
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent
          className="flex max-h-[min(86svh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[720px]"
          onCloseAutoFocus={() => {
            setDetail(null);
            setLoading(false);
            setLoadFailed(false);
          }}
        >
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{t("fullResultTitle")}</DialogTitle>
            <DialogDescription className="truncate font-mono text-xs">
              {detail?.toolName.trim() || toolName}
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-5 py-2">
            {loading || waitingForDetail ? (
              <div className="flex min-h-36 items-center justify-center rounded-md bg-muted/20">
                <Spinner label={t("loadingFullResult")} className="size-4 text-muted-foreground" />
              </div>
            ) : loadFailed ? (
              <p className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-xs leading-5 text-muted-foreground">
                {t("fullResultLoadFailed")}
              </p>
            ) : sections.length > 0 ? (
              <div className="space-y-3">
                {sections.map((section) => (
                  <section key={section.key} className="space-y-1.5">
                    {sections.length > 1 ? (
                      <h3 className="text-xs font-medium text-muted-foreground">{section.label}</h3>
                    ) : null}
                    {section.omitted ? (
                      <p className="rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-xs leading-5 text-muted-foreground">
                        {t("fullResultOmitted", { size: formatBytes(section.omittedSize) })}
                      </p>
                    ) : section.editorValue !== null ? (
                      <JsonCodeEditor
                        value={section.editorValue}
                        readOnly
                        showFormatAction={false}
                        height={resultViewerHeight(section.editorValue, sections.length > 1)}
                        wordWrap={section.largeJSON ? "off" : "on"}
                        className="resize-none border-border/60 bg-muted/20 dark:bg-muted/20"
                      />
                    ) : (
                      <pre className="max-h-[min(60svh,32rem)] min-w-0 overflow-auto whitespace-pre rounded-md border border-border/60 bg-muted/25 p-3 font-mono text-xs leading-5 text-foreground/86">
                        <code>{section.value}</code>
                      </pre>
                    )}
                  </section>
                ))}
              </div>
            ) : (
              <p className="flex min-h-36 items-center justify-center rounded-md bg-muted/20 text-xs text-muted-foreground">
                {t("fullResultEmpty")}
              </p>
            )}
          </div>

          <DialogFooter className="shrink-0 px-5 py-3">
            {copyValue ? (
              <CopyActionButton
                type="button"
                variant="ghost"
                size="sm"
                value={copyValue}
                messages={{ copied: t("copied"), failed: t("copyFailed") }}
              >
                {t("copy")}
              </CopyActionButton>
            ) : null}
            <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(false)}>
              {t("close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
