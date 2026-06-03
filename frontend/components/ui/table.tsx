"use client"

import * as React from "react"

import { cn } from "@/lib/utils"

type TableProps = React.ComponentProps<"table"> & {
  shellClassName?: string
}

function Table({ className, shellClassName, ...props }: TableProps) {
  return (
    <div
      data-slot="table-container"
      className={cn("min-w-0 overflow-hidden rounded-lg border border-border/60 bg-background", shellClassName)}
    >
      <div className="w-full overflow-x-auto">
        <div className="min-w-full align-middle">
          <table
            data-slot="table"
            className={cn(
              "w-full min-w-max table-auto border-collapse text-[12px] leading-5",
              "[&_[data-slot=input]]:h-6 [&_[data-slot=input]]:px-2 [&_[data-slot=input]]:text-xs [&_[data-slot=input]]:placeholder:text-xs",
              "[&_[data-slot=select-trigger]]:h-6 [&_[data-slot=select-trigger]]:px-2 [&_[data-slot=select-trigger]]:text-xs",
              "[&_[data-slot=input-group]]:h-6 [&_[data-slot=input-group-control]]:h-6 [&_[data-slot=input-group-control]]:px-2 [&_[data-slot=input-group-control]]:text-xs [&_[data-slot=input-group-control]]:placeholder:text-xs",
              "[&_[role=combobox]]:h-6 [&_[role=combobox]]:text-xs",
              className
            )}
            {...props}
          />
        </div>
      </div>
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("data-table-header", className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={className}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0",
        className
      )}
      {...props}
    />
  )
}

type TableRowProps = React.ComponentProps<"tr"> & {
  interactive?: boolean
  selected?: boolean
  tone?: "muted"
  "data-interactive"?: string
  "data-selected"?: string
  "data-tone"?: "muted" | string
}

function TableRow({
  className,
  interactive,
  selected,
  tone,
  "data-interactive": dataInteractive,
  "data-selected": dataSelected,
  "data-tone": dataTone,
  ...props
}: TableRowProps) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "data-table-row border-b border-border/60 last:border-b-0",
        className
      )}
      data-interactive={interactive === false ? "false" : dataInteractive}
      data-selected={selected ? "true" : dataSelected}
      data-tone={tone ?? dataTone}
      {...props}
    />
  )
}

type TableHeadProps = React.ComponentProps<"th"> & {
  stickyEnd?: boolean
}

function TableHead({ className, stickyEnd, ...props }: TableHeadProps) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-8 px-3 py-1.5 text-left align-middle text-[11px] font-medium text-muted-foreground whitespace-nowrap",
        stickyEnd && "data-table-sticky-end-head sticky right-0 z-10",
        className
      )}
      {...props}
    />
  )
}

type TableCellProps = React.ComponentProps<"td"> & {
  stickyEnd?: boolean
}

function TableCell({ className, stickyEnd, ...props }: TableCellProps) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "px-3 py-2.5 align-middle text-xs leading-5 whitespace-nowrap",
        stickyEnd && "data-table-sticky-end-cell sticky right-0 z-10",
        className
      )}
      {...props}
    />
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

type TableEmptyRowProps = {
  colSpan: number
  children: React.ReactNode
  rowClassName?: string
  cellClassName?: string
}

function TableEmptyRow({
  colSpan,
  children,
  rowClassName,
  cellClassName,
}: TableEmptyRowProps) {
  return (
    <TableRow className={rowClassName}>
      <TableCell
        colSpan={colSpan}
        className={cn(
          "py-8 text-center text-xs text-muted-foreground",
          cellClassName
        )}
      >
        {children}
      </TableCell>
    </TableRow>
  )
}

type TableSkeletonRowsProps = {
  colSpan: number
  rowCount?: number
  rowClassName?: string
  cellClassName?: string
}

function TableSkeletonRows({
  colSpan,
  rowCount = 8,
  rowClassName,
  cellClassName,
}: TableSkeletonRowsProps) {
  return (
    <>
      {Array.from({ length: rowCount }).map((_, index) => (
        <TableRow key={`table-skeleton-${index}`} className={rowClassName}>
          <TableCell
            colSpan={colSpan}
            className={cn("h-10 px-3 py-2", cellClassName)}
          >
            <div className="flex min-w-0 animate-pulse items-center gap-3">
              <span className="h-3 w-3 rounded-sm bg-muted" />
              <span className="h-3 w-[18%] rounded-sm bg-muted" />
              <span className="h-3 w-[24%] rounded-sm bg-muted/80" />
              <span className="h-3 w-[14%] rounded-sm bg-muted/70" />
              <span className="ml-auto h-3 w-8 rounded-sm bg-muted/70" />
            </div>
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
  TableEmptyRow,
  TableSkeletonRows,
}
