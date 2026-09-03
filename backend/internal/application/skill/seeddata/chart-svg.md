---
name: chart-svg
title: SVG Data Visualization
description: Draw accurate, well-labeled SVG charts (bar, line, area, pie, scatter) computed from the user's data, with correct scales, axes, and legends.
sort: 50
---

# SVG Data Visualization

Render the user's data as precise hand-computed SVG charts in ` ```svg ` blocks.

## When to use

- "Chart/graph/plot this data", comparisons, trends over time, composition, distribution.
- Small-to-medium datasets (≤ ~30 points). For larger data, aggregate first and say so.

## Output contract

- One ` ```svg ` block per chart, self-contained, light background rect, `viewBox` around `0 0 760 460`, system font stack via root `font-family`.
- Always include: title (16–18px semibold), axis labels with units, tick values, and — when more than one series — a legend. Direct-label lines/bars when space allows (legend then optional).
- After the chart, include the underlying numbers as a small markdown table so the user can verify and copy them.

## Chart choice

- Time series / trend → line or area. Composition of a whole (≤ 5 slices) → bar preferred, pie only if the user insists. Comparison across categories → sorted horizontal bars. Relationship of two variables → scatter. Never 3D, never dual axes without an explicit warning.

## Accuracy rules (non-negotiable)

- Compute the scale honestly: nice tick steps (1/2/5 × 10^n), start bar/line charts at zero unless there's a stated reason, and if a truncated axis is truly needed, mark it clearly.
- Map values to pixels with an explicit formula (`x = left + (value - min) / (max - min) * width`) and compute each coordinate; round to 1 decimal. A chart that misencodes its data is a defect regardless of looks.
- Label exact values on bars (10–12px, above the bar) when there are ≤ 12 bars.
- Pie: convert percentages to arc endpoints with `path` arcs (not stroke-dash tricks), first slice at 12 o'clock clockwise, slices ≥ 3% get labels, everything smaller groups into "Other".

## Design rules

- Palette: `#2563eb`, `#16a34a`, `#d97706`, `#dc2626`, `#7c3aed`; grid lines `#e2e8f0`; text `#0f172a` / secondary `#475569`.
- Gridlines: horizontal only, behind data, no chartjunk (no gradients, shadows, decorative icons).
- 16px margins around the plot area; series labels never overlap — if they collide, rotate or shorten them deliberately.

## Quality checklist

- [ ] Every plotted pixel traces back to the data (recompute two points mentally).
- [ ] Axes show units; ticks are round numbers; zero baseline unless flagged.
- [ ] Title answers "what am I looking at"; legend/direct labels identify every series.
- [ ] Data table below matches the chart exactly.

State any assumption (aggregation applied, excluded rows, currency) in one sentence after the table. Never invent data points to fill gaps — show gaps as gaps.
