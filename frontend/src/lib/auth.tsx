import { createContext, useContext, useEffect, useState, type ReactNode } from "react"
import { api, ApiError } from "@/lib/api"

export interface Member {
  ID: string
  OrgID: string
  DepartmentID: string | null
  RoleID: string
  Name: string
  Email: string | null
  Phone: string | null
  Status: string
}

interface MeResponse {
  member: Member
  permissions: Record<string, unknown>
}

interface AuthState {
  member: Member | null
  permissions: Set<string>
  loading: boolean
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

// AuthProvider loads the current session once (GET /api/me) and hands
// the result down as context: which member is logged in, and which
// permission points their role grants, mirroring the rbac.Principal the
// backend builds for every authenticated request (DESIGN.md 7.1).
export function AuthProvider({ children }: { children: ReactNode }) {
  const [member, setMember] = useState<Member | null>(null)
  const [permissions, setPermissions] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    setLoading(true)
    try {
      const res = await api.get<MeResponse>("/api/me")
      setMember(res.member)
      setPermissions(new Set(Object.keys(res.permissions ?? {})))
    } catch (err) {
      if (err instanceof ApiError) {
        setMember(null)
        setPermissions(new Set())
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <AuthContext.Provider value={{ member, permissions, loading, refresh }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider")
  return ctx
}

// useHasPermission mirrors rbac.Principal.Has on the backend: the
// frontend guard that decides what to show is the same shape as the
// guard that decides what the API will actually allow, so there is
// never a UI control for an action the backend would reject anyway.
export function useHasPermission(permission: string): boolean {
  const { permissions } = useAuth()
  return permissions.has(permission)
}

// Permission point codes, mirrored from internal/rbac/permission.go so
// callers get autocomplete instead of typo-prone string literals.
export const Permission = {
  ProviderManageCredentials: "provider.manage_credentials",
  ProviderView: "provider.view",
  ProviderManageRouting: "provider.manage_routing",
  ProviderRecordProcurement: "provider.record_procurement",
  ProviderUsePlayground: "provider.use_playground",
  OrgManageMembers: "org.manage_members",
  OrgManageDepartments: "org.manage_departments",
  OrgManageRoles: "org.manage_roles",
  OrgManageIdentitySources: "org.manage_identity_sources",
  OrgManageNotifyChannels: "org.manage_notify_channels",
  OrgManageKeys: "org.manage_keys",
  OrgViewOwnUsage: "org.view_own_usage",
  OrgManagePersonalRouting: "org.manage_personal_routing",
  OrgRequestQuota: "org.request_quota",
  OrgApproveDepartmentQuota: "org.approve_department_quota",
  QuotaAdjustAnyMember: "quota.adjust_any_member",
  QuotaApproveAny: "quota.approve_any",
  SecurityManageDLPRules: "security.manage_dlp_rules",
  SecurityViewEvents: "security.view_events",
  AuditViewCallLogs: "audit.view_call_logs",
  AuditViewOperationLogs: "audit.view_operation_logs",
} as const
