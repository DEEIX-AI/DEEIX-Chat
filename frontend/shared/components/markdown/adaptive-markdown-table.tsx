"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { TableDownloadDropdown } from "streamdown";

import { cn } from "@/lib/utils";
import { useScrollFadeFallbackRef } from "@/shared/hooks/use-scroll-fade-fallback-ref";
import {
  type ColumnAnalyzerOptions,
  type ColumnType,
  classifyTableColumns,
  mergeColumnType,
} from "./markdown-table-analyzer";

const INITIAL_STREAMING_ROWS = 3;
const INITIAL_ANALYSIS_DELAY_MS = 120;
const REANALYSIS_DEBOUNCE_MS = 420;

export const MarkdownTableStreamingContext = React.createContext(false);
export const MarkdownTableAnalyzerOptionsContext = React.createContext<ColumnAnalyzerOptions | undefined>(undefined);

type MarkdownTableProps = React.TableHTMLAttributes<HTMLTableElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type TableSnapshot = {
  headers: string[];
  rows: string[][];
};

type ElementWithChildren = React.ReactElement<{
  children?: React.ReactNode;
  className?: string;
  node?: unknown;
  scope?: string;
}>;

export function AdaptiveMarkdownTable({ children, className, node: _node, ...props }: MarkdownTableProps) {
  const t = useTranslations("chat.markdown");
  const streaming = React.useContext(MarkdownTableStreamingContext);
  const analyzerOptions = React.useContext(MarkdownTableAnalyzerOptionsContext);
  const columnTypes = useStableColumnTypes(children, analyzerOptions, streaming);
  const contentColumnCount = columnTypes.filter((type) => type === "content").length;
  const contentColumnBucket = contentColumnCount >= 3 ? "many" : String(contentColumnCount);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const scrollFadeRef = useScrollFadeFallbackRef(scrollRef);
  const hintID = React.useId();
  const [hasHorizontalOverflow, setHasHorizontalOverflow] = React.useState(false);

  const updateOverflowState = React.useCallback(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const hasHiddenContent = scrollElement.scrollWidth - scrollElement.clientWidth > 2;
    setHasHorizontalOverflow(hasHiddenContent);
  }, []);

  React.useLayoutEffect(() => {
    updateOverflowState();
    const scrollElement = scrollRef.current;
    if (!scrollElement || typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(updateOverflowState);
    observer.observe(scrollElement);
    const table = scrollElement.querySelector("table");
    if (table) {
      observer.observe(table);
    }
    return () => observer.disconnect();
  }, [columnTypes, updateOverflowState]);

  const decoratedChildren = React.useMemo(
    () => decorateTableChildren(children, columnTypes),
    [children, columnTypes],
  );

  return (
    <div
      className="markdown-table-shell relative"
      data-content-columns={contentColumnBucket}
      data-streamdown="table-wrapper"
    >
      <div className="flex min-w-0 items-start gap-2">
        {/* Keyboard users need to focus the horizontal scroll region itself. */}
        <div
          ref={scrollFadeRef}
          aria-describedby={hasHorizontalOverflow ? hintID : undefined}
          aria-label={t("scrollableTable")}
          className="markdown-table-scroll scroll-fade-x scroll-fade-12 min-w-0 flex-1"
          role="region"
          tabIndex={hasHorizontalOverflow ? 0 : undefined}
        >
          <table {...props} className={cn("markdown-table", className)} data-streamdown="table">
            <colgroup>
              {columnTypes.map((type, index) => (
                <col
                  // Column order is dynamic; this key only identifies the current rendered column.
                  key={`${index}-${type}`}
                  className={`markdown-table-col markdown-table-col--${type}`}
                  data-markdown-column-type={type}
                />
              ))}
            </colgroup>
            {decoratedChildren}
          </table>
        </div>
        <div className="mt-3 shrink-0" data-streamdown="table-download-actions">
          <TableDownloadDropdown
            className="inline-flex size-5 items-center justify-center rounded-none p-1 align-middle outline-none hover:bg-foreground/[0.04] focus-visible:bg-foreground/[0.04] focus-visible:ring-0"
            onError={() => toast.error(t("table.downloadFailed"))}
          />
        </div>
      </div>
      <span id={hintID} className="sr-only">
        {t("scrollableTableHint")}
      </span>
    </div>
  );
}

