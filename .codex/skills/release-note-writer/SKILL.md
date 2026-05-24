---
name: release-note-writer
description: Create user-facing GitHub release notes from real git evidence. Use when asked to compare git tags, branches, commits, or version ranges such as v0.1.0...v0.1.1 and draft release notes, changelogs, or bilingual release summaries that emphasize product-visible Features, Optimizations, and Fixes while excluding README-only, version-bump, CI, template, and internal-only noise.
---

# Release Note Writer

## Overview

Draft release notes from the actual repository diff, not from guesses. Keep the output suitable for a GitHub Release page and focused on what users, admins, operators, or integrators can observe.

## Workflow

1. Confirm the comparison target.
   - Use the user-provided range exactly when present, for example `v0.1.0...v0.1.1`.
   - Check that refs exist with `git tag --list`, `git rev-parse`, or `git log --oneline --decorate <range>`.
   - Inspect worktree state with `git status --short`, but do not include unrelated local dirt in release content unless the user explicitly asks for it.

2. Gather evidence from git.
   - Start broad: `git log --oneline --decorate <range>`, `git diff --name-status <range>`, and `git diff --stat <range>`.
   - Read focused diffs for likely product behavior. Prefer files under application, API, route, service, UI, i18n, and shared client layers.
   - Use `rg` to locate labels, routes, settings, and UI strings that reveal user-facing behavior.
   - When commit messages and code disagree, trust the code.

3. Filter the content.
   - Include: new capabilities, changed workflows, admin/user controls, integrations, validation behavior, visible errors, security-relevant behavior users must know, performance or reliability improvements that affect use.
   - Exclude unless explicitly requested: README/docs-only edits, version bumps, generated docs, CI workflow changes, issue templates, config-example-only edits, tests, refactors with no behavior change, dependency churn, formatting, and internal implementation details.
   - Collapse several low-level changes into one release-note item when they support the same user-facing outcome.

4. Classify each item.
   - `Feature`: new or newly exposed product capability.
   - `Optimization`: improved workflow, behavior, compatibility, validation clarity, reliability, or management experience.
   - `Fixed`: resolved defect, incorrect behavior, missing error handling, broken display, or mismatch.
   - If an item could fit multiple sections, choose the section most useful to a release reader.

5. Write in a popular-project release style.
   - Use short bullets with concrete product nouns.
   - Use restrained emoji in headings or major bullets only.
   - Avoid implementation jargon, commit hashes, and file paths unless the user asks for evidence.
   - Prefer active, user-facing wording: "Added Projects for organizing conversations" instead of "Implemented project APIs".
   - Keep release-note text concise; do not over-explain.

## Output Format

When the user asks for English plus Chinese, put English first, then a Chinese version. Use exactly these three sections unless the user requests a different structure:

```markdown
## <version> Release Notes

### ✨ Feature
- ...

### ⚡ Optimization
- ...

### 🐛 Fixed
- ...

---

## <version> 中文版本

### ✨ Feature
- ...

### ⚡ Optimization
- ...

### 🐛 Fixed
- ...
```

## Quality Checks

- Ensure every bullet is backed by the inspected git range.
- Ensure no README-only, version-bump, template-only, or config-example-only item appears unless the user explicitly requested it.
- Ensure Feature, Optimization, and Fixed are all present. If one section has no meaningful user-facing items, include the heading and write `- None.` only if the user requires the section.
- Ensure the Chinese version matches the English version in meaning, not necessarily word-for-word.
- Do not run development servers or long-lived services just to draft release notes.

## Example Request

User: "Compare `v0.1.0...v0.1.1`; draft a GitHub Release Note with Feature, Optimization, Fixed sections, English first and Chinese after it."
