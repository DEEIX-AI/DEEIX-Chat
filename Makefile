# DEEIX Chat — single entry point for local development.
# `make` / `make help` lists targets. Package-level tooling (pnpm, turbo, cargo,
# go) stays where it is; this file only routes to it so nothing needs a `cd`.

.DEFAULT_GOAL := help
SHELL := /bin/bash
PNPM ?= pnpm

.PHONY: help setup dev api web desktop build build-api build-web build-desktop \
        release-desktop check test verify lint fmt api-docs version version-check clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make <target>\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Setup
setup: ## Install JS dependencies and Go modules
	$(PNPM) install
	cd backend && go mod download

##@ Develop
dev: ## Run API + web together (turbo)
	$(PNPM) dev

api: ## Run the Go API (backend/config.yaml)
	$(MAKE) -C backend run

web: ## Run the web app at http://localhost:3000
	$(PNPM) --filter @deeix/web dev

desktop: ## Run the desktop app (dev identifier, own data dir)
	$(PNPM) --filter @deeix/desktop dev

##@ Build
build: ## Build every workspace (turbo, cached)
	$(PNPM) build

build-api: ## Build the Go server into .cache/deeix-chat/
	$(MAKE) -C backend build

build-web: ## Static export of the web app into apps/web/out
	$(PNPM) --filter @deeix/web build

build-desktop: ## Quick local desktop build (.app only, relaxed LTO)
	$(PNPM) --filter @deeix/desktop build:app

release-desktop: ## Desktop release build, signed when deploy/secrets/desktop-signing.env is filled
	$(PNPM) --filter @deeix/desktop build:signed

##@ Quality
check: ## Lint + typecheck + architecture rules, all workspaces
	$(PNPM) check

test: ## Tests, all workspaces
	$(PNPM) test

verify: ## check + test + build (what CI runs)
	$(PNPM) verify

lint: ## Go lint + JS lint
	$(MAKE) -C backend lint
	$(PNPM) -r --parallel --if-present lint

fmt: ## Format Go and JS sources
	$(MAKE) -C backend fmt
	$(PNPM) -r --parallel --if-present lint:fix

##@ Maintenance
api-docs: ## Regenerate Swagger docs and the TypeScript API contract
	$(PNPM) api:generate

version: ## Propagate VERSION to every package manifest
	node scripts/sync-version.mjs

version-check: ## Fail if any manifest drifted from VERSION
	node scripts/sync-version.mjs --check

clean: ## Remove build outputs
	$(PNPM) clean
