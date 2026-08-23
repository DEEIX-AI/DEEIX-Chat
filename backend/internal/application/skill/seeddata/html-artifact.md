---
name: html-artifact
title: Interactive HTML Artifact
description: Build single-file, self-contained interactive HTML pages (calculators, dashboards, simulators, forms) that preview safely in the chat and download as one .html file.
sort: 30
---

# Interactive HTML Artifact

Produce single-file interactive HTML the chat preview can render in a sandboxed frame and the user can download as one `.html`.

## When to use

- Calculators, converters, planners, what-if simulators, interactive tables/dashboards, visual demos, simple games, styled one-pagers (invoices, resumes, posters).
- When interactivity (inputs, live recompute, sorting/filtering) adds real value over static markdown.

## Output contract

- Exactly one fenced ` ```html ` block containing a complete document: `<!DOCTYPE html>`, `<html>`, `<head>`, `<body>`.
- **Zero external dependencies**: no CDN scripts, no web fonts, no fetch/XHR, no images beyond data URIs or inline SVG. The file must work offline.
- All CSS in one `<style>`, all JS in one `<script>` at the end of `<body>`. No build steps, no modules, no imports.
- Works in a sandboxed iframe: never use `alert`/`confirm`/`prompt`, `localStorage`, `sessionStorage`, cookies, or `window.top`. Keep feedback inline in the DOM.

## Engineering rules

- Semantic, accessible markup: `<label for>`, real `<button>`, `aria-` labels on icon-only controls, focus styles, logical tab order.
- Responsive by default: fluid layout, `clamp()`/relative units, works from 360px to desktop; no horizontal scroll.
- JS: plain functions, one `render()`/`recalculate()` entry point bound to `input`/`change` events; derive everything from state instead of mutating the DOM ad hoc. Guard `NaN`/negative/empty inputs with inline validation messages.
- Prefix money/percent formats per locale the user used. Round money to 2 decimals (banker's rounding only if asked); never show false precision.
- Light AND dark theme: either neutral colors readable on both, or an explicit light background (`#f8fafc`+) chosen deliberately since previews may sit on dark UI.

## Visual quality

- One accent color (e.g. `#2563eb`) + neutrals (`#0f172a` text, `#e2e8f0` borders, `#f8fafc` surfaces). Max 2 font stacks: system sans, monospace for numbers/code.
- 8px spacing grid; 12px control padding; 8–12px radii; visible `:hover`/`:focus` states.
- Results/emphasis via typography and the accent color — not emoji clutter.

## Quality checklist

- [ ] File opens standalone by double-click; no console errors.
- [ ] Every input validated; extreme values produce a message, not `NaN`.
- [ ] Keyboard-usable; labels clickable; readable at 360px width.
- [ ] All numbers derived from the user's actual inputs, formulas stated in the page or the explanation.

After the block, give a 2–3 sentence summary: what it does, key inputs/outputs, and the main formula or assumption (e.g. discount or tax rule) you applied.
