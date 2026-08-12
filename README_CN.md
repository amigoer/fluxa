<div align="center">

# Fluxa

**Go 语言实现的高性能 AI 模型网关**

统一路由 · 虚拟 Key 管理 · Token 用量追踪 · DLP 安全防护

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/yourname/fluxa)](https://github.com/yourname/fluxa/releases)
[![Docker](https://img.shields.io/docker/pulls/fluxa/fluxa)](https://hub.docker.com/r/fluxa/fluxa)

[English](README.md) · [中文](README_CN.md)

</div>

---

Fluxa 是一款面向企业和个人开发者的开源 AI 模型网关，使用 Go 语言开发。它解决了一个普遍存在的问题：当你同时使用 OpenAI、Anthropic、DeepSeek、通义千问等多个 AI 服务时，需要维护多套 API Key 和 SDK 配置，Token 用量分散在各个平台控制台，无法统一管控。

Fluxa 让你只需对接一个兼容 OpenAI 格式的接口，由网关负责路由到对应的 Provider，统一管理 Key 权限、用量预算和安全策略。

**你的 Key 直连 Provider，Fluxa 只做路由和管控，不碰你的钱，不赚差价。**

编译为单一二进制文件，零外部依赖，一条命令启动。

---

## 为什么需要 Fluxa

随着 AI 用量增长，几乎每个团队都会遇到这些问题：

- **Key 管理混乱** — API Key 散落在各个项目里，通过微信群传来传去，谁在用、用了多少完全不清楚
- **费用不透明** — 每个月账单来了才发现某个项目花了好几百美元，完全没有预警
- **数据安全隐患** — 员工把数据库密码或客户信息粘进了 Prompt，直接裸奔到 OpenAI 服务器
- **多源配置繁琐** — 项目 A 用 OpenAI，项目 B 用 Claude，项目 C 用 DeepSeek，各自维护一套 SDK 配置

Fluxa 用一个自托管网关解决上面所有问题。

---

## 核心功能

### 🔀 统一路由
- 单一 OpenAI 兼容接口，只改 `base_url`，现有代码零改动
- 原生支持 Anthropic `/v1/messages`，Claude Code、Cursor 等工具直接对接
- 真正的 SSE Streaming 透传，不在网关层 buffer，流式体验零损耗
- 自动 Fallback 链，Provider 故障时无感切换备用模型

### 🔑 虚拟 Key 管理
- 为每个项目、团队或成员颁发独立的虚拟 Key
- 真实 Provider Key 加密存储，永远不暴露给调用方
- 按虚拟 Key 设置可用模型、Token 上限、美元预算、每分钟请求数
- 支持有效期、IP 白名单、一键禁用

### 📊 用量可观测
- 每次请求完整记录：模型、Provider、Token 数、延迟、费用估算
- 内置各主流模型定价表，自动换算美元费用
- Web Dashboard：今日用量、本月费用、各 Key 用量明细、趋势图表
- 用量达到预算阈值时 Webhook / 钉钉 / 飞书通知

### 🛡️ AI Firewall（v5.0）
- 内置 20+ DLP 规则：手机号、身份证、银行卡、邮箱等 PII 检测
- 凭证泄露检测：API Key、数据库密码、私钥等格式识别
- 企业自定义敏感词库，支持通配符匹配
- 三种处置策略：`block`（拒绝）/ `mask`（脱敏放行）/ `alert`（放行并告警）
- 观察模式：先记录不拦截，评估误报率后再开启强制模式

### ⚡ 高性能
- Go 语言实现，网关额外延迟 P99 < 5ms，远优于 Python 方案的 50–200ms
- 单二进制，零依赖，一台 $5 的 VPS 即可运行
- 单实例支持 10,000+ 并发连接，冷启动时间 < 1 秒

---

## 支持的模型 Provider

| Provider | 代表模型 | kind | 状态 |
|----------|---------|------|------|
| OpenAI | GPT-4o, GPT-4o-mini, o1, o3 | `openai` | ✅ |
| Anthropic | Claude 3.5, Claude 3.7 | `anthropic` | ✅ |
| DeepSeek | deepseek-chat, deepseek-reasoner | `deepseek` | ✅ |
| 通义千问 | qwen-max, qwen-plus, qwen-turbo | `qwen` | ✅ |
| Ollama | 任意本地模型 | `ollama` | ✅ |
| Kimi / Moonshot | moonshot-v1, kimi-k2 | `moonshot` | ✅ |
| 智谱 GLM | glm-4, glm-4-flash | `zhipu` | ✅ |
| 文心一言 | ernie-4.0, ernie-3.5 | `ernie` | ✅ |
| 豆包 (火山方舟) | doubao-pro | `doubao` | ✅ |
| Google Gemini | gemini-1.5-pro, gemini-2.0 | `gemini` | ✅ |
| AWS Bedrock | Claude / Llama / Titan（Converse API，内置 SigV4） | `bedrock` | ✅ |
| Azure OpenAI | 基于 deployment 映射的 GPT 系列 | `azure` | ✅ |
| Mistral | mistral-large, codestral | `mistral` | ✅ |
| Groq | Llama 3.3、Mixtral（超低延迟） | `groq` | ✅ |
| xAI | grok-2, grok-2-mini | `xai` | ✅ |
| Perplexity | sonar 在线/聊天 | `perplexity` | ✅ |
| Together AI | Llama / Qwen / Mixtral 聚合 | `together` | ✅ |
| Fireworks | Llama / Mixtral / DeepSeek | `fireworks` | ✅ |
| OpenRouter | 300+ 聚合模型 | `openrouter` | ✅ |
| Cohere | command-r-plus, command-r | `cohere` | ✅ |
| NVIDIA NIM | build.nvidia.com 上的 Llama / Mixtral | `nvidia` | ✅ |
| 硅基流动 (SiliconFlow) | Qwen / DeepSeek / Llama 镜像 | `siliconflow` | ✅ |
| MiniMax | abab6.5s-chat | `minimax` | ✅ |
| 百川智能 | Baichuan4 | `baichuan` | ✅ |
| 阶跃星辰 | step-1, step-2 | `stepfun` | ✅ |
| 讯飞星火 | Spark v3.5 | `spark` | ✅ |
| 零一万物 (Yi) | yi-large, yi-medium | `zero-one` | ✅ |
| 腾讯混元 | hunyuan-pro, hunyuan-standard | `tencent` | ✅ |

> 未列出的任何 OpenAI 兼容厂商同样开箱可用：设 `kind: openai`，把
> `base_url` 指向厂商的 `/v1` 端点即可。

### Adapter 架构：5 个协议，29+ 厂商

Fluxa 的 adapter **按协议方言划分，而不是按厂商名划分**。同一种协议的
所有厂商共用一份经过充分测试的代码路径，SSE 解析、重试、错误映射改一次
所有兼容厂商同时受益；二进制体积始终控制在 15 MiB 以内。

| Adapter 包 | 覆盖的厂商 | 为何独立 |
|-----------|-----------|---------|
| `internal/adapter/openai` | 22 个厂商：OpenAI、DeepSeek、通义千问、Ollama、Kimi、GLM、豆包、文心、Mistral、Groq、xAI、Perplexity、Together、Fireworks、OpenRouter、Cohere、NVIDIA、硅基流动、MiniMax、百川、阶跃、星火、Yi、腾讯混元 | 共用 OpenAI `/v1/chat/completions` 方言，只有 BaseURL 和 Key 不同，在 `router.openaiCompatibleDefaults` 里一行注册即可 |
| `internal/adapter/anthropic` | Anthropic Claude | 原生 `/v1/messages` 格式带 `thinking` / `tool_use` 块——对 `/v1/messages` 做字节级透传以保留原始字段 |
| `internal/adapter/gemini` | Google Gemini | `contents[].parts[].text`、`systemInstruction`、`generationConfig`——整个请求/响应形状不同，做双向 OpenAI ↔ Gemini 翻译 |
| `internal/adapter/bedrock` | AWS Bedrock | 统一 Converse API + **内置 SigV4 签名** + **二进制 EventStream 帧**解析，零 AWS SDK 依赖 |
| `internal/adapter/azure` | Azure OpenAI | URL 里嵌 deployment 名，鉴权走 `api-key` 头（非 Bearer），请求体需剥离 `model` 字段 |

**新增一个兼容 OpenAI 的厂商只需一行修改**：在
`internal/router/router.go` 的 `openaiCompatibleDefaults` 里追加
`"vendor": "https://api.vendor.com/v1"` 即可。只有协议本身不兼容
OpenAI 时才需要新建 adapter 包。

---

## 快速开始

Fluxa 只依赖一个 Postgres 数据库，启动时会自动执行 schema 迁移。

### Docker Compose 启动（推荐）

一条命令同时拉起 Postgres 和网关：

```bash
docker compose up -d
```

### Docker 启动

```bash
docker run -d \
  --name fluxa \
  -p 8080:8080 \
  -e FLUXA_DATABASE_URL="postgres://fluxa:fluxa@postgres:5432/fluxa?sslmode=disable" \
  -e FLUXA_BOOTSTRAP_PASSWORD=change-me \
  fluxa/fluxa:latest
```

### 二进制启动

```bash
# 下载最新版本
curl -L https://github.com/yourname/fluxa/releases/latest/download/fluxa-linux-amd64 -o fluxa
chmod +x fluxa

# 指向 Postgres 即可启动，无需任何配置文件
export FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable"
./fluxa
```

首次启动会创建引导管理员账号（默认 `admin` / `admin`），通过
`/admin/auth/login` 登录后请立刻修改密码。

### 接入你的应用

只需改两行，其余代码完全不动：

```python
# Python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",  # ← 改这里
    api_key="vk-your-virtual-key",        # ← 改这里
)

# 之后所有代码保持不变
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "你好"}]
)
```

```typescript
// TypeScript / Node.js
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8080/v1",  // ← 改这里
  apiKey: "vk-your-virtual-key",        // ← 改这里
});
```

```bash
# curl 直接调用
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer vk-your-virtual-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

---

## 配置

没有配置文件。Provider、Route、虚拟密钥、模型别名、DLP 规则全部存放在
Postgres 中，唯一的修改入口是 `/admin` REST API，每次写入都会热加载路由器。
进程本身完全由环境变量配置。

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `FLUXA_DATABASE_URL` | — | 完整 Postgres DSN；设置后下面的 `FLUXA_DB_*` 全部忽略 |
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
| `FLUXA_HOST` / `FLUXA_PORT` | `0.0.0.0` / `8080` | 监听地址 |
| `FLUXA_READ_TIMEOUT` | `30s` | HTTP 读超时 |
| `FLUXA_WRITE_TIMEOUT` | `5m` | HTTP 写超时（需覆盖流式响应） |
| `FLUXA_SHUTDOWN_TIMEOUT` | `20s` | 优雅退出等待时间 |
| `FLUXA_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `FLUXA_LOG_FORMAT` | `json` | `json` \| `text` |
| `FLUXA_STORE_CONTENT` | `false` | 是否把请求/响应正文写入 `request_logs`（隐私敏感场景建议关闭） |
| `FLUXA_BOOTSTRAP_USER` | `admin` | 首次启动创建的管理员用户名（仅在 `admin_users` 为空时生效） |
| `FLUXA_BOOTSTRAP_PASSWORD` | `admin` | 首次启动创建的管理员密码 |

### 运行时管理 Provider 与 Route

每次修改都会写入数据库并热加载路由器，不丢请求、不停机：

```bash
# 新增一个 Provider
curl -X POST http://localhost:8080/admin/providers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"deepseek","kind":"deepseek","api_key":"sk-xxx"}'

# 绑定模型路由及 fallback 链
curl -X PUT http://localhost:8080/admin/routes/gpt-4o \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"provider":"openai","fallback":["deepseek"]}'

# 删除一条路由
curl -X DELETE http://localhost:8080/admin/routes/gpt-4o \
  -H "Authorization: Bearer $TOKEN"

# 强制从数据库重新加载
curl -X POST http://localhost:8080/admin/reload \
  -H "Authorization: Bearer $TOKEN"
```

---

## 创建虚拟 Key

```bash
# 为前端团队创建一个 Key：只能用 GPT-4o，每月限额 $50
curl -X POST http://localhost:8080/admin/keys \
  -H "Authorization: Bearer your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "前端团队",
    "models": ["gpt-4o", "gpt-4o-mini"],
    "budget_usd_monthly": 50.0,
    "rate_limit_rpm": 100
  }'

# 返回
{
  "key": "vk-xxxxxxxxxxxxxx",
  "name": "前端团队",
  "created_at": "2026-04-06T10:00:00Z"
}
```

把 `vk-xxxxxxxxxxxxxx` 给到前端团队，他们用这个 Key 调用 Fluxa，超出预算自动拒绝，你的真实 OpenAI Key 永远不需要暴露出去。

---

## 与其他工具的对比

| | One API / New API | LiteLLM | **Fluxa** |
|---|---|---|---|
| 定位 | Token 倒卖中转 | 开发者 SDK | **自托管网关** |
| 实现语言 | JavaScript | Python | **Go** |
| 部署方式 | 需要 Node 环境 | 需要 Python 环境 | **单二进制，零依赖** |
| 网关延迟 | 中 | 50–200ms | **< 5ms** |
| 国内模型 | 部分支持 | 较弱 | **一等公民** |
| DLP / 安全防护 | 无 | 无 | **内置** |
| 会碰你的钱吗 | 会 | 否 | **否** |

---

## 版本规划

| 版本 | 主题 | 核心功能 | 状态 |
|------|------|----------|------|
| **v1.0** | 核心路由 | 多 Provider 适配 + Streaming 透传 | ✅ 开发中 |
| **v2.0** | Key 管理 | 虚拟 Key + 预算控制 + 访问控制 | 📋 规划中 |
| **v3.0** | 可观测性 | Dashboard + 用量统计 + 费用追踪 | 📋 规划中 |
| **v4.0** | 高可靠 | 熔断 + 缓存 + 更多国内模型 | 📋 规划中 |
| **v5.0** | AI Firewall | DLP + 内容安全 + 告警通知 | 📋 规划中 |
| **v6.0** | 企业治理 | RBAC + SSO + 审计日志 + 集群部署 | 📋 规划中 |

完整的功能规划详见 [docs/PLANNING.md](docs/PLANNING.md)。

---

## 本地开发

仓库里有两部分：Go 网关，以及 `web/` 下的管理控制台（React 19 + Vite +
Tailwind v4 + shadcn/ui）。`make build` 会先把控制台编译到 `web/dist`，
再通过 `go:embed` 打进二进制，所以发布产物依然是单个文件。

```bash
# 一次性：起一个用于开发的 Postgres
docker compose up -d postgres

# 终端 1 —— 网关跑在 :8080（根路径同时提供上次构建的控制台）
FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable" make run

# 终端 2 —— 控制台热更新跑在 :5173，/admin 和 /v1 代理到 :8080
cd web && npm install && npm run dev
```

开发 UI 时访问 `http://localhost:5173`。`make build` 产出嵌入式生产构建，
之后 `./bin/fluxa` 会在 `:8080` 单一来源下同时提供 API 和界面。

测试：

```bash
make test      # 未配置测试库时自动跳过依赖数据库的用例
make test-db   # 用 docker 起一个一次性 Postgres 跑全量
```

控制台使用 react-router 做客户端路由，界面全部由 shadcn/ui 原生组件搭建，
沿用其默认主题——尚未叠加任何项目配色，后续换肤只需改
`web/src/index.css` 一个文件。

## 参与贡献

欢迎提交 Issue 和 Pull Request。在提交较大改动的 PR 之前，建议先开 Issue 讨论方案。

```bash
git clone https://github.com/yourname/fluxa.git
cd fluxa
```

---

## 开源协议

MIT License，详见 [LICENSE](LICENSE)。

---

<div align="center">

*Flow Through, Stay in Control*

*让 AI 流量流动起来，让控制权留在你手中*

</div>
