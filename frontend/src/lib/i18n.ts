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