function useStableColumnTypes(
  children: React.ReactNode,
  analyzerOptions: ColumnAnalyzerOptions | undefined,
  streaming: boolean,
): ColumnType[] {
  const latestChildrenRef = React.useRef(children);
  const latestAnalyzerOptionsRef = React.useRef(analyzerOptions);
  const streamingRef = React.useRef(streaming);
  const analysisTimeoutRef = React.useRef<number | null>(null);
  const analyzedCurrentStreamRef = React.useRef(false);
  const hasStreamedRef = React.useRef(streaming);
  const [streamingTypes, setStreamingTypes] = React.useState<ColumnType[]>([]);

  latestChildrenRef.current = children;
  latestAnalyzerOptionsRef.current = analyzerOptions;
  streamingRef.current = streaming;

  const staticTypes = React.useMemo(() => {
    if (streaming) {
      return null;
    }
    const snapshot = getTableSnapshot(children);
    return classifyTableColumns(snapshot.headers, snapshot.rows, analyzerOptions);
  }, [analyzerOptions, children, streaming]);

  const resolvedStaticTypes = React.useMemo(
    () => staticTypes === null
      ? null
      : hasStreamedRef.current
        ? mergeColumnTypes(streamingTypes, staticTypes)
        : staticTypes,
    [staticTypes, streamingTypes],
  );

  React.useEffect(() => {
    if (!streaming) {
      if (analysisTimeoutRef.current !== null) {
        window.clearTimeout(analysisTimeoutRef.current);
        analysisTimeoutRef.current = null;
      }
      analyzedCurrentStreamRef.current = false;
      if (resolvedStaticTypes !== null) {
        setStreamingTypes((previousTypes) =>
          areColumnTypesEqual(previousTypes, resolvedStaticTypes)
            ? previousTypes
            : resolvedStaticTypes,
        );
      }
      return;
    }

    hasStreamedRef.current = true;
    if (analysisTimeoutRef.current !== null) {
      return;
    }

    const delay = analyzedCurrentStreamRef.current
      ? REANALYSIS_DEBOUNCE_MS
      : INITIAL_ANALYSIS_DELAY_MS;
    analysisTimeoutRef.current = window.setTimeout(() => {
      analysisTimeoutRef.current = null;
      if (!streamingRef.current) {
        return;
      }
      const snapshot = getTableSnapshot(latestChildrenRef.current);
      if (snapshot.rows.length < INITIAL_STREAMING_ROWS) {
        return;
      }
      const candidateTypes = classifyTableColumns(
        snapshot.headers,
        snapshot.rows,
        latestAnalyzerOptionsRef.current,
      );
      setStreamingTypes((previousTypes) => {
        const mergedTypes = mergeColumnTypes(previousTypes, candidateTypes);
        return areColumnTypesEqual(previousTypes, mergedTypes) ? previousTypes : mergedTypes;
      });
      analyzedCurrentStreamRef.current = true;
    }, delay);
  }, [analyzerOptions, children, resolvedStaticTypes, streaming]);

  React.useEffect(() => () => {
    if (analysisTimeoutRef.current !== null) {
      window.clearTimeout(analysisTimeoutRef.current);
    }
  }, []);

  return streaming ? streamingTypes : (resolvedStaticTypes ?? []);
}

function mergeColumnTypes(
  previousTypes: readonly ColumnType[],
  candidateTypes: readonly ColumnType[],
): ColumnType[] {
  return candidateTypes.map((nextType, index) => {
    const previousType = previousTypes[index];
    return previousType === undefined
      ? nextType
      : mergeColumnType(previousType, nextType);
  });
}

function areColumnTypesEqual(
  leftTypes: readonly ColumnType[],
  rightTypes: readonly ColumnType[],
): boolean {
  return leftTypes.length === rightTypes.length
    && leftTypes.every((type, index) => type === rightTypes[index]);
}

function getTableSnapshot(children: React.ReactNode): TableSnapshot {
  const sections = React.Children.toArray(children).filter(React.isValidElement) as ElementWithChildren[];
  let headers: string[] = [];
  const rows: string[][] = [];

  for (const section of sections) {
    const sectionRows = getChildElements(section.props.children);
    const tagName = getElementTagName(section);
    for (const row of sectionRows) {
      const values = getChildElements(row.props.children).map((cell) => getReactNodeText(cell.props.children));
      if (tagName === "thead" && headers.length === 0) {
        headers = values;
      } else {
        rows.push(values);
      }
    }
  }

  const columnCount = Math.max(headers.length, ...rows.map((row) => row.length), 0);
  if (headers.length < columnCount) {
    headers = [...headers, ...Array.from({ length: columnCount - headers.length }, () => "")];
  }
  return { headers, rows };
}

function decorateTableChildren(children: React.ReactNode, columnTypes: readonly ColumnType[]): React.ReactNode {
  return React.Children.map(children, (sectionNode) => {
    if (!React.isValidElement(sectionNode)) {
      return sectionNode;
    }
    const section = sectionNode as ElementWithChildren;
    const headerSection = getElementTagName(section) === "thead";
    const rows = React.Children.map(section.props.children, (rowNode) => {
      if (!React.isValidElement(rowNode)) {
        return rowNode;
      }
      const row = rowNode as ElementWithChildren;
      const cells = React.Children.map(row.props.children, (cellNode, columnIndex) => {
        if (!React.isValidElement(cellNode)) {
          return cellNode;
        }
        const cell = cellNode as ElementWithChildren;
        const type = columnTypes[columnIndex] ?? "normal";
        return React.cloneElement(cell, {
          className: cn(cell.props.className, "markdown-table-cell", `markdown-table-cell--${type}`),
          "data-markdown-column-type": type,
          ...(headerSection ? { scope: "col" } : {}),
        } as React.HTMLAttributes<HTMLTableCellElement>);
      });
      return React.cloneElement(row, undefined, cells);
    });
    return React.cloneElement(section, undefined, rows);
  });
}

function getChildElements(children: React.ReactNode): ElementWithChildren[] {
  return React.Children.toArray(children).filter(React.isValidElement) as ElementWithChildren[];
}

function getElementTagName(element: ElementWithChildren): string {
  const node = element.props.node as { tagName?: string } | undefined;
  if (node?.tagName) {
    return node.tagName.toLowerCase();
  }
  if (typeof element.type === "string") {
    return element.type.toLowerCase();
  }
  const displayName = (element.type as React.ComponentType).displayName ?? (element.type as React.ComponentType).name ?? "";
  return displayName.toLowerCase().replace(/^markdown/, "");
}

function getReactNodeText(node: React.ReactNode): string {
  return React.Children.toArray(node)
    .map((child) => {
      if (typeof child === "string" || typeof child === "number") {
        return String(child);
      }
      if (React.isValidElement<{ children?: React.ReactNode }>(child)) {
        return getReactNodeText(child.props.children);
      }
      return "";
    })
    .join("")
    .trim();
}
