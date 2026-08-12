import { useState } from "react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { PageHeader } from "@/components/page";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

export function SettingsPage() {
  const t = useT();
  const { user, refresh } = useAuth();

  return (
    <>
      <PageHeader title={t("settings.title")} description={t("settings.description")} />

      <div className="grid gap-6 lg:grid-cols-2">
        <ProfileCard
          initial={{
            nickname: user?.nickname ?? "",
            email: user?.email ?? "",
            avatar_url: user?.avatar_url ?? "",
          }}
          username={user?.username ?? ""}
          createdAt={user?.created_at}
          onSaved={refresh}
        />
        <PasswordCard />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("settings.routerTitle")}</CardTitle>
<CardDescription>{t("settings.routerDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            variant="outline"
            onClick={async () => {
              try {
                await api.reload();
                toast.success(t("settings.routerReloaded"));
              } catch (err) {
                toast.error(err instanceof Error ? err.message : String(err));
              }
            }}
          >
            {t("settings.reloadRouter")}
          </Button>
        </CardContent>
      </Card>
    </>
  );
}

function ProfileCard({
  initial,
  username,
  createdAt,
  onSaved,
}: {
  initial: { nickname: string; email: string; avatar_url: string };
  username: string;
  createdAt?: string;
  onSaved: () => Promise<void>;
}) {
  const t = useT();
  const [form, setForm] = useState(initial);
  const [busy, setBusy] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await api.updateProfile(form);
      await onSaved();
      toast.success(t("settings.profileUpdated"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.profileTitle")}</CardTitle>
        <CardDescription>
          {createdAt
            ? t("settings.profileJoined", { username, date: formatDateTime(createdAt) })
            : t("settings.profileSignedIn", { username })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="nickname">{t("settings.displayName")}</Label>
            <Input
              id="nickname"
              value={form.nickname}
              onChange={(event) => setForm({ ...form, nickname: event.target.value })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">{t("settings.email")}</Label>
            <Input
              id="email"
              type="email"
              value={form.email}
              onChange={(event) => setForm({ ...form, email: event.target.value })}
            />
          </div>
          <Separator />
          <Button type="submit" disabled={busy}>
            {busy ? t("common.saving") : t("settings.saveProfile")}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function PasswordCard() {
  const t = useT();
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);

  const mismatch = confirm.length > 0 && confirm !== newPassword;

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await api.changePassword(oldPassword, newPassword);
      // The gateway revokes every session on a password change, so the
      // next request will 401 and bounce us to the login screen.
      toast.success(t("settings.passwordChanged"));
      setOldPassword("");
      setNewPassword("");
      setConfirm("");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.passwordTitle")}</CardTitle>
        <CardDescription>{t("settings.passwordDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="old-password">{t("settings.currentPassword")}</Label>
            <Input
              id="old-password"
              type="password"
              autoComplete="current-password"
              value={oldPassword}
              onChange={(event) => setOldPassword(event.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">{t("settings.newPassword")}</Label>
            <Input
              id="new-password"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-password">{t("settings.confirmPassword")}</Label>
            <Input
              id="confirm-password"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(event) => setConfirm(event.target.value)}
              aria-invalid={mismatch}
              required
            />
            {mismatch ? <p className="text-destructive text-xs">{t("settings.mismatch")}</p> : null}
          </div>
          <Separator />
          <Button type="submit" disabled={busy || mismatch || !newPassword}>
            {busy ? t("settings.changing") : t("settings.changePassword")}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
