<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./frontend/public/logo-white.svg" />
    <img src="./frontend/public/logo-black.svg" alt="DEEIX Chat" width="160" />
  </picture>
</p>

<p align="center">
  An enterprise AI workspace for model routing, multimodal chat, files, tools, billing, identity, and operations.
</p>

<p align="center">
  English | <a href="./README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://deeix.com"><img alt="Website" src="https://img.shields.io/badge/Website-deeix.com-black" /></a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0"><img alt="License" src="https://img.shields.io/badge/License-Apache%202.0-blue" /></a>
  <img alt="Next.js" src="https://img.shields.io/badge/Next.js-16-black" />
  <img alt="React" src="https://img.shields.io/badge/React-19-149eca" />
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26-00add8" />
</p>

## Overview

DEEIX Chat gives teams a unified workspace for working with multiple AI models and providers. It combines multimodal chat, model routing, file and RAG workflows, MCP tools, usage billing, identity, audit logs, and operational controls in one product.

The architecture is designed for simple deployment, efficient static delivery, and a predictable Go runtime footprint. The admin console centralizes upstream channels, platform model names, routing priority, pricing, subscriptions, users, and security policies, while the conversation workspace keeps the user experience stable and focused.

![DEEIX Chat workspace](./frontend/public/DEEIX-Chat.jpg)

## Features

| Area | Capabilities |
| --- | --- |
| Conversations | Multi-branch chat, streaming, retries, edits, feedback, sharing, cloned shared conversations, rich markdown rendering, file cards, model metadata, usage details, and execution traces. |
| Media generation | Dedicated image generation and image edit flow with task-aware routing, provider-native OpenAI, Google, and xAI image protocols, default streaming where supported, admin-controlled image streaming capability, generated file storage, preview, download, and run history separated from text chat. |
| Model control plane | Platform model catalog, upstream channels, real upstream models, route bindings, priority and weight routing, model capability JSON, visual option controls, official native tool capability keys, display ordering, vendor mapping, automatic icons, and circuit breaker state. |
| Provider protocols | OpenAI Responses, Chat Completions, Images Generations, and Images Edits; Anthropic Messages; Google/Gemini Generate Content and Image Generation; xAI Responses, Images Generations, and Images Edits; OpenRouter defaults; and custom OpenAI-compatible routes. |
| Request governance | Protocol-aware request assembly, user option allowlists and denylists, system-protected fields, model-scoped official native tool pass-through, previous-response continuation where supported, and context snapshots for review. |
| Files and RAG | File upload, preview, download, single and bulk deletion, per-user storage quota with unlimited mode, MIME detection, text extraction, OCR, full-context injection, image context, chunking, embeddings, and semantic retrieval. |
| Memory and context | Message-window truncation, token-budget truncation, context compression, conversation memory, long-term user memory, RAG evidence records, and prompt trace inspection. |
| Tools | Admin-managed MCP servers, provider-native official tools, tool discovery, per-tool enablement, user-side tool selection, execution limits, retries, trace rendering, and tool result handling. |
| Billing and payments | Subscription plans, top-ups, balances, token/call/duration/tiered model pricing, configurable per-call native tool pricing, free models, prepaid thresholds, usage ledgers, billing snapshots, Stripe Checkout, EPay, and webhook validation. |
| Identity and security | Local login, registration, session management, HttpOnly refresh cookies, 2FA/TOTP, recovery codes, trusted devices, SSO/OIDC/OAuth providers, contact verification, timezone, and locale. |
| Administration | Users, roles, auth providers, upstreams, platform models, route bindings, model pricing, subscriptions, balances, usage logs, audit logs, auth events, system events, and runtime settings. |
| Operations | Efficient static delivery, predictable Go runtime footprint, Docker builds, single-runtime frontend/API serving, public build/version endpoint, new-version refresh prompts, Swagger docs, structured logs, request IDs, Redis caching, PostgreSQL pgvector, optional GeoIP, optional OpenTelemetry, and S3-compatible storage. |

<p>
  <img src="./frontend/public/DEEIX-Chat-Image.png" alt="DEEIX Chat image generation" width="32%" />
  <img src="./frontend/public/DEEIX-Chat-Dark.png" alt="DEEIX Chat dark mode" width="32%" />
  <img src="./frontend/public/DEEIX-Chat-Usage.png" alt="DEEIX Chat usage and billing" width="32%" />
