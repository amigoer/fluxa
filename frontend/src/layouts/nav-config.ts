import { Permission } from "@/lib/auth"

export interface NavItem {
  to: string
  label: string
  icon: string
  permission?: string
  /** Which live counter, if any, drives this item's badge. */
  badge?: "security" | "quota"
}

export interface NavGroup {
  label?: string
  items: NavItem[]
}

// The sidebar exactly as the hi-fi design draws it (five groups, icons
// included). One entry from the mockup is deliberately missing: 审批通道
// is a design-only screen -- there is no approval-channel API behind it
// yet, and a nav item that leads to a dead page is worse than one that
// isn't there.
export const adminNav: NavGroup[] = [
  {
    items: [
      { to: "/admin/overview", label: "概览", icon: "layout-grid" },
      { to: "/admin/quickstart", label: "快速接入", icon: "rocket" },
    ],
  },
  {
    label: "资源管理",
    items: [
      { to: "/admin/providers", label: "供应商", icon: "server", permission: Permission.ProviderView },
      { to: "/admin/models-routing", label: "模型与路由", icon: "waypoints", permission: Permission.ProviderView },
      { to: "/admin/playground", label: "Playground", icon: "flask", permission: Permission.ProviderUsePlayground },
      { to: "/admin/procurement", label: "入库记录", icon: "package-plus", permission: Permission.ProviderView },
    ],
  },
  {
    label: "组织与权限",
    items: [
      { to: "/admin/members", label: "成员与部门", icon: "users", permission: Permission.OrgManageMembers },
      { to: "/admin/roles", label: "角色权限", icon: "shield-check", permission: Permission.OrgManageRoles },
      { to: "/admin/keys", label: "Key 管理", icon: "key", permission: Permission.OrgManageKeys },
      { to: "/admin/identity-sources", label: "身份源", icon: "fingerprint", permission: Permission.OrgManageIdentitySources },
      { to: "/admin/notify-channels", label: "短信 / 邮件配置", icon: "mail", permission: Permission.OrgManageNotifyChannels },
    ],
  },
  {
    label: "安全",
    items: [
      { to: "/admin/dlp-rules", label: "DLP 规则", icon: "shield-alert", permission: Permission.SecurityManageDLPRules },
      {
        to: "/admin/security-events",
        label: "安全事件",
        icon: "alert-triangle",
        permission: Permission.SecurityViewEvents,
        badge: "security",
      },
    ],
  },
  {
    label: "审计",
    items: [
      { to: "/admin/call-logs", label: "调用日志", icon: "scroll-text", permission: Permission.AuditViewCallLogs },
      { to: "/admin/operation-logs", label: "操作审计", icon: "clipboard-list", permission: Permission.AuditViewOperationLogs },
    ],
  },
]

export const employeeNav: NavGroup[] = [
  {
    items: [
      { to: "/app/usage", label: "我的用量", icon: "activity" },
      { to: "/app/pricing", label: "资费一览", icon: "wallet" },
      { to: "/app/routing", label: "我的路由配置", icon: "waypoints" },
      { to: "/app/quota-requests", label: "配额申请", icon: "clock", badge: "quota" },
    ],
  },
]

export const routeLabel: Record<string, string> = {}
for (const group of [...adminNav, ...employeeNav]) {
  for (const item of group.items) routeLabel[item.to] = item.label
}

export function filterNav(nav: NavGroup[], permissions: Set<string>): NavGroup[] {
  return nav
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => !item.permission || permissions.has(item.permission)),
    }))
    .filter((group) => group.items.length > 0)
}

export function adminHasAnyPermission(permissions: Set<string>): boolean {
  return adminNav.some((group) =>
    group.items.some((item) => !item.permission || permissions.has(item.permission)),
  )
}
