import { Navigate, Outlet, useLocation } from "react-router";

import { AppSidebar } from "@/components/app-sidebar";
import { useAuth } from "@/components/auth-provider";
import { LanguageToggle } from "@/components/language-toggle";
import { ModeToggle } from "@/components/mode-toggle";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { SidebarInset, SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { useT, type TranslationKey } from "@/lib/i18n";

const TITLES: Record<string, TranslationKey> = {
  "/": "nav.overview",
  "/providers": "nav.providers",
  "/routes": "nav.routes",
  "/virtual-models": "nav.virtualModels",
  "/regex-models": "nav.regexModels",
  "/keys": "nav.keys",
  "/logs": "nav.logs",
  "/dlp": "nav.dlp",
  "/settings": "nav.settings",
};

/**
 * The authenticated shell. Everything inside <Outlet /> can assume a
 * signed-in user; unauthenticated visitors are redirected to /login with
 * their destination remembered.
 */
export function AppLayout() {
  const t = useT();
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className="flex min-h-svh items-center justify-center p-8">
        <Skeleton className="h-32 w-full max-w-md" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-2 !h-4" />
          <h2 className="text-sm font-medium">
            {TITLES[location.pathname] ? t(TITLES[location.pathname]) : t("app.name")}
          </h2>
          <div className="ml-auto flex items-center gap-2">
            <LanguageToggle />
            <ModeToggle />
          </div>
        </header>
        <div className="flex flex-1 flex-col gap-6 p-6">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
