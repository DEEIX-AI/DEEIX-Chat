# Frontend Biome policy

DEEIX Chat uses Biome as the frontend linter. TypeScript 7 (`tsc --noEmit`) is responsible for type checking, and `scripts/check-architecture.mjs` enforces architecture boundaries that lint rules cannot express. ESLint, `typescript-eslint`, and `eslint-config-next` are not part of the frontend toolchain.

Architecture and layering rules are summarized in [README.md](./README.md) and enforced by `scripts/check-architecture.mjs`; this file only covers Biome.

## Commands

```bash
pnpm check        # pnpm lint && pnpm typecheck && pnpm check:arch
pnpm lint         # biome lint .
pnpm lint:fix     # biome lint --write . (safe fixes only)
pnpm typecheck    # tsc --noEmit
pnpm check:arch   # node scripts/check-architecture.mjs
```

`pnpm check` is the required non-mutating gate, both locally and in CI (`.github/workflows/ci.yml` runs `turbo run check` for `@deeix/web`).

The Biome formatter is disabled (`formatter.enabled: false`). Enabling a repository-wide formatter requires a separate mechanical baseline so that tooling changes stay reviewable and do not rewrite unrelated files.

Files excluded from linting (`files.includes` in `biome.jsonc`): `node_modules`, `.next`, `out`, `build`, `next-env.d.ts`, `public/sw.js`, `public/vendor`, and the generated `shared/generated`.

## Severity

`biome.jsonc` enables the `recommended` preset plus the recommended `next` and `react` domains. Each rule keeps Biome's own default severity (`error`, `warn` or `info`) unless `biome.jsonc` overrides it:

- `a11y/noLabelWithoutControl` stays at `error` and lists the shadcn controls that render a native form control (`inputComponents`: `Input`, `Checkbox`, `Switch`, `SelectTrigger`), so a `<label>` wrapping one of them counts as associated. Any other custom control needs an explicit `htmlFor`/`id` pair.
- `correctness/useExhaustiveDependencies` is downgraded to `warn`, with `reportUnnecessaryDependencies: false`.
- `nursery/noComponentHookFactories` and `nursery/noReactStringRefs` are opted in at `error`.
- `suspicious/noAlert` is raised to `error` (its default is `info`, which would not fail the gate). Use the app's dialogs and toasts instead of `alert`/`confirm`/`prompt`.
- `style/useConsistentTypeDefinitions` is opted in at `error` with `style: "type"`: object shapes are declared with `type`. `interface` is kept only where declaration merging is required (augmenting `Window`), with an inline suppression.

`biome lint` exits non-zero only on `error` diagnostics; `warn` and `info` diagnostics are printed but do not fail `pnpm lint`. The current baseline is zero diagnostics of any level, and new warnings are treated as blocking in review. Note that recommended rules such as `suspicious/noExplicitAny`, `style/noNonNullAssertion`, `style/useConst`, `style/useImportType`, `complexity/useOptionalChain` and `correctness/noUnusedFunctionParameters` default to `warn`, and `complexity/noUselessFragments` to `info`.

## Recommended rules turned off globally

| Rule | Reason |
| --- | --- |
| `a11y/noStaticElementInteractions` | The remaining handlers on static elements are hover/pointer affordances (animated icon wrappers in `components/ui` and `components/animate-ui`, hover previews on chat rails, drag/zoom surfaces in media previews and UI blocks, row hover tracking) whose keyboard-operable control is a nested or sibling button. Adding roles to the wrappers would announce duplicate controls. |
| `a11y/useSemanticElements` | The flagged roles are deliberate: `role="group"` on flex/grid containers where `<fieldset>` brings UA border, padding and `min-width` quirks; `role="button"`/`role="checkbox"` on composite rows that contain their own nested buttons, which a native `<button>` or `<input>` cannot contain; `role="region"` on scrollable tables. |
| `performance/noImgElement` (`next` domain) | The app renders administrator-configured provider icons, arbitrary Markdown/tool images, and local previews whose URLs cannot be declared in a fixed Next image domain list; static export also sets `images.unoptimized`. |
| `suspicious/noArrayIndexKey` (`react` domain) | Known debt (about 80 sites): skeleton placeholders, parsed Markdown/UI-block segments and positional table cells that have no stable identity. Re-enable and fix per area when those lists gain stable keys; do not add new index keys for data with an ID. |
| `complexity/noUselessSwitchCase` | Sort/filter switches list the default value as an explicit `case` right before `default` so every accepted value is visible at the switch; the rule would delete that documentation. |

## Not enabled (outside the recommended preset)

These rules are not part of the `recommended` preset or of an enabled domain, so they are off without any entry in `biome.jsonc`. They are listed so that nobody adds a redundant `"off"` entry, and with the reason they are not opted in.

