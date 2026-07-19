# syntax=docker/dockerfile:1

FROM node:24-bookworm-slim AS frontend-builder

WORKDIR /src

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0

ARG NEXT_PUBLIC_API_BASE_URL=""
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json ./frontend/package.json
COPY backend/package.json ./backend/package.json
COPY packages/api-contract/package.json ./packages/api-contract/package.json
COPY frontend/scripts ./frontend/scripts
COPY frontend/public/pwa ./frontend/public/pwa

RUN corepack enable

RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm config set store-dir /pnpm/store \
    && pnpm install --frozen-lockfile --prefer-offline --filter @deeix/web

COPY VERSION /src/VERSION
COPY scripts /src/scripts
COPY frontend ./frontend
COPY packages/api-contract ./packages/api-contract

WORKDIR /src/frontend

# 如果你的 Next 版本支持，可以在 next.config 里开启 turbopack build filesystem cache
RUN --mount=type=cache,id=next-cache,target=/src/frontend/.next/cache \
    pnpm build


FROM golang:1.26-bookworm AS backend-builder

WORKDIR /src/backend

ARG GIT_COMMIT=unknown
ARG BUILD_TIME=""
COPY VERSION /src/VERSION
COPY backend/go.mod backend/go.sum ./

RUN apt-get update \
  && apt-get install -y --no-install-recommends libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
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
COPY --from=frontend-builder /src/frontend/out /app/frontend/out
COPY LICENSE NOTICE /app/licenses/DEEIX-Chat/

ENV FRONTEND_DIST_DIR=/app/frontend/out

EXPOSE 8080

VOLUME ["/app/storage", "/app/data"]

CMD ["/app/deeix-chat"]
