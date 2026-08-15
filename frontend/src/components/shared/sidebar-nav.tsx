import { NavLink } from "react-router-dom"
import { cn } from "@/lib/utils"
import { useAuth } from "@/lib/auth"
import { Logo } from "@/components/shared/logo"
import type { NavGroup } from "@/layouts/nav-config"

export function SidebarNav({ groups, narrow }: { groups: NavGroup[]; narrow?: boolean }) {
  const { member, permissions, roleName } = useAuth()

  return (
    <div
      className={cn(
        "flex h-full flex-col overflow-y-auto border-r border-border bg-side-bg p-3",
        narrow ? "w-[200px]" : "w-[224px]",
      )}
    >
      <div className="flex items-center gap-2 px-2 pb-4 pt-1 text-sm font-bold text-foreground">
        <Logo />
        Fluxa
      </div>

      <nav className="flex flex-1 flex-col gap-4">
        {groups.map((group, i) => {
          const items = group.items.filter((item) => !item.permission || permissions.has(item.permission))
          if (items.length === 0) return null
          return (
            <div key={group.label ?? i}>
              {group.label && (
                <p className="px-2 pb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                  {group.label}
                </p>
              )}
              <div className="flex flex-col gap-0.5">
                {items.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    className={({ isActive }) =>
                      cn(
                        "flex items-center gap-2.5 rounded-md px-2 py-1.5 text-[13px] text-muted-foreground",
                        isActive && "bg-accent font-semibold text-accent-foreground",
                      )
                    }
                  >
                    <item.icon className="size-4 flex-none" strokeWidth={1.5} />
                    {item.label}
                  </NavLink>
                ))}
              </div>
            </div>
          )
        })}
      </nav>

      {member && (
        <div className="mt-auto flex items-center gap-2.5 border-t border-border px-2 pb-1 pt-3.5">
          <span className="flex size-[26px] flex-none items-center justify-center rounded-full bg-accent text-[11px] font-bold text-accent-foreground ring-1 ring-inset ring-border">
            {member.Name.slice(0, 1)}
          </span>
          <div className="min-w-0">
            <p className="truncate text-xs font-semibold text-foreground">{member.Name}</p>
            <p className="truncate text-[10.5px] text-muted-foreground">{roleName || "—"}</p>
          </div>
        </div>
      )}
    </div>
  )
}
