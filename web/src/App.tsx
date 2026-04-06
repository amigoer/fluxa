import { useEffect, useState } from "react";
import {
  LayoutDashboard,
  Server,
  Waypoints,
  KeyRound,
  BarChart3,
  LogOut,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { getMasterKey, setMasterKey } from "@/lib/api";
import { DashboardPage } from "@/pages/Dashboard";
import { ProvidersPage } from "@/pages/Providers";
import { RoutesPage } from "@/pages/Routes";
import { KeysPage } from "@/pages/Keys";
import { UsagePage } from "@/pages/Usage";

// Tab is the top-level navigation entry. The dashboard is deliberately
// a single-file router: five tabs, no nested pages, keeps the bundle
// tiny and matches the scope of the admin REST surface.
type Tab = "dashboard" | "providers" | "routes" | "keys" | "usage";

const NAV: { id: Tab; label: string; icon: typeof LayoutDashboard }[] = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard },
  { id: "providers", label: "Providers", icon: Server },
  { id: "routes", label: "Routes", icon: Waypoints },
  { id: "keys", label: "Virtual keys", icon: KeyRound },
  { id: "usage", label: "Usage", icon: BarChart3 },
];

export default function App() {
  const [authed, setAuthed] = useState<boolean>(!!getMasterKey());
  const [tab, setTab] = useState<Tab>("dashboard");

  if (!authed) {
    return <LoginScreen onAuth={() => setAuthed(true)} />;
  }

  return (
    <div className="min-h-screen flex bg-background text-foreground">
      <aside className="w-60 border-r flex flex-col">
        <div className="px-6 py-5 border-b">
          <div className="text-xl font-bold tracking-tight">Fluxa</div>
          <div className="text-xs text-muted-foreground">AI gateway admin</div>
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
                {n.label}
              </button>
            );
          })}
        </nav>
        <div className="p-3 border-t">
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start"
            onClick={() => {
              setMasterKey("");
              setAuthed(false);
            }}
          >
            <LogOut className="h-4 w-4" />
            Sign out
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
        </div>
      </main>
    </div>
  );
}

// LoginScreen captures the master key once and pushes it into
// sessionStorage. It optimistically probes /admin/providers to give
// instant feedback when the key is wrong rather than waiting for the
// first real action to fail.
function LoginScreen({ onAuth }: { onAuth: () => void }) {
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setValue(getMasterKey());
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setMasterKey(value);
    try {
      const res = await fetch("/admin/providers", {
        headers: { Authorization: `Bearer ${value}` },
      });
      if (!res.ok) {
        throw new Error("invalid master key");
      }
      onAuth();
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed");
      setMasterKey("");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Fluxa admin</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="key">Master key</Label>
              <Input
                id="key"
                type="password"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                placeholder="server.master_key from fluxa.yaml"
                autoFocus
              />
              {error && (
                <p className="text-xs text-destructive">{error}</p>
              )}
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Checking…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
