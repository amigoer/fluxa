# Makefile for the Fluxa AI gateway.
#
# Targets are intentionally thin wrappers around the Go toolchain so the
# same commands work locally, in CI, and inside Docker builds. The build
# and run targets also compile the admin dashboard first so the resulting
# binary serves a ready-to-use admin UI at the root URL — one command, no
# separate npm step.
#
# The dashboard front-end has been removed pending a rewrite. Until a new
# web/package.json exists the dashboard steps are skipped and the binary
# serves the placeholder page baked into web/embed.go; once the new
# front-end lands the npm build wires itself back up automatically.

BINARY  ?= fluxa
PKG     ?= ./cmd/fluxa
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)

# Front-end source files that, when changed, force a dashboard rebuild.
# Using a find here keeps the dependency list honest without listing
# every file manually.
WEB_SRC := $(shell find web/src web/index.html web/package.json web/vite.config.ts web/tailwind.config.js web/postcss.config.js web/tsconfig.json 2>/dev/null)

# Empty while no front-end exists, which turns `make web` into a no-op
# instead of a hard failure on the missing web/package.json prerequisite.
WEB_TARGET := $(if $(wildcard web/package.json),web/dist/.built,)

.PHONY: build run web web-clean test test-db vet fmt tidy clean clean-all docker help

help: ## show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: web ## build the fluxa binary (dashboard + gateway) into ./bin
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

run: web ## build dashboard then run fluxa from env vars
	FLUXA_MASTER_KEY=$${FLUXA_MASTER_KEY:-dev} go run $(PKG)

# web is a phony alias for the real dist artefact so callers can type
# `make web` without caring about the file path.
web: $(WEB_TARGET) ## build the embedded admin dashboard (skipped when absent)
	@test -n "$(WEB_TARGET)" || \
	  echo ">> no web/package.json — skipping dashboard build"

# The real build target: a sentinel file inside web/dist. Make rebuilds
# it whenever any of the front-end sources are newer, and skips the
# (slow) npm install step when node_modules is already up to date.
web/dist/.built: web/node_modules/.install-stamp $(WEB_SRC)
	@echo ">> building admin dashboard"
	@command -v npm >/dev/null 2>&1 || { \
	  echo "error: npm not found in PATH. Install Node.js 18+ to build the dashboard."; \
	  exit 1; \
	}
	cd web && npm run build
	@# Vite empties dist/ at the start of every build, which would wipe
	@# the tracked .gitkeep placeholder. Recreate it so a subsequent
	@# `git status` stays clean and a fresh clone still has a non-empty
	@# dist/ for the go:embed directive to latch onto.
	@touch web/dist/.gitkeep
	@touch web/dist/.built

# Cache-friendly npm install: only re-runs when package.json changes.
web/node_modules/.install-stamp: web/package.json
	@command -v npm >/dev/null 2>&1 || { \
	  echo "error: npm not found in PATH. Install Node.js 18+ to build the dashboard."; \
	  exit 1; \
	}
	cd web && npm install
	@mkdir -p web/node_modules
	@touch web/node_modules/.install-stamp

web-clean: ## remove the dashboard build output and node_modules
	rm -rf web/dist web/node_modules
	@# dist/ must stay non-empty for the go:embed directive in embed.go.
	@mkdir -p web/dist && touch web/dist/.gitkeep

# Database-backed tests need a Postgres server; they skip when
# FLUXA_TEST_DATABASE_URL is unset, so `make test` is still meaningful
# without one. See internal/testdb.
test: ## run unit tests (set FLUXA_TEST_DATABASE_URL to include db tests)
	go test ./...

test-db: ## run the full suite against a throwaway Postgres in docker
	docker run -d --rm --name fluxa-pg-test \
	  -e POSTGRES_USER=fluxa -e POSTGRES_PASSWORD=fluxa -e POSTGRES_DB=fluxa_test \
	  -p 5433:5432 postgres:16-alpine >/dev/null
	@until docker exec fluxa-pg-test pg_isready -U fluxa -d fluxa_test >/dev/null 2>&1; do sleep 1; done
	-FLUXA_TEST_DATABASE_URL="postgres://fluxa:fluxa@localhost:5433/fluxa_test?sslmode=disable" go test ./...
	docker stop fluxa-pg-test >/dev/null

vet: ## run go vet
	go vet ./...

fmt: ## run gofmt across the repo
	gofmt -s -w .

tidy: ## clean go.mod / go.sum
	go mod tidy

clean: ## remove go build artefacts
	rm -rf bin dist

clean-all: clean web-clean ## remove every build artefact (go + node)

docker: ## build the docker image
	docker build -t fluxa/fluxa:$(VERSION) .
