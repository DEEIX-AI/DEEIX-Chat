"use client";

import * as React from "react";
import { useVirtualizer, type VirtualItem } from "@tanstack/react-virtual";

export const VIRTUAL_TABLE_PAGE_ROW_LIMIT = 100;
export const VIRTUAL_TABLE_OVERSCAN = 12;
export const VIRTUAL_TABLE_ROW_HEIGHT = 40;
const VIRTUAL_TABLE_MAX_HEIGHT_PX = 768;

type VirtualTableOptions = {
  enabled?: boolean;
  estimateSize?: number;
  maxHeight?: number;
  overscan?: number;
};

export type VirtualTableRow<T> = {
  item: T;
  index: number;
  virtualItem: VirtualItem | null;
};

export function useVirtualTableRows<T>(
  items: readonly T[],
  {
    enabled = items.length > VIRTUAL_TABLE_PAGE_ROW_LIMIT,
    estimateSize = VIRTUAL_TABLE_ROW_HEIGHT,
    maxHeight = VIRTUAL_TABLE_MAX_HEIGHT_PX,
    overscan = VIRTUAL_TABLE_OVERSCAN,
  }: VirtualTableOptions = {},
) {
  const viewportRef = React.useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: enabled ? items.length : 0,
    enabled,
    estimateSize: () => estimateSize,
    getScrollElement: () => viewportRef.current,
    overscan,
  });

  const virtualItems = enabled ? virtualizer.getVirtualItems() : [];
  const firstVirtualItem = virtualItems[0];
  const lastVirtualItem = virtualItems.at(-1);
  const totalSize = enabled ? virtualizer.getTotalSize() : 0;
  const paddingTop = firstVirtualItem?.start ?? 0;
  const paddingBottom = lastVirtualItem ? Math.max(0, totalSize - lastVirtualItem.end) : 0;
  const rows: Array<VirtualTableRow<T>> = enabled
    ? virtualItems
      .map((virtualItem): VirtualTableRow<T> | null => {
        const item = items[virtualItem.index];
        return item === undefined
          ? null
          : { item: item as T, index: virtualItem.index, virtualItem };
      })
      .filter((row): row is VirtualTableRow<T> => row !== null)
    : items.map((item, index): VirtualTableRow<T> => ({ item, index, virtualItem: null }));

  return {
    enabled,
    paddingTop,
    paddingBottom,
    rows,
    viewportClassName: "max-h-[var(--virtual-table-max-height)] overflow-auto [&_thead]:sticky [&_thead]:top-0 [&_thead]:z-20",
    viewportRef,
    viewportStyle: {
      "--virtual-table-max-height": `${maxHeight}px`,
    } as React.CSSProperties,
  };
}

export function VirtualTablePaddingRow({
  colSpan,
  height,
}: {
  colSpan: number;
  height: number;
}) {
  if (height <= 0) {
    return null;
  }
  return (
    <tr aria-hidden="true" data-slot="virtual-table-padding-row">
      <td
        colSpan={colSpan}
        className="border-0 p-0"
        style={{ height }}
      />
    </tr>
  );
}