</p>

## Architecture

```text
frontend/  Next.js App Router web application
backend/   Go API service, domain/application layers, infra adapters, Swagger docs
docker/    Optional document extraction and OCR services
```

Backend code follows a layered structure:

```text
cmd -> internal/cli -> internal/app
transport/http -> application -> repository interfaces -> infra implementations
domain -> shared domain types and constants
pkg -> dependency-free technical helpers
```

The database uses domain-prefixed tables for identity, LLM routing, billing, conversations, files, RAG, settings, tools, audit logs, and system events. Financial records, audit trails, system events, and high-growth vector data are kept as separate sources of truth.

## Tech Stack

- Frontend: Next.js 16, React 19, TypeScript, Tailwind CSS, shadcn/ui-style components, Radix/Base UI, Streamdown, KaTeX, Mermaid, Recharts, Motion
- Backend: Go 1.26, Gin, Gorm, PostgreSQL, pgvector, Redis, Swagger, OpenTelemetry, Zap
- Storage: local filesystem or S3-compatible object storage
- File processing: built-in extractors, Apache Tika, Docling, RapidOCR, Tesseract OCR, Paddle OCR, cloud OCR adapters, MinerU, and LLM OCR fallback
- Tooling: MCP Streamable HTTP JSON-RPC
- Runtime: Docker, Docker Compose, PostgreSQL, Redis

## Quick Start

### Local Development

```bash
cp config.example.yaml config.yaml
cd backend
make run
```

```bash
cd frontend
pnpm install
cp .env.example .env.local
pnpm dev
```

URLs: frontend `http://localhost:3000`, API `http://localhost:8080`, Swagger `http://localhost:8080/swagger/index.html`.

The frontend uses `NEXT_PUBLIC_API_BASE_URL` for API requests. For local development, set it in `frontend/.env.local`:

```env
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
```

If omitted, local development defaults to `localhost:8080`; same-origin deployments use the current origin.

### Docker Deployment

Priority: `environment variables > config.yaml > built-in defaults`.

The Docker image runs from `/app`. With the default compose mount below, the backend reads `/app/config.yaml` automatically:

```yaml
volumes:
  - ./config.yaml:/app/config.yaml:ro
```

To use a different file path, set `CONFIG_FILE` to the container path:

```yaml
environment:
  CONFIG_FILE: "/app/config.yaml"
```

Custom config file paths are read from `CONFIG_FILE`. If both compose environment variables and `config.yaml` define the same key, the environment variable wins. In `docker-compose.full.yml`, `POSTGRES_DSN`, `REDIS_ADDR`, and `REDIS_PASSWORD` are set in `environment`, so they override the PostgreSQL and Redis values in `config.yaml`.

`config.yaml` is for static infrastructure and security configuration such as server URLs, database, Redis, storage, GeoIP, tracing, JWT, and encryption keys. Runtime business settings are stored in the database and managed in the admin console, so changing those values in YAML after startup is not the source of truth.

`APP_ENV` accepts `dev`/`development` and `prod`/`production`, normalizes them to `dev` or `prod`, and defaults to `prod` when omitted. Use `dev` only for local development. Public production deployments should keep `APP_ENV=prod` or `APP_ENV=production` and use production secrets.

#### Lightweight Start

Starts only the `app` container. PostgreSQL and Redis must be provided externally. Use this when database and cache services already exist.

```bash
cp config.docker.example.yaml config.yaml
# Edit database.postgres.dsn and database.redis.*.
docker compose up -d
```

This mode primarily uses `config.yaml`; keep compose `environment` empty unless you intentionally want an environment variable to override the file.

#### Full Stack Start

Starts `app`, `postgres`, and `redis`. Use this for local evaluation, development smoke tests, or single-machine deployments without external services.

```bash
cp config.docker.example.yaml config.yaml
docker compose -f docker-compose.full.yml up -d
```

This mode uses compose environment variables for the bundled PostgreSQL and Redis services. Edit `docker-compose.full.yml` or remove those environment entries if you want `config.yaml` to provide these connection values instead.

