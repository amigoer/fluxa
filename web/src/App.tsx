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
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { Auth, getSessionToken, type AdminUser } from "@/lib/api";
import { I18nProvider, useT, type TranslationKey } from "@/lib/i18n";
import { DashboardPage } from "@/pages/Dashboard";
import { ProvidersPage } from "@/pages/Providers";
import { RoutesPage } from "@/pages/Routes";
import { KeysPage } from "@/pages/Keys";
import { UsagePage } from "@/pages/Usage";
import { SettingsPage } from "@/pages/Settings";

// Tab is the top-level navigation entry. The dashboard is deliberately
// a single-file router: six tabs, no nested pages, keeps the bundle
// tiny and matches the scope of the admin REST surface.
type Tab = "dashboard" | "providers" | "routes" | "keys" | "usage" | "settings";

interface NavEntry {
  id: Tab;
  labelKey: TranslationKey;
  icon: typeof LayoutDashboard;
}

const NAV: NavEntry[] = [
  { id: "dashboard", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { id: "providers", labelKey: "nav.providers", icon: Server },
  { id: "routes", labelKey: "nav.routes", icon: Waypoints },
  { id: "keys", labelKey: "nav.keys", icon: KeyRound },
  { id: "usage", labelKey: "nav.usage", icon: BarChart3 },
  { id: "settings", labelKey: "nav.settings", icon: SettingsIcon },
];

// App is the entry component. It wraps the real shell in I18nProvider so
// every descendant — including the login screen — can call useT().
export default function App() {
  return (
    <I18nProvider>
      <Shell />
    </I18nProvider>
  );
}

function Shell() {
  const { t, locale, setLocale } = useT();
  const [user, setUser] = useState<AdminUser | null>(null);
  const [loadingMe, setLoadingMe] = useState<boolean>(!!getSessionToken());
  const [tab, setTab] = useState<Tab>("dashboard");

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
    <div className="min-h-screen flex bg-background text-foreground">
      <aside className="w-60 border-r flex flex-col">
        <div className="px-6 py-5 border-b">
          <div className="text-xl font-bold tracking-tight">{t("app.title")}</div>
          <div className="text-xs text-muted-foreground">{t("app.subtitle")}</div>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {NAV.map((n) => {
            const Icon = n.icon;
            return (
              <button
                key={n.id}
                onClick={() => setTab(n.id)}
                className={cn(
                  "w-full flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors",
                  tab === n.id
                    ? "bg-accent text-accent-foreground font-medium"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {t(n.labelKey)}
              </button>
            );
          })}
        </nav>
        <div className="p-3 border-t space-y-2">
          <div className="text-xs text-muted-foreground px-2">
            {t("nav.account")} <span className="font-medium text-foreground">{user.username}</span>
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start"
            onClick={() => setLocale(locale === "en" ? "zh" : "en")}
          >
            <Languages className="h-4 w-4" />
            {t("lang.toggle")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start"
            onClick={signOut}
          >
            <LogOut className="h-4 w-4" />
            {t("nav.signOut")}
          </Button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto">
        <div className="max-w-6xl mx-auto px-8 py-8">
          {tab === "dashboard" && <DashboardPage />}
          {tab === "providers" && <ProvidersPage />}
          {tab === "routes" && <RoutesPage />}
          {tab === "keys" && <KeysPage />}
          {tab === "usage" && <UsagePage />}
          {tab === "settings" && <SettingsPage onSignOut={signOut} />}
        </div>
      </main>
    </div>
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
    <div className="min-h-screen flex items-center justify-center bg-background relative">
      <button
        onClick={() => setLocale(locale === "en" ? "zh" : "en")}
        className="absolute top-4 right-4 text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1"
      >
        <Languages className="h-3 w-3" />
        {t("lang.toggle")}
      </button>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t("login.title")}</CardTitle>
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
