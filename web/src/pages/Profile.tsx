import { useState } from "react";
import { useT } from "@/lib/i18n";
import { Auth } from "@/lib/api";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

interface ProfilePageProps {
  onSignOut: () => void | Promise<void>;
}

export function ProfilePage({ onSignOut }: ProfilePageProps) {
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
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.accountTitle")}</h1>
        <p className="text-sm text-muted-foreground">{t("settings.accountSubtitle")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("settings.changePass")}</CardTitle>
          <CardDescription>
            {t("settings.accountSubtitle")}
          </CardDescription>
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
            {status && <p className="text-xs font-medium text-green-600 dark:text-green-400">{status}</p>}
            {error && <p className="text-xs font-medium text-destructive">{error}</p>}
            <div className="pt-2">
              <Button type="submit" disabled={busy}>
                {t("settings.changePass")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
