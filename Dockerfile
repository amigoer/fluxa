# syntax=docker/dockerfile:1.6
#
# Multi-stage Dockerfile for the Fluxa AI gateway. The final image ships
# only the static binary and a minimal root filesystem.
#
# The admin dashboard front-end has been removed pending a rewrite, so the
# image currently builds from Go sources alone and the root URL serves the
# placeholder page baked into web/embed.go. When the new front-end lands,
# reintroduce a node build stage that compiles it into web/dist/ and copy
# that output into the go-build stage before `go build` so go:embed picks
# it up.

FROM golang:1.25-alpine AS go-build
WORKDIR /src

# Cache module downloads separately from the source tree for fast rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

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
