"use client"

import * as React from "react"
import { ScrollArea as ScrollAreaPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"
import { Spinner, SpinnerLabel } from "@/components/ui/spinner"
import { useTableViewportHeight } from "@/components/ui/use-table-viewport-height"

type TableBodyProps = React.ComponentProps<"tbody">

type TableProps = React.ComponentProps<"table"> & {
  shellClassName?: string
  viewportClassName?: string
  viewportRef?: React.Ref<HTMLDivElement>
  viewportStyle?: React.CSSProperties
}

function Table({
  className,
  shellClassName,
  viewportClassName,
  viewportRef,
  viewportStyle,
  ...props
}: TableProps) {
  const viewportHeight = useTableViewportHeight({
    disabled: viewportStyle?.height !== undefined,
    externalRef: viewportRef,
  })
  const {
    contentRef,
    heightStyle,
    viewportRef: resolvedViewportRef,
  } = viewportHeight

  return (
    <div
      data-slot="table-container"
      className={cn("min-w-0 overflow-hidden rounded-lg border border-border/60 bg-background", shellClassName)}
    >
      <ScrollAreaPrimitive.Root type="hover" scrollHideDelay={500} className="relative">
        <ScrollAreaPrimitive.Viewport
          ref={resolvedViewportRef}
          className={cn("data-table-viewport w-full", viewportClassName)}
          style={{
            ...viewportStyle,
            ...heightStyle,
          }}
        >
          <div ref={contentRef} className="min-w-full align-middle">
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
        </ScrollAreaPrimitive.Viewport>
        <ScrollAreaPrimitive.Scrollbar
          orientation="vertical"
          className="z-30 flex w-2 touch-none p-0.5 select-none"
        >
          <ScrollAreaPrimitive.Thumb className="relative flex-1 rounded-full bg-border/80" />
        </ScrollAreaPrimitive.Scrollbar>
        <ScrollAreaPrimitive.Scrollbar
          orientation="horizontal"
          className="z-30 flex h-2 touch-none flex-col p-0.5 select-none"
        >
          <ScrollAreaPrimitive.Thumb className="relative flex-1 rounded-full bg-border/80" />
        </ScrollAreaPrimitive.Scrollbar>
        <ScrollAreaPrimitive.Corner className="bg-transparent" />
      </ScrollAreaPrimitive.Root>
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

function TableBody({ className, ...props }: TableBodyProps) {
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
  tone?: "muted" | "warning"
  "data-interactive"?: string
  "data-selected"?: string
  "data-tone"?: "muted" | "warning" | string
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

type TableLoadingRowProps = {
  colSpan: number
  children?: React.ReactNode
  rowClassName?: string
  cellClassName?: string
}

function TableLoadingRow({
  colSpan,
  children,
  rowClassName,
  cellClassName,
}: TableLoadingRowProps) {
  return (
    <TableRow className={cn("hover:bg-transparent", rowClassName)}>
      <TableCell
        colSpan={colSpan}
        className={cn("h-32 py-8 text-center text-xs text-muted-foreground", cellClassName)}
      >
        {children ? (
          <SpinnerLabel className="justify-center">
            {children}
          </SpinnerLabel>
        ) : (
          <Spinner className="mx-auto text-muted-foreground" />
        )}
      </TableCell>
    </TableRow>
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
  TableLoadingRow,
}
