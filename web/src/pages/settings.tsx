import { useState } from "react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { PageHeader } from "@/components/page";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

export function SettingsPage() {
  const { user, refresh } = useAuth();

  return (
    <>
      <PageHeader title="Settings" description="Your administrator account." />

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
          <CardTitle>Router</CardTitle>
          <CardDescription>
            Writes reload the router automatically; use this if you changed data out of band.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            variant="outline"
            onClick={async () => {
              try {
                await api.reload();
                toast.success("Router reloaded");
              } catch (err) {
                toast.error(err instanceof Error ? err.message : String(err));
              }
            }}
          >
            Reload router
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
  const [form, setForm] = useState(initial);
  const [busy, setBusy] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await api.updateProfile(form);
      await onSaved();
      toast.success("Profile updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>
          Signed in as {username}
          {createdAt ? ` · joined ${formatDateTime(createdAt)}` : ""}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="nickname">Display name</Label>
            <Input
              id="nickname"
              value={form.nickname}
              onChange={(event) => setForm({ ...form, nickname: event.target.value })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              value={form.email}
              onChange={(event) => setForm({ ...form, email: event.target.value })}
            />
          </div>
          <Separator />
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save profile"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function PasswordCard() {
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
      toast.success("Password changed — sign in again with the new one");
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
        <CardTitle>Password</CardTitle>
        <CardDescription>Changing it signs out every other active session.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="old-password">Current password</Label>
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
            <Label htmlFor="new-password">New password</Label>
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
            <Label htmlFor="confirm-password">Confirm new password</Label>
            <Input
              id="confirm-password"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(event) => setConfirm(event.target.value)}
              aria-invalid={mismatch}
              required
            />
            {mismatch ? <p className="text-destructive text-xs">Passwords do not match</p> : null}
          </div>
          <Separator />
          <Button type="submit" disabled={busy || mismatch || !newPassword}>
            {busy ? "Changing…" : "Change password"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
