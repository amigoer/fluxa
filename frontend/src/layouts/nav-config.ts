import {
  LayoutGrid,
  Rocket,
  Server,
  Waypoints,
  FlaskConical,
  PackagePlus,
  Users,
  ShieldCheck,
  KeyRound,
  Fingerprint,
  Mail,
  ShieldAlert,
  AlertTriangle,
  ScrollText,
  ClipboardList,
  BarChart3,
  Tag,
  Route,
  Inbox,
  type LucideIcon,
} from "lucide-react"
import { Permission } from "@/lib/auth"

export interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  permission?: string
}

export interface NavGroup {
  label?: string
  items: NavItem[]
}

// Mirrors DESIGN.md 6.3's admin sidebar grouping exactly (15 items, 4
// groups plus the two top-level entries).
export const adminNav: NavGroup[] = [
  {
    items: [
      { to: "/admin/overview", label: "概览", icon: LayoutGrid },
      { to: "/admin/quickstart", label: "快速接入", icon: Rocket },
    ],
  },
  {
    label: "资源管理",
    items: [
      { to: "/admin/providers", label: "供应商", icon: Server, permission: Permission.ProviderView },
      { to: "/admin/models-routing", label: "模型与路由", icon: Waypoints, permission: Permission.ProviderView },
      { to: "/admin/playground", label: "Playground", icon: FlaskConical, permission: Permission.ProviderUsePlayground },
      { to: "/admin/procurement", label: "入库记录", icon: PackagePlus, permission: Permission.ProviderView },
    ],
  },
  {
    label: "组织与权限",
    items: [
      { to: "/admin/members", label: "成员与部门", icon: Users, permission: Permission.OrgManageMembers },
      { to: "/admin/roles", label: "角色权限", icon: ShieldCheck, permission: Permission.OrgManageRoles },
      { to: "/admin/keys", label: "Key 管理", icon: KeyRound, permission: Permission.OrgManageKeys },
      { to: "/admin/identity-sources", label: "身份源", icon: Fingerprint, permission: Permission.OrgManageIdentitySources },
      { to: "/admin/notify-channels", label: "短信 / 邮件配置", icon: Mail, permission: Permission.OrgManageNotifyChannels },
    ],
  },
  {
    label: "安全",
    items: [
      { to: "/admin/dlp-rules", label: "DLP 规则", icon: ShieldAlert, permission: Permission.SecurityManageDLPRules },
      { to: "/admin/security-events", label: "安全事件", icon: AlertTriangle, permission: Permission.SecurityViewEvents },
    ],
  },
  {
    label: "审计",
    items: [
      { to: "/admin/call-logs", label: "调用日志", icon: ScrollText, permission: Permission.AuditViewCallLogs },
      { to: "/admin/operation-logs", label: "操作审计", icon: ClipboardList, permission: Permission.AuditViewOperationLogs },
    ],
  },
]

// Mirrors DESIGN.md 6.3's employee sidebar (4 items).
export const employeeNav: NavGroup[] = [
  {
    items: [
      { to: "/app/usage", label: "我的用量", icon: BarChart3 },
      { to: "/app/pricing", label: "资费一览", icon: Tag },
      { to: "/app/routing", label: "我的路由配置", icon: Route },
      { to: "/app/quota-requests", label: "配额申请", icon: Inbox },
    ],
  },
]

export function adminHasAnyPermission(permissions: Set<string>): boolean {
  return adminNav.some((group) =>
    group.items.some((item) => !item.permission || permissions.has(item.permission)),
  )
}