The default application image is `ghcr.io/deeix-ai/deeix-chat:latest`. Override it with `DEEIX_CHAT_IMAGE` when testing a custom build:

```bash
DEEIX_CHAT_IMAGE=deeix-chat:local docker compose up -d --build
```

Docker URL: `http://localhost:8080`. Keep the Docker `server` section unchanged unless changing ports or public domains; then update compose ports, public URLs, and CORS together.

For troubleshooting, inspect startup logs and verify that the mounted file exists inside the container:

```bash
docker compose exec app ls -l /app/config.yaml
docker compose logs app
```

#### Separated Deployment

Use this mode when the frontend and backend are served from different public origins, for example `https://chat.example.com` and `https://api.example.com`.

1. Configure public URLs.

   - Frontend build variable: `NEXT_PUBLIC_API_BASE_URL=https://api.example.com`
   - Backend config: `server.public_api_base_url=https://api.example.com`
   - Backend config: `server.public_web_base_url=https://chat.example.com`
   - Backend config: `server.cors_allow_origin=https://chat.example.com`

   For Docker image builds, pass the frontend API URL at build time:

   ```bash
   docker build --build-arg NEXT_PUBLIC_API_BASE_URL=https://api.example.com -t deeix-chat .
   ```

2. Build and publish the frontend.

   ```bash
   cd frontend
   pnpm install
   NEXT_PUBLIC_API_BASE_URL=https://api.example.com pnpm build
   ```

   The static output is `frontend/out`. Serve it with Nginx, CDN, object storage, or any static web server. To let the Go backend serve the frontend, place `frontend/out` under `server.frontend_dist_dir`; the Docker image defaults to `/app/frontend/out`.

3. Apply CDN rules.

   | Path | Rule |
   | --- | --- |
   | `/_next/static/*` | Cache for 1 year with immutable assets enabled. |
   | `/logo*.svg`, `/*.ico`, `/*.png`, `/*.jpg`, `/*.webp`, `/*.woff2` | Cache for 1 day to 30 days. |
   | `/`, `/*.html`, `/chat*`, `/recent*`, `/files*`, `/setting*`, `/admin*`, `/share*` | Do not long-cache. Use `no-cache` or a short TTL. |
   | `/api/*`, `/healthz`, `/readyz`, `/swagger/*` | Bypass CDN cache and forward all request headers, methods, query strings, and request bodies. |

   If the CDN serves `frontend/out` from object storage, enable route fallback so clean URLs resolve to their exported `index.html` files, for example `/chat` -> `/chat/index.html`.

4. Configure Stripe Webhook if Stripe is enabled.

   Add this endpoint in Stripe Dashboard:

   ```text
   https://api.example.com/api/v1/billing/payments/stripe/webhook
   ```

   Enable the `checkout.session.completed` event and paste the generated `whsec_...` signing secret into Admin -> Billing -> Payment settings -> Stripe Webhook Secret. This endpoint must bypass CDN cache and preserve the raw request body plus the `Stripe-Signature` header.

## Main Routes

- `/chat` - conversation workspace
- `/share` - public conversation snapshot page
- `/recent` - recent conversations, share status, starred and archived states
- `/files` - file manager
- `/setting` - user account, subscription, preferences, security settings, and product information
- `/admin` - administration console
- `/admin/chat-files` - file, extraction, OCR, RAG, and user storage quota settings

## Common Commands

Backend:

```bash
cd backend
go build ./cmd/server
go test ./...
go vet ./...
make swagger
```

Frontend:

```bash
cd frontend
pnpm lint
pnpm build
```

## Configuration

Static infrastructure configuration is loaded from the repository-level `config.yaml` and can be overridden by environment variables. Runtime business settings are stored in `system_settings` and managed from the admin console.

Docker deployments read `/app/config.yaml` by default when `./config.yaml` is mounted to `/app/config.yaml`. `CONFIG_FILE` can point to another container path.

Frontend build-time variables:

| Variable | Purpose |
| --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | Browser API base URL; set in `frontend/.env.local` for local dev or at build time for separated deployment. |

Common backend environment variables:

