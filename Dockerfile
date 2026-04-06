# syntax=docker/dockerfile:1.6
#
# Multi-stage Dockerfile for the Fluxa AI gateway. The final image ships
# only the static binary and a minimal root filesystem. The build runs
# in two front-of-the-pipe stages:
#
#   1. web-build — compiles the React + shadcn/ui admin dashboard with
#      Vite into web/dist/ so the next stage can go:embed it.
#   2. go-build  — compiles the Go gateway binary with the embedded
#      dashboard baked in.
#
# This keeps the published image tiny while still shipping the UI so
# operators get /ui/ for free on a cold docker pull.

FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /src

# Cache module downloads separately from the source tree for fast rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Drop in the freshly-built dashboard so go:embed picks it up.
COPY --from=web-build /web/dist ./web/dist

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /out/fluxa ./cmd/fluxa

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/fluxa /app/fluxa
COPY configs/fluxa.example.yaml /app/fluxa.example.yaml

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/fluxa"]
CMD ["-config", "/app/fluxa.yaml"]
