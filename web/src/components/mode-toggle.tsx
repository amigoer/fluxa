import { Monitor, Moon, Sun } from "lucide-react";

import { SegmentedGroup, SegmentedItem } from "@/components/segmented";
import { useTheme, type Theme } from "@/components/theme-provider";
import { useT, type TranslationKey } from "@/lib/i18n";

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
    <SegmentedGroup label={t("theme.label")}>
      {OPTIONS.map(({ value, label, Icon }) => (
        <SegmentedItem
          key={value}
          checked={theme === value}
          onSelect={() => setTheme(value)}
          label={t(label)}
          className="w-7"
        >
          <Icon className="size-4" />
        </SegmentedItem>
      ))}
    </SegmentedGroup>
  );
}
