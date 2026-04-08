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
  GitBranch,
  Code2,
  Network,
  ChevronLeft,
  ChevronRight,
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
import { RouteGraphPage } from "@/components/RouteGraph";
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
  | "route-graph"
  | "keys"
  | "usage"
  | "settings";

interface NavEntry {
  id: Tab;
  labelKey: TranslationKey;
  icon: typeof LayoutDashboard;
}

interface NavGroup {
  title: string;
  items: NavEntry[];
}

const NAV_GROUPS: NavGroup[] = [
  {
    title: "",
    items: [{ id: "dashboard", labelKey: "nav.dashboard", icon: LayoutDashboard }],
  },
  {
    title: "路由管理",
    items: [
      { id: "route-graph", labelKey: "nav.routeGraph", icon: Network },
      { id: "providers", labelKey: "nav.providers", icon: Server },
      { id: "routes", labelKey: "nav.routes", icon: Waypoints },
      { id: "virtual-models", labelKey: "nav.virtualModels", icon: GitBranch },
      { id: "regex-routes", labelKey: "nav.regexRoutes", icon: Code2 },
    ],
  },
  {
    title: "访问控制",
    items: [{ id: "keys", labelKey: "nav.keys", icon: KeyRound }],
  },
  {
    title: "监控",
    items: [{ id: "usage", labelKey: "nav.usage", icon: BarChart3 }],
  },
  {
    title: "系统",
    items: [{ id: "settings", labelKey: "nav.settings", icon: SettingsIcon }],
  },
];

