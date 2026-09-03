---
name: polish-writing
title: Polish & Edit Text
description: Improve clarity, flow, grammar, and tone of the user's text while preserving meaning, with a short summary of what changed.
sort: 70
---

# Polish & Edit Text

Edit and improve text the user provides while staying faithful to its meaning.

## When to use

- "Polish/proofread/improve/rewrite this", emails, announcements, documentation, essays, messages, UX copy.

## Workflow

1. Detect the text's language and write the revision in that same language (translate only if asked).
2. Read for meaning first: identify audience, intent, and register before touching words.
3. Revise in passes: structure → clarity → concision → grammar → punctuation → consistency.
4. Return the full revised text in a markdown blockquote or fenced block (so it's copyable), followed by a short change summary.

## What to fix

- Grammar, spelling, punctuation, tense agreement, dangling modifiers.
- Wordiness: cut filler ("in order to" → "to"), redundant adverbs, empty qualifiers.
- Weak structure: burying the ask, paragraphs without a point sentence, lists with inconsistent grammar.
- Tone mismatches: slang in formal docs, stiffness in casual messages, ALL-CAPS or excessive exclamation marks.
- Consistency: terminology, capitalization, number/date formats, Oxford comma usage within one document.

## What to preserve

- Meaning and facts — never add claims, data, or opinions the author didn't state.
- The author's voice and intent: polish, don't homogenize. Keep deliberate stylistic choices (rhetorical questions, contractions) when they serve the intent.
- Formatting conventions already present (markdown, list styles) unless broken.

## Tone presets (apply when specified, else infer from the source)

- **Professional** — clear, courteous, action-first (default for business email/docs).
- **Technical** — precise, unambiguous, present tense, imperative in instructions.
- **Friendly** — warm, contractions welcome, still concise.
- **Concise** — compress to the shortest form that keeps every fact; cut greetings/pleasantries only when clearly safe.

## Change summary format

After the revision, list the 3–6 most meaningful changes in one line each (category: what changed, e.g. "Structure: moved the deadline request to the first sentence"). If a sentence was ambiguous, quote it and note the interpretation you chose. If you suspect a factual error (not grammar), flag it separately instead of "fixing" it.
