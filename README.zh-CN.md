<div align="center">

<img src="docs/assets/logo.svg" width="128" height="128" alt="Fluxa">

# Fluxa

**企业内部 AI 资源分发网关：统一采购 LLM 额度，分发给员工使用。**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](docker-compose.yml) [![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](frontend/package.json) [![TypeScript](https://img.shields.io/badge/TypeScript-6.0-3178C6?logo=typescript&logoColor=white)](frontend/package.json) [![Self-hosted](https://img.shields.io/badge/deploy-single%20binary-6366F1)](#本地运行) [![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md)

</div>

## 界面

<img src="docs/assets/screenshots/quickstart.png" alt="快速接入：OpenAI 兼容端点、认证方式和可直接复制的示例">

<p align="center"><em>把已有的 OpenAI 客户端指向 Fluxa 的 base_url，其余代码一行都不用改。</em></p>

|  |  |
|---|---|
| <img src="docs/assets/screenshots/models.png" alt="模型与路由"> | <img src="docs/assets/screenshots/roles.png" alt="角色权限"> |
| **模型与路由** — 每个模型的价格、上下文长度、发布状态，下面接企业的全局路由链。 | **角色权限** — 内置四个角色加自定义角色，落到每一个权限点。 |
| <img src="docs/assets/screenshots/dlp.png" alt="DLP 规则"> | <img src="docs/assets/screenshots/providers.png" alt="供应商"> |
| **DLP 规则** — 正则加真实校验位双重匹配，按优先级排序，可脱敏也可拦截。 | **供应商** — 上游凭证和实时健康状态，供应商出问题不用等人来报。 |

<img src="docs/assets/screenshots/login.png" alt="登录页">

<p align="center"><em>登录：有统一 IM 走飞书，没有的话用手机号 / 邮箱验证码兜底。</em></p>

## 这是什么

企业向各家 LLM 供应商（OpenAI、Anthropic、Azure 等）统一采购额度，Fluxa 就是夹在员工和这些供应商之间的那一层：所有人对外只调用同一个 OpenAI 兼容接口，请求实际打到哪个供应商、允不允许调用、花了多少钱、内容安不安全，都由 Fluxa 判断。

产品定位是每个企业自己部署一套独立系统（不是多租户 SaaS），单一二进制、内嵌前端界面。

## 功能

- **统一入库台账** — 每次向供应商充值都会记一笔，为后续对接真实账单、核对实际花费留好数据基础，而不是全靠人工记账。
- **部门 / 个人两级配额** — 管理员给部门分配总额度池，部门负责人在池子里二次分配给成员，员工也能自己申请。两条路径写的是同一张配额表，数字不会对不上。
- **个人路由 + 备用链** — 员工在企业已启用的模型范围内，按任务类型配置自己的优先级和备用模型，备用环节还能设置成本上限。
- **Provider 健康是真状态机** — 正常 → 熔断 → 半开 → 恢复，不是写死的降级顺序。
- **DLP 真的校验校验位** — 身份证号、银行卡号按真实的校验算法核验，不是单靠长度和正则形似匹配，减少误报。
- **登录方式跟着企业已有习惯走** — 有统一 IM 的用飞书扫码登录；没有的话有手机号/邮箱验证码兜底（全程没有密码），由管理员开关控制是否开放。
- **RBAC 精确到权限点** — 内置四个角色，管理员还能自定义角色；管理员和员工共用同一套界面，看到什么、能做什么由权限决定。

## 本地运行

```bash
# 启动 Postgres
docker compose up -d postgres

# 构建前端，产物会被内嵌进 Go 二进制
cd frontend && npm install && npm run build && cd ..

# 启动后端（启动时自动跑 migration）
FLUXA_DATABASE_URL="postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable" \
  go run ./cmd/server
```

打开 `http://localhost:8080`，首次访问会引导你创建企业和第一个管理员账号。

## License

[MIT](LICENSE)
