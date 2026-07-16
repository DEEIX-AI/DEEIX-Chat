.PHONY: build run test fmt swagger tidy sync-version check-version

VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.Version=$(VERSION) -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.Commit=$(COMMIT) -X github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo.BuildTime=$(BUILD_TIME)

SWAG := $(shell command -v swag 2>/dev/null || echo "$(shell go env GOPATH)/bin/swag")

sync-version:
	bun ../scripts/sync-version.mjs backend

check-version:
	bun ../scripts/sync-version.mjs --check backend

build: check-version
	mkdir -p ../.cache/deeix-chat
	go build -trimpath -ldflags "$(LDFLAGS)" -o ../.cache/deeix-chat/deeix-chat ./cmd/server

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w $(shell find . -name '*.go' -type f)

swagger: sync-version
	@test -x "$(SWAG)" || (echo "swag 未安装，请先执行: go install github.com/swaggo/swag/cmd/swag@latest" && exit 1)
	$(SWAG) init -g cmd/server/main.go -o docs --parseDependency --parseInternal

tidy:
	go mod tidy
