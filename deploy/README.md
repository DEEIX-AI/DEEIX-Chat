# Deployment

Everything needed to run DEEIX Chat with Docker lives in this directory. Copy it to your
server as-is; the application source is not required.

```
deploy/
├── docker-compose.yml            Default: app only, external PostgreSQL + Redis
├── docker-compose.sqlite.yml     Lightweight: app only, SQLite + in-memory cache
├── docker-compose.full.yml       Full: app + PostgreSQL + Redis containers
├── config.example.yaml           Template for the default profile (also used for local development)
├── config.sqlite.example.yaml    Template for the lightweight profile
├── config.full.example.yaml      Template for the full profile
└── services/                     Optional document extraction / OCR services
```

## Run

Pick one profile, copy its template to `config.yaml` **in this directory**, then start it:

```bash
cd deploy
cp config.sqlite.example.yaml config.yaml
docker compose -f docker-compose.sqlite.yml up -d
```

Every compose file mounts `./config.yaml` (relative to this directory) to `/app/config.yaml`.
`deploy/config.yaml` is git-ignored and excluded from the image build context.

The full guide, including profile trade-offs, persistence paths and optional services, is in the
[root README](../README.md#docker-deployment).

## Build a local image

The application image is built from the repository root:

```bash
docker build -t deeix-chat:local .
cd deploy && DEEIX_CHAT_IMAGE=deeix-chat:local docker compose up -d
```

## Optional services

Each service under `services/` is self-contained (its own build context) and attaches to
`deeix-chat-network`, so start an application profile first:

```bash
docker compose -f services/tika/docker-compose.yml up -d
docker compose -f services/tesseract/docker-compose.yml up -d --build
docker compose -f services/docling/docker-compose.yml up -d --build
```
