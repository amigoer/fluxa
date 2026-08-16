import type { ReactNode } from "react"
import { Navigate } from "react-router-dom"
import { useAuth } from "@/lib/auth"

export function RequireAuth({ children }: { children: ReactNode }) {
  const { member, loading } = useAuth()

  if (loading) {
    return (
      <div className="screen cn" style={{ alignItems: "center", justifyContent: "center" }}>
        <span className="cn-loading">加载中…</span>
      </div>
    )
  }
  if (!member) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
