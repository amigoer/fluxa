import { Bell, ChevronDown, LogOut, LayoutGrid, User } from "lucide-react"
import { Link } from "react-router-dom"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useAuth } from "@/lib/auth"
import { adminHasAnyPermission } from "@/layouts/nav-config"
import { api } from "@/lib/api"

export function TopBar({ title, sideAction }: { title: string; sideAction?: React.ReactNode }) {
  const { permissions, roleName, departmentName, refresh } = useAuth()
  const canSeeAdmin = adminHasAnyPermission(permissions)

  const logout = async () => {
    await api.post("/api/auth/logout")
    await refresh()
    window.location.href = "/login"
  }

  return (
    <div className="flex h-[54px] flex-none items-center justify-between border-b border-border px-5">
      <div className="flex items-center gap-2">
        {sideAction}
        <span className="text-[14.5px] font-semibold text-foreground">{title}</span>
      </div>
      <div className="flex items-center gap-2.5">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="inline-flex items-center gap-1.5 rounded-full border border-border bg-card py-1 pl-3 pr-1.5 text-[11.5px] text-foreground shadow-[var(--shadow-card)]">
              {roleName}
              {departmentName && <span className="text-muted-foreground"> · {departmentName}</span>}
              <ChevronDown className="size-2.5 text-muted-foreground" strokeWidth={1.8} />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {canSeeAdmin && (
              <DropdownMenuItem asChild>
                <Link to="/admin/overview">
                  <LayoutGrid className="size-4" /> 管理台
                </Link>
              </DropdownMenuItem>
            )}
            <DropdownMenuItem asChild>
              <Link to="/app/usage">
                <User className="size-4" /> 我的工作台
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => void logout()}>
              <LogOut className="size-4" /> 退出登录
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <span className="relative flex size-[30px] items-center justify-center rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
          <Bell className="size-[15px] text-muted-foreground" strokeWidth={1.5} />
        </span>
      </div>
    </div>
  )
}
