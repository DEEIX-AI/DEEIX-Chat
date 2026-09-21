import {
  Calculator,
  ChartPie,
  FileDiff,
  FileQuestionMark,
  LayoutGrid,
  type LucideIcon,
  Network,
  Sheet,
  SquareChartGantt,
  SquareFunction,
  WalletCards,
} from "lucide-react";

// Icon per builtin component name; custom components fall back to LayoutGrid.
// stat-grid wants PlayingCardsFan, which lands in a newer lucide release.
const BUILTIN_ICONS: Record<string, LucideIcon> = {
  "card-grid": LayoutGrid,
  chart: ChartPie,
  "function-plot": SquareFunction,
  "stat-grid": WalletCards,
  gantt: SquareChartGantt,
  "data-table": Sheet,
  calculator: Calculator,
  "decision-tree": Network,
  diff: FileDiff,
  quiz: FileQuestionMark,
};

export function uiComponentIcon(name: string): LucideIcon {
  return BUILTIN_ICONS[name] ?? LayoutGrid;
}
