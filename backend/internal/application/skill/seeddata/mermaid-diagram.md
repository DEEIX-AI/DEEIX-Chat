---
name: mermaid-diagram
title: Mermaid Diagrams
description: Render flowcharts, sequence diagrams, state machines, gantt charts, and mind maps as Mermaid code the chat renders natively. Use when the user asks for Mermaid or wants an editable text diagram.
sort: 20
---

# Mermaid Diagrams

Produce Mermaid diagrams the chat renders natively from a fenced ` ```mermaid ` block.

## When to use

- The user explicitly asks for Mermaid, or wants a diagram they can edit as text or paste into docs (GitHub, GitLab, Notion, Obsidian).
- Flowcharts, sequence diagrams, state diagrams, ER diagrams, gantt/timeline, pie, mindmap, class diagrams.
- Prefer the `diagram-svg` skill when the user wants pixel-level visual polish or a downloadable image.

## Output contract

- Exactly one fenced ` ```mermaid ` block per diagram. Nothing else inside the fence.
- Pick the correct diagram type for the question: flowchart for decisions/pipelines, `sequenceDiagram` for API/service interactions over time, `stateDiagram-v2` for lifecycles, `gantt` for schedules, `mindmap` for hierarchies of ideas.

## Syntax rules that keep diagrams rendering

- First line is the diagram declaration with `direction` for flowcharts: `flowchart TD` / `flowchart LR`. Use `TD` for flows, `LR` for pipelines.
- Node ids are alphanumeric (`A1`, `svc`, `db`); put human labels in brackets: `A1[Validate input]`. Never reuse an id, never use reserved words (`end`, `graph`) as ids.
- Quote labels containing punctuation or non-ASCII: `A["Total (USD $)"]`.
- Edge labels: `A -->|yes| B`. Wrap long labels; keep them ≤ 4 words.
- In `sequenceDiagram`: declare participants with `participant X as "Display Name"`; use `->>` for calls, `-->>` for returns, `-x` for failures. Add `activate`/`deactivate` or `+`/`-` only when lifelines matter. `alt/else`, `loop`, `opt` blocks for branching.
- In `stateDiagram-v2`: `[*]` for start/end states; `state "Long name" as s1`.
- Escape problematic characters inside labels with quotes rather than HTML entities; avoid `<br/>` unless a label truly needs two lines.

## Style discipline

- Keep styling minimal and portable: at most classDef for 2–3 semantic colors (e.g. highlight the failure path), no `init` directives, no themes, no `%%{init}%%` blocks — many renderers ignore or break on them.
- 6–12 nodes per diagram. If the story needs more, split into an overview + one detail diagram and say so.
- One idea per diagram: branch on the question being asked ("how does auth fail?", "what's the request lifecycle?").

## Quality checklist

- [ ] Diagram type matches the question (sequence for interactions, flowchart for logic).
- [ ] Parses with standard Mermaid (no plugin-only syntax).
- [ ] Every node reachable, no orphan labels, arrows read left-to-right or top-to-bottom consistently.
- [ ] Labels short and unambiguous; ids unique; no reserved words.

After the block, briefly say what the diagram covers and any assumption made. If the user's description is contradictory, draw the most reasonable interpretation and note the conflict in one sentence.
