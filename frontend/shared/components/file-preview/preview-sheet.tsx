"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { Maximize2, Minimize2, Minus, Plus } from "lucide-react";
import { useTranslations } from "next-intl";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { LoadingReveal } from "@/shared/components/loading-reveal";
import { PreviewStageSkeleton } from "@/shared/components/file-preview/preview-skeleton";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type PreviewSheetProps = {
  source: string;
  toolbarContainer?: HTMLElement | null;
};

type WorkbookState = {
  sheetNames: string[];
  sheets: Record<string, string[][]>;
};

const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2;
const ZOOM_STEP = 0.1;

function clampZoom(value: number): number {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value));
}

function detectDelimiter(text: string): "," | "\t" {
  const firstLine = text.split(/\r?\n/, 1)[0] ?? "";
  const tabCount = (firstLine.match(/\t/g) ?? []).length;
  const commaCount = (firstLine.match(/,/g) ?? []).length;
  return tabCount > commaCount ? "\t" : ",";
}

function parseDelimitedRows(text: string): string[][] {
  const delimiter = detectDelimiter(text);
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = "";
  let quoted = false;

  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    const next = text[index + 1];

    if (char === "\"") {
      if (quoted && next === "\"") {
        cell += "\"";
        index += 1;
        continue;
      }
      quoted = !quoted;
      continue;
    }

    if (!quoted && char === delimiter) {
      row.push(cell);
      cell = "";
      continue;
    }

    if (!quoted && (char === "\n" || char === "\r")) {
      if (char === "\r" && next === "\n") {
        index += 1;
      }
      row.push(cell);
      rows.push(row);
      row = [];
      cell = "";
      continue;
    }

    cell += char;
  }

  if (cell !== "" || row.length > 0 || text.endsWith(delimiter)) {
    row.push(cell);
    rows.push(row);
  }

  return rows;
}

