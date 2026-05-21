# Contributing to DEEIX Chat

Thank you for contributing to DEEIX Chat.

## Before You Start

- Search existing issues and pull requests before opening a new one.
- Keep changes focused. Avoid unrelated refactors in feature or bug-fix pull requests.
- For security issues, follow [SECURITY.md](./SECURITY.md) instead of opening a public issue.

## Development Setup

Backend:

```bash
cd backend
go test ./...
```

Frontend:

```bash
cd frontend
pnpm install
pnpm lint
pnpm build
```

Use the example configuration files for local development. Do not commit local secrets or production credentials.

## Pull Request Guidelines

- Explain the problem and the approach.
- Include tests for behavior changes when practical.
- Update documentation when user-facing behavior, deployment steps, API contracts, or configuration changes.
- Keep generated artifacts out of commits unless the project explicitly requires them.
- Do not commit caches, build output, `.pyc` files, `.env` files, or local storage data.

## Architecture Boundaries

- `frontend/` owns the user interface, client-side state, message rendering, and admin/user workflows.
- `backend/` owns business APIs, authentication, authorization, model routing, file processing, billing, audit logs, and persistence.
- `docker/` contains optional local services for document extraction, OCR, and related runtime dependencies.
- The frontend should not duplicate backend authorization, billing, provider routing, or file-processing business rules.
- Shared API contracts should stay explicit through backend DTOs, generated Swagger files, and frontend API types.
- Keep cross-cutting concerns such as security, tracing, storage, and provider clients behind backend infrastructure boundaries.
- Backend startup flows through `cmd -> internal/cli -> internal/app`.
- Backend requests flow through `transport/http -> application -> repository interfaces -> infra implementations`.
- Domain packages own core business types and constants. Shared packages provide reusable response, request metadata, and security helpers.
- Database tables are grouped by domain, including identity, conversations, files/RAG, model routing, tools, billing, settings, audit logs, and system events.
- Financial records, audit logs, system events, file objects, and vector data should remain separate sources of truth.
- Standard HTTP responses use `errorMsg + data`; do not introduce alternate response envelopes.
- User data access must be scoped by authenticated user context unless an admin-only path explicitly requires broader access.
- Request IDs, structured logs, audit records, and generated Swagger files are part of the operational contract.

## Backend Contributions

Read the backend documentation index before making backend changes:

- [Backend docs](./backend/docs/README.md)

Core expectations:

- keep HTTP handlers thin
- keep business orchestration in the application layer
- keep infrastructure implementations behind repository or adapter boundaries
- use structured errors and existing response helpers
- add focused tests for shared behavior and security-sensitive changes

## Frontend Contributions

Read the frontend documentation before making frontend changes:

- [Frontend docs](./frontend/README.md)

Core expectations:

- keep route files thin and place feature logic under `features/*`
- use existing UI components and local design patterns
- keep API access inside `shared/api` or feature-level API modules
- do not hard-code provider-private model behavior in the frontend
- keep authentication tokens aligned with the existing session model
- run `pnpm lint`, and run `pnpm build` for routing, dependency, or Next.js changes

## Code Style

Follow the existing code style and local patterns. Prefer small, direct changes over broad compatibility layers.

## License

By contributing, you agree that your contributions will be licensed under the Apache License, Version 2.0.
