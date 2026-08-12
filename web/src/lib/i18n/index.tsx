// i18n — a small in-house translation layer for the console.
//
// No i18next: the whole string surface is one file, the console ships as
// an embedded SPA where every kilobyte is in the binary, and the typed
// dictionary already gives the one guarantee that matters — a missing
// translation is a compile error, not a blank label at runtime.
//
// What it does provide: locale persistence, OS-language detection on
// first visit, {placeholder} interpolation, and locale-aware number and
// date formatting through lib/format.

import { createContext, use, useCallback, useEffect, useMemo, useState } from "react";

import { setFormatLocale } from "@/lib/format";

import { en, type Dictionary, type TranslationKey } from "./en";
import { zhCN } from "./zh-CN";

export type Locale = "en" | "zh-CN";

export const LOCALES: { value: Locale; label: string }[] = [
  { value: "en", label: "English" },
  { value: "zh-CN", label: "中文" },
];

const STORAGE_KEY = "fluxa-locale";

const DICTIONARIES: Record<Locale, Dictionary> = { en, "zh-CN": zhCN };

/** Values substituted into {placeholder} slots. */
export type TranslateVars = Record<string, string | number>;

export type Translate = (key: TranslationKey, vars?: TranslateVars) => string;

type I18nState = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: Translate;
};

const I18nContext = createContext<I18nState | null>(null);

function detectLocale(): Locale {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "en" || stored === "zh-CN") return stored;
  } catch {
    // Storage unavailable; fall through to the browser preference.
  }
  // Any Chinese variant (zh, zh-TW, zh-Hans-CN…) gets the Chinese copy;
  // everything else falls back to English.
  return navigator.language?.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

const PLACEHOLDER = /\{(\w+)\}/g;

function interpolate(template: string, vars?: TranslateVars) {
  if (!vars) return template;
  return template.replace(PLACEHOLDER, (match, name: string) =>
    name in vars ? String(vars[name]) : match,
  );
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(detectLocale);

  // Keep the document in sync so screen readers, spell-checkers and the
  // browser's own translation prompt see the right language.
  useEffect(() => {
    document.documentElement.lang = locale;
    setFormatLocale(locale);
  }, [locale]);

  const setLocale = useCallback((next: Locale) => {
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Non-fatal: the choice just will not survive a reload.
    }
    setLocaleState(next);
  }, []);

  const t = useCallback<Translate>(
    (key, vars) => interpolate(DICTIONARIES[locale][key], vars),
    [locale],
  );

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext value={value}>{children}</I18nContext>;
}

export function useI18n() {
  const context = use(I18nContext);
  if (!context) throw new Error("useI18n must be used inside an I18nProvider");
  return context;
}

/** Shorthand for the common case of only needing the translate function. */
export function useT(): Translate {
  return useI18n().t;
}

export type { TranslationKey };
