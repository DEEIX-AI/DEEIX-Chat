# syntax=docker/dockerfile:1
#
# Single build path for the runtime image: the web app and the API are both built from source here,
# so `docker build .` and the release workflow (.github/workflows/image.yml) produce the same image.
# Dependency and compiler caches live in BuildKit cache mounts; CI persists them between runs.

FROM node:24-bookworm-slim AS frontend-builder

WORKDIR /src

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0

ARG NEXT_PUBLIC_API_BASE_URL=""
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json
COPY backend/package.json ./backend/package.json
COPY packages/api-contract/package.json ./packages/api-contract/package.json
COPY packages/core/package.json ./packages/core/package.json
COPY apps/web/scripts ./apps/web/scripts
COPY apps/web/public/pwa ./apps/web/public/pwa

RUN corepack enable

RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    --mount=type=cache,id=corepack,target=/root/.cache/node/corepack \
    pnpm config set store-dir /pnpm/store \
    && pnpm install --frozen-lockfile --prefer-offline --filter @deeix/web

COPY VERSION /src/VERSION
COPY scripts /src/scripts
COPY apps/web ./apps/web
COPY packages/api-contract ./packages/api-contract
COPY packages/core ./packages/core

WORKDIR /src/apps/web

# Turbopack keeps its build cache in .next/cache (enabled by default since Next.js 16).
RUN --mount=type=cache,id=next-cache,target=/src/apps/web/.next/cache \
    --mount=type=cache,id=corepack,target=/root/.cache/node/corepack \
    pnpm build


FROM golang:1.26.8-bookworm AS backend-builder

WORKDIR /src/backend

ARG GIT_COMMIT=unknown
ARG BUILD_TIME=""
COPY VERSION /src/VERSION
COPY backend/go.mod backend/go.sum ./

RUN apt-get update \
  && apt-get install -y --no-install-recommends libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*

RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    go mod download

COPY backend ./

# Opt-in: the repository ships catalog snapshots, so a plain build needs no network. The release
# workflow passes true to ship the freshest data; a failed fetch keeps the committed snapshot.
ARG REFRESH_CATALOG_SNAPSHOTS=false
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    if [ "${REFRESH_CATALOG_SNAPSHOTS}" = "true" ]; then \
      echo "refreshing catalog snapshots for ${GIT_COMMIT}" \
      && go run ./cmd/catalog-snapshot -keep-on-error; \
    fi

RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    VERSION="$(cat /src/VERSION)" \
    && if [ -z "${BUILD_TIME}" ]; then BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"; fi \
    && CGO_ENABLED=1 \
       go build -trimpath \
       -ldflags="-s -w -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.Version=${VERSION} -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.Commit=${GIT_COMMIT} -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.BuildTime=${BUILD_TIME}" \
       -o /out/deeix-chat ./cmd/server


FROM debian:bookworm-slim AS runtime-deps

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata \
  && rm -rf /var/lib/apt/lists/*


FROM debian:bookworm-slim AS runtime

WORKDIR /app

COPY --from=runtime-deps /etc/ssl/certs /etc/ssl/certs
COPY --from=runtime-deps /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=runtime-deps /etc/localtime /etc/localtime
COPY --from=runtime-deps /etc/timezone /etc/timezone
COPY --from=backend-builder /out/deeix-chat /app/deeix-chat
# Runtime path stays /app/frontend/out so existing FRONTEND_DIST_DIR / config.yaml keep working.
COPY --from=frontend-builder /src/apps/web/out /app/frontend/out
COPY LICENSE NOTICE /app/licenses/DEEIX-Chat/

ENV FRONTEND_DIST_DIR=/app/frontend/out

EXPOSE 8080

VOLUME ["/app/storage", "/app/data"]

CMD ["/app/deeix-chat"]
