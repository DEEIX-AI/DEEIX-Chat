import {
  Calculator,
  ChartPie,
  FileDiff,
  FileQuestionMark,
  Images,
  LayoutGrid,
  type LucideIcon,
  Network,
  Sheet,
  SlidersHorizontal,
  Sparkles,
  SquareChartGantt,
  SquareFunction,
  WalletCards,
} from "lucide-react";

// Icon per builtin component name; custom components fall back to LayoutGrid.
// stat-grid wants PlayingCardsFan, which lands in a newer lucide release.
// widget shares Sparkles with the illustration's loading placeholder.
const BUILTIN_ICONS: Record<string, LucideIcon> = {
  "card-grid": LayoutGrid,
  chart: ChartPie,
  "function-plot": SquareFunction,
  "stat-grid": WalletCards,
  "image-gallery": Images,
  gantt: SquareChartGantt,
  "data-table": Sheet,
  calculator: Calculator,
  "decision-tree": Network,
  diff: FileDiff,
  quiz: FileQuestionMark,
  panel: SlidersHorizontal,
  widget: Sparkles,
};

export function uiComponentIcon(name: string): LucideIcon {
  return BUILTIN_ICONS[name] ?? LayoutGrid;
}
