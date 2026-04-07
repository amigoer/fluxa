import { useEffect, useState } from "react";
import {
  LayoutDashboard,
  Server,
  Waypoints,
  KeyRound,
  BarChart3,
  Settings as SettingsIcon,
  LogOut,
  Languages,
  Zap,
  PanelLeftClose,
  PanelLeftOpen,
  GitBranch,
  Regex,
  TestTube2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { Auth, getSessionToken, type AdminUser } from "@/lib/api";
import { I18nProvider, useT, type TranslationKey } from "@/lib/i18n";
import { DashboardPage } from "@/pages/Dashboard";
import { ProvidersPage } from "@/pages/Providers";
import { RoutesPage } from "@/pages/Routes";
import { VirtualModelsPage } from "@/pages/VirtualModels";
import { RegexRoutesPage } from "@/pages/RegexRoutes";
import { ResolveTesterPage } from "@/pages/ResolveTester";
import { KeysPage } from "@/pages/Keys";
import { UsagePage } from "@/pages/Usage";
import { SettingsPage } from "@/pages/Settings";

// Tab is the top-level navigation entry. The dashboard is deliberately
// a single-file router: six tabs, no nested pages, keeps the bundle
// tiny and matches the scope of the admin REST surface.
type Tab =
  | "dashboard"
  | "providers"
  | "routes"
  | "virtual-models"
  | "regex-routes"
  | "resolve-tester"
  | "keys"
  | "usage"
  | "settings";

interface NavEntry {
  id: Tab;
  labelKey: TranslationKey;
  icon: typeof LayoutDashboard;
}

const NAV: NavEntry[] = [
  { id: "dashboard", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { id: "providers", labelKey: "nav.providers", icon: Server },
  { id: "routes", labelKey: "nav.routes", icon: Waypoints },
  { id: "virtual-models", labelKey: "nav.virtualModels", icon: GitBranch },
  { id: "regex-routes", labelKey: "nav.regexRoutes", icon: Regex },
  { id: "resolve-tester", labelKey: "nav.resolveTester", icon: TestTube2 },
  { id: "keys", labelKey: "nav.keys", icon: KeyRound },
  { id: "usage", labelKey: "nav.usage", icon: BarChart3 },
  { id: "settings", labelKey: "nav.settings", icon: SettingsIcon },
];

// Set of valid tab ids, derived once from NAV so the URL-routing
// helpers below cannot drift out of sync with the navigation list.
const TAB_IDS = new Set<Tab>(NAV.map((n) => n.id));

// pathToTab maps a pathname like "/providers" or "/" to a Tab id.
// Anything we do not recognise (including "/") falls back to the
// dashboard so a stale bookmark or a typo never breaks the UI.
function pathToTab(pathname: string): Tab {
  const slug = pathname.replace(/^\/+/, "").split("/")[0];
  if (slug && TAB_IDS.has(slug as Tab)) return slug as Tab;
  return "dashboard";
}

// tabToPath is the inverse: dashboard collapses to "/", every other
// tab gets its own short slug. Keeping the dashboard at "/" matches
// the natural "I just opened the app" mental model.
function tabToPath(tab: Tab): string {
  return tab === "dashboard" ? "/" : `/${tab}`;
}

// App is the entry component. It wraps the real shell in I18nProvider so
// every descendant — including the login screen — can call useT().
export default function App() {
  return (
    <I18nProvider>
      <Shell />
    </I18nProvider>
  );
}

// SIDEBAR_STORAGE persists the collapsed/expanded preference across
// reloads so the operator does not have to re-collapse on every visit.
const SIDEBAR_STORAGE = "fluxa-sidebar-collapsed";

function Shell() {
  const { t, locale, setLocale } = useT();
  const [user, setUser] = useState<AdminUser | null>(null);
  const [loadingMe, setLoadingMe] = useState<boolean>(!!getSessionToken());
  // The active tab is derived from the URL pathname so a hard refresh
  // (or a shared link) lands the user on the same page they were on.
  // We seed from window.location.pathname and then keep state and URL
  // in sync via pushState below.
  const [tab, setTabState] = useState<Tab>(() =>
    pathToTab(window.location.pathname),
  );
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    return localStorage.getItem(SIDEBAR_STORAGE) === "1";
  });

  // Listen for back/forward navigation so the browser's history
  // controls work the same way they would in a multi-page app.
  useEffect(() => {
    function onPop() {
      setTabState(pathToTab(window.location.pathname));
    }
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  // setTab is the only mutator the rest of the shell calls. It both
  // updates local state and pushes a new history entry so refreshing
  // (or copying the URL) lands on the same page. We skip pushState
  // when the target matches the current path to avoid polluting
  // history with no-op duplicates.
  function setTab(next: Tab) {
    setTabState(next);
    const path = tabToPath(next);
    if (window.location.pathname !== path) {
      window.history.pushState({}, "", path);
    }
  }

  function toggleCollapsed() {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(SIDEBAR_STORAGE, next ? "1" : "0");
      return next;
    });
  }

  // On boot, if a token is already in localStorage validate it via the
  // /admin/auth/me probe so a stale token does not leave the user
  // staring at a broken dashboard.
  useEffect(() => {
    if (!getSessionToken()) {
      setLoadingMe(false);
      return;
    }
    Auth.me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoadingMe(false));
  }, []);

  if (loadingMe) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background text-muted-foreground text-sm">
        …
      </div>
    );
  }

  if (!user) {
    return <LoginScreen onAuth={(u) => setUser(u)} />;
  }

  async function signOut() {
    await Auth.logout();
    setUser(null);
  }

  return (
    // h-screen (not min-h-screen) pins the shell to the viewport so the
    // sidebar's bottom block (account / language / sign-out / collapse)
    // stays anchored even when the active page is taller than the
    // window. With min-h-screen the aside would grow with the document
    // and its footer would scroll out of sight along with the page.
    <div className="h-screen flex bg-muted/30 text-foreground">
      <aside
        className={cn(
          // The width is the only thing that animates on collapse — all
          // child rows switch to icon-only layouts via the `collapsed`
          // prop, which keeps the transition cheap and avoids reflow
          // jitter on the main content area.
          "shrink-0 border-r border-border/60 bg-background flex flex-col transition-[width] duration-200 ease-in-out",
          collapsed ? "w-[68px]" : "w-64",
        )}
      >
        {/* Brand block: a tiny logomark + wordmark. The wordmark hides
            when the sidebar is collapsed so only the logo square
            remains, perfectly centered in the narrow column. */}
        <div
          className={cn(
            "py-5 flex items-center gap-3",
            collapsed ? "px-0 justify-center" : "px-5",
          )}
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
            <Zap className="h-4 w-4" strokeWidth={2.5} />
          </div>
          {!collapsed && (
            <div className="leading-tight min-w-0">
              <div className="text-sm font-semibold tracking-tight truncate">
                {t("app.title")}
              </div>
              <div className="text-[11px] text-muted-foreground truncate">
                {t("app.subtitle")}
              </div>
            </div>
          )}
        </div>
        <Separator />
        <nav
          className={cn(
            "flex-1 py-3",
            collapsed
              ? "flex flex-col items-center gap-1 px-0"
              : "px-3 space-y-0.5",
          )}
        >
          {NAV.map((n) => {
            const Icon = n.icon;
            const active = tab === n.id;
            const label = t(n.labelKey);
            return (
              <button
                key={n.id}
                onClick={() => setTab(n.id)}
                title={collapsed ? label : undefined}
                aria-label={label}
                className={cn(
                  "group relative flex items-center rounded-md text-sm transition-colors",
                  collapsed
                    ? "h-9 w-9 justify-center"
                    : "w-full gap-2.5 px-3 py-2",
                  active
                    ? "bg-accent text-accent-foreground font-medium"
                    : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
                )}
              >
                {/* The left "you-are-here" indicator stays in expanded
                    mode but is hidden when collapsed — at icon-only
                    width the filled background already does the job
                    and a 2px bar would look squashed. */}
                {!collapsed && (
                  <span
                    className={cn(
                      "absolute left-0 top-1.5 bottom-1.5 w-0.5 rounded-r-full transition-colors",
                      active ? "bg-foreground" : "bg-transparent",
                    )}
                  />
                )}
                <Icon
                  className={cn(
                    "h-4 w-4 shrink-0 transition-colors",
                    active
                      ? "text-foreground"
                      : "text-muted-foreground group-hover:text-foreground",
                  )}
                />
                {!collapsed && label}
              </button>
            );
          })}
        </nav>
        <Separator />
        <div
          className={cn(
            "py-3",
            collapsed
              ? "flex flex-col items-center gap-1 px-0"
              : "px-3 space-y-1",
          )}
        >
          {/* Account row: avatar + username when expanded, just the
              avatar centered when collapsed. The avatar always shows
              the first two letters of the username so identity stays
              recognisable in the narrow layout. In collapsed mode the
              avatar is sized to exactly h-9 w-9 — same square as the
              action buttons below — so every item in this column
              shares one vertical axis. */}
          {collapsed ? (
            <div
              title={user.username}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent text-[11px] font-semibold uppercase"
            >
              {user.username.slice(0, 2)}
            </div>
          ) : (
            <div className="flex items-center gap-2.5 px-2 py-2">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-semibold uppercase">
                {user.username.slice(0, 2)}
              </div>
              <div className="min-w-0 leading-tight">
                <div className="text-xs text-muted-foreground">
                  {t("nav.account")}
                </div>
                <div className="text-sm font-medium truncate">
                  {user.username}
                </div>
              </div>
            </div>
          )}
          <SidebarAction
            collapsed={collapsed}
            icon={Languages}
            label={t("lang.toggle")}
            onClick={() => setLocale(locale === "en" ? "zh" : "en")}
          />
          <SidebarAction
            collapsed={collapsed}
            icon={LogOut}
            label={t("nav.signOut")}
            onClick={signOut}
          />
          <SidebarAction
            collapsed={collapsed}
            icon={collapsed ? PanelLeftOpen : PanelLeftClose}
            label={collapsed ? t("nav.expand") : t("nav.collapse")}
            onClick={toggleCollapsed}
          />
        </div>
      </aside>
      <main className="flex-1 overflow-auto">
        <div className="max-w-6xl mx-auto px-10 py-10">
          {tab === "dashboard" && <DashboardPage />}
          {tab === "providers" && <ProvidersPage />}
          {tab === "routes" && <RoutesPage />}
          {tab === "virtual-models" && <VirtualModelsPage />}
          {tab === "regex-routes" && <RegexRoutesPage />}
          {tab === "resolve-tester" && <ResolveTesterPage />}
          {tab === "keys" && <KeysPage />}
          {tab === "usage" && <UsagePage />}
          {tab === "settings" && <SettingsPage onSignOut={signOut} />}
        </div>
      </main>
    </div>
  );
}