export function PreviewSheet({ source, toolbarContainer }: PreviewSheetProps) {
  const t = useTranslations("files.previewErrors");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const previewRef = React.useRef<HTMLDivElement | null>(null);
  const scrollRegionRef = React.useRef<HTMLDivElement | null>(null);
  const contentRef = React.useRef<HTMLDivElement | null>(null);
  const [status, setStatus] = React.useState<"loading" | "ready" | "error">("loading");
  const [errorMessage, setErrorMessage] = React.useState(t("sheetLoadFailed"));
  const [workbook, setWorkbook] = React.useState<WorkbookState>({ sheetNames: [], sheets: {} });
  const [activeSheet, setActiveSheet] = React.useState("");
  const [zoom, setZoom] = React.useState(0.8);
  const [isFullscreen, setIsFullscreen] = React.useState(false);
  const [contentWidth, setContentWidth] = React.useState(0);
  const [contentHeight, setContentHeight] = React.useState(0);
  const [availableWidth, setAvailableWidth] = React.useState(0);

  React.useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(document.fullscreenElement === previewRef.current);
    };

    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };
  }, []);

  React.useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        setStatus("loading");
        const response = await fetch(source);
        const text = await response.text();
        const rows = parseDelimitedRows(text);

        if (cancelled) {
          return;
        }

        setWorkbook({ sheetNames: ["CSV"], sheets: { CSV: rows } });
        setActiveSheet("CSV");
        setStatus("ready");
      } catch (error) {
        if (cancelled) {
          return;
        }
        setStatus("error");
        setErrorMessage(resolveErrorMessage(error, t("sheetLoadFailed")));
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [resolveErrorMessage, source, t]);

  React.useEffect(() => {
    if (status !== "ready") {
      setAvailableWidth(0);
      return undefined;
    }

    const node = scrollRegionRef.current;
    if (!node) {
      return undefined;
    }

    const updateWidth = () => {
      setAvailableWidth(Math.max(node.clientWidth - 8, 0));
    };

    updateWidth();

    const observer = new ResizeObserver(updateWidth);
    observer.observe(node);
    return () => observer.disconnect();
  }, [status]);

  React.useEffect(() => {
    if (status !== "ready") {
      return undefined;
    }

    const node = contentRef.current;
    if (!node) {
      return undefined;
    }

    const updateSize = () => {
      setContentWidth(node.scrollWidth || node.offsetWidth || 0);
      setContentHeight(node.scrollHeight || node.offsetHeight || 0);
    };

    updateSize();

    const observer = new ResizeObserver(updateSize);
    observer.observe(node);
    return () => observer.disconnect();
  }, [activeSheet, status, workbook.sheets]);

  const activeRows = React.useMemo(() => {
    if (!activeSheet) {
      return [];
    }
    return workbook.sheets[activeSheet] ?? [];
  }, [activeSheet, workbook.sheets]);

  const columnCount = activeRows.reduce((max, row) => Math.max(max, row.length), 0);
  const fitScale = React.useMemo(() => {
    if (!contentWidth || !availableWidth) {
      return 1;
    }
    return Math.min(1, availableWidth / contentWidth);
  }, [availableWidth, contentWidth]);
  const effectiveScale = fitScale * zoom;

  const toggleFullscreen = React.useCallback(async () => {
    const element = previewRef.current;
    if (!element) {
      return;
    }

    if (document.fullscreenElement === element) {
      await document.exitFullscreen();
      return;
    }

    await element.requestFullscreen();
  }, []);

  const toolbar = (
    <div className="flex items-center gap-1.5">
      <Button type="button" variant="ghost" size="icon" className="size-7 rounded-full" onClick={() => setZoom((value) => clampZoom(value - ZOOM_STEP))} disabled={status !== "ready" || zoom <= MIN_ZOOM}>
        <Minus className="size-3.5" />
      </Button>
      <span className="min-w-11 text-center text-[11px] text-muted-foreground">{Math.round(zoom * 100)}%</span>
      <Button type="button" variant="ghost" size="icon" className="size-7 rounded-full" onClick={() => setZoom((value) => clampZoom(value + ZOOM_STEP))} disabled={status !== "ready" || zoom >= MAX_ZOOM}>
        <Plus className="size-3.5" />
      </Button>
      <Button type="button" variant="ghost" size="icon" className="size-7 rounded-full" onClick={() => void toggleFullscreen()} disabled={status !== "ready"}>
        {isFullscreen ? <Minimize2 className="size-3.5" /> : <Maximize2 className="size-3.5" />}
      </Button>
    </div>
  );

  return (
    <div ref={previewRef} className="flex h-full min-h-0 flex-col bg-background">
      {toolbarContainer ? createPortal(toolbar, toolbarContainer) : (
        <div className="flex shrink-0 items-center justify-end gap-1.5 px-1 py-2">{toolbar}</div>
      )}

      <div className="relative flex min-h-0 flex-1 flex-col">
        {status === "error" ? (
          <div className="flex min-h-0 h-full items-center justify-center px-6 text-center text-sm text-muted-foreground">
            {errorMessage}
          </div>
        ) : null}

        {status === "ready" ? (
          <>
            <div ref={scrollRegionRef} className="min-h-0 flex-1 overflow-auto">
                <div className="flex min-w-full justify-center px-1 pb-2">
                  <div
                    className="relative shrink-0"
                    style={{
                      width: contentWidth > 0 ? `${contentWidth * effectiveScale}px` : undefined,
                      height: contentHeight > 0 ? `${contentHeight * effectiveScale}px` : undefined,
                    }}
                  >
                    <div
                      style={{
                        transform: `scale(${effectiveScale})`,
                        transformOrigin: "top left",
                        width: contentWidth > 0 ? `${contentWidth}px` : undefined,
                      }}
                    >
                      <div ref={contentRef} className="w-max min-w-full">
                        <table className="border-collapse text-[12.5px] leading-6">
                          <tbody>
                            {activeRows.map((row, rowIndex) => (
                              <tr key={rowIndex} className="border-b border-border/30 align-top">
                                {Array.from({ length: Math.max(columnCount, 1) }, (_, columnIndex) => {
                                  const value = row[columnIndex] || "";
                                  const isHeaderRow = rowIndex === 0;
                                  return (
                                    <td
                                      key={`${rowIndex}-${columnIndex}`}
                                      className={cn(
                                        "min-w-[120px] max-w-[320px] border-r border-border/30 px-3 py-2 text-left align-top whitespace-pre-wrap break-words",
                                        isHeaderRow ? "bg-muted/50 font-medium text-foreground" : "text-foreground/90",
                                      )}
                                    >
                                      {value || " "}
                                    </td>
                                  );
                                })}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  </div>
                </div>
            </div>

            {workbook.sheetNames.length > 0 ? (
              <div className="mt-2 shrink-0 overflow-x-auto whitespace-nowrap">
                <div className="flex w-max items-center gap-1 px-1 pb-1">
                  {workbook.sheetNames.map((sheetName) => (
                    <button
                      key={sheetName}
                      type="button"
                      className={cn(
                        "shrink-0 rounded-full px-3 py-1 text-xs transition-colors",
                        activeSheet === sheetName ? "bg-foreground text-background" : "text-muted-foreground hover:bg-accent",
                      )}
                      onClick={() => setActiveSheet(sheetName)}
                    >
                      {sheetName}
                    </button>
                  ))}
                </div>
              </div>
            ) : null}

          </>
        ) : null}

        <LoadingReveal
          loading={status === "loading"}
          className="pointer-events-none absolute inset-0 z-10"
          contentClassName="h-full"
          skeletonClassName="h-full"
          skeleton={<PreviewStageSkeleton className="h-full" />}
        >
          <div className="h-full" />
        </LoadingReveal>
      </div>
    </div>
  );
}
