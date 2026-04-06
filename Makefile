# Makefile for the Fluxa AI gateway.
#
# Targets are intentionally thin wrappers around the Go toolchain so the
# same commands work locally, in CI, and inside Docker builds.

BINARY ?= fluxa
PKG    ?= ./cmd/fluxa
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build run test vet fmt tidy clean docker

build: ## build the fluxa binary into ./bin
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

run: fluxa.yaml ## run fluxa against a local fluxa.yaml
	go run $(PKG) -config fluxa.yaml

# Auto-seed fluxa.yaml from the committed example on first run so
# `make run` works out of the box on a fresh clone.
fluxa.yaml:
	@echo "fluxa.yaml not found — copying configs/fluxa.example.yaml"
	@cp configs/fluxa.example.yaml fluxa.yaml

test: ## run unit tests
	go test ./...

vet: ## run go vet
	go vet ./...

fmt: ## run gofmt across the repo
	gofmt -s -w .

tidy: ## clean go.mod / go.sum
	go mod tidy

clean: ## remove build artefacts
	rm -rf bin dist

docker: ## build the docker image
	docker build -t fluxa/fluxa:$(VERSION) .
