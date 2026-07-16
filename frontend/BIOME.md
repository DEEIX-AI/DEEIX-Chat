# Frontend Biome policy

DEEIX Chat uses Biome as the frontend linter. TypeScript 7 remains responsible for type checking; ESLint, `typescript-eslint`, and `eslint-config-next` are not part of the frontend toolchain.

## Commands

```bash
bun run check
bun run typecheck
bun run lint
bun run lint:fix
```

`bun run check` is the required non-mutating local and CI gate. It runs `bun run lint` followed by `bun run typecheck`. `lint:fix` applies safe Biome lint fixes only.

Formatting is intentionally outside this migration. Enabling a repository-wide formatter requires a separate mechanical baseline so lint-tooling changes remain reviewable and do not rewrite unrelated application files.

## Rule policy

Biome's recommended preset and the recommended React and Next.js domains are enabled in `biome.jsonc`. Project-specific deviations are listed explicitly so an upgrade cannot silently change the pull request gate.

- `error`: correctness, React hook ordering, duplicate JSX props, invalid React attributes, unsafe script usage, and concrete security defects. Errors fail `bun run lint`.
- `warn`: accessibility guidance, exhaustive hook dependencies, framework performance guidance, and Next.js conventions that can require project-specific exceptions. Warnings stay visible without blocking an otherwise valid build.
- `off`: only for rules that are not reliable with the project's component abstractions. `useAriaPropsSupportedByRole` is disabled because Radix and polymorphic controls expose roles and ARIA attributes through runtime composition that Biome cannot resolve statically. `performance/noImgElement` is disabled because the application deliberately renders administrator-configured provider icons, arbitrary Markdown/tool images, and local previews whose URLs cannot be declared in a fixed Next image domain list.

Rule exceptions use file-scoped Biome overrides only for stable framework/library contracts. Inline suppressions are reserved for a single unavoidable statement and must include a reason. File-wide blanket disables and category-wide exceptions are not accepted.

The Animate UI registry owns `components/animate-ui/icons/icon.tsx` and exports its hook-backed helper as `getVariants`. That upstream API name is preserved so newly downloaded or overwritten registry components remain compatible; the Hook naming rule is disabled only for that exact registry-owned file.

## Migration coverage

Biome covers the correctness, hooks, accessibility, security, and performance rules that can be mapped from the former Next.js ESLint configuration. Biome 2.5 does not yet implement every React Compiler rule or every Next.js-specific rule. TypeScript and Next.js builds remain required checks, and unsupported rules should be reconsidered when Biome adds native equivalents.

Notable unsupported groups include React Compiler diagnostics such as immutability, refs, purity, set-state-in-render, and static component analysis, plus Next.js rules for page-specific HTML, Head, Script, and relative `location` assignments. These gaps are documented rather than hidden behind a second TypeScript runtime.
