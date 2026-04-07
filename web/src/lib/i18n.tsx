// i18n.tsx — tiny in-house translation layer for the dashboard.
//
// We deliberately do not pull in i18next or react-intl: the full string
// surface fits in one file, the dashboard ships as a single embedded
// SPA, and adding 200 KB of dependency for ~120 strings is not the
// trade-off we want. Instead this module exposes:
//
//   - I18nProvider: a context that holds the active locale ("en" | "zh")
//     and persists the user's choice in localStorage so it survives
//     page reloads.
//   - useT(): returns a `t(key)` function plus the active locale and a
//     setLocale setter for the language toggle UI.
//   - All keys live in the `dict` object below; the type system enforces
//     that every locale supplies every key, so a missing translation is
//     a build-time error rather than a runtime "??missing??".

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Locale = "en" | "zh";

const STORAGE_KEY = "fluxa-locale";

// dict is the single source of truth for every visible string. Add a
// new entry here, then reference it via t("yourKey") anywhere in the
// app. Both locales must define the key — TypeScript will reject any
// drift across the two records.
const dict = {
  en: {
    "app.title": "Fluxa",
    "app.subtitle": "AI gateway admin",

    "nav.dashboard": "Dashboard",
    "nav.providers": "Providers",
    "nav.routes": "Routes",
    "nav.virtualModels": "Virtual models",
    "nav.regexRoutes": "Regex routes",
    "nav.resolveTester": "Resolve tester",
    "nav.routeGraph": "Route graph",
    "nav.keys": "Virtual keys",
    "nav.usage": "Usage",
    "nav.settings": "Settings",
    "nav.signOut": "Sign out",
    "nav.account": "Signed in as",
    "nav.collapse": "Collapse sidebar",
    "nav.expand": "Expand sidebar",

    "lang.toggle": "中文",

    "login.title": "Fluxa admin",
    "login.username": "Username",
    "login.password": "Password",
    "login.submit": "Sign in",
    "login.checking": "Checking…",
    "login.placeholderUser": "admin",
    "login.placeholderPass": "Your password",
    "login.invalid": "Invalid username or password",
    "login.failed": "Login failed",
    "login.firstRunHint":
      "First run? Default credentials are admin / admin — change them in Settings after sign-in.",

    "dashboard.title": "Dashboard",
    "dashboard.subtitle": "Fleet-wide snapshot of routing state and key usage.",
    "dashboard.providers": "Providers",
    "dashboard.routes": "Routes",
    "dashboard.keys": "Virtual keys",
    "dashboard.usageToday": "Usage today",
    "dashboard.usageMonth": "Usage this month",
    "dashboard.requests": "Requests",
    "dashboard.promptTokens": "Prompt tokens",
    "dashboard.completionTokens": "Completion tokens",
    "dashboard.totalTokens": "Total tokens",
    "dashboard.costUSD": "Cost (USD)",

    "providers.title": "Providers",
    "providers.subtitle": "Upstream vendors Fluxa can route to.",
    "providers.new": "New provider",
    "providers.colName": "Name",
    "providers.colKind": "Kind",
    "providers.colBaseURL": "Base URL",
    "providers.colStatus": "Status",
    "providers.statusEnabled": "enabled",
    "providers.statusDisabled": "disabled",
    "providers.empty": "No providers yet.",
    "providers.deleteConfirm": "Delete provider {name}?",
    "providers.edit": "Edit provider",
    "providers.editAction": "Edit",
    "providers.fieldName": "Name",
    "providers.fieldKind": "Kind",
    "providers.fieldAPIKey": "API key",
    "providers.fieldBaseURL": "Base URL (optional)",
    "providers.fieldAPIVersion": "API version (Azure OpenAI)",
    "providers.fieldRegion": "Region (AWS Bedrock)",
    "providers.fieldAccessKey": "Access key (AWS)",
    "providers.fieldSecretKey": "Secret key (AWS)",
    "providers.fieldSessionToken": "Session token (AWS, optional)",
    "providers.fieldDeployments": "Azure deployments",
    "providers.fieldDeploymentsHint": "One per line: model=deployment",
    "providers.fieldModels": "Advertised models",
    "providers.fieldModelsHint": "Comma-separated list used by the router fallback",
    "providers.fieldHeaders": "Custom headers",
    "providers.fieldHeadersHint": "One per line: Header-Name: value",
    "providers.fieldTimeout": "Timeout (seconds)",
    "providers.fieldEnabled": "Enabled",
    "providers.sectionConnection": "Connection",
    "providers.sectionAdvanced": "Advanced",
    "providers.sectionAzure": "Azure OpenAI",
    "providers.sectionBedrock": "AWS Bedrock",
    "providers.keyUnchanged": "Leave blank to keep the existing secret",

    "routes.title": "Routes",
    "routes.subtitle": "Model-name → provider mapping with optional fallback chain.",
    "routes.new": "New route",
    "routes.colModel": "Model",
    "routes.colProvider": "Provider",
    "routes.colFallback": "Fallback",
    "routes.empty": "No routes yet.",
    "routes.deleteConfirm": "Delete route for {model}?",
    "routes.fieldFallback": "Fallback (comma-separated)",
    "routes.edit": "Edit route",
    "routes.editAction": "Edit",

    "vmodels.title": "Virtual models",
    "vmodels.subtitle":
      "User-facing model aliases that fan out to weighted real or virtual targets. The data plane resolves them before the legacy provider chain.",
    "vmodels.new": "New virtual model",
    "vmodels.edit": "Edit virtual model",
    "vmodels.colName": "Name",
    "vmodels.colDescription": "Description",
    "vmodels.colRoutes": "Routes",
    "vmodels.colStatus": "Status",
    "vmodels.empty": "No virtual models yet.",
    "vmodels.deleteConfirm": "Delete virtual model {name}?",
    "vmodels.fieldName": "Name",
    "vmodels.fieldDescription": "Description",
    "vmodels.fieldEnabled": "Enabled",
    "vmodels.routesTitle": "Targets",
    "vmodels.routesHint":
      "Each row is a weighted target. Higher weight = larger share of traffic. Targets can point at a real model (then a provider is required) or another virtual model (recursive, capped at 5 hops).",
    "vmodels.addRoute": "Add target",
    "vmodels.routeWeight": "Weight",
    "vmodels.routeType": "Type",
    "vmodels.routeTarget": "Target model",
    "vmodels.routeProvider": "Provider",
    "vmodels.routeReal": "real",
    "vmodels.routeVirtual": "virtual",
    "vmodels.routeRemove": "Remove",
    "vmodels.routesEmpty": "Add at least one target.",

    "rx.title": "Regex routes",
    "rx.subtitle":
      "Pattern-based interception. Lower priority value wins; first match short-circuits the rest. Useful for redirecting whole families of model names without touching application code.",
    "rx.new": "New regex route",
    "rx.edit": "Edit regex route",
    "rx.colPriority": "Priority",
    "rx.colPattern": "Pattern",
    "rx.colTarget": "Target",
    "rx.colDescription": "Description",
    "rx.colStatus": "Status",
    "rx.empty": "No regex routes yet.",
    "rx.deleteConfirm": "Delete this regex route?",
    "rx.fieldPattern": "Regex pattern",
    "rx.fieldPatternHint": "Go regexp syntax (RE2). Example: ^gpt-4.*",
    "rx.fieldPriority": "Priority (lower = higher precedence)",
    "rx.fieldType": "Target type",
    "rx.fieldTarget": "Target model",
    "rx.fieldProvider": "Provider (required when target type is real)",
    "rx.fieldDescription": "Description",
    "rx.fieldEnabled": "Enabled",

    "resolve.title": "Resolve tester",
    "resolve.subtitle":
      "Probe the resolver without making an upstream request. Type a model name and see exactly which virtual models or regex routes would fire and what the eventual target would be.",
    "resolve.input": "Model name",
    "resolve.run": "Resolve",
    "resolve.placeholder": "gpt-4o",
    "resolve.passthrough":
      "Passthrough — the resolver did not intervene; the legacy provider chain handles this model.",
    "resolve.target": "Resolved target",
    "resolve.targetProvider": "Provider",
    "resolve.targetModel": "Model",
    "resolve.trace": "Trace",
    "resolve.traceEmpty": "No trace yet — run a probe.",
    "resolve.errorTitle": "Resolver error",

    "keys.title": "Virtual keys",
    "keys.subtitle": "Per-application credentials with rate limits and budgets.",
    "keys.new": "New key",
    "keys.colName": "Name",
    "keys.colId": "ID",
    "keys.colModels": "Models",
    "keys.colRPM": "RPM",
    "keys.colStatus": "Status",
    "keys.empty": "No virtual keys yet.",
    "keys.deleteConfirm": "Delete key {id}? This also removes its usage history.",
    "keys.copyOnce": "Copy this key — it will not be shown again",
    "keys.copy": "Copy",
    "keys.dismiss": "Dismiss",
    "keys.fieldName": "Name",
    "keys.fieldDescription": "Description",
    "keys.fieldModels": "Models (comma-separated, * for any)",
    "keys.fieldIPs": "IP allowlist (CIDR or exact, comma-separated)",
    "keys.fieldRPM": "RPM limit",
    "keys.fieldDailyTokens": "Daily tokens budget",
    "keys.fieldMonthlyTokens": "Monthly tokens budget",
    "keys.fieldDailyUSD": "Daily USD budget",
    "keys.fieldMonthlyUSD": "Monthly USD budget",
    "keys.formTitle": "New virtual key",
    "keys.formTitleEdit": "Edit virtual key",
    "keys.create": "Create",
    "keys.editAction": "Edit",
    "keys.fieldEnabled": "Enabled",
    "keys.fieldExpiresAt": "Expires at (optional)",

    "usage.title": "Usage",
    "usage.subtitle": "Recent per-request accounting. Up to 200 rows per view.",
    "usage.filterLabel": "Filter by virtual key id",
    "usage.refresh": "Refresh",
    "usage.colTime": "Time",
    "usage.colKey": "Key",
    "usage.colModel": "Model",
    "usage.colProvider": "Provider",
    "usage.colPrompt": "Prompt",
    "usage.colCompletion": "Completion",
    "usage.colTotal": "Total",
    "usage.colUSD": "USD",
    "usage.colLatency": "Latency",
    "usage.empty": "No usage recorded yet.",

    "settings.title": "Settings",
    "settings.subtitle":
      "Snapshot the full provider + route state as YAML, or bulk-restore a saved bundle. Same format as legacy v1.x fluxa.yaml — imports round-trip cleanly.",
    "settings.bundleTitle": "Config bundle",
    "settings.loadFromGateway": "Load from gateway",
    "settings.download": "Download YAML",
    "settings.applyImport": "Apply import",
    "settings.loaded": "Loaded current config from gateway.",
    "settings.imported": "Imported {providers} provider(s) and {routes} route(s).",
    "settings.pasteFirst": "Paste a YAML bundle first.",
    "settings.envHint":
      "Environment variables of the form ${VAR} and ${VAR:-default} are expanded on import, so you can keep secrets out of the YAML and inject them at runtime.",

    "settings.accountTitle": "Account",
    "settings.accountSubtitle": "Rotate the password used for this dashboard.",
    "settings.fieldOldPass": "Current password",
    "settings.fieldNewPass": "New password",
    "settings.fieldConfirmPass": "Confirm new password",
    "settings.changePass": "Change password",
    "settings.passChanged": "Password changed. Please sign in again.",
    "settings.passMismatch": "New password and confirmation do not match.",
    "settings.passTooShort": "Password must be at least 4 characters.",

    "common.save": "Save",
    "common.saving": "Saving…",
    "common.cancel": "Cancel",
    "common.delete": "Delete",
    "common.edit": "Edit",
    "common.loadFailed": "load failed",
    "common.saveFailed": "save failed",
    "common.deleteFailed": "delete failed",
    "common.toggleFailed": "toggle failed",

    "graph.toolbar.regex": "Regex Route",
    "graph.toolbar.virtual": "Virtual Model",
    "graph.toolbar.autoLayout": "Auto Layout",
    "graph.toolbar.autoLayoutTitle": "Re-run auto layout",
    "graph.toolbar.live": "Live",
    "graph.toolbar.fitView": "Fit View",
    "graph.source.label": "Incoming Request",
    "graph.fallback.label": "Passthrough",
    "graph.fallback.hint": "original model name forwarded",
    "graph.virtual.badge": "virtual",
    "graph.virtual.more": "+{count} more",
    "graph.edge.matched": "matched",
    "graph.edge.noMatch": "no match",
    "graph.regex.statusEnabled": "enabled",
    "graph.regex.statusDisabled": "disabled",
    "graph.empty.title": "No routes yet",
    "graph.empty.hint":
      "Use the toolbar at the top-left to create a regex route or a virtual model. The graph will materialise as soon as you add your first rule.",
    "graph.loading": "loading routing graph…",
    "graph.loadFailed": "Failed to load graph",
    "graph.synthetic": "This is a synthetic node — nothing to configure.",
    "graph.panel.regexTitle": "Regex Route",
    "graph.panel.regexSubtitle": "Intercepts incoming model names by regex.",
    "graph.panel.virtualTitle": "Virtual Model",
    "graph.panel.virtualSubtitle":
      "Alias with weighted fanout to one or more targets.",
    "graph.panel.providerTitle": "Provider",
    "graph.panel.providerSubtitle":
      "Concrete upstream endpoint (read-only here).",
    "graph.field.pattern": "Pattern",
    "graph.field.priority": "Priority",
    "graph.field.targetType": "Target type",
    "graph.field.targetModel": "Target model",
    "graph.field.provider": "Provider",
    "graph.field.description": "Description",
    "graph.field.enabled": "Enabled",
    "graph.field.name": "Name",
    "graph.field.kind": "Kind",
    "graph.field.baseUrl": "Base URL",
    "graph.field.routes": "Routes",
    "graph.field.add": "Add",
    "graph.field.weightPlaceholder": "w",
    "graph.field.targetPlaceholder": "target model",
    "graph.field.providerPlaceholder": "provider",
    "graph.field.initialTarget": "Initial target model",
    "graph.targetType.real": "real",
    "graph.targetType.virtual": "virtual",
    "graph.action.save": "Save",
    "graph.action.delete": "Delete",
    "graph.action.create": "Create",
    "graph.action.cancel": "Cancel",
    "graph.action.saving": "Saving…",
    "graph.weights.warning":
      "Weights sum to {total} (not 100). Traffic still splits proportionally.",
    "graph.routes.empty": "Add at least one route",
    "graph.dialog.newRegex": "New regex route",
    "graph.dialog.newVirtual": "New virtual model",
    "graph.dialog.virtualHint": "Add more weighted targets later from the side panel.",
    "graph.dialog.namePlaceholder": "qwen-latest",
    "graph.dialog.targetPlaceholder": "qwen3-72b-instruct",
    "graph.dialog.providerPlaceholder": "qwen",
    "graph.dialog.patternPlaceholder": "^gpt-4.*",
    "graph.confirm.deleteRegex": "Delete this regex route?",
    "graph.confirm.deleteVirtual": "Delete virtual model \"{name}\"?",
    "graph.live.title": "Live (last 30s)",
    "graph.live.throughput": "Throughput",
    "graph.live.errorRate": "Error rate",
    "graph.live.noTraffic": "no traffic",
    "graph.errors.create": "Create failed",
    "graph.errors.save": "Save failed",
    "graph.errors.delete": "Delete failed",
    "graph.errors.statusOk": "ok",
    "graph.errors.statusWarn": "warn",
    "graph.errors.statusDown": "down",
    "graph.errors.statusUnknown": "unknown",
  },
  zh: {
    "app.title": "Fluxa",
    "app.subtitle": "AI 网关管理后台",

    "nav.dashboard": "概览",
    "nav.providers": "上游供应商",
    "nav.routes": "路由",
    "nav.virtualModels": "虚拟模型",
    "nav.regexRoutes": "正则路由",
    "nav.resolveTester": "解析测试",
    "nav.routeGraph": "路由拓扑图",
    "nav.keys": "虚拟密钥",
    "nav.usage": "用量",
    "nav.settings": "设置",
    "nav.signOut": "退出登录",
    "nav.account": "当前用户：",
    "nav.collapse": "收起侧边栏",
    "nav.expand": "展开侧边栏",

    "lang.toggle": "English",

    "login.title": "Fluxa 管理后台",
    "login.username": "账号",
    "login.password": "密码",
    "login.submit": "登录",
    "login.checking": "登录中…",
    "login.placeholderUser": "admin",
    "login.placeholderPass": "请输入密码",
    "login.invalid": "账号或密码错误",
    "login.failed": "登录失败",
    "login.firstRunHint":
      "首次启动？默认账号 admin / admin，登录后请立即在「设置」中修改密码。",

    "dashboard.title": "概览",
    "dashboard.subtitle": "整个网关的路由状态与密钥用量快照。",
    "dashboard.providers": "供应商",
    "dashboard.routes": "路由",
    "dashboard.keys": "虚拟密钥",
    "dashboard.usageToday": "今日用量",
    "dashboard.usageMonth": "本月用量",
    "dashboard.requests": "请求数",
    "dashboard.promptTokens": "输入 token",
    "dashboard.completionTokens": "输出 token",
    "dashboard.totalTokens": "总 token",
    "dashboard.costUSD": "费用（美元）",

    "providers.title": "上游供应商",
    "providers.subtitle": "Fluxa 可以转发请求的目标厂商。",
    "providers.new": "新建供应商",
    "providers.colName": "名称",
    "providers.colKind": "类型",
    "providers.colBaseURL": "Base URL",
    "providers.colStatus": "状态",
    "providers.statusEnabled": "已启用",
    "providers.statusDisabled": "已停用",
    "providers.empty": "暂无供应商。",
    "providers.deleteConfirm": "确认删除供应商 {name} 吗？",
    "providers.edit": "编辑供应商",
    "providers.editAction": "编辑",
    "providers.fieldName": "名称",
    "providers.fieldKind": "类型",
    "providers.fieldAPIKey": "API key",
    "providers.fieldBaseURL": "Base URL（可选）",
    "providers.fieldAPIVersion": "API 版本（Azure OpenAI）",
    "providers.fieldRegion": "区域（AWS Bedrock）",
    "providers.fieldAccessKey": "Access key（AWS）",
    "providers.fieldSecretKey": "Secret key（AWS）",
    "providers.fieldSessionToken": "Session token（AWS，可选）",
    "providers.fieldDeployments": "Azure 部署映射",
    "providers.fieldDeploymentsHint": "每行一条：model=deployment",
    "providers.fieldModels": "声明的模型列表",
    "providers.fieldModelsHint": "逗号分隔，用于路由的兜底匹配",
    "providers.fieldHeaders": "自定义请求头",
    "providers.fieldHeadersHint": "每行一条：Header-Name: value",
    "providers.fieldTimeout": "超时（秒）",
    "providers.fieldEnabled": "启用",
    "providers.sectionConnection": "连接",
    "providers.sectionAdvanced": "高级",
    "providers.sectionAzure": "Azure OpenAI",
    "providers.sectionBedrock": "AWS Bedrock",
    "providers.keyUnchanged": "留空表示保持原有密钥不变",

    "routes.title": "路由",
    "routes.subtitle": "模型名 → 供应商 的映射，可配置回退链。",
    "routes.new": "新建路由",
    "routes.colModel": "模型",
    "routes.colProvider": "供应商",
    "routes.colFallback": "回退",
    "routes.empty": "暂无路由。",
    "routes.deleteConfirm": "确认删除模型 {model} 的路由吗？",
    "routes.fieldFallback": "回退（用英文逗号分隔）",
    "routes.edit": "编辑路由",
    "routes.editAction": "编辑",

    "vmodels.title": "虚拟模型",
    "vmodels.subtitle":
      "对外暴露的模型别名，按权重分流到一个或多个真实/虚拟目标。数据面会在传统供应商链之前先解析它们。",
    "vmodels.new": "新建虚拟模型",
    "vmodels.edit": "编辑虚拟模型",
    "vmodels.colName": "名称",
    "vmodels.colDescription": "描述",
    "vmodels.colRoutes": "目标数",
    "vmodels.colStatus": "状态",
    "vmodels.empty": "暂无虚拟模型。",
    "vmodels.deleteConfirm": "确认删除虚拟模型 {name} 吗？",
    "vmodels.fieldName": "名称",
    "vmodels.fieldDescription": "描述",
    "vmodels.fieldEnabled": "启用",
    "vmodels.routesTitle": "目标",
    "vmodels.routesHint":
      "每一行是一个加权目标，权重越大占比越高。目标可以指向真实模型（必须填写供应商）或另一个虚拟模型（可递归，最多 5 层）。",
    "vmodels.addRoute": "添加目标",
    "vmodels.routeWeight": "权重",
    "vmodels.routeType": "类型",
    "vmodels.routeTarget": "目标模型",
    "vmodels.routeProvider": "供应商",
    "vmodels.routeReal": "真实",
    "vmodels.routeVirtual": "虚拟",
    "vmodels.routeRemove": "删除",
    "vmodels.routesEmpty": "至少需要一个目标。",

    "rx.title": "正则路由",
    "rx.subtitle":
      "按模式拦截请求。优先级数字越小越靠前，命中即停。适合在不改业务代码的前提下统一改写一类模型名。",
    "rx.new": "新建正则路由",
    "rx.edit": "编辑正则路由",
    "rx.colPriority": "优先级",
    "rx.colPattern": "正则",
    "rx.colTarget": "目标",
    "rx.colDescription": "描述",
    "rx.colStatus": "状态",
    "rx.empty": "暂无正则路由。",
    "rx.deleteConfirm": "确认删除这条正则路由？",
    "rx.fieldPattern": "正则表达式",
    "rx.fieldPatternHint": "Go regexp 语法（RE2）。例：^gpt-4.*",
    "rx.fieldPriority": "优先级（数字越小优先级越高）",
    "rx.fieldType": "目标类型",
    "rx.fieldTarget": "目标模型",
    "rx.fieldProvider": "供应商（target_type=real 时必填）",
    "rx.fieldDescription": "描述",
    "rx.fieldEnabled": "启用",

    "resolve.title": "解析测试",
    "resolve.subtitle":
      "不发起真实请求，仅在控制面侧探测解析器：输入模型名，可以看到会命中哪些虚拟模型或正则路由，最终落到哪个上游。",
    "resolve.input": "模型名",
    "resolve.run": "解析",
    "resolve.placeholder": "gpt-4o",
    "resolve.passthrough":
      "未拦截——解析器未介入，将走传统供应商链。",
    "resolve.target": "命中目标",
    "resolve.targetProvider": "供应商",
    "resolve.targetModel": "模型",
    "resolve.trace": "调用链",
    "resolve.traceEmpty": "尚未运行——请输入并点击解析。",
    "resolve.errorTitle": "解析器错误",

    "keys.title": "虚拟密钥",
    "keys.subtitle": "为每个应用单独发放的凭证，可配置速率限制与预算。",
    "keys.new": "新建密钥",
    "keys.colName": "名称",
    "keys.colId": "ID",
    "keys.colModels": "可用模型",
    "keys.colRPM": "RPM",
    "keys.colStatus": "状态",
    "keys.empty": "暂无虚拟密钥。",
    "keys.deleteConfirm": "确认删除密钥 {id}？同时会清除其用量历史。",
    "keys.copyOnce": "请立即复制此密钥，关闭后将无法再次查看",
    "keys.copy": "复制",
    "keys.dismiss": "关闭",
    "keys.fieldName": "名称",
    "keys.fieldDescription": "描述",
    "keys.fieldModels": "可用模型（用英文逗号分隔，* 表示全部）",
    "keys.fieldIPs": "IP 白名单（CIDR 或具体 IP，用英文逗号分隔）",
    "keys.fieldRPM": "RPM 限制",
    "keys.fieldDailyTokens": "每日 token 预算",
    "keys.fieldMonthlyTokens": "每月 token 预算",
    "keys.fieldDailyUSD": "每日费用预算（美元）",
    "keys.fieldMonthlyUSD": "每月费用预算（美元）",
    "keys.formTitle": "新建虚拟密钥",
    "keys.formTitleEdit": "编辑虚拟密钥",
    "keys.create": "创建",
    "keys.editAction": "编辑",
    "keys.fieldEnabled": "启用",
    "keys.fieldExpiresAt": "过期时间（可选）",

    "usage.title": "用量",
    "usage.subtitle": "最近的每请求计量记录，单视图最多 200 条。",
    "usage.filterLabel": "按虚拟密钥 ID 过滤",
    "usage.refresh": "刷新",
    "usage.colTime": "时间",
    "usage.colKey": "密钥",
    "usage.colModel": "模型",
    "usage.colProvider": "供应商",
    "usage.colPrompt": "输入",
    "usage.colCompletion": "输出",
    "usage.colTotal": "合计",
    "usage.colUSD": "美元",
    "usage.colLatency": "耗时",
    "usage.empty": "暂无用量记录。",

    "settings.title": "设置",
    "settings.subtitle":
      "将完整的供应商 + 路由状态导出为 YAML，或粘贴一个备份批量恢复。格式与历史版本的 fluxa.yaml 兼容，导入导出可无损往返。",
    "settings.bundleTitle": "配置包",
    "settings.loadFromGateway": "从网关加载",
    "settings.download": "下载 YAML",
    "settings.applyImport": "应用导入",
    "settings.loaded": "已从网关加载当前配置。",
    "settings.imported": "已导入 {providers} 个供应商和 {routes} 条路由。",
    "settings.pasteFirst": "请先粘贴 YAML 配置内容。",
    "settings.envHint":
      "导入时会展开 ${VAR} 和 ${VAR:-default} 形式的环境变量，因此 YAML 里可以不写死密钥，运行时再注入。",

    "settings.accountTitle": "账号",
    "settings.accountSubtitle": "修改登录该后台的密码。",
    "settings.fieldOldPass": "当前密码",
    "settings.fieldNewPass": "新密码",
    "settings.fieldConfirmPass": "确认新密码",
    "settings.changePass": "修改密码",
    "settings.passChanged": "密码已更新，请重新登录。",
    "settings.passMismatch": "新密码两次输入不一致。",
    "settings.passTooShort": "密码长度至少 4 位。",

    "common.save": "保存",
    "common.saving": "保存中…",
    "common.cancel": "取消",
    "common.delete": "删除",
    "common.edit": "编辑",
    "common.loadFailed": "加载失败",
    "common.saveFailed": "保存失败",
    "common.deleteFailed": "删除失败",
    "common.toggleFailed": "切换失败",

    "graph.toolbar.regex": "正则路由",
    "graph.toolbar.virtual": "虚拟模型",
    "graph.toolbar.autoLayout": "自动布局",
    "graph.toolbar.autoLayoutTitle": "重新执行自动布局",
    "graph.toolbar.live": "实时",
    "graph.toolbar.fitView": "适应画布",
    "graph.source.label": "请求入口",
    "graph.fallback.label": "透传",
    "graph.fallback.hint": "原模型名直接转发",
    "graph.virtual.badge": "虚拟",
    "graph.virtual.more": "另外 {count} 项",
    "graph.edge.matched": "命中",
    "graph.edge.noMatch": "未命中",
    "graph.regex.statusEnabled": "启用中",
    "graph.regex.statusDisabled": "已停用",
    "graph.empty.title": "尚未配置任何路由",
    "graph.empty.hint":
      "使用左上角工具栏新建一条正则路由或虚拟模型，添加首条规则后拓扑图会立即生成。",
    "graph.loading": "正在加载路由拓扑图…",
    "graph.loadFailed": "加载拓扑图失败",
    "graph.synthetic": "这是一个合成节点 — 无可配置项。",
    "graph.panel.regexTitle": "正则路由",
    "graph.panel.regexSubtitle": "通过正则表达式拦截传入的模型名。",
    "graph.panel.virtualTitle": "虚拟模型",
    "graph.panel.virtualSubtitle": "按权重将一个别名分流到一个或多个目标。",
    "graph.panel.providerTitle": "供应商",
    "graph.panel.providerSubtitle": "具体的上游端点（此处只读）。",
    "graph.field.pattern": "正则表达式",
    "graph.field.priority": "优先级",
    "graph.field.targetType": "目标类型",
    "graph.field.targetModel": "目标模型",
    "graph.field.provider": "供应商",
    "graph.field.description": "描述",
    "graph.field.enabled": "启用",
    "graph.field.name": "名称",
    "graph.field.kind": "类型",
    "graph.field.baseUrl": "Base URL",
    "graph.field.routes": "目标列表",
    "graph.field.add": "添加",
    "graph.field.weightPlaceholder": "权重",
    "graph.field.targetPlaceholder": "目标模型",
    "graph.field.providerPlaceholder": "供应商",
    "graph.field.initialTarget": "初始目标模型",
    "graph.targetType.real": "真实模型",
    "graph.targetType.virtual": "虚拟模型",
    "graph.action.save": "保存",
    "graph.action.delete": "删除",
    "graph.action.create": "创建",
    "graph.action.cancel": "取消",
    "graph.action.saving": "保存中…",
    "graph.weights.warning":
      "权重总和为 {total}（非 100），流量仍按比例分配。",
    "graph.routes.empty": "请至少添加一个目标",
    "graph.dialog.newRegex": "新建正则路由",
    "graph.dialog.newVirtual": "新建虚拟模型",
    "graph.dialog.virtualHint": "稍后可在侧边面板中继续添加按权重的目标。",
    "graph.dialog.namePlaceholder": "qwen-latest",
    "graph.dialog.targetPlaceholder": "qwen3-72b-instruct",
    "graph.dialog.providerPlaceholder": "qwen",
    "graph.dialog.patternPlaceholder": "^gpt-4.*",
    "graph.confirm.deleteRegex": "确认删除该正则路由？",
    "graph.confirm.deleteVirtual": "确认删除虚拟模型 \"{name}\"？",
    "graph.live.title": "实时数据（近 30 秒）",
    "graph.live.throughput": "吞吐量",
    "graph.live.errorRate": "错误率",
    "graph.live.noTraffic": "暂无流量",
    "graph.errors.create": "创建失败",
    "graph.errors.save": "保存失败",
    "graph.errors.delete": "删除失败",
    "graph.errors.statusOk": "正常",
    "graph.errors.statusWarn": "异常",
    "graph.errors.statusDown": "故障",
    "graph.errors.statusUnknown": "未知",
  },
} as const;

