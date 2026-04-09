import { useRef, useState } from "react";
import { Download, Upload, Monitor, Sun, Moon, Type, Languages, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Config } from "@/lib/api";
import { useT } from "@/lib/i18n";
import { useAppearance, Theme, FontSize } from "@/components/appearance-provider";

export function SettingsPage() {
  const { t } = useT();
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("settings.subtitle")}</p>
      </div>
      <AppearanceCard />
      <BackupCard />
    </div>
  );
}

function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
}: {
  options: { label: string; value: T; icon?: React.ReactNode }[];
  value: T;
  onChange: (val: T) => void;
}) {
  return (
    <div className="flex w-full sm:w-auto items-center p-1 rounded-xl bg-accent/50 text-muted-foreground border border-border/40">
      {options.map((opt) => {
        const isActive = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={`flex flex-1 sm:flex-initial items-center justify-center whitespace-nowrap rounded-lg px-4 py-1.5 text-[13px] font-medium transition-all duration-300 ${
              isActive
                ? "bg-background text-foreground shadow-sm ring-1 ring-border/20 scale-[0.98] sm:scale-100"
                : "hover:text-foreground opacity-80 hover:opacity-100 hover:bg-background/40"
            }`}
          >
            {opt.icon && <span className="mr-1.5 shrink-0">{opt.icon}</span>}
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

function AppearanceCard() {
  const { t, locale, setLocale } = useT();
  const { theme, setTheme, fontSize, setFontSize } = useAppearance();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.appearanceTitle")}</CardTitle>
        <CardDescription>{t("settings.appearanceSubtitle")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Languages className="w-4 h-4 text-muted-foreground" />
            {t("settings.language")}
          </div>
          <SegmentedControl<"en" | "zh">
            value={locale}
            onChange={(l) => setLocale(l)}
            options={[
              { label: "English", value: "en" },
              { label: "简体中文", value: "zh" },
            ]}
          />
        </div>

        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Monitor className="w-4 h-4 text-muted-foreground" />
            {t("settings.theme")}
          </div>
          <SegmentedControl<Theme>
            value={theme}
            onChange={(t_val) => setTheme(t_val)}
            options={[
              { label: t("settings.themeLight"), value: "light", icon: <Sun className="w-3.5 h-3.5" /> },
              { label: t("settings.themeDark"), value: "dark", icon: <Moon className="w-3.5 h-3.5" /> },
              { label: t("settings.themeSystem"), value: "system", icon: <Monitor className="w-3.5 h-3.5" /> },
            ]}
          />
        </div>

        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Type className="w-4 h-4 text-muted-foreground" />
            {t("settings.fontSize")}
          </div>
          <SegmentedControl<FontSize>
            value={fontSize}
            onChange={(s) => setFontSize(s)}
            options={[
              { label: t("settings.fontSmall"), value: "small" },
              { label: t("settings.fontMedium"), value: "medium" },
              { label: t("settings.fontLarge"), value: "large" },
            ]}
          />
        </div>
      </CardContent>
    </Card>
  );
}

function BackupCard() {
  const { t } = useT();
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function handleExport() {
    setBusy(true);
    setError(null);
    setStatus(null);
    try {
      const text = await Config.export();
      const blob = new Blob([text], { type: "application/yaml" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "fluxa_backup.yaml";
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setBusy(true);
    setError(null);
    setStatus(null);
    
    try {
      const yaml = await file.text();
      await Config.import(yaml);
      setStatus(t("settings.backupRestoreDone"));
      // Clear the file input so the same file could theoretically be picked again
      if (fileInputRef.current) fileInputRef.current.value = "";
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.backupTitle")}</CardTitle>
        <CardDescription>{t("settings.backupSubtitle")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {(status || error) && (
          <div className={`p-3 rounded-md text-sm flex items-start gap-2 ${
            status ? "bg-green-500/10 text-green-700 dark:text-green-400" : "bg-destructive/10 text-destructive"
          }`}>
            {status ? <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" /> : null}
            <div>{status || error}</div>
          </div>
        )}
        
        <div className="flex flex-col sm:flex-row gap-3">
          <Button
            variant="outline"
            className="w-full sm:w-auto"
            onClick={handleExport}
            disabled={busy}
          >
            <Download className="mr-2 h-4 w-4" />
            {t("settings.backupDownload")}
          </Button>

          <Button 
            className="w-full sm:w-auto relative overflow-hidden" 
            disabled={busy}
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload className="mr-2 h-4 w-4" />
            {t("settings.backupUpload")}
            <input
              ref={fileInputRef}
              type="file"
              accept=".yaml,.yml"
              className="hidden"
              onChange={handleFileChange}
            />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
