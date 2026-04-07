// Settings page — YAML import / export plus account management.
//
// Two cards live here: the config bundle round-trip (load from gateway,
// download, paste & apply) and the password rotation form for the
// currently signed-in admin user. Both are driven entirely by the admin
// REST surface so the page survives a backend version bump as long as
// the endpoints keep their shape.

import { useState } from "react";
import { Download, Upload, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Auth, Config } from "@/lib/api";
import { useT } from "@/lib/i18n";

interface SettingsPageProps {
  // onSignOut is invoked after a successful password change so the
  // shell can drop the user back to the login screen — the backend
  // already revoked every existing session for this user as part of
  // the password rotation, so the current token is dead anyway.
  onSignOut: () => void | Promise<void>;
}

export function SettingsPage({ onSignOut }: SettingsPageProps) {
  return (
    <div className="space-y-6">
      <ConfigBundleCard />
      <AccountCard onSignOut={onSignOut} />
    </div>
  );
}

function ConfigBundleCard() {
  const { t } = useT();
  const [yaml, setYaml] = useState<string>("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function handleExport() {
    setBusy(true);
    setError(null);
    setStatus(null);
    try {
      const text = await Config.export();
      setYaml(text);
      setStatus(t("settings.loaded"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setBusy(false);
    }
  }

  function handleDownload() {
    if (!yaml) return;
    const blob = new Blob([yaml], { type: "application/yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "fluxa.yaml";
    a.click();
    URL.revokeObjectURL(url);
  }

  async function handleImport() {
    if (!yaml.trim()) {
      setError(t("settings.pasteFirst"));
      return;
    }
    setBusy(true);
    setError(null);
    setStatus(null);
    try {
      const result = await Config.import(yaml);
      setStatus(
        t("settings.imported", {
          providers: result.providers,
          routes: result.routes,
        }),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("settings.subtitle")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>{t("settings.bundleTitle")}</span>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={handleExport}
                disabled={busy}
              >
                <RefreshCw className="h-4 w-4" />
                {t("settings.loadFromGateway")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={handleDownload}
                disabled={!yaml}
              >
                <Download className="h-4 w-4" />
                {t("settings.download")}
              </Button>
              <Button size="sm" onClick={handleImport} disabled={busy}>
                <Upload className="h-4 w-4" />
                {t("settings.applyImport")}
              </Button>
            </div>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {status && (
            <p className="text-xs text-green-600 dark:text-green-400">{status}</p>
          )}
          {error && <p className="text-xs text-destructive">{error}</p>}
          <textarea
            value={yaml}
            onChange={(e) => setYaml(e.target.value)}
            spellCheck={false}
            placeholder={
              "providers:\n  - name: openai\n    kind: openai\n    api_key: ${OPENAI_API_KEY}\nroutes:\n  - model: gpt-4o\n    provider: openai\n"
            }
            className="w-full h-[28rem] rounded-md border bg-muted/30 px-3 py-2 font-mono text-xs resize-none focus:outline-none focus:ring-2 focus:ring-ring"
          />
          <p className="text-xs text-muted-foreground">{t("settings.envHint")}</p>
        </CardContent>
      </Card>
    </>
  );
}

function AccountCard({ onSignOut }: SettingsPageProps) {
  const { t } = useT();
  const [oldPass, setOldPass] = useState("");
  const [newPass, setNewPass] = useState("");
  const [confirm, setConfirm] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setStatus(null);
    if (newPass.length < 4) {
      setError(t("settings.passTooShort"));
      return;
    }
    if (newPass !== confirm) {
      setError(t("settings.passMismatch"));
      return;
    }
    setBusy(true);
    try {
      await Auth.changePassword(oldPass, newPass);
      setStatus(t("settings.passChanged"));
      setOldPass("");
      setNewPass("");
      setConfirm("");
      // Backend revokes every existing session for this user as part of
      // the rotation; sign out locally so the user lands back on the
      // login screen with fresh credentials.
      setTimeout(() => {
        void onSignOut();
      }, 800);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.accountTitle")}</CardTitle>
        <p className="text-sm text-muted-foreground">{t("settings.accountSubtitle")}</p>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4 max-w-sm">
          <div className="space-y-2">
            <Label htmlFor="oldpass">{t("settings.fieldOldPass")}</Label>
            <Input
              id="oldpass"
              type="password"
              value={oldPass}
              onChange={(e) => setOldPass(e.target.value)}
              autoComplete="current-password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="newpass">{t("settings.fieldNewPass")}</Label>
            <Input
              id="newpass"
              type="password"
              value={newPass}
              onChange={(e) => setNewPass(e.target.value)}
              autoComplete="new-password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirmpass">{t("settings.fieldConfirmPass")}</Label>
            <Input
              id="confirmpass"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
            />
          </div>
          {status && <p className="text-xs text-green-600 dark:text-green-400">{status}</p>}
          {error && <p className="text-xs text-destructive">{error}</p>}
          <Button type="submit" disabled={busy}>
            {t("settings.changePass")}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
