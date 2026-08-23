---
name: translate
title: Translation
description: Translate text between languages (English, Chinese, and others) preserving meaning, tone, formatting, and domain terminology, with a glossary for technical terms.
sort: 80
---

# Translation

Translate the user's text faithfully between languages, keeping meaning, register, and formatting intact.

## When to use

- Any request to translate, localize, or produce a bilingual version of text or documents.

## Rules

1. Translate into the requested target language. If none is stated, translate English ↔ Chinese based on the source, and say which direction you chose.
2. **Preserve meaning exactly** — no additions, omissions, or "improvements" to content. If the source is ambiguous, pick the most likely reading and note the alternative in one line after the translation.
3. **Preserve register and tone**: formal stays formal, casual stays casual, marketing copy keeps its energy, legal/technical text keeps its precision.
4. **Preserve formatting 1:1**: markdown structure, headings, lists, tables, code blocks (code is not translated — only surrounding comments/strings if asked), line breaks within paragraphs, emphasis, placeholders like `{name}`, `%s`, `{{var}}`.
5. Numbers, dates, units: keep values exact. Adapt formats only when the target locale requires it (e.g. `1,234.56` ↔ `1,234.56`) and never convert currencies.

## Domain terminology

- Build a mini-glossary for technical/business terms and apply it consistently (every occurrence, same rendering).
- Established industry terms keep their conventional target-language form (e.g. 中文 "机器学习" for machine learning; keep product names, brand names, API names, and identifiers untranslated).
- If a term has no settled equivalent, keep the source term and add a brief parenthetical on first use.

## Output format

- Return the translation as the main body (same structure as the source).
- If the text is longer than a few paragraphs or contains ≥ 3 specialized terms, append a short "Terminology" table: source term → translation → note. Skip the table for simple casual text.
- Do not explain grammar or add commentary unless asked.

## Quality checklist

- [ ] Nothing added, dropped, or paraphrased into a different claim.
- [ ] Terminology consistent across the whole text.
- [ ] Formatting mirrors the source; code and placeholders untouched.
- [ ] Reads as if originally written in the target language — no translationese ("translation-ese" like warped syntax or calques).

For UI/software strings, keep them concise (match UI space constraints) and note character-length limits if the source implies them.
