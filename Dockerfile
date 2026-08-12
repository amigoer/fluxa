# syntax=docker/dockerfile:1.6
#
# Multi-stage Dockerfile for the Fluxa AI gateway. The final image ships
# only the static binary and a minimal root filesystem. The build runs in
# two front-of-the-pipe stages:
#
#   1. web-build — compiles the React + Vite + shadcn/ui admin console
#      into web/dist/ so the next stage can go:embed it.
#   2. go-build  — compiles the Go gateway binary with the embedded
#      console baked in.
#
# This keeps the published image tiny while still shipping the UI, so
# operators get the console at the root URL for free on a cold docker pull.

FROM node:22-alpine AS web-build
WORKDIR /web
# Copy the manifests first so the dependency layer is reused whenever only
# application source changed.
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /src

# Cache module downloads separately from the source tree for fast rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Drop in the freshly-built console so go:embed picks it up.
COPY --from=web-build /web/dist ./web/dist

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /out/fluxa ./cmd/fluxa

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/fluxa /app/fluxa

# Fluxa boots from environment variables only — FLUXA_MASTER_KEY is
# the one secret operators must set before the /admin surface opens.
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/fluxa"]
