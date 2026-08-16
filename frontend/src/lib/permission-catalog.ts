import { Permission } from "@/lib/auth"

// Human labels for every permission point, grouped the same way the
// role-permissions admin page groups them. Translations of the
// descriptions seeded in migrations/000001_init.up.sql.
export const permissionCatalog: { group: string; items: { code: string; label: string }[] }[] = [
  {
    group: "资源管理",
    items: [
      { code: Permission.ProviderManageCredentials, label: "管理 Provider 凭证" },
      { code: Permission.ProviderView, label: "查看供应商" },
      { code: Permission.ProviderManageRouting, label: "管理全局路由" },
      { code: Permission.ProviderRecordProcurement, label: "登记入库记录" },
      { code: Permission.ProviderUsePlayground, label: "使用 Playground" },
    ],
  },
  {
    group: "组织与权限",
    items: [
      { code: Permission.OrgManageMembers, label: "管理成员" },
      { code: Permission.OrgManageDepartments, label: "管理部门" },
      { code: Permission.OrgManageRoles, label: "管理角色" },
      { code: Permission.OrgManageIdentitySources, label: "管理身份源" },
      { code: Permission.OrgManageNotifyChannels, label: "管理短信/邮件配置" },
      { code: Permission.OrgManageKeys, label: "管理任意 Key" },
    ],
  },
  {
    group: "配额",
    items: [
      { code: Permission.OrgApproveDepartmentQuota, label: "审批本部门配额申请" },
      { code: Permission.QuotaAdjustAnyMember, label: "直接调整任意成员配额" },
      { code: Permission.QuotaApproveAny, label: "兜底审批任意配额申请" },
    ],
  },
  {
    group: "安全与审计",
    items: [
      { code: Permission.SecurityManageDLPRules, label: "管理 DLP 规则" },
      { code: Permission.SecurityViewEvents, label: "查看安全事件" },
      { code: Permission.AuditViewCallLogs, label: "查看调用日志" },
      { code: Permission.AuditViewOperationLogs, label: "查看操作审计" },
    ],
  },
  {
    group: "自助",
    items: [
      { code: Permission.OrgViewOwnUsage, label: "查看自己的用量与资费" },
      { code: Permission.OrgManagePersonalRouting, label: "配置自己的路由" },
      { code: Permission.OrgRequestQuota, label: "申请配额" },
    ],
  },
]
