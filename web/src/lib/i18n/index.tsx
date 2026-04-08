// i18n.tsx — tiny in-house translation layer for the dashboard.
//
// We deliberately do not pull in i18next or react-intl: the full string
// surface fits in one file, the dashboard ships as a single embedded
// SPA, and adding 200 KB of dependency for ~120 strings is not the
// trade-off we want. Instead this module exposes:
//
//   - I18nProvider: a context that holds the active locale ("en" | "zh")
//     and persists the user's choice in localStorage so it survives
//     page reloads.
//   - useT(): returns a `t(key)` function plus the active locale and a
//     setLocale setter for the language toggle UI.
//   - All keys live in the `dict` object below; the type system enforces
//     that every locale supplies every key, so a missing translation is
//     a build-time error rather than a runtime "??missing??".

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Locale = "en" | "zh";

const STORAGE_KEY = "fluxa-locale";

// dict is the single source of truth for every visible string. Add a
// new entry here, then reference it via t("yourKey") anywhere in the
// app. Both locales must define the key — TypeScript will reject any
// drift across the two records.
import { en, type TranslationKey } from "./en";
import { zh } from "./zh";

const dict = { en, zh } as const;

export type { TranslationKey };

interface I18nValue {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: (key: TranslationKey, params?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | undefined>(undefined);

// I18nProvider wraps the application root and exposes the t() helper to
// any descendant. Initial locale is read from localStorage, then falls
// back to the browser's preferred language, then to English.
export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "en" || stored === "zh") return stored;
    if (typeof navigator !== "undefined" && navigator.language?.toLowerCase().startsWith("zh")) {
      return "zh";
    }
    return "en";
  });

  useEffect(() => {
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
  }, [locale]);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    localStorage.setItem(STORAGE_KEY, l);
  }, []);

  const t = useCallback(
    (key: TranslationKey, params?: Record<string, string | number>) => {
      const template = dict[locale][key] ?? key;
      if (!params) return template;
      // Tiny mustache: {name} → params.name, no escaping fanciness.
      return template.replace(/\{(\w+)\}/g, (_, k) =>
        params[k] !== undefined ? String(params[k]) : `{${k}}`,
      );
    },
    [locale],
  );

  const value = useMemo<I18nValue>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

// useT is the hook every page calls. Throws if used outside the
// provider so a missing wrapper surfaces immediately in dev.
export function useT(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useT must be used within I18nProvider");
  return ctx;
}
