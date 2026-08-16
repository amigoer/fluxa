import type { ReactNode } from "react"
import { Navigate } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import { adminHasAnyPermission } from "@/layouts/nav-config"

function Loading() {
  return (
    <div className="screen cn" style={{ alignItems: "center", justifyContent: "center" }}>
      <span className="cn-loading">加载中…</span>
    </div>
  )
}

// The two consoles are mutually exclusive, and RBAC decides which one a
// person belongs to -- the same rule Landing uses to pick where a fresh
// login goes. Guarding both directions matters because landing in the
// wrong shell is not merely cosmetic: an employee in /admin got a sidebar
// stripped down to the two un-gated items, and an admin in /app got the
// employee shell with no way back now that the persona switch is gone.
function Guard({ persona, children }: { persona: "admin" | "employee"; children: ReactNode }) {
  const { member, permissions, loading } = useAuth()

  if (loading) return <Loading />
  if (!member) return <Navigate to="/login" replace />

  const isAdmin = adminHasAnyPermission(permissions)
  if (persona === "admin" && !isAdmin) return <Navigate to="/app/usage" replace />
  if (persona === "employee" && isAdmin) return <Navigate to="/admin/overview" replace />
  return <>{children}</>
}

export function RequireAdmin({ children }: { children: ReactNode }) {
  return <Guard persona="admin">{children}</Guard>
}

export function RequireEmployee({ children }: { children: ReactNode }) {
  return <Guard persona="employee">{children}</Guard>
}
