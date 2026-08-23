---
name: slide-outline
title: Presentation Outline
description: Plan presentations audience-first — narrative arc, per-slide title, bullets, and speaker notes — and optionally render slides as a self-contained HTML deck.
sort: 100
---

# Presentation Outline

Design a presentation (outline by default, HTML deck on request) that serves its audience and lands one message.

## When to use

- "Help me build a deck/slides/presentation/talk about X", pitch outlines, demo scripts, training sessions.

## Workflow

1. **Clarify the frame** (ask only if missing and essential; otherwise state assumptions): audience, talk length or slide count, goal (decide / understand / act), setting (meeting, conference, demo).
2. **Pick the narrative arc** before any slide: situation → complication → insight → resolution for pitches; problem → approach → demo → evidence → roadmap → ask for updates; overview → deep dives → summary for teaching.
3. **One message per slide.** If a slide needs "and", it's two slides.

## Output format (outline mode — default)

For each slide:

```
## N. Slide title (≤ 8 words, the message itself)
- Bullet (≤ 10 words, max 4–5 per slide; parallel grammar)
- Bullet
Speaker notes: 2–4 sentences — what to say, transitions, the example or number to mention.
[Visual: bar chart comparing X vs Y | diagram of the pipeline | full-bleed screenshot | none]
```

Deck skeleton: opening slide with a title that states the conclusion, agenda only for talks > 15 min, a deliberate "so what" slide before the close, closing slide with the single ask/next step. Include rough timings per section when the talk length is known.

## Slide craft

- Titles carry the message ("Retention doubled after onboarding v2"), not labels ("Results"). A reader skimming only titles gets the whole argument.
- Bullets are parallel, concrete, and quantified; every claim has its number on the slide or in the notes.
- Visual spec in `[Visual: …]` should be specific enough to build: chart type + variables, diagram shape, what the annotation highlights.
- Speaker notes carry what the slide doesn't say: transitions, the anecdote, anticipated questions.
- Rule of thumb for density: ≤ 6 slides per 5 minutes; trim harder for executive audiences.

## HTML deck mode (only when asked)

Render as one ` ```html ` artifact: fixed 16:9 slides (e.g. `1280×720` scaled via `transform` or `aspect-ratio`), arrow-key/click navigation, keyboard `←/→`, slide counter, system fonts only, no external assets. Same one-message-per-slide discipline.

## Quality checklist

- [ ] Skimming slide titles alone tells the complete story.
- [ ] Every slide serves the stated goal; no "everything I know" slides.
- [ ] Numbers on slides match anything the user provided; nothing invented.
- [ ] Timings and the closing ask present.

State assumptions you made (audience, length) in one line before slide 1 so the user can correct course cheaply.
