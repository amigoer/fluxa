import type { ReactNode } from "react"
import { ConsoleFooter } from "@/layouts/console-layout"

// The login screens sit outside the main frame: no sidebar, no top bar,
// a centred card on the same cold-grey ground the console uses, with the
// same fixed footer underneath (DESIGN.md 6.3).
export function LoginShell({ children }: { children: ReactNode }) {
  return (
    <div className="screen cn" style={{ flexDirection: "column" }}>
      <div className="cn-login">
        <div className="cn-login-bg" />
        {children}
      </div>
      <ConsoleFooter />
    </div>
  )
}
