import { createContext, use, useCallback, useEffect, useMemo, useState } from "react";

export type Theme = "dark" | "light" | "system";

const STORAGE_KEY = "fluxa-theme";

type ThemeProviderState = {
  /** What the operator picked, including "system". */
  theme: Theme;
  /** What is actually on screen right now — never "system". */
  resolvedTheme: "dark" | "light";
  setTheme: (theme: Theme) => void;
};

const ThemeProviderContext = createContext<ThemeProviderState | null>(null);

function prefersDark() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function readStoredTheme(fallback: Theme): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "light" || stored === "system") {
      return stored;
    }
  } catch {
    // Storage can be unavailable (private mode, blocked cookies). Falling
    // back to the default is better than refusing to render.
  }
  return fallback;
}

/**
 * ThemeProvider owns the `dark` class on <html>, which is the single switch
 * every design token in index.css keys off. The inline script in index.html
 * applies the same decision before first paint; this component takes over
 * once React mounts and keeps the two in sync through STORAGE_KEY.
 */
export function ThemeProvider({
  children,
  defaultTheme = "dark",
}: {
  children: React.ReactNode;
  defaultTheme?: Theme;
}) {
  const [theme, setThemeState] = useState<Theme>(() => readStoredTheme(defaultTheme));
  const [systemDark, setSystemDark] = useState(prefersDark);

  // Track the OS preference so "system" stays live rather than being
  // sampled once at mount.
  useEffect(() => {
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = (event: MediaQueryListEvent) => setSystemDark(event.matches);
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);

  const resolvedTheme = theme === "system" ? (systemDark ? "dark" : "light") : theme;

  useEffect(() => {
    document.documentElement.classList.toggle("dark", resolvedTheme === "dark");
  }, [resolvedTheme]);

  const setTheme = useCallback((next: Theme) => {
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Non-fatal: the choice simply will not survive a reload.
    }
    setThemeState(next);
  }, []);

  const value = useMemo(
    () => ({ theme, resolvedTheme, setTheme }),
    [theme, resolvedTheme, setTheme],
  );

  return <ThemeProviderContext value={value}>{children}</ThemeProviderContext>;
}

export function useTheme() {
  const context = use(ThemeProviderContext);
  if (!context) {
    throw new Error("useTheme must be used inside a ThemeProvider");
  }
  return context;
}
