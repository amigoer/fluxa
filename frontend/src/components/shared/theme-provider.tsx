import { ThemeProvider as NextThemesProvider } from "next-themes"
import type { ReactNode } from "react"

// Light is the default theme, dark is opt-in (DESIGN.md 6.1: "默认浅色，
// 深色为可选主题（不是默认）"), so defaultTheme is explicitly "light"
// rather than "system" -- an OS set to dark mode should not silently
// change what a new visitor sees.
export function ThemeProvider({ children }: { children: ReactNode }) {
  return (
    <NextThemesProvider attribute="class" defaultTheme="light" enableSystem={false}>
      {children}
    </NextThemesProvider>
  )
}