| Variable | Purpose |
| --- | --- |
| `APP_ENV` | Runtime environment. Accepts `dev`/`development` and `prod`/`production`; omitted values default to `prod`. |
| `CONFIG_FILE` | Optional custom config file path inside the running process or container. Docker defaults to `/app/config.yaml` through the working directory and compose mount. |
| `HTTP_PORT` | API/runtime port. |
| `JWT_SECRET` | JWT signing secret. Must be strong in production. |
| `DATA_ENCRYPTION_KEY` | Key material for encrypted secrets such as upstream API keys, SSO client secrets, MCP tokens, and TOTP secrets. |
| `POSTGRES_DSN` | PostgreSQL DSN. |
| `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB` | Redis connection settings. |
| `STORAGE_BACKEND` | `local` or `s3`. |
| `STORAGE_ROOT_DIR` | Local storage root. |
| `STORAGE_S3_ENDPOINT`, `STORAGE_S3_REGION`, `STORAGE_S3_BUCKET`, `STORAGE_S3_PREFIX`, `STORAGE_S3_ACCESS_KEY_ID`, `STORAGE_S3_SECRET_ACCESS_KEY` | S3-compatible storage settings. |
| `PUBLIC_API_BASE_URL`, `PUBLIC_WEB_BASE_URL` | Public URLs used for links and callbacks. |
| `GEOIP_PROVIDER` | GeoIP provider. The default `ipwhois` uses the built-in public endpoint. |
| `OTEL_ENABLED`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_INSECURE`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_SAMPLING_RATE` | OpenTelemetry tracing settings. |

Production mode rejects unsafe default secrets, weak encryption keys, wildcard CORS, and non-HTTPS public URLs.

The initial superadmin username is `admin`. When the database has no superadmin account, the backend generates a random password and prints it once in the startup logs while creating the account. The first login forces changing the username and password; later changes are managed from the account flow, not from `config.yaml`.

To retrieve the initial admin password, inspect the backend logs from the first startup and search for `bootstrap superadmin created`; the `username` and `password` fields are the initial login credentials. If a superadmin already exists in the database, the service does not regenerate or print this password again.

## Security Notes

- User passwords are hashed with bcrypt.
- Refresh tokens and recovery-style secrets are stored as hashes.
- Upstream API keys, SSO client secrets, MCP auth tokens, sensitive settings, and TOTP secrets are encrypted with AES-GCM using `DATA_ENCRYPTION_KEY`.
- Access tokens are short-lived and held client-side in memory; refresh tokens are issued through HttpOnly cookies.
- User-supplied model options are filtered before provider requests. System-generated fields such as model, messages, tools, system prompts, headers, and previous-response identifiers are not user-overridable.

## Optional Services

The compose files below attach to `deeix-chat-network`. Create it with `docker network create deeix-chat-network`, or start the root compose stack once before launching these services.

Apache Tika:

```bash
docker compose -f docker/tika/docker-compose.yml up -d
```

Tesseract OCR:

```bash
docker compose -f docker/tesseract/docker-compose.yml up -d --build
```

Docling:

```bash
docker compose -f docker/docling/docker-compose.yml up -d --build
```

RapidOCR:

```bash
docker build -t deeix-chat-rapidocr ./docker/rapidocr
```

These services are optional. The admin file settings decide which extraction or OCR engine is active.

## Documentation

- Backend guide: [backend/README.md](./backend/README.md)
- Backend standards: [backend/docs/README.md](./backend/docs/README.md)
- Frontend guide: [frontend/README.md](./frontend/README.md)
- Contributing: [CONTRIBUTING.md](./CONTRIBUTING.md)
- Security policy: [SECURITY.md](./SECURITY.md)
- Swagger UI: `http://localhost:8080/swagger/index.html`

## Acknowledgements

DEEIX Chat is built on the open-source ecosystem. Thanks to all maintainers and communities in the AI tooling ecosystem.

- [Next.js](https://nextjs.org)
- [Go](https://go.dev)
- [LINUX DO](https://linux.do)

## Contact & Community

- Email: [support@deeix.com](mailto:support@deeix.com)
- Telegram: [t.me/deeix_chat](https://t.me/deeix_chat)

## License

DEEIX Chat is licensed under the [Apache License 2.0](./LICENSE).
