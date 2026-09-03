---
name: svg-icon
title: SVG Icon & Logo Design
description: Design clean, scalable SVG icons and simple logos on a consistent grid (24px UI icons, geometric marks) ready for product use.
sort: 40
---

# SVG Icon & Logo Design

Produce production-usable SVG icons and simple logos/marks as standalone ` ```svg ` blocks.

## When to use

- UI icons, app/brand marks, favicons, illustrative spot graphics, simple logos (wordmark + geometric mark).
- Not for detailed illustration or photorealism — recommend an image-generation model for that.

## Output contract

- One fenced ` ```svg ` block per asset, self-contained, no external refs.
- UI icons: `viewBox="0 0 24 24"`, `fill="none"`, `stroke="currentColor"`, `stroke-width="2"`, `stroke-linecap="round"`, `stroke-linejoin="round"` (Feather/Lucide conventions so they inherit surrounding color).
- Logos/marks: `viewBox="0 0 64 64"` (mark) or a wide viewBox for wordmark combos; use explicit brand colors and include a transparent background (no background rect).
- Paths over shapes where possible; expand any text to paths or provide the mark without text (fonts are not embeddable reliably) — if a wordmark is required, note the font choice as a spec comment above the block.

## Design rules

- Grid discipline: UI icons snap to the 24px grid, key points on integers or half-units, 2px padding from the edge (`x`/`y` in `[1, 23]`).
- Optical balance over mathematical: a 20px circle reads equal to a 22px square; nudge so icons look the same weight side by side.
- One visual language per set: same stroke weight, same corner rounding, same density. Never mix filled and outlined styles in one set.
- Logos: start from one geometric idea (letterform, monogram, negative space, meaningful object simplified to ≤ 6 shapes). Must read at 16px. Provide the construction rationale in one sentence.
- Palettes: monochrome for icons; logos get ≤ 3 colors with hex values listed after the block.

## Workflow

1. Restate the concept in one sentence; list constraints (style references, brand colors, where it will be used).
2. Sketch the geometry as coordinates on the grid before writing path data.
3. Emit the SVG, then a short spec: viewBox, stroke rule, palette hexes, padding/safe-area, and how it behaves in dark mode (`currentColor` or explicit dual version).

## Quality checklist

- [ ] Valid standalone SVG; ids/classes prefixed to avoid collisions when inlined.
- [ ] Renders identically at 16px, 24px, 512px.
- [ ] Consistent stroke/weight across the set; nothing touches the viewBox edge.
- [ ] Dark-mode behavior stated; `currentColor` used for tintable icons.

If the user gave a brand name but no direction, propose 2 distinct directions (each its own small SVG block) with one line of rationale instead of one arbitrary take.
