<div align="center">

<img src="docs/assets/logo.svg" width="76" height="76" alt="Fluxa">

# Fluxa

**An internal AI gateway for companies that buy LLM access in bulk and hand it out to employees.**

[简体中文](README.zh-CN.md)

</div>

## What it is

A company buys a chunk of LLM budget from various providers (OpenAI, Anthropic, Azure, ...), and Fluxa is what sits between employees and those providers: everyone calls one OpenAI-compatible endpoint, and Fluxa figures out where the request actually goes, whether it's allowed, what it costs, and whether it's safe to send.

It's meant to be deployed once per company (self-hosted, not multi-tenant SaaS) and run as a single binary with an embedded web UI.

## Features

- **Unified procurement ledger** — every top-up from a provider is logged, so spend can eventually be reconciled against real bills instead of trusting a spreadsheet.
- **Quota by department or by person** — admins allocate a department's budget pool, department leads sub-allocate it to their team, employees request more when they need it. Two paths write the same key budget, so the numbers never drift apart.
- **Personal routing with fallback** — employees configure their own model priority per task type on top of the org's default chain, with an optional cost ceiling on the fallback hop.
- **Provider health as a real state machine** — normal → circuit-open → half-open → normal, not a static failover order.
- **DLP that actually checks the checksum** — ID card and bank card numbers are validated against their real check digits, not just matched by length, to cut down on false positives.
- **Login however the company already works** — Feishu SSO if there's a unified IM, or a phone/email one-time-code fallback (no passwords anywhere) for companies without one, gated behind an admin on/off switch.
- **RBAC down to the permission point** — four built-in roles, custom roles on top, one shared admin/employee UI gated by what a session actually holds.

## Stack

Go (chi, pgx, golang-migrate) + PostgreSQL on the backend; React, TypeScript, Tailwind v4, and shadcn/ui on the frontend, built and embedded into the Go binary so the whole thing ships as one deployable artifact. See [`docs/DESIGN.md`](docs/DESIGN.md) for the full design doc — architecture, module boundaries, and the open questions still being worked through.

## Running it locally

```bash
# Postgres
docker compose up -d postgres

# frontend build, embedded into the Go binary
cd frontend && npm install && npm run build && cd ..

# backend (runs migrations on startup)
FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable" \
  go run ./cmd/server
```

Then open `http://localhost:8080` — first run walks you through creating the organization and its first admin.

## License

[MIT](LICENSE)
