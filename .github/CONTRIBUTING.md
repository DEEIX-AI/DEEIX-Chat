# Contributing to DEEIX Chat

Thank you for contributing to DEEIX Chat.

## Before You Start

- Search existing issues and pull requests before opening a new one.
- Keep changes focused. Avoid unrelated refactors in feature or bug-fix pull requests.
- For security issues, follow [SECURITY.md](./SECURITY.md) instead of opening a public issue.

## Development Setup

Install all workspace dependencies:

```bash
pnpm install
```

Run the shared quality and test pipelines:

```bash
pnpm check
pnpm test
pnpm build
```

Use `pnpm dev`, `pnpm dev:web`, or `pnpm dev:api` for local development.

Use the example configuration files for local development. Do not commit local secrets or production credentials.

## Pull Request Guidelines

- Explain the problem and the approach.
- Include tests for behavior changes when practical.
- Update documentation when user-facing behavior, deployment steps, API contracts, or configuration changes.
- Commit generated artifacts only when the project requires them. API contract changes must include the generated Swagger files and `packages/api-contract/src/types.generated.ts`.
- Do not commit caches, build output, `.pyc` files, `.env` files, or local storage data.

## Commit Messages

Use the `type: subject` format for every commit message subject.
Use only English letters, numbers, spaces, periods, hyphens, and underscores in the subject.

Allowed types:

- `build`
- `chore`
- `ci`
- `docs`
- `feat`
- `fix`
- `perf`
- `refactor`
- `revert`
- `style`
- `test`

Examples:

- `feat: add model routing priority`
- `fix: handle expired refresh tokens`
- `refactor: simplify channel lookup`

Commit message subjects that do not match this format will fail CI.

## Architecture Boundaries

- `frontend/` owns the user interface, client-side state, message rendering, and admin/user workflows.
- `backend/` owns business APIs, authentication, authorization, model routing, file processing, billing, audit logs, and persistence.
- `docker/` contains optional local services for document extraction, OCR, and related runtime dependencies.
- The frontend should not duplicate backend authorization, billing, provider routing, or file-processing business rules.
- Keep cross-cutting concerns such as security, tracing, storage, and provider clients behind backend infrastructure boundaries.
- Backend startup flows through `cmd -> internal/cli -> internal/app`.
- Backend requests flow through `transport/http -> application -> repository interfaces -> infra implementations`.
- Domain packages own core business types and constants. Shared packages provide reusable response, request metadata, and security helpers.
- Database tables are grouped by domain, including identity, conversations, files/RAG, model routing, tools, billing, settings, audit logs, and system events.
- Financial records, audit logs, system events, file objects, and vector data should remain separate sources of truth.
- Standard HTTP responses use `errorMsg + data`; do not introduce alternate response envelopes.
- User data access must be scoped by authenticated user context unless an admin-only path explicitly requires broader access.
- Request IDs, structured logs, audit records, and generated Swagger files are part of the operational contract.

## API Contract Workflow

- Backend HTTP DTOs and Swagger annotations are the single source of truth for transport contracts.
- `backend/docs/docs.go`, `backend/docs/swagger.json`, `backend/docs/swagger.yaml`, and `packages/api-contract/src/types.generated.ts` are generated together. Never edit them manually.
- After changing a route, HTTP DTO, JSON tag, validation tag, response document, or Swagger annotation, run `pnpm api:generate` from the repository root. `cd backend && make swagger` invokes the same pipeline.
- Run `pnpm api:check` before submitting. It regenerates into a temporary directory and rejects Swagger or TypeScript drift without modifying the worktree.
- Requiredness, optionality, and nullability must be correct in backend DTOs. Use JSON and validation tags deliberately; use pointer fields when omission must be distinguished from an explicit zero, `false`, or empty value.
- Frontend transport types must import from `@deeix/api-contract`. Do not copy generated request or response fields into local interfaces and do not repair generated contracts with `Required<>`.
- Frontend API adapters may use `Omit`, intersections, or narrow unions only for a real UI/domain invariant. Keep form state and view models separate from wire contracts.
- New HTTP endpoints must expose Swagger contracts before the frontend consumes them. Do not introduce a second handwritten transport schema as a shortcut.

## Backend Contributions

Read the backend documentation index before making backend changes:

- [Backend docs](../backend/docs/README.md)

Core expectations:

- keep HTTP handlers thin
- map transport DTOs to application inputs at the HTTP boundary; do not pass Gin DTOs into domain or infrastructure packages
- keep business orchestration in the application layer
- keep infrastructure implementations behind repository or adapter boundaries
- use structured errors and existing response helpers
- add focused tests for shared behavior and security-sensitive changes

## Frontend Contributions

Read the frontend documentation before making frontend changes:

- [Frontend docs](../frontend/README.md)

Core expectations:

- keep route files thin and place feature logic under `features/*`
- use existing UI components and local design patterns
- keep API access inside `shared/api` or feature-level API modules
- derive wire types from `@deeix/api-contract` and keep UI state types inside the owning feature
- do not hard-code provider-private model behavior in the frontend
- keep authentication tokens aligned with the existing session model
- run `pnpm --filter @deeix/web lint`, and run `pnpm build` for routing, dependency, or Next.js changes

## Code Style

Follow the existing code style and local patterns. Prefer small, direct changes over broad compatibility layers.

## License

By contributing, you agree that your contributions will be licensed under the Apache License, Version 2.0.
