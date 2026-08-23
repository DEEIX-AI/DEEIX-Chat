---
name: code-review
title: Code Review
description: Systematic code review with severity-tagged findings (correctness, security, performance, readability), concrete fixes, and praised strengths.
sort: 90
---

# Code Review

Review provided code (snippet, file, diff, or PR) systematically and return prioritized, actionable findings.

## When to use

- "Review this code/PR/diff", pre-merge checks, security or performance-focused passes, or reviewing AI-generated code before committing.

## Review pass order

1. **Understand intent** — what is this code supposed to do? State your understanding in one sentence; findings are judged against intent.
2. **Correctness** — logic errors, edge cases (empty/null/zero/overflow), error paths, race conditions, off-by-one, resource leaks, broken contracts.
3. **Security** — injection (SQL/command/template), authz checks, secret handling, unsafe deserialization, SSRF, path traversal, missing input validation at trust boundaries.
4. **Performance & cost** — N+1 queries, unbounded loops/collections, repeated work in hot paths, missing pagination, sync calls that should be async.
5. **Maintainability** — naming, dead code, duplicated logic, functions > ~50 lines, missing error context, misleading comments, test coverage of changed behavior.

## Output format

Start with a one-paragraph verdict: overall assessment + whether it's safe to merge/ship.

Then findings, each with:

- **[Severity] Title** — severity: `P0 must-fix` (bug/security/data loss), `P1 should-fix` (likely bug, major design flaw), `P2 nice-to-fix` (style, minor perf), `P3 note` (consideration, question).
- Location: file/function/line reference as given (`auth.go:42`, `parseToken` in the diff hunk).
- Why it matters: 1–2 sentences with the concrete failure scenario ("an empty `items` list returns `ErrNotFound`, callers treat it as 404").
- Fix: a concrete patch suggestion (short code block) or a precise instruction.

Order findings by severity, then by impact. End with **Strengths** — 1–3 things done well (guard clauses, good naming, tight tests) — and, if meaningful, **Not reviewed** (aspects out of scope: infra, dependencies I couldn't see).

## Rules

- Verify before flagging: re-read the surrounding code/context for each suspected issue; false positives erode trust. If unsure, mark it a question ("P3: is X guaranteed non-nil here?") instead of asserting a bug.
- Flag what's actually wrong, not style differences from your preferences. Follow the project's existing conventions unless they cause bugs.
- Every P0/P1 must come with a fix suggestion or a clear mitigation path.
- For diffs, review ONLY changed lines and their blast radius — don't pad with pre-existing issues unless they interact with the change (then say they're pre-existing).
- Don't rewrite the whole thing; suggest the minimal sound change.

## Quality checklist

- [ ] Verdict first; findings sorted; each has location, scenario, and fix.
- [ ] No false alarms; uncertain items phrased as questions.
- [ ] Strengths and out-of-scope noted.
