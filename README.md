<div align="center">

<img src="docs/assets/logo.svg" width="128" height="128" alt="Fluxa">

# Fluxa

**An internal AI gateway for companies that buy LLM access in bulk and hand it out to employees.**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](docker-compose.yml) [![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](frontend/package.json) [![TypeScript](https://img.shields.io/badge/TypeScript-6.0-3178C6?logo=typescript&logoColor=white)](frontend/package.json) [![Self-hosted](https://img.shields.io/badge/deploy-single%20binary-6366F1)](#running-it-locally) [![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[简体中文](README.zh-CN.md)

</div>

## Screenshots

<img src="docs/assets/screenshots/quickstart.png" alt="Quick start: the OpenAI-compatible endpoint, auth header, and copy-paste examples">

<p align="center"><em>Point an existing OpenAI client at Fluxa's base URL and nothing else changes.</em></p>

|  |  |
|---|---|
| <img src="docs/assets/screenshots/models.png" alt="Models and routing"> | <img src="docs/assets/screenshots/roles.png" alt="Role permissions"> |
| **Models & routing** — per-model pricing and context window, published or draft, with the org's global routing chain underneath. | **Roles** — four built-in roles plus custom ones, resolved down to individual permission points. |
| <img src="docs/assets/screenshots/dlp.png" alt="DLP rules"> | <img src="docs/assets/screenshots/providers.png" alt="Providers"> |
| **DLP rules** — matched by regex *and* the real check digit, ordered by priority, redact or block. | **Providers** — upstream credentials and live health, so a failing provider is visible before anyone reports it. |

<img src="docs/assets/screenshots/login.png" alt="Sign-in screen">

<p align="center"><em>Sign-in: Feishu SSO, or a phone/email one-time code where there is no unified IM.</em></p>

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
