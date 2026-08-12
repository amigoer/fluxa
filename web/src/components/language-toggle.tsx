import { SegmentedGroup, SegmentedItem } from "@/components/segmented";
import { LOCALES, useI18n } from "@/lib/i18n";

/**
 * With only two locales a dropdown costs a click and a popover to do what
 * two buttons do inline, so this is a segmented switch matching the theme
 * control beside it. Each label is written in its own language — the
 * convention that lets someone stranded in the wrong locale find the way
 * back. Revisit if a third language lands: past two or three options this
 * should become a menu again.
 */
export function LanguageToggle() {
  const { locale, setLocale, t } = useI18n();

  return (
    <SegmentedGroup label={t("common.language")}>
      {LOCALES.map((option) => (
        <SegmentedItem
          key={option.value}
          checked={locale === option.value}
          onSelect={() => setLocale(option.value)}
          label={option.label}
          className="px-2"
        >
          {option.short}
        </SegmentedItem>
      ))}
    </SegmentedGroup>
  );
}