// Set of valid tab ids
const TAB_IDS = new Set<Tab>([
  "dashboard",
  "providers",
  "routes",
  "virtual-models",
  "regex-routes",
  "resolve-tester", // Kept in tabs, but removed from sidebar
  "route-graph",
  "keys",
  "usage",
  "settings",
]);

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
    <div className="h-screen flex bg-background text-foreground font-[Inter,sans-serif]">
      <aside
        className={cn(
          "relative shrink-0 border-r border-border/40 bg-background/95 backdrop-blur flex flex-col transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] z-20",
          collapsed ? "w-[68px]" : "w-64",
        )}
      >
        <button
          onClick={toggleCollapsed}
          className="absolute -right-3.5 top-1/2 -translate-y-1/2 flex h-7 w-7 items-center justify-center rounded-full border border-border/40 bg-background text-muted-foreground shadow-sm hover:bg-accent hover:text-foreground transition-all duration-200 z-50"
          title={collapsed ? t("nav.expand") : t("nav.collapse")}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </button>
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
            "flex-1 overflow-y-auto py-2",
            collapsed
              ? "flex flex-col items-center gap-1.5 px-0"
              : "px-2 space-y-4",
          )}
        >
          {NAV_GROUPS.map((group, gIdx) => (
            <div key={gIdx} className={cn("flex flex-col", collapsed ? "gap-1.5" : "gap-0.5")}>
              {!collapsed && group.title && (
                <div className="px-3.5 py-1 text-[11px] font-semibold tracking-wider text-muted-foreground/60 uppercase">
                  {group.title}
                </div>
              )}
              {group.items.map((n) => {
                const Icon = n.icon;
                const active = tab === n.id;
                const label = t(n.labelKey);
                return (
                  <button
                    key={n.id}
                    onClick={() => setTab(n.id)}
                    aria-label={label}
                    className={cn(
                      "group relative flex items-center rounded-xl text-sm transition-all duration-200",
                      collapsed
                        ? "h-10 w-10 justify-center mx-auto"
                        : "w-full gap-3 px-3.5 py-2",
                      active
                        ? "bg-indigo-500/10 text-indigo-700 font-semibold shadow-sm"
                        : "text-muted-foreground hover:bg-accent hover:text-foreground",
                    )}
                  >
                    {!collapsed && active && (
                      <div className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-1 bg-indigo-600 rounded-r-full shadow-[0_0_8px_rgba(79,70,229,0.4)]" />
                    )}
                    <Icon
                      className={cn(
                        "h-[18px] w-[18px] shrink-0 transition-colors",
                        active
                          ? "text-indigo-600"
                          : "text-muted-foreground group-hover:text-foreground",
                      )}
                    />
                    {!collapsed && label}
                    {collapsed && <SidebarTooltip label={label} />}
                  </button>
                );
              })}
            </div>
          ))}
        </nav>
        {/* Footer block. Expanded layout is a single rounded "user
            card" — avatar + username on top, a row of three icon
            actions on the bottom — so the whole thing reads as one
            unit instead of four loose rows. Collapsed layout stacks
            the same elements vertically as before so the narrow
            column stays scannable. */}
        <div
          className={cn(
            "p-3",
            collapsed && "flex flex-col items-center gap-1.5",
          )}
        >
          {collapsed ? (
            <>
              <div
                title={user.username}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent text-[11px] font-semibold uppercase tracking-wide"
              >
                {user.username.slice(0, 2)}
              </div>
              <SidebarIconAction
                icon={Languages}
                label={t("lang.toggle")}
                onClick={() => setLocale(locale === "en" ? "zh" : "en")}
              />
              <SidebarIconAction
                icon={LogOut}
                label={t("nav.signOut")}
                onClick={signOut}
              />
            </>
          ) : (
            <div className="flex items-center justify-between gap-2 px-2 py-1.5 rounded-xl hover:bg-accent/40 border border-transparent transition-colors">
              <div className="flex items-center gap-2.5">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-indigo-500/10 text-indigo-700 text-[11px] font-bold uppercase tracking-wide">
                  {user.username.slice(0, 2)}
                </div>
                <div className="min-w-0 leading-tight">
                  <div className="text-sm font-semibold truncate text-foreground">
                    {user.username}
                  </div>
                  <div className="text-[11px] text-muted-foreground/80 truncate font-medium">
                    {t("nav.account") === "Account" ? "Administrator" : "管理员"}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-0.5 shrink-0">
                <SidebarIconAction
                  icon={Languages}
                  label={t("lang.toggle")}
                  onClick={() => setLocale(locale === "en" ? "zh" : "en")}
                />
                <SidebarIconAction
                  icon={LogOut}
                  label={t("nav.signOut")}
                  onClick={signOut}
                />
              </div>
            </div>
          )}
        </div>
      </aside>
      <main className="flex-1 overflow-auto relative">
        {/* The route graph is full-bleed: it owns the entire main
            area so React Flow can size its canvas to the viewport.
            Every other page sits inside the centred max-width
            container. */}
        {tab === "route-graph" ? (
          <RouteGraphPage />
        ) : (
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
        )}
      </main>
    </div>
  );
}

// SidebarIconAction is the compact icon-only button used in the
// sidebar footer. It always renders as a 8x8 square; because it has no
// visible label in either collapsed or expanded mode, it always shows
// a hover tooltip so the operator can tell what each glyph does.
function SidebarIconAction({
  icon: Icon,
  label,
  onClick,
}: {
  icon: typeof LayoutDashboard;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      onClick={onClick}
      className="group relative inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-all duration-200 hover:bg-accent hover:text-foreground"
    >
      <Icon className="h-[18px] w-[18px]" />
      <SidebarTooltip label={label} />
    </button>
  );
}

// SidebarTooltip is a CSS-only hover hint that floats to the right of
// its parent button. The parent must be `relative group` so that the
// `group-hover` selector + `left-full` positioning work; we keep it
// pointer-events-none and z-50 so it never blocks clicks and always
// renders above adjacent rows. Using a custom tooltip (rather than the
// browser's `title` attribute) lets us match the dashboard's font and
// theming, and crucially shows up instantly instead of after the OS's
// ~1s delay — important when scanning a column of unfamiliar icons.
function SidebarTooltip({ label }: { label: string }) {
  return (
    <span className="pointer-events-none absolute left-full top-1/2 z-50 ml-2 -translate-y-1/2 whitespace-nowrap rounded-md border border-border/60 bg-foreground px-2 py-1 text-xs font-medium text-background opacity-0 shadow-md transition-opacity group-hover:opacity-100">
      {label}
    </span>
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
