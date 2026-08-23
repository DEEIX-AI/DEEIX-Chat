---
name: diagram-svg
title: SVG Diagram Builder
description: Design clear, self-contained SVG diagrams (flowcharts, architecture, sequence, ER, network) that render inline and download cleanly. Use when the user asks for a diagram, visual, or schematic.
sort: 10
---

# SVG Diagram Builder

Produce polished, standalone SVG diagrams the chat UI can preview and the user can download as `.svg`.

## When to use

- Flowcharts, architecture / system diagrams, sequence-style interactions, ER diagrams, network topologies, org charts, pipelines, timelines.
- Anything the user calls a "diagram", "schematic", "visual overview", "map of how X works".
- Prefer the `mermaid-diagram` skill when the user explicitly asks for Mermaid or wants an editable text source.

## Output contract

- Emit exactly one fenced block per diagram: ` ```svg ` … ` ``` `. Never inline SVG in raw HTML.
- The SVG must be fully self-contained: no external fonts, images, CSS, or scripts.
- Start with `<?xml version="1.0" encoding="UTF-8"?>` and use a root like:

```
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 W H" font-family="-apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif">
```

- Draw an explicit light background (`<rect width="100%" height="100%" fill="#ffffff"/>`) so the diagram stays readable on dark UI themes.
- Keep width between 720–1200 viewBox units. Do not set fixed `width`/`height` attributes; let `viewBox` scale.

## Design rules

- **Palette (max 5 colors):** neutral container `#f8fafc` / stroke `#334155`; primary accent `#2563eb`; success `#16a34a`; warning `#d97706`; danger `#dc2626`. Text `#0f172a`, secondary text `#475569`.
- Consistent corner radius (8–12), one stroke weight for boxes (1.5–2) and one for connectors (1.5). Arrowheads via `<marker>` with `markerWidth/markerHeight` around 8.
- One font family, 2 sizes: title 18–20 semibold, labels 13–14. Node labels must be short (≤ 4 words); put detail in a legend or caption below the diagram.
- Every node: rounded rect + centered text. Compute text width honestly (≈7px per character at 14px) and pad boxes 16px horizontal, 10px vertical. Never let text overflow a box.
- Connectors are orthogonal or smooth curves with elbow routing; leave ≥ 24px gap between any line and unrelated boxes. Label conditional edges (`yes` / `no`, `on success`) in 12–13px on a small white-backed pill.
- Group related nodes with a light container (`fill="#f1f5f9"`, dashed stroke) and a 12–13px uppercase group label.

## Layout workflow

1. List the nodes and edges first (mentally or in a short plan). Choose a direction: top-to-bottom for flows, left-to-right for pipelines and layered architecture.
2. Assign rows/columns; identical node types share size. Compute coordinates on a grid (multiples of 8) before writing any tag.
3. Draw containers → nodes → connectors → labels → legend. Title top-left, optional caption bottom.
4. Re-verify every coordinate pair against your grid math; overlapping or crossing elements are defects.

## Quality checklist (self-check before answering)

- [ ] Renders standalone (paste into any browser) with correct namespaces.
- [ ] No text overflows, overlaps, or clipped edges; all arrows touch their target.
- [ ] Colors/typography follow the palette; total distinct colors ≤ 5.
- [ ] Meaning is graspable in < 10 seconds; legend present if shapes encode anything.

After the block, add one or two sentences explaining what the diagram shows and how to read it. If requirements were ambiguous, state the assumption you drew with (e.g. "assumed synchronous calls") instead of asking first.
