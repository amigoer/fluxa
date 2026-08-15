import { NavLink, Outlet, useLocation } from "react-router-dom"
import { cn } from "@/lib/utils"
import { SidebarNav } from "@/components/shared/sidebar-nav"
import { TopBar } from "@/components/shared/top-bar"
import { AppFooter } from "@/components/shared/app-footer"
import { employeeNav } from "@/layouts/nav-config"

// Employee self-service is the responsive half of the product
// (DESIGN.md 6.4): a bottom tab bar replaces the sidebar below md, since
// four items fit a tab bar cleanly (the 15-item admin nav does not,
// hence AdminLayout using a drawer instead).
export function EmployeeLayout({ title }: { title: string }) {
  const location = useLocation()
  const items = employeeNav[0].items

  return (
    <div className="flex h-screen w-full bg-background">
      <div className="hidden md:block">
        <SidebarNav groups={employeeNav} narrow />
      </div>

      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar title={title} />
        <div className="flex-1 overflow-y-auto p-5 pb-20 md:pb-5">
          <Outlet key={location.pathname} />
        </div>
        <div className="hidden md:block">
          <AppFooter />
        </div>

        <nav className="fixed inset-x-0 bottom-0 z-10 flex h-[58px] border-t border-border bg-card md:hidden">
          {items.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  "flex flex-1 flex-col items-center justify-center gap-0.5 text-[9.5px] font-semibold text-muted-foreground",
                  isActive && "text-primary",
                )
              }
            >
              <item.icon className="size-[17px]" strokeWidth={1.6} />
              {item.label}
            </NavLink>
          ))}
        </nav>
      </div>
    </div>
  )
}
