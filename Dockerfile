# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS frontend-builder

WORKDIR /src/frontend

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH

ARG NEXT_PUBLIC_API_BASE_URL=""
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}

COPY VERSION /src/VERSION
COPY scripts /src/scripts
COPY frontend/package.json frontend/pnpm-lock.yaml ./
COPY frontend/scripts ./scripts
COPY frontend/public/pwa ./public/pwa
COPY frontend/public/sw.js ./public/sw.js

RUN corepack enable

RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm config set store-dir /pnpm/store \
    && pnpm install --frozen-lockfile

COPY frontend ./

# 如果你的 Next 版本支持，可以在 next.config 里开启 turbopack build filesystem cache
RUN --mount=type=cache,id=next-cache,target=/src/frontend/.next/cache \
    pnpm build


FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS backend-builder

WORKDIR /src/backend

ARG TARGETOS
ARG TARGETARCH
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=""
COPY VERSION /src/VERSION
COPY backend/go.mod backend/go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    VERSION="$(cat /src/VERSION)" \
    && if [ -z "${BUILD_TIME}" ]; then BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"; fi \
    && CGO_ENABLED=0 \
       GOOS=${TARGETOS} \
       GOARCH=${TARGETARCH} \
       go build -trimpath \
       -ldflags="-s -w -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.Version=${VERSION} -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.Commit=${GIT_COMMIT} -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.BuildTime=${BUILD_TIME}" \
       -o /out/deeix-chat ./cmd/server


FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS runtime-deps

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

EXPOSE 8080

VOLUME ["/app/storage"]

CMD ["/app/deeix-chat"]
