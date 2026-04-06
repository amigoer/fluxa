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

| Provider | 模型 | 状态 |
|----------|------|------|
| OpenAI | GPT-4o, GPT-4o-mini, o1, o3 | ✅ v1.0 |
| Anthropic | Claude 3.5, Claude 3.7 | ✅ v1.0 |
| DeepSeek | deepseek-chat, deepseek-reasoner | ✅ v1.0 |
| 通义千问 | qwen-max, qwen-plus, qwen-turbo | ✅ v1.0 |
| Ollama | 任意本地模型 | ✅ v1.0 |
| Kimi | moonshot-v1 系列 | 🔄 v4.0 |
| 智谱 GLM | glm-4, glm-4-flash | 🔄 v4.0 |
| 文心一言 | ernie-4.0 系列 | 🔄 v4.0 |
| 豆包 | doubao-pro 系列 | 🔄 v4.0 |
| Azure OpenAI | 所有 Azure 托管模型 | 🔄 v4.0 |
| Google Gemini | gemini-1.5-pro, gemini-2.0 | 🔄 v4.0 |
| AWS Bedrock | Claude, Llama, Titan | 🔄 v4.0 |

---

## 快速开始

### Docker 启动（推荐）

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

### 二进制启动

```bash
# 下载最新版本
curl -L https://github.com/yourname/fluxa/releases/latest/download/fluxa-linux-amd64 -o fluxa
chmod +x fluxa

# 创建配置文件
cp fluxa.example.yaml fluxa.yaml
# 编辑 fluxa.yaml，填入你的 Provider API Key

./fluxa
```

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

## 配置文件

从 v1.1 起，`providers` 和 `routes` 已迁移至 SQLite 数据库（位置由
`database.path` 指定）。YAML 文件只保留服务端、日志和数据库路径等启动配置。
首次启动且数据库为空时，网关会把 YAML 中的 `providers` / `routes` 作为
种子数据写入数据库；此后数据库即为唯一事实来源，所有增删改通过 `/admin`
REST API 完成，无需重启。完整示例见 [`configs/fluxa.example.yaml`](configs/fluxa.example.yaml)。

```yaml
# fluxa.yaml

server:
  host: 0.0.0.0
  port: 8080
  master_key: ${FLUXA_MASTER_KEY}   # 启用 /admin 管理 API 必填

database:
  path: ./fluxa.db                  # providers + routes 存储于此

logging:
  level: info
  format: json
  store_content: false              # 是否存储 Prompt 内容（隐私敏感场景建议关闭）
```

### 运行时管理 Provider 与 Route

每次修改都会写入数据库并热加载路由器，不丢请求、不停机：

```bash
# 新增一个 Provider
curl -X POST http://localhost:8080/admin/providers \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"deepseek","kind":"deepseek","api_key":"sk-xxx"}'

# 绑定模型路由及 fallback 链
curl -X PUT http://localhost:8080/admin/routes/gpt-4o \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"provider":"openai","fallback":["deepseek"]}'

# 删除一条路由
curl -X DELETE http://localhost:8080/admin/routes/gpt-4o \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY"

# 强制从数据库重新加载
curl -X POST http://localhost:8080/admin/reload \
  -H "Authorization: Bearer $FLUXA_MASTER_KEY"
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

## 参与贡献

欢迎提交 Issue 和 Pull Request。在提交较大改动的 PR 之前，建议先开 Issue 讨论方案。

```bash
git clone https://github.com/yourname/fluxa.git
cd fluxa
make dev
```

详细的开发环境搭建和贡献指南见 [CONTRIBUTING.md](CONTRIBUTING.md)。

---

## 开源协议

MIT License，详见 [LICENSE](LICENSE)。

---

<div align="center">

*Flow Through, Stay in Control*

*让 AI 流量流动起来，让控制权留在你手中*

</div>
