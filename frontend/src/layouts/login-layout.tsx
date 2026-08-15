import type { ReactNode } from "react"

// Independent of the main frame (DESIGN.md 6.3): no sidebar, no top bar,
// just a centered card. Naturally responsive without any special-casing
// since it's a single column at any width.
export function LoginLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-side-bg px-4">
      <div className="w-full max-w-[360px] rounded-xl border border-border bg-card p-8 shadow-[var(--shadow-app)]">
        {children}
      </div>
    </div>
  )
}