// SidebarAction is a tiny helper for the bottom-of-sidebar buttons
// (language toggle, sign out, collapse). It collapses gracefully into
// an icon-only square when the sidebar is folded, and shows a native
// tooltip with the label so users still know what each icon does.
function SidebarAction({
  collapsed,
  icon: Icon,
  label,
  onClick,
}: {
  collapsed: boolean;
  icon: typeof LayoutDashboard;
  label: string;
  onClick: () => void;
}) {
  return (
    <Button
      variant="ghost"
      size="sm"
      title={collapsed ? label : undefined}
      aria-label={label}
      className={cn(
        "text-muted-foreground hover:text-foreground",
        collapsed
          ? "h-9 w-9 p-0 justify-center"
          : "w-full justify-start",
      )}
      onClick={onClick}
    >
      <Icon className="h-4 w-4" />
      {!collapsed && label}
    </Button>
  );
}

// LoginScreen is the unauthenticated landing page. It posts username +
// password to /admin/auth/login and stashes the resulting token via
// the Auth client; on success it hands the resolved user back to the
// shell via onAuth so we never have to read /me on the same boot.
function LoginScreen({ onAuth }: { onAuth: (u: AdminUser) => void }) {
  const { t, locale, setLocale } = useT();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await Auth.login(username, password);
      onAuth(res.user);
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("login.failed");
      setError(/invalid/i.test(msg) ? t("login.invalid") : msg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted/40 relative px-4">
      <button
        onClick={() => setLocale(locale === "en" ? "zh" : "en")}
        className="absolute top-4 right-4 text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1"
      >
        <Languages className="h-3 w-3" />
        {t("lang.toggle")}
      </button>
      <Card className="w-full max-w-sm shadow-lg">
        <CardHeader className="space-y-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
            <Zap className="h-5 w-5" strokeWidth={2.5} />
          </div>
          <div className="space-y-1">
            <CardTitle className="text-xl">{t("login.title")}</CardTitle>
            <CardDescription>{t("app.subtitle")}</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">{t("login.username")}</Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={t("login.placeholderUser")}
                autoFocus
                autoComplete="username"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">{t("login.password")}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("login.placeholderPass")}
                autoComplete="current-password"
              />
              {error && <p className="text-xs text-destructive">{error}</p>}
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? t("login.checking") : t("login.submit")}
            </Button>
            <p className="text-xs text-muted-foreground">
              {t("login.firstRunHint")}
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