| Rule | Reason |
| --- | --- |
| `correctness/useImageSize` | Only recommended for the Qwik domain. Image dimensions are unknown ahead of time for the same sources listed under `noImgElement`. |
| `performance/noBarrelFile` | Features and entities intentionally expose a public `index.ts` entry; cross-feature imports must go through it (`scripts/check-architecture.mjs`). |
| `performance/noNamespaceImport` | The codebase uses `import * as React from "react"` consistently; tree shaking is unaffected for React. |
| `performance/useTopLevelRegex` | About 200 regex literals, mostly small and in cold paths; hoisting every one to module scope would hurt readability more than it saves. |
| `suspicious/noEmptyBlockStatements` | The only empty blocks are no-op unsubscribe functions (`() => {}`) returned by subscription helpers when there is nothing to subscribe to (server render, empty ID); empty `catch` blocks carry a comment. |
| `suspicious/useAwait` | API wrappers (about 230) are `async` and return the request promise directly, so every exported call has a uniform `Promise` signature and synchronous throws become rejections. |
| `complexity/noExcessiveCognitiveComplexity` | Would flag about 350 functions, mostly large React components and payload normalizers; complexity is handled in review rather than by a fixed score. |
| `style/noNestedTernary` | Nested ternaries (about 275) are the established JSX pattern for multi-state rendering (loading / empty / content) in this codebase. |
| `style/useBlockStatements` | Single-line guard clauses without braces are accepted (about 880); with the formatter disabled this would be a large mechanical rewrite. |

## File-scoped overrides

Each override group in `biome.jsonc` applies to the listed files only.

| Files | Rules off | Reason |
| --- | --- | --- |
| `components/animate-ui/icons/icon.tsx` | `correctness/useExhaustiveDependencies`, `correctness/useHookAtTopLevel` | Registry-owned Animate UI file. Its hook-backed helper keeps the upstream name `getVariants` so re-downloaded registry components stay compatible. |
| `components/animate-ui/utils/get-strict-context.tsx` | `nursery/noComponentHookFactories` | Registry-owned Animate UI utility whose purpose is to return a `Provider` component and a context hook from one factory. |
| `components/ui/live-waveform.tsx`, `components/ui/virtual-table.tsx` | `a11y/noAriaHiddenOnFocusable` | Decorative `aria-hidden` elements (waveform canvas, virtual-table padding rows) that are not actually focusable. |
| `features/admin/components/sections/billing/billing-redemption.tsx`, `features/chat/components/message/message-meta.tsx` | `a11y/noNoninteractiveTabindex` | `tabIndex={0}` on a `<span>` tooltip trigger so keyboard users can reach the tooltip. |
| `features/chat/components/sections/chat-input.tsx`, `features/shell/components/navigation/sidebar-conversation-item.tsx`, `features/settings/components/sections/chat/settings-chat.tsx` | `a11y/noAutofocus` | `autoFocus` on inputs that appear in response to an explicit user action (inline edit/rename fields). |
| `entities/file/components/file-preview/preview-media.tsx` | `a11y/useMediaCaption` | Previews user-uploaded audio/video, for which no caption track exists. |
| `components/ui/input-group.tsx` | `a11y/useKeyWithClickEvents` | shadcn component: clicking the addon forwards focus to the input, which keyboard users reach directly. |

Vendored registry code (`components/ui`, `components/animate-ui`, `components/reactbits`) follows the same rules as the rest of the app; when a registry update reintroduces a pattern the rules reject (for example `interface` props or decorative SVGs without `aria-hidden`), fix it while importing instead of adding a directory override.

## Inline suppressions

| Location | Rule | Reason |
| --- | --- | --- |
| `i18n/app-i18n-provider.tsx` | `suspicious/noDocumentCookie` | Writes the locale cookie; the Cookie Store API is missing in the supported WebKit (macOS 10.15 / Safari 15.6). |
| `components/ui/sidebar.tsx` | `suspicious/noDocumentCookie` | shadcn sidebar state cookie; same Cookie Store API constraint. |
| `features/auth/components/shared/turnstile-widget.tsx`, `components/ui/live-waveform.tsx`, `types/tauri-globals.d.ts` | `style/useConsistentTypeDefinitions` | Augmenting the global `Window` requires `interface` declaration merging. |
| `shared/components/theme-bootstrap-script.tsx` | `security/noDangerouslySetInnerHtml` | Build-time constant inline `<script>` that must run before paint; no user input. |
| `shared/components/markdown/ui-blocks/function-plot.tsx` | `security/noDangerouslySetInnerHtml` | KaTeX markup rendered from a parsed expression AST. |

## Exception policy

- Inline suppressions use the single-statement form `// biome-ignore lint/<group>/<rule>: <concrete reason>`, placed directly above the statement, and are listed in the table above.
- `biome-ignore-all`, `biome-ignore-start`/`biome-ignore-end` range suppressions, and whole-group disables are forbidden.
- A new file-level exception goes into `biome.jsonc` `overrides` and must be added to the file-scoped table with a reason.
- A new global disable (or a changed global severity) requires a reason in the tables above in the same change. Do not add `"off"` entries for rules that are not enabled in the first place.

## Coverage gaps

Biome covers the correctness, hooks, accessibility, security, and performance rules that could be mapped from the former Next.js ESLint configuration. Biome 2.5 does not yet implement every React Compiler rule or every Next.js-specific rule; notable gaps include React Compiler diagnostics (immutability, refs, purity, set-state-in-render, static components) and Next.js rules for page-specific HTML, `Head`, `Script`, and relative `location` assignments. `tsc` and `pnpm build` remain required checks, and these gaps should be revisited when Biome adds native equivalents.
