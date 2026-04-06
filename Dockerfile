# syntax=docker/dockerfile:1.6
#
# Multi-stage Dockerfile for the Fluxa AI gateway. The final image ships only
# the static binary and a minimal root filesystem, keeping the published
# image under 15 MiB.

FROM golang:1.22-alpine AS build
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
COPY --from=build /out/fluxa /app/fluxa
COPY configs/fluxa.example.yaml /app/fluxa.example.yaml

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/fluxa"]
CMD ["-config", "/app/fluxa.yaml"]
