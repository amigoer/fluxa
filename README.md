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

| Provider | Models | Status |
|----------|--------|--------|
| OpenAI | GPT-4o, GPT-4o-mini, o1, o3 | ✅ v1.0 |
| Anthropic | Claude 3.5, Claude 3.7 | ✅ v1.0 |
| DeepSeek | deepseek-chat, deepseek-reasoner | ✅ v1.0 |
| 通义千问 (Qwen) | qwen-max, qwen-plus, qwen-turbo | ✅ v1.0 |
| Ollama | Any local model | ✅ v1.0 |
| Kimi | moonshot-v1 | 🔄 v4.0 |
| 智谱 GLM | glm-4, glm-4-flash | 🔄 v4.0 |
| 文心一言 | ernie-4.0 | 🔄 v4.0 |
| 豆包 | doubao-pro | 🔄 v4.0 |
| Azure OpenAI | All Azure-hosted models | 🔄 v4.0 |
| Google Gemini | gemini-1.5-pro, gemini-2.0 | 🔄 v4.0 |
| AWS Bedrock | Claude, Llama, Titan | 🔄 v4.0 |

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

```yaml
# fluxa.yaml

server:
  port: 8080
  master_key: ${FLUXA_MASTER_KEY}

database:
  path: ./fluxa.db

providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1

  - name: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    base_url: https://api.anthropic.com

  - name: deepseek
    api_key: ${DEEPSEEK_API_KEY}
    base_url: https://api.deepseek.com/v1

routes:
  - model: gpt-4o
    provider: openai
    fallback: [deepseek]
  - model: claude-3-5-sonnet
    provider: anthropic
  - model: deepseek-chat
    provider: deepseek

logging:
  level: info
  format: json
  store_content: false
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
