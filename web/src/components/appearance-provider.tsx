import { createContext, useContext, useEffect, useState } from "react";

export type Theme = "dark" | "light" | "system";
export type FontSize = "small" | "medium" | "large";

type AppearanceProviderProps = {
  children: React.ReactNode;
  defaultTheme?: Theme;
  defaultFontSize?: FontSize;
  storageKeyTheme?: string;
  storageKeyFontSize?: string;
};

type AppearanceProviderState = {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  fontSize: FontSize;
  setFontSize: (size: FontSize) => void;
};

const initialState: AppearanceProviderState = {
  theme: "system",
  setTheme: () => null,
  fontSize: "medium",
  setFontSize: () => null,
};

const AppearanceProviderContext = createContext<AppearanceProviderState>(initialState);

export function AppearanceProvider({
  children,
  defaultTheme = "system",
  defaultFontSize = "medium",
  storageKeyTheme = "fluxa-ui-theme",
  storageKeyFontSize = "fluxa-ui-fontsize",
  ...props
}: AppearanceProviderProps) {
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem(storageKeyTheme) as Theme) || defaultTheme
  );
  const [fontSize, setFontSizeState] = useState<FontSize>(
    () => (localStorage.getItem(storageKeyFontSize) as FontSize) || defaultFontSize
  );

  useEffect(() => {
    const root = window.document.documentElement;
    root.classList.remove("light", "dark");
    if (theme === "system") {
      const systemTheme = window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
      root.classList.add(systemTheme);
      return;
    }
    root.classList.add(theme);
  }, [theme]);

  useEffect(() => {
    const root = window.document.documentElement;
    // Base is 16px. Rem scales dynamically based on html font-size.
    if (fontSize === "small") root.style.fontSize = "14px";
    else if (fontSize === "large") root.style.fontSize = "18px";
    else root.style.fontSize = "16px";
  }, [fontSize]);

  const value = {
    theme,
    setTheme: (theme: Theme) => {
      localStorage.setItem(storageKeyTheme, theme);
      setTheme(theme);
    },
    fontSize,
    setFontSize: (size: FontSize) => {
      localStorage.setItem(storageKeyFontSize, size);
      setFontSizeState(size);
    },
  };

  return (
    <AppearanceProviderContext.Provider {...props} value={value}>
      {children}
    </AppearanceProviderContext.Provider>
  );
}

export const useAppearance = () => {
  const context = useContext(AppearanceProviderContext);
  if (context === undefined)
    throw new Error("useAppearance must be used within an AppearanceProvider");
  return context;
};
