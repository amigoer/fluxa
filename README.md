<div align="center">

# Fluxa

**A self-hosted AI gateway written in Go**

One OpenAI-compatible endpoint in front of every provider you use — with
per-project keys, budgets, request logs and a DLP firewall.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/yourname/fluxa)](https://github.com/yourname/fluxa/releases)
[![Docker](https://img.shields.io/docker/pulls/fluxa/fluxa)](https://hub.docker.com/r/fluxa/fluxa)

[English](README.md) · [中文](README.zh-CN.md)

</div>

---

Point your app at Fluxa instead of at OpenAI. It routes each call to the
right provider, enforces per-key budgets and rate limits, records what was
sent, and blocks prompts that leak secrets.

Your provider keys never leave the server and your traffic goes straight to
the vendor — Fluxa takes no cut. It ships as a single Go binary with the
admin console embedded, and needs nothing but a Postgres database.

## Quick start

```bash
docker compose up -d
```

Open <http://localhost:8080>, sign in with `admin` / `admin`, and change the
password. Schema migrations run automatically at startup.

Running the binary against your own Postgres instead:

```bash
export FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable"
./fluxa
```

## Connect your app

Create a virtual key in the console, then change two lines:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",  # ← your gateway
    api_key="vk-your-virtual-key",        # ← not your provider key
)

client.chat.completions.create(model="gpt-4o", messages=[...])
```

Everything else stays the same, streaming included. Anthropic clients
(Claude Code, Cursor) can point at `/v1/messages` instead.

## How it works

Everything below lives in Postgres and is edited in the console or through
the `/admin` REST API. Writes hot-reload the router — no restart, no
dropped requests.

| Concept | What it does |
|---|---|
| **Provider** | An upstream vendor plus its credentials and base URL |
| **Route** | Maps a model name to a primary provider and a fallback chain |
| **Virtual model** | One caller-facing name that fans out to weighted targets |
| **Regex model** | Pattern rewrite, e.g. `^gpt-4.*` → `gpt-4o` |
| **Virtual key** | Per-project credential with budgets, rate limit, model allowlist and expiry |
| **DLP rule** | Keyword or regex over request/response bodies — `block`, `mask` or `log` |

Every call lands in a request log with model, provider, tokens, cost,
latency and — when `FLUXA_STORE_CONTENT` is on — the full payloads, so you
can replay and audit it later.

## Configuration

There is no config file. The process reads environment variables only.

| Variable | Default | Purpose |
|---|---|---|
| `FLUXA_DATABASE_URL` | — | Postgres DSN. Overrides the discrete `FLUXA_DB_*` vars |
| `FLUXA_PORT` | `8080` | Listen port |
| `FLUXA_STORE_CONTENT` | `false` | Persist request/response bodies in the log |
| `FLUXA_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `FLUXA_BOOTSTRAP_PASSWORD` | `admin` | First-run admin password |

<details>
<summary>Every variable</summary>

| Variable | Default | Purpose |
|---|---|---|
| `FLUXA_DB_HOST` | `localhost` | Postgres host |
| `FLUXA_DB_PORT` | `5432` | Postgres port |
| `FLUXA_DB_USER` | `fluxa` | Postgres role |
| `FLUXA_DB_PASSWORD` | — | Role password |
| `FLUXA_DB_NAME` | `fluxa` | Database name |
| `FLUXA_DB_SSLMODE` | `disable` | libpq sslmode |
| `FLUXA_DB_MAX_OPEN_CONNS` | `25` | Connection pool ceiling |
| `FLUXA_DB_MAX_IDLE_CONNS` | `5` | Idle connections kept open |
| `FLUXA_DB_CONN_MAX_LIFETIME` | `1h` | Recycle connections older than this |
| `FLUXA_DB_CONN_MAX_IDLETIME` | `10m` | Close connections idle longer than this |
| `FLUXA_HOST` | `0.0.0.0` | Listen address |
| `FLUXA_READ_TIMEOUT` | `30s` | HTTP read timeout |
| `FLUXA_WRITE_TIMEOUT` | `5m` | HTTP write timeout (must cover streaming) |
| `FLUXA_SHUTDOWN_TIMEOUT` | `20s` | Graceful shutdown budget |
| `FLUXA_LOG_FORMAT` | `json` | `json` \| `text` |
| `FLUXA_BOOTSTRAP_USER` | `admin` | First-run admin username |

</details>

## Providers

OpenAI, Anthropic, Gemini, AWS Bedrock, Azure OpenAI, DeepSeek, Qwen,
Ollama, Moonshot, GLM, ERNIE, Doubao, Hunyuan, MiniMax, Baichuan, StepFun,
Spark, Yi, SiliconFlow, Mistral, Groq, xAI, Perplexity, Together,
Fireworks, OpenRouter, Cohere and NVIDIA NIM.

Any other OpenAI-compatible vendor works out of the box: set
`kind: openai` and point `base_url` at its `/v1` endpoint.

<details>
<summary>Why 29 vendors need only five adapters</summary>

Adapters are split by **wire protocol**, not by vendor, so one tested code
path serves every vendor speaking the same dialect and a fix to SSE parsing
benefits all of them at once.

| Adapter | Handles | Why it is separate |
|---|---|---|
| `adapter/openai` | 22 vendors | Shared `/v1/chat/completions` dialect — only base URL and key differ |
| `adapter/anthropic` | Claude | Native `/v1/messages` with `thinking` / `tool_use` blocks, passed through byte for byte |
| `adapter/gemini` | Gemini | Different request shape — bidirectional OpenAI ↔ Gemini translation |
| `adapter/bedrock` | Bedrock | Converse API, in-tree SigV4 signer and binary EventStream parser, no AWS SDK |
| `adapter/azure` | Azure OpenAI | Deployment name in the URL, `api-key` header, `model` stripped from the body |

Adding an OpenAI-compatible vendor is one line in `openaiCompatibleDefaults`
(`internal/router/router.go`).

</details>

## Development

The Go gateway lives at the repository root; the admin console is in `web/`
(React 19 + Vite + Tailwind v4 + shadcn/ui). `make build` compiles the
console into `web/dist`, embeds it with `go:embed`, and writes a single
binary to `bin/`.

The console speaks English and Chinese, picked from the browser on first
visit and switchable in the header. Strings live in
`web/src/lib/i18n/en.ts`; every other locale is typed against it, so a
missing translation fails the build rather than leaking English.

```bash
docker compose up -d postgres

# gateway on :8080, serving the last built console at /
FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable" make run

# console with hot reload on :5173, /admin and /v1 proxied to :8080
cd web && npm install && npm run dev
```

```bash
make test      # database-backed tests skip unless FLUXA_TEST_DATABASE_URL is set
make test-db   # runs everything against a throwaway Postgres in docker
```

## License

MIT.
