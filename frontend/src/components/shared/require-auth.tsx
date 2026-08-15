import type { ReactNode } from "react"
import { Navigate } from "react-router-dom"
import { useAuth } from "@/lib/auth"

export function RequireAuth({ children }: { children: ReactNode }) {
  const { member, loading } = useAuth()

  if (loading) {
    return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">加载中…</div>
  }
  if (!member) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
