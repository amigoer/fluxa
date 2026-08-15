import { useState } from "react"
import { Outlet, useLocation } from "react-router-dom"
import { Menu } from "lucide-react"
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet"
import { SidebarNav } from "@/components/shared/sidebar-nav"
import { TopBar } from "@/components/shared/top-bar"
import { AppFooter } from "@/components/shared/app-footer"
import { adminNav } from "@/layouts/nav-config"

// Desktop-first (DESIGN.md 6.4: admin config work happens on a computer
// by default): the sidebar is always visible from md up. Below that, it
// collapses into a Sheet drawer opened from the top bar's hamburger --
// there are too many admin nav items (15) for a bottom tab bar, unlike
// the employee layout.
export function AdminLayout({ title }: { title: string }) {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const location = useLocation()

  return (
    <div className="flex h-screen w-full bg-background">
      <div className="hidden md:block">
        <SidebarNav groups={adminNav} />
      </div>

      <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
        <SheetContent side="left" className="w-[224px] p-0">
          <SheetTitle className="sr-only">导航</SheetTitle>
          <SidebarNav groups={adminNav} />
        </SheetContent>
      </Sheet>

      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar
          title={title}
          sideAction={
            <button
              className="mr-1 flex size-7 items-center justify-center rounded-md md:hidden"
              onClick={() => setDrawerOpen(true)}
              aria-label="打开导航"
            >
              <Menu className="size-[18px]" strokeWidth={1.6} />
            </button>
          }
        />
        <div className="flex-1 overflow-y-auto p-5">
          <Outlet key={location.pathname} />
        </div>
        <AppFooter />
      </div>
    </div>
  )
}
