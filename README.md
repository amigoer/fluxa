<div align="center">

# Fluxa

**A high-performance AI gateway written in Go**

Unified routing · Virtual key management · Token tracking · DLP firewall

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/yourname/fluxa)](https://github.com/yourname/fluxa/releases)
[![Docker](https://img.shields.io/docker/pulls/fluxa/fluxa)](https://hub.docker.com/r/fluxa/fluxa)

[English](README.md) · [中文](README_CN.md)

</div>

---

Fluxa is an open-source AI gateway built in Go, designed for teams and individual developers who manage their own API keys across multiple providers. Instead of scattering keys across projects and losing track of token usage, Fluxa gives you a single OpenAI-compatible endpoint that routes to any provider — OpenAI, Anthropic, DeepSeek, Qwen, Ollama, and more.

**No middlemen. No markup. Your keys talk directly to providers.**

Fluxa sits in between just long enough to enforce budgets, track usage, and scan for data leaks — then gets out of the way. Ships as a single binary. Runs in one command.

---

## Why Fluxa

Most teams hit the same problems as their AI usage grows:

- **Key chaos** — API keys scattered across projects, shared over Slack, no idea who's using what
- **Zero visibility** — no way to know which team spent $800 last month until the bill arrives
- **No guardrails** — one engineer pastes a database connection string into a prompt, and it goes straight to OpenAI
- **Provider lock-in** — switching from GPT-4o to DeepSeek means rewriting integrations

Fluxa fixes all of this with one self-hosted binary.

---

## Features

### 🔀 Unified Routing
- Single OpenAI-compatible endpoint — change `base_url`, nothing else
- Native Anthropic `/v1/messages` support for Claude Code, Cursor, and similar tools
- True SSE streaming passthrough — no buffering, no added latency
- Automatic fallback chains when providers go down

### 🔑 Virtual Key Management
- Issue isolated virtual keys per project, team, or developer
- Real provider keys never leave the server
- Set token budgets, dollar budgets, and rate limits per key
- Expiry dates, IP allowlists, enable/disable with one API call

### 📊 Observability
- Every request logged: model, provider, token count, latency, cost
- Built-in cost estimation with up-to-date pricing tables
- Dashboard with usage trends, per-key breakdowns, and provider health
- Export usage data as CSV for finance or reporting

### 🛡️ AI Firewall (v5.0)
- DLP rules engine: phone numbers, ID cards, bank cards, email addresses, and 20+ built-in patterns
- Credential leak detection: API keys, private keys, tokens
- Custom keyword blocklists for internal project names and sensitive terms
- Three enforcement modes: `block`, `mask`, or `alert`
- Observation mode for rule validation before enforcement

### ⚡ Built for Performance
- Written in Go — gateway overhead under 5ms P99
- Single binary, zero external dependencies
- Runs on a $5 VPS, handles 10,000+ concurrent connections
- Cold start under 1 second

---

## Supported Providers

| Provider | Models | Kind | Status |
|----------|--------|------|--------|
| OpenAI | GPT-4o, GPT-4o-mini, o1, o3 | `openai` | ✅ |
| Anthropic | Claude 3.5, Claude 3.7 | `anthropic` | ✅ |
| DeepSeek | deepseek-chat, deepseek-reasoner | `deepseek` | ✅ |
| 通义千问 (Qwen) | qwen-max, qwen-plus, qwen-turbo | `qwen` | ✅ |
| Ollama | Any local model | `ollama` | ✅ |
| Kimi / Moonshot | moonshot-v1, kimi-k2 | `moonshot` | ✅ |
| 智谱 GLM | glm-4, glm-4-flash | `zhipu` | ✅ |
| 文心一言 | ernie-4.0, ernie-3.5 | `ernie` | ✅ |
| 豆包 (Volcengine Ark) | doubao-pro | `doubao` | ✅ |
| Google Gemini | gemini-1.5-pro, gemini-2.0 | `gemini` | ✅ |
| AWS Bedrock | Claude, Llama, Titan (Converse API, in-tree SigV4) | `bedrock` | ✅ |
| Azure OpenAI | Deployment-mapped GPT-4o, GPT-4o-mini | `azure` | ✅ |
| Mistral | mistral-large, codestral | `mistral` | ✅ |
| Groq | Llama 3.3, Mixtral (ultra-fast) | `groq` | ✅ |
| xAI | grok-2, grok-2-mini | `xai` | ✅ |
| Perplexity | sonar online & chat | `perplexity` | ✅ |
| Together AI | Llama, Qwen, Mixtral | `together` | ✅ |
| Fireworks | Llama, Mixtral, DeepSeek | `fireworks` | ✅ |
| OpenRouter | 300+ aggregated models | `openrouter` | ✅ |
| Cohere | command-r-plus, command-r | `cohere` | ✅ |
| NVIDIA NIM | Llama, Mixtral on build.nvidia.com | `nvidia` | ✅ |
| 硅基流动 (SiliconFlow) | Qwen, DeepSeek, Llama mirrors | `siliconflow` | ✅ |
| MiniMax | abab6.5s-chat | `minimax` | ✅ |
| 百川智能 (Baichuan) | Baichuan4 | `baichuan` | ✅ |
| 阶跃星辰 (StepFun) | step-1, step-2 | `stepfun` | ✅ |
| 讯飞星火 (Spark) | Spark v3.5 | `spark` | ✅ |
| 零一万物 (01.AI / Yi) | yi-large, yi-medium | `zero-one` | ✅ |
| 腾讯混元 (Hunyuan) | hunyuan-pro, hunyuan-standard | `tencent` | ✅ |

> Any OpenAI-compatible vendor not listed above still works out of the box:
> set `kind: openai` and point `base_url` at the vendor's `/v1` endpoint.

### Adapter architecture: 5 protocols, 29+ vendors

Fluxa splits adapters by **wire protocol**, not by vendor. One well-tested
code path serves every vendor that speaks the same dialect, so a fix to
SSE parsing or retry logic benefits all of them at once and the binary
stays under 15 MiB.

| Adapter package | Handles | Why it is separate |
|-----------------|---------|--------------------|
| `internal/adapter/openai` | 22 vendors including OpenAI, DeepSeek, Qwen, Ollama, Moonshot, GLM, Doubao, ERNIE, Mistral, Groq, xAI, Perplexity, Together, Fireworks, OpenRouter, Cohere, NVIDIA, SiliconFlow, MiniMax, Baichuan, StepFun, Spark, Yi, Hunyuan | Shared OpenAI `/v1/chat/completions` dialect — only BaseURL and API key differ, registered as a one-liner in `router.openaiCompatibleDefaults` |
| `internal/adapter/anthropic` | Anthropic Claude | Native `/v1/messages` format with `thinking` / `tool_use` blocks — byte-level passthrough preserves original fields |
| `internal/adapter/gemini` | Google Gemini | `contents[].parts[].text`, `systemInstruction`, `generationConfig` — full bidirectional OpenAI ↔ Gemini translation |
| `internal/adapter/bedrock` | AWS Bedrock | Unified Converse API + in-tree SigV4 signer + binary EventStream parser, zero AWS SDK dependency |
| `internal/adapter/azure` | Azure OpenAI | URL embeds deployment name, `api-key` header instead of Bearer, `model` field stripped from request body |

**Adding a new vendor is a one-line change** when it speaks an OpenAI-compatible
API: append `"vendor": "https://api.vendor.com/v1"` to `openaiCompatibleDefaults`
in `internal/router/router.go`. Only write a new adapter package when the
protocol itself is incompatible.

---

## Quick Start

### Docker

```bash
docker run -d \
  --name fluxa \
  -p 8080:8080 \
  -e OPENAI_API_KEY=sk-xxx \
  -e ANTHROPIC_API_KEY=sk-ant-xxx \
  -e DEEPSEEK_API_KEY=sk-xxx \
  -e FLUXA_MASTER_KEY=your-admin-key \
  -v ./fluxa.db:/app/fluxa.db \
  fluxa/fluxa:latest
```

### Binary

```bash
# Download the latest release
curl -L https://github.com/yourname/fluxa/releases/latest/download/fluxa-linux-amd64 -o fluxa
chmod +x fluxa

# Create config
cp fluxa.example.yaml fluxa.yaml
# Edit fluxa.yaml — add your provider keys

./fluxa
```

### Connect your app

Change two lines. Everything else stays the same.

```python
# Python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",  # <- change this
    api_key="vk-your-virtual-key",        # <- change this
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello"}]
)
```

```typescript
// TypeScript / Node.js
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8080/v1",  // <- change this
  apiKey: "vk-your-virtual-key",        // <- change this
});
```

```bash
# curl
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer vk-your-virtual-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## Configuration

Providers and routes live in the SQLite database referenced by
`database.path`. The YAML file only carries server, logging and database
bootstrap settings — on first run the gateway will seed the database from
the `providers` / `routes` sections of the file, and thereafter the
`/admin` REST API is the source of truth. See [`configs/fluxa.example.yaml`](configs/fluxa.example.yaml)
for a complete seed.

```yaml
# fluxa.yaml

server:
  host: 0.0.0.0
  port: 8080
  master_key: ${FLUXA_MASTER_KEY}   # required to enable /admin

database:
  path: ./fluxa.db                  # providers + routes live here

logging:
  level: info
  format: json
  store_content: false
```

### Managing providers and routes at runtime

Every mutation writes to the database and hot-reloads the router with
zero downtime:

```bash
# Add a new provider
curl -X POST http://localhost:8080/admin/providers \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"deepseek","kind":"deepseek","api_key":"sk-xxx"}'

# Attach a route with a fallback chain
curl -X PUT http://localhost:8080/admin/routes/gpt-4o \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"provider":"openai","fallback":["deepseek"]}'

# Remove a route
curl -X DELETE http://localhost:8080/admin/routes/gpt-4o \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY"

# Force a reload from the database
curl -X POST http://localhost:8080/admin/reload \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY"
```

---

## Create a Virtual Key

```bash
# Create a key for your frontend team — GPT-4o only, $50/month limit
curl -X POST http://localhost:8080/admin/keys \
  -H "Authorization: Bearer your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "frontend-team",
    "models": ["gpt-4o", "gpt-4o-mini"],
    "budget_usd_monthly": 50.0,
    "rate_limit_rpm": 100
  }'

# Response
{
  "key": "vk-xxxxxxxxxxxxxx",
  "name": "frontend-team",
  "created_at": "2026-04-06T10:00:00Z"
}
```

---

## Roadmap

| Version | Theme | ETA |
|---------|-------|-----|
| **v1.0** | Core routing — multi-provider + streaming | ✅ |
| **v2.0** | Virtual key management + budget control | Q2 2026 |
| **v3.0** | Observability — dashboard + usage stats | Q2 2026 |
| **v4.0** | Reliability — circuit breaker + caching + more providers | Q3 2026 |
| **v5.0** | AI Firewall — DLP + content security | Q3 2026 |
| **v6.0** | Enterprise — RBAC + SSO + audit logs + clustering | Q4 2026 |

See [PLANNING.md](docs/PLANNING.md) for detailed feature breakdown per version.

---

## vs. Other Tools

| | One API / New API | LiteLLM | Fluxa |
|---|---|---|---|
| Purpose | Token reselling | Developer SDK | Self-hosted gateway |
| Language | JavaScript | Python | **Go** |
| Deployment | Node environment | Python environment | **Single binary** |
| Gateway latency | Medium | 50–200ms | **< 5ms** |
| Chinese models | Partial | Weak | **First-class** |
| DLP / Firewall | No | No | **Built-in** |
| Touches your money | Yes | No | **No** |

---

## Contributing

Contributions are welcome. Please open an issue before submitting a pull request for significant changes.

```bash
git clone https://github.com/yourname/fluxa.git
cd fluxa
make dev
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">

*Flow Through, Stay in Control*

</div>
