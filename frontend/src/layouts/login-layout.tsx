import type { ReactNode } from "react"
import { AuthShowcase } from "@/pages/login/showcase"
import { FluxaLogo } from "@/components/console/brand"

// The sign-in screens sit outside the main frame: no sidebar, no top bar,
// and no footer either -- two columns and nothing else (DESIGN.md 6.3).
// Version and licence are console chrome; on this screen they were a
// third band competing with the form for a page that asks one thing.
//
// The mark lives here rather than inside each view's own heading block.
// It is pinned to the top of the form column with the form hanging a
// fixed distance below it, and the pair centred as one group -- so every
// view gets it in the same place, and the three of them (entry, form,
// pending) stop drifting apart from each other.
//
// The right column is decoration in the strict sense -- nothing in it is
// needed to sign in -- so it is the half that goes when the viewport is
// too narrow for two, and it is marked aria-hidden for a screen reader
// walking the page to sign in.
export function LoginShell({ children }: { children: ReactNode }) {
  return (
    <div className="screen cn cn-auth">
      <div className="cn-auth-main">
        <div className="cn-auth-col">
          <div className="cn-auth-logo">
            <FluxaLogo size={48} />
          </div>
          <div className="cn-auth-page">{children}</div>
        </div>
      </div>
      <AuthShowcase />
    </div>
  )
}