// TranslationKey is the union of every key declared in the EN dictionary.
// Because both records share the same keys (TypeScript verifies via the
// `satisfies` indirection below), this type covers both locales.
export type TranslationKey = keyof (typeof dict)["en"];

// Compile-time guarantee: ZH must mirror every key from EN.
const _zhCheck: Record<TranslationKey, string> = dict.zh;
void _zhCheck;

interface I18nValue {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: (key: TranslationKey, params?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | undefined>(undefined);

// I18nProvider wraps the application root and exposes the t() helper to
// any descendant. Initial locale is read from localStorage, then falls
// back to the browser's preferred language, then to English.
export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "en" || stored === "zh") return stored;
    if (typeof navigator !== "undefined" && navigator.language?.toLowerCase().startsWith("zh")) {
      return "zh";
    }
    return "en";
  });

  useEffect(() => {
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
  }, [locale]);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    localStorage.setItem(STORAGE_KEY, l);
  }, []);

  const t = useCallback(
    (key: TranslationKey, params?: Record<string, string | number>) => {
      const template = dict[locale][key] ?? key;
      if (!params) return template;
      // Tiny mustache: {name} → params.name, no escaping fanciness.
      return template.replace(/\{(\w+)\}/g, (_, k) =>
        params[k] !== undefined ? String(params[k]) : `{${k}}`,
      );
    },
    [locale],
  );

  const value = useMemo<I18nValue>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

// useT is the hook every page calls. Throws if used outside the
// provider so a missing wrapper surfaces immediately in dev.
export function useT(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useT must be used within I18nProvider");
  return ctx;
}
