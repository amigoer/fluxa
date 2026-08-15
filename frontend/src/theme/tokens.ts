// Cobalt Blue palette (DESIGN.md 6.2) as plain JS values, for the rare
// places that need an actual color string instead of a CSS class --
// chart bars, canvas drawing. Everything else should use the Tailwind
// utilities (bg-primary, text-muted-foreground, ...) backed by the CSS
// custom properties in index.css; this file exists only where CSS
// custom properties can't reach.
export const tokens = {
  light: {
    accent: "#2F5FEA",
    accentSoft: "#E9EEFF",
    background: "#FFFFFF",
    sideBg: "#F7F8FA",
    surface: "#FFFFFF",
    border: "#E2E4E9",
    ink: "#0A0B0D",
    muted: "#6B7078",
    ok: "#1E7A4E",
    okSoft: "#E3F2E9",
    warn: "#9A5E23",
    warnSoft: "#F5E6D3",
    bad: "#C23B3B",
    badSoft: "#FBE6E6",
  },
  dark: {
    accent: "#4B7CFF",
    accentSoft: "#16233F",
    background: "#0A0B0D",
    sideBg: "#0E0F12",
    surface: "#16171B",
    border: "#24262B",
    ink: "#F2F3F5",
    muted: "#8B909B",
    ok: "#4FBE85",
    okSoft: "#12281D",
    warn: "#D99A4E",
    warnSoft: "#332812",
    bad: "#E5695F",
    badSoft: "#2E1917",
  },
} as const

export type ThemeMode = keyof typeof tokens
