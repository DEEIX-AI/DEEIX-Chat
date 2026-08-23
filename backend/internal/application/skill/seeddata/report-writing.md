---
name: report-writing
title: Professional Report Writing
description: Structure research, analysis, incidents, and proposals into professional reports with executive summary, clear sections, tables, and explicitly sourced claims.
sort: 60
---

# Professional Report Writing

Turn scattered input (notes, data, code, conversation) into a well-structured professional document.

## When to use

- Status/progress reports, incident postmortems, analysis/evaluation documents, decision proposals, technical reviews, one-pagers.

## Structure template (adapt section names to the domain)

1. **Title + metadata line** — date, author (the user or "prepared for …"), scope in one line.
2. **Executive summary** — 3–6 sentences: what was examined, the key findings, the recommendation. A reader who stops here still leaves informed.
3. **Background / context** — why this document exists, in ≤ 2 short paragraphs.
4. **Findings / analysis** — the body. Each finding gets: claim → evidence → implication. Use tables for comparisons (criteria × options), short paragraphs elsewhere.
5. **Risks / open questions** — honest list with severity and owner where known.
6. **Recommendation / next steps** — concrete actions, each with a verb and, when possible, a deadline or trigger.

## Writing rules

- Lead with the conclusion of each section in its first sentence; support after.
- One idea per paragraph, ≤ 4 sentences. Prefer tables and lists over paragraph-stacked enumerations.
- Quantify: "reduced p95 latency from 820ms to 310ms", not "significantly faster".
- **Sourcing discipline:** only claim what the user supplied or what you were asked to assume. Mark inferred items "(inferred)" and missing data as gaps to fill — never fabricate numbers, dates, quotes, or citations.
- Neutral, concrete register. No filler ("it is important to note that"), no unexplained acronyms on first use, no hedging stacked on hedging.

## Formatting

- Markdown headings (`##`) in the exact template order, bold sparingly for verdicts, `>` blockquotes only for quoted material.
- Numbers: thousands separators, consistent decimal precision, units always. Dates in ISO `2026-08-23` format unless the user's locale convention is established.
- Longer analyses may add a short TOC after the summary; skip it for < 800 words.

## Quality checklist

- [ ] Executive summary stands alone; recommendation is actionable.
- [ ] Every finding has evidence; nothing asserted without basis or marked as assumption.
- [ ] Consistent terminology throughout (one name per concept).
- [ ] A busy reader can extract the 5 key points in 60 seconds.

If the user's input contradicts itself, surface the contradiction explicitly in "Open questions" rather than silently picking a side.
