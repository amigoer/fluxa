import type { ReactNode } from "react"
import { AuthShowcase } from "@/pages/login/showcase"

// The sign-in screens sit outside the main frame: no sidebar, no top bar,
// and no footer either -- two columns and nothing else (DESIGN.md 6.3).
// Version and licence are console chrome; on this screen they were a
// third band competing with the form for a page that asks one thing.
//
// The right column is decoration in the strict sense -- nothing in it is
// needed to sign in -- so it is the half that goes when the viewport is
// too narrow for two, and it is marked aria-hidden for a screen reader
// walking the page to sign in.
export function LoginShell({ children }: { children: ReactNode }) {
  return (
    <div className="screen cn cn-auth">
      <div className="cn-auth-main">
        <div className="cn-auth-col">{children}</div>
      </div>
      <AuthShowcase />
    </div>
  )
}
