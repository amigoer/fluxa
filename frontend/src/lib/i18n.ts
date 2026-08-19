// Key-based text lookup (DESIGN.md 6.4): "默认简体中文；但前端架构上要
// 预留 i18n 框架（文案走 key 而不是硬编码中文），方便以后加英文，不用
// 推倒重来". Only zh-CN is populated -- v1 doesn't invest in translation
// -- but every string that recurs across the app (nav labels, common
// actions, status words) goes through t() so adding an en dictionary
// later is a new file, not a rewrite of every page. One-off copy inside
// a single page is not forced through this: that's the incremental,
// not-a-rewrite point of keying it at all.
const zhCN = {
  "nav.overview": "概览",
  "nav.quickstart": "快速接入",
  "nav.providers": "供应商",
  "nav.modelsRouting": "模型与路由",
  "nav.playground": "Playground",
  "nav.procurement": "入库记录",
  "nav.membersDepartments": "成员与部门",
  "nav.roles": "角色权限",
  "nav.keys": "Key 管理",
  "nav.identitySources": "身份源",
  "nav.notifyChannels": "短信 / 邮件配置",
  "nav.dlpRules": "DLP 规则",
  "nav.securityEvents": "安全事件",
  "nav.callLogs": "调用日志",
  "nav.operationLogs": "操作审计",
  "nav.myUsage": "我的用量",
  "nav.pricing": "资费一览",
  "nav.myRouting": "我的路由配置",
  "nav.quotaRequests": "配额申请",

  "action.save": "保存",
  "action.cancel": "取消",
  "action.edit": "编辑",
  "action.delete": "删除",
  "action.create": "创建",
  "action.confirm": "确认",
  "action.revoke": "吊销",
  "action.approve": "通过",
  "action.reject": "驳回",
  "action.configure": "配置",
  "action.viewAll": "查看全部",
  "action.logout": "退出登录",

  "status.enabled": "已启用",
  "status.disabled": "未配置",
  "status.active": "正常",
  "status.pendingReview": "待审批",
  "status.circuitOpen": "熔断",
  "status.halfOpen": "半开",
  "status.revoked": "已吊销",
  "status.success": "成功",
  "status.failed": "失败",

  "common.admin": "管理员",
  "common.employee": "员工",
  "common.loading": "加载中…",
  "common.noData": "暂无数据",
  "common.department": "部门",
  "common.copyright": "Fluxa v0.1.0 · © 2026 保留所有权利",
} as const

export type TranslationKey = keyof typeof zhCN

export function t(key: TranslationKey): string {
  return zhCN[key]
}

// The i18n.Key values the backend returns on errors. Without this the
// only thing a page could show was the server's English `detail`, so
// every failure toast read like a stack trace in an otherwise Chinese
// console.
const errorText: Record<string, string> = {
  "auth.invalid_credentials": "验证码或凭据无效",
  "auth.account_pending_review": "账号待管理员审批",
  "auth.session_expired": "登录已过期，请重新登录",
  "auth.notify_channel_missing": "尚未配置可用的发信通道",
  "notify.send_failed": "发送失败",
  "notify.channel_incomplete": "必填项没有填完，无法启用该通道",
  "rbac.permission_denied": "没有权限执行该操作",
  "quota.exceeded": "配额已用尽",
  "quota.cost_ceiling_exceeded": "这次请求的预估成本超过了单次调用上限",
  "provider.unavailable": "上游服务不可用",
  "provider.kind_unsupported": "该供应商类型还未支持，暂时无法接入",
  "common.validation_failed": "请求参数有误",
  "common.not_found": "资源不存在",
  "common.too_many_requests": "操作过于频繁，请稍后再试",
  "common.internal_error": "服务器内部错误",
}

// Keys whose server-side detail is the actionable part. For an SMTP
// failure "认证失败" and "连接超时" need different fixes, and only the
// relay knows which happened -- dropping the detail to keep the copy tidy
// would throw away the reason the admin pressed the button.
const detailedErrorKeys = new Set(["notify.send_failed"])

type KeyedError = { key: string; detail?: string }

function isKeyedError(err: unknown): err is KeyedError {
  return typeof err === "object" && err !== null && typeof (err as KeyedError).key === "string"
}

// tError renders an API error for display. Pass the fallback the call
// site would have shown on its own.
export function tError(err: unknown, fallback: string): string {
  if (!isKeyedError(err)) return fallback

  const text = errorText[err.key]
  if (!text) {
    // An unmapped key still beats a generic message when the server
    // bothered to explain itself.
    return err.detail || fallback
  }
  if (detailedErrorKeys.has(err.key) && err.detail) {
    return `${text}：${err.detail}`
  }
  return text
}
