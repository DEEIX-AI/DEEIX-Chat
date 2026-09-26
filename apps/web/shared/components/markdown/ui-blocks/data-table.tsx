"use client";

import { ArrowDown, ArrowUp, ArrowUpDown, Download, Search } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { downloadBlob } from "@/shared/lib/export-download";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

const COLUMN_TYPES = ["string", "number", "date", "boolean"] as const;
type ColumnType = (typeof COLUMN_TYPES)[number];
type Cell = string | number | boolean | null | undefined;

export type DataTableProps = {
  title?: string;
  columns: { key: string; label: string; type?: ColumnType; align?: "start" | "end" }[];
  rows: Record<string, Cell>[];
  pageSize?: number;
};

type Sort = { key: string; direction: "asc" | "desc" } | null;

const NUMBER_FORMAT = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });
const DATE_LIKE = /^\d{4}-\d{2}-\d{2}(?:[T ]|$)/;
const NUMERIC_TEXT = /^[+-]?[\d,\uff0c\s]*\.?\d+\s*%?$/;

function parseNumeric(value: Cell): number | null {
  if (typeof value === "number") return Number.isFinite(value) ? value : null;
  if (typeof value !== "string" || !NUMERIC_TEXT.test(value.trim())) return null;
  const parsed = Number(value.replace(/[,\uff0c\s%]/g, ""));
  return Number.isFinite(parsed) ? parsed : null;
}

function inferType(value: Cell): ColumnType {
  if (typeof value === "boolean") return "boolean";
  if (typeof value === "string" && DATE_LIKE.test(value)) return "date";
  return parseNumeric(value) === null ? "string" : "number";
}

function compareCells(a: Cell, b: Cell, type: ColumnType): number {
  if (a === null || a === undefined) return b === null || b === undefined ? 0 : 1;
  if (b === null || b === undefined) return -1;
  switch (type) {
    case "number":
      return (parseNumeric(a) ?? 0) - (parseNumeric(b) ?? 0);
    case "date": {
      const left = Date.parse(String(a));
      const right = Date.parse(String(b));
      if (Number.isNaN(left) || Number.isNaN(right)) return String(a).localeCompare(String(b));
      return left - right;
    }
    case "boolean":
      return Number(Boolean(a)) - Number(Boolean(b));
    default:
      return String(a).localeCompare(String(b));
  }
}

function formatCell(value: Cell, type: ColumnType): string {
  if (value === null || value === undefined || value === "") return "—";
  if (type === "boolean") return value ? "✓" : "✗";
  if (type === "number" && typeof value === "number") return NUMBER_FORMAT.format(value);
  return String(value);
}

function highlight(text: string, needle: string) {
  const lower = text.toLowerCase();
  const parts: React.ReactNode[] = [];
  let cursor = 0;
  while (cursor < text.length) {
    const at = lower.indexOf(needle, cursor);
    if (at < 0) break;
    if (at > cursor) parts.push(text.slice(cursor, at));
    parts.push(
      <span key={at} className="rounded-[3px] bg-primary/20 text-foreground">
        {text.slice(at, at + needle.length)}
      </span>,
    );
    cursor = at + needle.length;
  }
  if (parts.length === 0) return text;
  if (cursor < text.length) parts.push(text.slice(cursor));
  return parts;
}

