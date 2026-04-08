import { useState } from "react";
import { useT } from "@/lib/i18n";
import { Auth, type AdminUser } from "@/lib/api";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { useRef } from "react";
import { Camera, Save, Loader2, UserRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

interface ProfilePageProps {
  user: AdminUser;
  onUserUpdate: (u: AdminUser) => void;
  onSignOut: () => void | Promise<void>;
}

export function ProfilePage({ user, onUserUpdate, onSignOut }: ProfilePageProps) {
  const { t } = useT();
  const [nickname, setNickname] = useState(user.nickname || "");
  const [email, setEmail] = useState(user.email || "");
  const [avatar, setAvatar] = useState(user.avatar_url || "");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [profStatus, setProfStatus] = useState<string | null>(null);
  const [profError, setProfError] = useState<string | null>(null);
  const [profBusy, setProfBusy] = useState(false);

  const [oldPass, setOldPass] = useState("");
  const [newPass, setNewPass] = useState("");
  const [confirm, setConfirm] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function handleAvatarPick(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setProfBusy(true);
    setProfError(null);
    try {
      const url = await Auth.uploadAvatar(file);
      setAvatar(url);
      setProfStatus(t("settings.avatarUploaded"));
      // auto-save to propagate to App.tsx
      await Auth.updateProfile(nickname, email, url);
      onUserUpdate({ ...user, avatar_url: url });
    } catch (err) {
      setProfError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setProfBusy(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  async function handleProfileSubmit(e: React.FormEvent) {
    e.preventDefault();
    setProfBusy(true);
    setProfError(null);
    setProfStatus(null);
    try {
      await Auth.updateProfile(nickname, email, avatar);
      onUserUpdate({ ...user, nickname, email, avatar_url: avatar });
      setProfStatus(t("settings.profileSaved"));
    } catch (err) {
      setProfError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setProfBusy(false);
    }
  }

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
          <CardTitle>{t("settings.profileDetails")}</CardTitle>
          <CardDescription>
            {t("settings.profileSubtitle")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleProfileSubmit} className="space-y-6 max-w-sm">
            <div className="flex items-center gap-6">
              <div className="relative group">
                <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-full bg-muted border border-border shadow-sm overflow-hidden bg-indigo-500/10 text-indigo-700">
                  {avatar ? (
                    <img src={avatar} alt="Avatar" className="h-full w-full object-cover" />
                  ) : (
                    <UserRound className="h-8 w-8" />
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  className="absolute inset-0 flex items-center justify-center bg-black/40 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer backdrop-blur-sm"
                >
                  {profBusy ? <Loader2 className="h-5 w-5 animate-spin" /> : <Camera className="h-5 w-5" />}
                </button>
                <input
                  type="file"
                  accept="image/png, image/jpeg, image/webp"
                  className="hidden"
                  ref={fileInputRef}
                  onChange={handleAvatarPick}
                />
              </div>
              <div className="flex-1 space-y-1">
                <h3 className="font-medium text-lg">{user.nickname || user.username}</h3>
                <p className="text-xs text-muted-foreground">{user.email || t("settings.noEmail")}</p>
              </div>
            </div>

            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="nickname">{t("settings.displayName")}</Label>
                <Input
                  id="nickname"
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                  placeholder={t("settings.displayNameHint")}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="email">{t("settings.emailAddr")}</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t("settings.emailHint")}
                />
              </div>
            </div>

            {profStatus && <p className="text-xs font-medium text-green-600 dark:text-green-400">{profStatus}</p>}
            {profError && <p className="text-xs font-medium text-destructive">{profError}</p>}
            
            <div className="pt-2">
              <Button type="submit" disabled={profBusy}>
                <Save className="h-4 w-4 mr-2" />
                {t("settings.saveProfile")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

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
