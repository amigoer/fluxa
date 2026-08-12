<div align="center">

# Fluxa

**Go 语言编写的自托管 AI 网关**

用一个兼容 OpenAI 的接口统一接管所有模型厂商，附带按项目发放的密钥、
预算控制、请求日志和 DLP 防护。

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/yourname/fluxa)](https://github.com/yourname/fluxa/releases)
[![Docker](https://img.shields.io/docker/pulls/fluxa/fluxa)](https://hub.docker.com/r/fluxa/fluxa)

[English](README.md) · [中文](README.zh-CN.md)

</div>

---

把应用指向 Fluxa 而不是 OpenAI。它负责把每次调用路由到正确的厂商，执行
按密钥的预算与限流，记录发出去的内容，并拦截会泄露机密的 Prompt。

你的厂商密钥不会离开服务器，流量直连厂商，Fluxa 不赚差价。编译产物是单个
Go 二进制，管理控制台已内嵌其中，只依赖一个 Postgres 数据库。

## 快速开始

```bash
docker compose up -d
```

打开 <http://localhost:8080>，用 `admin` / `admin` 登录后立刻改密码。
数据库 schema 在启动时自动迁移。

如果想用自己的 Postgres 直接跑二进制：

```bash
export FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable"
./fluxa
```

## 接入应用

在控制台里创建一个虚拟密钥，然后改两行代码：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",  # ← 指向你的网关
    api_key="vk-your-virtual-key",        # ← 不是厂商密钥
)

client.chat.completions.create(model="gpt-4o", messages=[...])
```

其余代码完全不动，流式输出照常。Anthropic 生态的客户端（Claude Code、
Cursor）可以直接指向 `/v1/messages`。

## 核心概念

下面这些全部存放在 Postgres 中，通过控制台或 `/admin` REST API 修改。
每次写入都会热加载路由器——不重启，不丢请求。

| 概念 | 作用 |
|---|---|
| **Provider** | 上游厂商，含凭证和 base URL |
| **Route** | 把模型名映射到主 Provider 和一条 fallback 链 |
| **虚拟模型** | 一个对外名称，按权重分流到多个目标 |
| **正则模型** | 按模式改写，例如 `^gpt-4.*` → `gpt-4o` |
| **虚拟密钥** | 按项目发放的凭证，带预算、限流、模型白名单和有效期 |
| **DLP 规则** | 对请求/响应正文做关键词或正则匹配——`block` / `mask` / `log` |

每次调用都会写入请求日志：模型、Provider、Token 数、费用、延迟，开启
`FLUXA_STORE_CONTENT` 后还包含完整正文，便于复现和审计。

## 配置

没有配置文件，进程只读环境变量。

| 变量 | 默认值 | 作用 |
|---|---|---|
| `FLUXA_DATABASE_URL` | — | Postgres DSN，设置后覆盖下面所有 `FLUXA_DB_*` |
| `FLUXA_PORT` | `8080` | 监听端口 |
| `FLUXA_STORE_CONTENT` | `false` | 是否把请求/响应正文写入日志 |
| `FLUXA_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `FLUXA_BOOTSTRAP_PASSWORD` | `admin` | 首次启动的管理员密码 |

<details>
<summary>全部变量</summary>

| 变量 | 默认值 | 作用 |
|---|---|---|
| `FLUXA_DB_HOST` | `localhost` | Postgres 主机 |
| `FLUXA_DB_PORT` | `5432` | Postgres 端口 |
| `FLUXA_DB_USER` | `fluxa` | 数据库角色 |
| `FLUXA_DB_PASSWORD` | — | 角色密码 |
| `FLUXA_DB_NAME` | `fluxa` | 数据库名 |
| `FLUXA_DB_SSLMODE` | `disable` | libpq sslmode |
| `FLUXA_DB_MAX_OPEN_CONNS` | `25` | 连接池上限 |
| `FLUXA_DB_MAX_IDLE_CONNS` | `5` | 保持的空闲连接数 |
| `FLUXA_DB_CONN_MAX_LIFETIME` | `1h` | 连接最大存活时间 |
| `FLUXA_DB_CONN_MAX_IDLETIME` | `10m` | 空闲连接最大存活时间 |
| `FLUXA_HOST` | `0.0.0.0` | 监听地址 |
| `FLUXA_READ_TIMEOUT` | `30s` | HTTP 读超时 |
| `FLUXA_WRITE_TIMEOUT` | `5m` | HTTP 写超时（需覆盖流式响应） |
| `FLUXA_SHUTDOWN_TIMEOUT` | `20s` | 优雅退出等待时间 |
| `FLUXA_LOG_FORMAT` | `json` | `json` \| `text` |
| `FLUXA_BOOTSTRAP_USER` | `admin` | 首次启动的管理员用户名 |

</details>

## 支持的厂商

OpenAI、Anthropic、Gemini、AWS Bedrock、Azure OpenAI、DeepSeek、通义千问、
Ollama、Kimi、智谱 GLM、文心一言、豆包、腾讯混元、MiniMax、百川、阶跃星辰、
讯飞星火、零一万物、硅基流动、Mistral、Groq、xAI、Perplexity、Together、
Fireworks、OpenRouter、Cohere、NVIDIA NIM。

未列出的 OpenAI 兼容厂商同样开箱可用：设 `kind: openai`，把 `base_url`
指向厂商的 `/v1` 端点即可。

<details>
<summary>29 个厂商为什么只需要 5 个 adapter</summary>

Adapter 按**协议方言**划分而不是按厂商划分，同一种协议的所有厂商共用一份
经过充分测试的代码路径，SSE 解析改一次所有厂商同时受益。

| Adapter | 覆盖 | 为何独立 |
|---|---|---|
| `adapter/openai` | 22 个厂商 | 共用 `/v1/chat/completions` 方言，只有 base URL 和密钥不同 |
| `adapter/anthropic` | Claude | 原生 `/v1/messages`，带 `thinking` / `tool_use` 块，字节级透传 |
| `adapter/gemini` | Gemini | 请求结构完全不同，做双向 OpenAI ↔ Gemini 翻译 |
| `adapter/bedrock` | Bedrock | Converse API + 内置 SigV4 签名 + 二进制 EventStream 解析，零 AWS SDK 依赖 |
| `adapter/azure` | Azure OpenAI | deployment 名嵌在 URL 里，走 `api-key` 头，请求体需剥离 `model` |

新增一个 OpenAI 兼容厂商只需在 `internal/router/router.go` 的
`openaiCompatibleDefaults` 里加一行。

</details>

## 本地开发

Go 网关在仓库根目录，管理控制台在 `web/`（React 19 + Vite + Tailwind v4 +
shadcn/ui）。`make build` 会先把控制台编译到 `web/dist`，再用 `go:embed`
打进 `bin/` 下的单个二进制。

控制台支持中英双语，首次访问按浏览器语言自动选择，之后可在顶栏切换。
文案集中在 `web/src/lib/i18n/en.ts`，其他语言按它做类型约束——漏翻一条会
直接构建失败，而不是在界面上漏出英文。

```bash
docker compose up -d postgres

# 网关跑在 :8080，根路径提供上次构建的控制台
FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable" make run

# 控制台热更新跑在 :5173，/admin 和 /v1 代理到 :8080
cd web && npm install && npm run dev
```

```bash
make test      # 未设置 FLUXA_TEST_DATABASE_URL 时自动跳过依赖数据库的用例
make test-db   # 用 docker 起一次性 Postgres 跑全量
```

## 开源协议

MIT。
