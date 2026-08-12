import { Monitor, Moon, Sun } from "lucide-react";

import { useTheme, type Theme } from "@/components/theme-provider";
import { useT, type TranslationKey } from "@/lib/i18n";
import { cn } from "@/lib/utils";

const OPTIONS: { value: Theme; label: TranslationKey; Icon: typeof Sun }[] = [
  { value: "light", label: "theme.light", Icon: Sun },
  { value: "dark", label: "theme.dark", Icon: Moon },
  { value: "system", label: "theme.system", Icon: Monitor },
];

/**
 * A three-way segmented switch rather than a toggle: an operator who has
 * explicitly chosen light or dark should be able to get back to "follow the
 * OS" without clearing site data.
 */
export function ModeToggle() {
  const t = useT();
  const { theme, setTheme } = useTheme();

  return (
    <div
      role="radiogroup"
      aria-label={t("theme.label")}
      className="border-border bg-muted/50 inline-flex items-center gap-0.5 rounded-md border p-0.5"
    >
      {OPTIONS.map(({ value, label, Icon }) => (
        <button
          key={value}
          role="radio"
          aria-checked={theme === value}
          aria-label={t(label)}
          title={t(label)}
          onClick={() => setTheme(value)}
          className={cn(
            "rounded-sm p-1.5 transition-colors",
            theme === value
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          <Icon className="size-4" />
        </button>
      ))}
    </div>
  );
}