function csvEscape(value: string): string {
  return /[",\n]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value;
}

function DataTable({ id, props }: UIBlockRenderProps<DataTableProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const [query, setQuery] = React.useState("");
  const [sort, setSort] = React.useState<Sort>(null);
  const [page, setPage] = React.useState(1);
  const pageSize = Math.max(5, Math.min(100, props.pageSize ?? 10));
  const types = React.useMemo(() => {
    const map = new Map<string, ColumnType>();
    for (const column of props.columns) {
      const sample = props.rows.find((row) => row[column.key] !== null && row[column.key] !== undefined && row[column.key] !== "")?.[column.key];
      map.set(column.key, column.type ?? (sample === undefined ? "string" : inferType(sample)));
    }
    return map;
  }, [props.columns, props.rows]);
  const needle = query.trim().toLowerCase();

  const filtered = React.useMemo(() => {
    if (!needle) return props.rows;
    return props.rows.filter((row) => props.columns.some((column) => String(row[column.key] ?? "").toLowerCase().includes(needle)));
  }, [props.columns, props.rows, needle]);

  const sorted = React.useMemo(() => {
    if (!sort) return filtered;
    const type = types.get(sort.key) ?? "string";
    const direction = sort.direction === "asc" ? 1 : -1;
    return [...filtered].sort((a, b) => direction * compareCells(a[sort.key], b[sort.key], type));
  }, [filtered, sort, types]);

  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
  const currentPage = Math.min(page, pageCount);
  const visible = sorted.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  const toggleSort = (key: string) =>
    setSort((current) => (!current || current.key !== key ? { key, direction: "asc" } : current.direction === "asc" ? { key, direction: "desc" } : null));

  const exportCSV = () => {
    const header = props.columns.map((column) => csvEscape(column.label)).join(",");
    const body = sorted.map((row) => props.columns.map((column) => csvEscape(String(row[column.key] ?? ""))).join(",")).join("\n");
    downloadBlob(new Blob([`\uFEFF${header}\n${body}`], { type: "text/csv;charset=utf-8" }), `${props.title?.trim() || "table"}.csv`);
  };

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-2 border-b-[0.5px] border-border px-4 pb-3 pt-5">
        <div className="flex min-w-0 items-center gap-3">
          {props.title ? <h3 className="truncate text-lg font-semibold leading-7">{props.title}</h3> : null}
          <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
            {needle ? t("rowCountFiltered", { filtered: sorted.length, total: props.rows.length }) : t("rowCount", { count: sorted.length })}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" strokeWidth={1.8} />
            <Input
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setPage(1);
              }}
              placeholder={t("search")}
              className="h-7 w-40 border-0 bg-muted/45 pl-7 text-xs shadow-none"
            />
          </div>
          <Button type="button" variant="ghost" size="icon-sm" onClick={exportCSV} aria-label={t("exportCSV")} title={t("exportCSV")} className="text-muted-foreground">
            <Download strokeWidth={1.8} />
          </Button>
        </div>
      </div>

      <Table
        shellClassName="rounded-none border-0 bg-transparent"
        viewportClassName="max-h-[24rem]"
        className="text-xs"
      >
        <TableHeader className="bg-transparent shadow-none">
          <TableRow>
            {props.columns.map((column, index) => {
              const active = sort?.key === column.key;
              const Icon = !active ? ArrowUpDown : sort.direction === "asc" ? ArrowUp : ArrowDown;
              const end = column.align === "end" || (!column.align && types.get(column.key) === "number");
              return (
                <TableHead
                  key={`${id}-${column.key}`}
                  scope="col"
                  aria-sort={active ? (sort.direction === "asc" ? "ascending" : "descending") : "none"}
                  className={cn(
                    "sticky top-0 z-10 border-b-[0.5px] border-border bg-card px-4 font-normal",
                    index === 0 && "left-0 z-20",
                    end && "text-right",
                  )}
                >
                  <button type="button" onClick={() => toggleSort(column.key)} className="inline-flex items-center gap-1 hover:text-foreground">
                    {column.label}
                    <Icon className={cn("size-3", active ? "opacity-100" : "opacity-40")} strokeWidth={1.8} />
                  </button>
                </TableHead>
              );
            })}
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.length === 0 ? (
            <TableEmptyRow colSpan={props.columns.length}>{t("empty")}</TableEmptyRow>
          ) : (
            visible.map((row, rowIndex) => (
              <TableRow key={`${id}-r-${(currentPage - 1) * pageSize + rowIndex}`} className="group bg-transparent hover:bg-accent/40">
                {props.columns.map((column, index) => {
                  const type = types.get(column.key) ?? "string";
                  const end = column.align === "end" || (!column.align && type === "number");
                  const text = formatCell(row[column.key], type);
                  return (
                    <TableCell
                      key={`${id}-c-${column.key}`}
                      className={cn(
                        "px-4",
                        index === 0 && "sticky left-0 z-10 bg-card group-hover:bg-accent/40",
                        end && "text-right tabular-nums",
                      )}
                    >
                      {needle && typeof row[column.key] === "string" ? highlight(text, needle) : text}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      {pageCount > 1 ? (
        <div className="flex items-center justify-between border-t-[0.5px] border-border px-4 py-1.5 text-[11px] text-muted-foreground">
          <span className="tabular-nums">{t("pageOf", { page: currentPage, total: pageCount })}</span>
          <div className="flex gap-1">
            <Button type="button" variant="ghost" size="xs" className="text-[11px]" disabled={currentPage <= 1} onClick={() => setPage(currentPage - 1)}>
              {t("prev")}
            </Button>
            <Button type="button" variant="ghost" size="xs" className="text-[11px]" disabled={currentPage >= pageCount} onClick={() => setPage(currentPage + 1)}>
              {t("next")}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

// Toolbar, header row and one 33px line per row, like the rendered table.
function DataTableSkeleton({ items = 0 }: UIBlockSkeletonProps) {
  return (
    <UIBlockFrame>
      <div className="flex items-center justify-between border-b-[0.5px] border-border px-4 py-3">
        <Skeleton className="h-6 w-1/4 rounded-md" />
        <Skeleton className="h-6 w-36 rounded-md" />
      </div>
      <div className="px-4">
        <div className="flex h-[33px] items-center gap-6 border-b-[0.5px] border-border">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={`data-table-head-${index}`} className="h-3 w-16 rounded-sm" />
          ))}
        </div>
        {Array.from({ length: Math.max(2, items) }).map((_, index) => (
          <div key={`data-table-${index}`} className="flex h-[33px] items-center gap-6 border-b-[0.5px] border-border last:border-b-0">
            {Array.from({ length: 4 }).map((_, cell) => (
              <Skeleton key={`data-table-${index}-${cell}`} className="h-3 rounded-sm" style={{ width: `${10 + ((index * 7 + cell * 11) % 14)}%` }} />
            ))}
          </div>
        ))}
      </div>
    </UIBlockFrame>
  );
}

export const dataTableDefinition: UIBlockDefinition<DataTableProps> = {
  name: "data-table",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      columns: s.array(s.object({ key: s.string(), label: s.string(), type: s.string({ enum: COLUMN_TYPES }), align: s.string({ enum: ["start", "end"] }) }, ["key", "label"]), { minItems: 1 }),
      rows: s.array(s.any(), { minItems: 1 }),
      pageSize: s.number(),
    },
    ["columns", "rows"],
  ),
  Component: DataTable,
  Skeleton: DataTableSkeleton,
};
