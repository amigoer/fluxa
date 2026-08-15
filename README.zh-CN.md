<div align="center">

# Fluxa

**企业内部 AI 资源分发网关：统一采购 LLM 额度，分发给员工使用。**

[English](README.md)

</div>

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

## 技术栈

后端 Go（chi + pgx + golang-migrate）+ PostgreSQL；前端 React + TypeScript + Tailwind v4 + shadcn/ui，构建产物打包进 Go 二进制，对外只交付一个部署产物。完整设计文档见 [`docs/DESIGN.md`](docs/DESIGN.md)（架构、模块划分、还在讨论中的开放问题）。

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
