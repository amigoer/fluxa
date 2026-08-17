import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react"
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Icon } from "@/components/console/icon"
import { FluxaLogo } from "@/components/console/brand"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { Permission, useAuth } from "@/lib/auth"
import { fmt, formatAgo } from "@/lib/format"
import type { Member, Provider, ProviderHealth, QuotaRequest, SecurityEvent, VirtualKey } from "@/lib/types"
import { GitHubIcon } from "@/components/shared/brand-icons"
import { adminNav, employeeNav, filterNav, routeLabel } from "@/layouts/nav-config"

const REPO_URL = "https://github.com/amigoer/fluxa"
const LICENSE_URL = `${REPO_URL}/blob/main/LICENSE`
const COPYRIGHT_HOLDER = "Amigoer"

// Below this the sidebar stops being a rail beside the content and
// becomes an overlay drawer over it. Kept in sync with the same query in
// console.css -- the CSS positions the drawer, this decides the
// behaviour that CSS cannot express (which button does what, whether the
// stored collapse preference applies at all).
const NARROW = "(max-width: 900px)"

function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches)
  useEffect(() => {
    const mq = window.matchMedia(query)
    const sync = () => setMatches(mq.matches)
    sync()
    mq.addEventListener("change", sync)
    return () => mq.removeEventListener("change", sync)
  }, [query])
  return matches
}

// The shell both personas share: sidebar, top bar, fixed footer and the
// to-do drawer (DESIGN.md 6.3 -- admin and employee are one app, RBAC is
// what differs). Everything here is the hi-fi design's markup; what the
// mockup faked with constants is wired to the real endpoints instead.

export type TodoKind = "熔断" | "预算" | "审批" | "安全"
export const TODO_KINDS: TodoKind[] = ["熔断", "预算", "审批", "安全"]

export interface Todo {
  id: string
  kind: TodoKind
  sev: "critical" | "warn" | "info"
  icon: string
  title: string
  time: string
  desc: string
  acts: { label: string; pri?: boolean; run: () => void }[]
}

const SEV_TONE = { critical: "bad", warn: "warn", info: "brand" } as const

interface ConsoleState {
  todos: Todo[]
  countOf: (kind: TodoKind) => number
  worstOf: (kind: TodoKind) => "bad" | "warn"
  openDrawer: (kind?: TodoKind | "全部") => void
  reloadTodos: () => void
}

const ConsoleContext = createContext<ConsoleState | null>(null)

export function useConsole(): ConsoleState {
  const ctx = useContext(ConsoleContext)
  if (!ctx) throw new Error("useConsole must be used inside ConsoleLayout")
  return ctx
}

// ---- to-do queue ------------------------------------------------------
// Four sources, one per kind, each of them something a person has to act
// on: a tripped circuit, a key about to run dry, a request waiting on a
// decision, a rule that fired. Anything that is merely informational
// stays out -- the drawer's whole value is that its contents are work.
function useTodos(enabled: boolean) {
  const navigate = useNavigate()
  const { permissions } = useAuth()
  const can = (p: string) => enabled && permissions.has(p)

  const health = useApiQuery<ProviderHealth[]>(can(Permission.ProviderView) ? "/api/provider-health" : null)
  const providers = useApiQuery<Provider[]>(can(Permission.ProviderView) ? "/api/providers" : null)
  const pending = useApiQuery<QuotaRequest[]>(enabled ? "/api/quota-requests/pending" : null)
  const keys = useApiQuery<VirtualKey[]>(can(Permission.OrgManageKeys) ? "/api/virtual-keys" : null)
  const events = useApiQuery<SecurityEvent[]>(can(Permission.SecurityViewEvents) ? "/api/security-events" : null)
  const members = useApiQuery<Member[]>(can(Permission.OrgManageMembers) ? "/api/members" : null)

  const decide = async (id: string, approve: boolean) => {
    try {
      await api.post(`/api/quota-requests/${id}/decide`, { approve })
      toast.success(approve ? "已通过该申请" : "已驳回该申请")
      pending.refetch()
    } catch {
      toast.error("处理失败，请确认你有审批权限")
    }
  }

  const todos = useMemo<Todo[]>(() => {
    const out: Todo[] = []
    const providerName = new Map((providers.data ?? []).map((p) => [p.ID, p.Name]))
    const memberName = new Map((members.data ?? []).map((m) => [m.ID, m.Name]))

    for (const h of health.data ?? []) {
      if (h.State === "normal") continue
      const name = providerName.get(h.ProviderID) ?? h.ProviderID
      const open = h.State === "circuit_open"
      out.push({
        id: `h-${h.ProviderID}`,
        kind: "熔断",
        sev: open ? "critical" : "warn",
        icon: open ? "alert-triangle" : "gauge",
        title: open ? `${name} 已熔断` : `${name} 进入半开`,
        time: `连续失败 ${h.ConsecutiveFailures} 次`,
        desc: open
          ? "连续失败触发熔断，流量已按路由规则回退到备用模型。"
          : "熔断器正在试探性放量，恢复前失败仍会重新熔断。",
        acts: [{ label: "查看日志", pri: true, run: () => navigate("/admin/call-logs") }],
      })
    }

    for (const k of keys.data ?? []) {
      if (k.Status !== "active" || k.BudgetCents <= 0) continue
      const rate = (k.SpentCents / k.BudgetCents) * 100
      if (rate < 90) continue
      out.push({
        id: `k-${k.ID}`,
        kind: "预算",
        sev: rate >= 100 ? "critical" : "warn",
        icon: "wallet",
        title: rate >= 100 ? `${k.Name} 额度已用尽` : `${k.Name} 额度将耗尽`,
        time: `${rate.toFixed(0)}%`,
        desc: `已用 ${fmt(k.SpentCents)} / ${fmt(k.BudgetCents)}，超出后该 Key 的调用会被直接拒绝。`,
        acts: [{ label: "查看明细", pri: true, run: () => navigate("/admin/keys") }],
      })
    }

    for (const q of pending.data ?? []) {
      const who = memberName.get(q.RequestedByMemberID) ?? "成员"
      const waited = Date.now() - new Date(q.CreatedAt).getTime()
      const stale = waited > 3 * 24 * 3600 * 1000
      out.push({
        id: `q-${q.ID}`,
        kind: "审批",
        sev: stale ? "warn" : "info",
        icon: "clock",
        title: `${who} 申请 ${fmt(q.AmountCents)}${stale ? " 已超时" : ""}`,
        time: formatAgo(q.CreatedAt),
        desc: q.Reason || "未填写事由。",
        acts: [
          { label: "驳回", run: () => void decide(q.ID, false) },
          { label: "通过", pri: true, run: () => void decide(q.ID, true) },
        ],
      })
    }

    const dayAgo = Date.now() - 24 * 3600 * 1000
    const blocked = (events.data ?? []).filter(
      (e) => e.ActionTaken === "block" && new Date(e.OccurredAt).getTime() > dayAgo,
    )
    if (blocked.length > 0) {
      out.push({
        id: "sec-blocked",
        kind: "安全",
        sev: "warn",
        icon: "shield-alert",
        title: `DLP 规则今日拦截 ${blocked.length} 次`,
        time: formatAgo(blocked[0].OccurredAt),
        desc: blocked[0].Description || "命中拦截类规则，请求已被终止。",
        acts: [{ label: "查看事件", pri: true, run: () => navigate("/admin/security-events") }],
      })
    }

    return out
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [health.data, providers.data, pending.data, keys.data, events.data, members.data])

  const reload = () => {
    health.refetch()
    pending.refetch()
    keys.refetch()
    events.refetch()
  }

  const securityBadge = useMemo(() => {
    const dayAgo = Date.now() - 24 * 3600 * 1000
    return (events.data ?? []).filter((e) => new Date(e.OccurredAt).getTime() > dayAgo).length
  }, [events.data])

  return { todos, reload, securityBadge }
}

// ---- drawer -----------------------------------------------------------

function TodoDrawer({
  open,
  todos,
  filter,
  setFilter,
  onClose,
}: {
  open: boolean
  todos: Todo[]
  filter: TodoKind | "全部"
  setFilter: (k: TodoKind | "全部") => void
  onClose: () => void
}) {
  const list = filter === "全部" ? todos : todos.filter((t) => t.kind === filter)
  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      {/* .cn-scrim / .cn-drawer, restated as utilities -- those rules sit
          in the `fluxa` layer and lose to the primitive's own classes. */}
      <SheetContent
        side="right"
        showCloseButton={false}
        overlayClassName="z-[40] bg-[rgba(16,24,40,.30)]"
        aria-label="待处理事项"
        className={
          "z-[41] flex w-[424px] max-w-full flex-col gap-0 border-l border-[var(--line)] " +
          "bg-[var(--panel)] p-0 shadow-[-14px_0_40px_-18px_rgba(22,35,58,.34)]"
        }
      >
        {/* SheetHeader ships `flex flex-col gap-1.5 p-4`, and Tailwind's
            utilities layer sits after the `fluxa` layer -- so .cn-drawer-head
            could never win, and the title, the count and the close button
            stacked down the top-left corner with no padding. The row is
            restated as utilities, the way the other primitives here do it. */}
        <SheetHeader className="cn-drawer-head flex-row items-center gap-[9px] space-y-0 px-4 pt-[15px] pb-[13px]">
          <SheetTitle className="cn-drawer-title text-inherit font-inherit">需要你处理</SheetTitle>
          <SheetDescription className="sr-only">熔断、预算、审批与安全事件的待办队列</SheetDescription>
          <span className="cn-rail-count">{todos.length}</span>
          <SheetClose className="cn-drawer-close" aria-label="关闭">
            <Icon name="x" size={16} />
          </SheetClose>
        </SheetHeader>

        <div className="cn-drawer-filters">
          <button className="cn-chip" data-on={filter === "全部"} onClick={() => setFilter("全部")}>
            全部<span>{todos.length}</span>
          </button>
          {TODO_KINDS.map((k) => (
            <button key={k} className="cn-chip" data-on={filter === k} onClick={() => setFilter(k)}>
              {k}<span>{todos.filter((t) => t.kind === k).length}</span>
            </button>
          ))}
        </div>

        <div className="cn-drawer-list">
          {list.length === 0 ? (
            <div className="cn-drawer-empty">
              <b>没有待处理事项</b>这一类目前是干净的。
            </div>
          ) : (
            <div className="cn-todo">
              {list.map((t) => (
                <div key={t.id} className="cn-todo-item">
                  <div className="cn-todo-top">
                    <span className="cn-todo-ico" data-t={SEV_TONE[t.sev]}>
                      <Icon name={t.icon} size={14} />
                    </span>
                    <span className="cn-todo-title">{t.title}</span>
                    <span className="cn-todo-time">{t.time}</span>
                  </div>
                  <p className="cn-todo-desc">{t.desc}</p>
                  <div className="cn-todo-acts">
                    {t.acts.map((a) => (
                      <button
                        key={a.label}
                        className={a.pri ? "cn-mini-btn cn-mini-pri" : "cn-mini-btn"}
                        onClick={() => {
                          a.run()
                          if (!a.pri) return
                          onClose()
                        }}
                      >
                        {a.label}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ---- command palette --------------------------------------------------
// The mockup's placeholder promises members, keys and request IDs, so the
// palette does exactly those three: it jumps to a page, to a member, to a
// key, or hands the raw string to the call log as a filter.
//
// It used to be a 260px field in the top bar with a panel hung underneath.
// A centred dialog serves the same job better: the field was the first
// thing to lose room as the top bar narrowed (on a phone it had to be
// hidden outright), an absolutely-positioned panel had nowhere to go at
// that width, and an empty field could only say "type something" -- the
// palette opens on the nav itself, so it is useful before the first
// keystroke. The top bar keeps the same 260x34 box as the trigger.

interface Hit {
  id: string
  icon: string
  label: string
  hint?: string
  to: string
}

function OmniSearch({ pages, logs }: { pages: { to: string; label: string }[]; logs: boolean }) {
  const navigate = useNavigate()
  const { permissions } = useAuth()
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState("")
  const [active, setActive] = useState(0)
  const list = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        setOpen((v) => !v)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  // Neither list is worth fetching until the palette is actually open.
  const members = useApiQuery<Member[]>(
    open && permissions.has(Permission.OrgManageMembers) ? "/api/members" : null,
  )
  const keys = useApiQuery<VirtualKey[]>(open ? "/api/virtual-keys" : null)

  const needle = q.trim().toLowerCase()

  const groups = useMemo(() => {
    const out: { label: string; items: Hit[] }[] = []

    // With an empty box the palette lists the nav itself, so it opens on
    // something useful instead of an instruction to type.
    const hitPages = needle ? pages.filter((p) => p.label.toLowerCase().includes(needle)) : pages
    if (hitPages.length > 0) {
      out.push({
        label: needle ? "页面" : "快速跳转",
        items: hitPages.slice(0, needle ? 5 : 8).map((p) => ({
          id: `page:${p.to}`,
          icon: "layout-grid",
          label: p.label,
          to: p.to,
        })),
      })
    }

    if (!needle) return out

    const hitMembers = (members.data ?? [])
      .filter((m) => `${m.Name}${m.Email ?? ""}${m.Phone ?? ""}`.toLowerCase().includes(needle))
      .slice(0, 5)
    if (hitMembers.length > 0) {
      out.push({
        label: "成员",
        items: hitMembers.map((m) => ({
          id: `member:${m.ID}`,
          icon: "users",
          label: m.Name,
          hint: m.Email ?? m.Phone ?? undefined,
          to: "/admin/members",
        })),
      })
    }

    const hitKeys = (keys.data ?? [])
      .filter((k) => `${k.Name}${k.SecretPrefix}`.toLowerCase().includes(needle))
      .slice(0, 5)
    if (hitKeys.length > 0) {
      out.push({
        label: "Key",
        items: hitKeys.map((k) => ({
          id: `key:${k.ID}`,
          icon: "key",
          label: k.Name,
          hint: k.SecretPrefix,
          to: "/admin/keys",
        })),
      })
    }

    // Always last: a request id matches nothing above, and this is what
    // the placeholder promises it can do with one.
    if (logs) {
      out.push({
        label: "调用日志",
        items: [
          {
            id: "logs",
            icon: "scroll-text",
            label: `按 “${q.trim()}” 查请求`,
            to: `/admin/call-logs?q=${encodeURIComponent(q.trim())}`,
          },
        ],
      })
    }

    return out
  }, [needle, q, pages, members.data, keys.data, logs])

  const flat = useMemo(() => groups.flatMap((g) => g.items), [groups])

  useEffect(() => setActive(0), [needle])

  // Keep the highlighted row in view while the keyboard drives it.
  useEffect(() => {
    list.current?.querySelector<HTMLElement>('[data-active="true"]')?.scrollIntoView({ block: "nearest" })
  }, [active])

  const go = (to: string) => {
    setOpen(false)
    navigate(to)
  }

  const onKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (flat.length === 0) return
    if (e.key === "ArrowDown") {
      e.preventDefault()
      setActive((i) => (i + 1) % flat.length)
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setActive((i) => (i - 1 + flat.length) % flat.length)
    } else if (e.key === "Enter") {
      e.preventDefault()
      go(flat[active].to)
    }
  }

  return (
    <>
      <button className="cn-search" onClick={() => setOpen(true)} aria-label="搜索">
        <Icon name="search" size={14} />
        <span className="cn-search-ph">搜索成员、Key、请求 ID…</span>
        <kbd className="cn-kbd">⌘K</kbd>
      </button>

      <Dialog
        open={open}
        onOpenChange={(v) => {
          setOpen(v)
          if (v) setQ("")
        }}
      >
        <DialogPortal>
          <DialogOverlay className="z-[60] bg-[rgba(16,24,40,.34)]" />
          <DialogContent
            showCloseButton={false}
            className={[
              // Sits high rather than dead-centre: the list grows downward,
              // and a vertically centred palette jumps as results arrive.
              "top-[12%] translate-y-0 z-[60] flex flex-col gap-0 overflow-hidden p-0",
              "w-[calc(100%-2rem)] sm:max-w-[560px]",
              "rounded-[14px] border-[var(--line)] bg-[var(--panel)]",
              "shadow-[0_1px_2px_rgba(22,35,58,.05),0_26px_60px_-26px_rgba(22,35,58,.42)]",
            ].join(" ")}
          >
            <DialogTitle className="sr-only">搜索</DialogTitle>
            <DialogDescription className="sr-only">搜索页面、成员、Key 与请求 ID</DialogDescription>

            <div className="cn-omni-field">
              <Icon name="search" size={16} />
              <input
                autoFocus
                value={q}
                onChange={(e) => setQ(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder="搜索页面、成员、Key、请求 ID…"
                aria-label="搜索"
              />
            </div>

            <div className="cn-omni-list" ref={list}>
              {flat.length === 0 ? (
                <div className="cn-omni-empty">没有匹配的结果</div>
              ) : (
                groups.map((g) => (
                  <div key={g.label}>
                    <div className="cn-omni-group">{g.label}</div>
                    {g.items.map((it) => {
                      const i = flat.indexOf(it)
                      return (
                        <button
                          key={it.id}
                          className="cn-omni-item"
                          data-active={i === active}
                          onMouseMove={() => setActive(i)}
                          onClick={() => go(it.to)}
                        >
                          <Icon name={it.icon} size={14} />
                          <b>{it.label}</b>
                          {it.hint && <span>{it.hint}</span>}
                        </button>
                      )
                    })}
                  </div>
                ))
              )}
            </div>

            <div className="cn-omni-foot">
              <kbd>↑</kbd>
              <kbd>↓</kbd>
              <span>选择</span>
              <kbd>↵</kbd>
              <span>打开</span>
              <kbd>esc</kbd>
              <span>关闭</span>
            </div>
          </DialogContent>
        </DialogPortal>
      </Dialog>
    </>
  )
}

// ---- identity menu ----------------------------------------------------

// .cn-menu / .cn-menu-item, restated as utilities: those rules live in the
// `fluxa` layer and would lose to the primitive's own classes.
const MENU =
  "min-w-[208px] rounded-[10px] border-[var(--line)] bg-[var(--panel)] p-[5px] " +
  "shadow-[0_1px_2px_rgba(22,35,58,.05),0_18px_40px_-18px_rgba(22,35,58,.34)]"
const MENU_ITEM =
  "gap-[9px] rounded-[7px] px-[9px] py-[7px] text-[12.5px] text-[var(--ink-2)] " +
  "focus:bg-[#f5f7fb] focus:text-[var(--ink)] [&_svg:not([class*='size-'])]:size-auto"
const MENU_DANGER =
  MENU_ITEM + " focus:bg-[var(--bad-soft)]! focus:text-[var(--bad)]! data-[variant=destructive]:text-[var(--ink-2)]"

// Account actions only. There is deliberately no persona switch: every
// employee-side page has a strictly more capable admin equivalent (概览 /
// 调用日志 for 我的用量, 模型与路由 for 资费, 全局路由规则 for 我的路由,
// and an admin adjusts any member's quota rather than requesting one), so
// "switch to employee self-service" moved an admin to a weaker view of
// their own data -- and, since the employee shell offered no way back,
// stranded them there.
//
// It sits at the foot of the sidebar for both personas. Admins used to
// get a chip in the top bar instead, which put one control in two places
// depending on who was looking and spent top-bar room on something that
// is consulted rarely and acted on almost never.
function IdentityMenu() {
  const navigate = useNavigate()
  const { member, roleName, departmentName, logout } = useAuth()
  const [open, setOpen] = useState(false)

  const initial = member?.Name?.slice(0, 1) ?? "?"

  const signOut = async () => {
    await logout()
    navigate("/login", { replace: true })
  }

  // .cn-menu on Radix. The outside-click hook and the manual `open` state
  // for dismissal are gone: the primitive brings keyboard navigation,
  // focus return and the layer stack the hand-rolled version lacked.
  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <button className="cn-side-foot" aria-label="账号">
          <span className="cn-av">{initial}</span>
          <div className="cn-side-foot-text">
            <div className="cn-side-foot-name">{member?.Name}</div>
            <div className="cn-side-foot-role">{[roleName, departmentName].filter(Boolean).join(" · ")}</div>
          </div>
          <Icon name="chevron-up-down" size={14} className="cn-side-foot-caret" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" side="top" sideOffset={6} className={MENU}>
        <div className="cn-menu-head">
          <div className="cn-menu-name">{member?.Name}</div>
          <div className="cn-menu-sub">{[roleName, departmentName].filter(Boolean).join(" · ")}</div>
        </div>
        <DropdownMenuItem className={MENU_DANGER} onSelect={() => void signOut()}>
          <Icon name="log-out" size={14} />
          退出登录
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ---- shell ------------------------------------------------------------

export function ConsoleLayout({ persona }: { persona: "admin" | "employee" }) {
  const location = useLocation()
  const { permissions, orgName } = useAuth()
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem("fluxa-side") === "collapsed")
  const [drawer, setDrawer] = useState(false)
  const [filter, setFilter] = useState<TodoKind | "全部">("全部")
  const narrow = useMediaQuery(NARROW)
  const [navOpen, setNavOpen] = useState(false)

  const isAdmin = persona === "admin"
  const { todos, reload, securityBadge } = useTodos(isAdmin)
  const myPending = useApiQuery<QuotaRequest[]>(!isAdmin ? "/api/quota-requests/mine" : null)

  useEffect(() => {
    localStorage.setItem("fluxa-side", collapsed ? "collapsed" : "expanded")
  }, [collapsed])

  useEffect(() => {
    if (!drawer) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setDrawer(false)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [drawer])

  // Tapping a nav item on a phone should navigate *and* get the drawer
  // out of the way; it covers the content it just navigated to.
  useEffect(() => setNavOpen(false), [location.pathname])

  useEffect(() => {
    if (!navOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setNavOpen(false)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [navOpen])

  const nav = useMemo(
    () => filterNav(isAdmin ? adminNav : employeeNav, permissions),
    [isAdmin, permissions],
  )
  const pages = useMemo(() => nav.flatMap((g) => g.items.map((i) => ({ to: i.to, label: i.label }))), [nav])

  const root = isAdmin ? "管理台" : "我的工作台"
  const current = routeLabel[location.pathname] ?? ""
  const quotaBadge = (myPending.data ?? []).filter((q) => q.Status === "pending").length
  const badgeOf = (kind?: string) =>
    kind === "security" ? securityBadge : kind === "quota" ? quotaBadge : 0

  const ctx: ConsoleState = {
    todos,
    countOf: (kind) => todos.filter((t) => t.kind === kind).length,
    worstOf: (kind) => (todos.some((t) => t.kind === kind && t.sev === "critical") ? "bad" : "warn"),
    openDrawer: (kind = "全部") => {
      setFilter(kind)
      setDrawer(true)
    },
    reloadTodos: reload,
  }

  return (
    <ConsoleContext.Provider value={ctx}>
      <div
        className="screen cn"
        // The collapsed rail is a desktop affordance only: on a phone the
        // sidebar is an overlay that is either open or absent, and a 60px
        // icon rail overlaying the content would be the worst of both.
        // The preference is kept, just not applied while narrow.
        data-side={!narrow && collapsed ? "collapsed" : "expanded"}
        data-nav={navOpen ? "open" : "closed"}
      >
        <div className="cn-side">
          <div className="cn-brand">
            <FluxaLogo size={30} radius={8} bg="var(--brand)" />
            <div className="cn-brand-text">
              <div className="cn-brand-name">Fluxa</div>
              <div className="cn-brand-sub">{orgName || "企业 AI 网关"}</div>
            </div>
            {/* Only the drawer needs a close button of its own; collapsing
                is driven from the top bar, where the control stays put
                instead of moving with the rail it resizes. */}
            {narrow && (
              <button
                className="cn-side-toggle"
                onClick={() => setNavOpen(false)}
                title="关闭导航"
                aria-label="关闭导航"
              >
                <Icon name="x" size={17} />
              </button>
            )}
          </div>

          <div className="cn-nav">
            {nav.map((g, gi) => (
              <div key={g.label ?? gi}>
                {g.label && <div className="cn-group-label">{g.label}</div>}
                <div className="cn-items">
                  {g.items.map((it) => {
                    const count = badgeOf(it.badge)
                    return (
                      <Link
                        key={it.to}
                        to={it.to}
                        className="cn-item"
                        data-on={location.pathname === it.to}
                        data-label={it.label}
                      >
                        <Icon name={it.icon} size={17} />
                        <span className="cn-item-label">{it.label}</span>
                        {count > 0 && <span className="cn-badge">{count}</span>}
                      </Link>
                    )
                  })}
                </div>
              </div>
            ))}
          </div>

          <IdentityMenu />
        </div>

        {narrow && navOpen && (
          <button className="cn-nav-scrim" aria-label="关闭导航" onClick={() => setNavOpen(false)} />
        )}

        <div className="cn-body">
          <div className="cn-top">
            {/* One control, two jobs: on a desktop it collapses the rail
                beside it, on a phone it opens the rail as a drawer over the
                content. Both live in the top bar so the button does not
                move (or vanish) with the thing it operates on. */}
            <button
              className="cn-nav-toggle"
              onClick={() => (narrow ? setNavOpen(true) : setCollapsed((v) => !v))}
              title={narrow ? "打开导航" : collapsed ? "展开侧边栏" : "收起侧边栏"}
              aria-label={narrow ? "打开导航" : collapsed ? "展开侧边栏" : "收起侧边栏"}
              aria-expanded={narrow ? navOpen : !collapsed}
            >
              <Icon name={narrow ? "menu" : collapsed ? "sidebar-expand" : "sidebar-collapse"} size={18} />
            </button>
            <div className="cn-crumb">
              {root} <Icon name="chevron-right" size={12} /> <b>{current}</b>
            </div>
            <OmniSearch pages={pages} logs={permissions.has(Permission.AuditViewCallLogs)} />
            <div className="cn-top-right">
              {isAdmin && (
                <button
                  className="cn-ghost"
                  onClick={() => {
                    setFilter("全部")
                    setDrawer(true)
                  }}
                  aria-label="待处理事项"
                >
                  <Icon name="bell" size={17} />
                  {todos.length > 0 && <span className="cn-bell-badge">{todos.length}</span>}
                </button>
              )}
            </div>
          </div>

          <div className="cn-scroll">
            <Outlet />
          </div>

          <ConsoleFooter />
        </div>

        <TodoDrawer
          open={drawer}
          todos={todos}
          filter={filter}
          setFilter={setFilter}
          onClose={() => setDrawer(false)}
        />
      </div>
    </ConsoleContext.Provider>
  )
}

// What a self-hosted deployment's footer is actually for: which build is
// running (the first thing a bug report needs), who holds the copyright,
// and where the source and its licence are. It carries nothing that
// belongs to a page -- the "数据延迟 < 1 分钟" note that used to sit here
// was rendered over 成员与部门 and 操作审计 too, where there is no
// aggregate to be stale, and nothing measured it in the first place.
export function ConsoleFooter() {
  return (
    <footer className="cn-foot">
      <div className="cn-foot-group">
        <span className="cn-foot-brand">Fluxa</span>
        <span className="cn-foot-ver">{__APP_VERSION__}</span>
        <span className="cn-foot-sep">·</span>
        <span>
          &copy; {new Date().getFullYear()} {COPYRIGHT_HOLDER}
        </span>
      </div>

      <div className="cn-foot-group cn-foot-links">
        <a href={LICENSE_URL} target="_blank" rel="noreferrer">
          MIT License
        </a>
        {/* The mark already says the link leaves for GitHub, so the
            generic external-link arrow that used to follow it is gone --
            two glyphs for one link in an 11.5px bar is a lot of chrome
            for one destination. */}
        <a className="cn-foot-repo" href={REPO_URL} target="_blank" rel="noreferrer">
          <GitHubIcon style={{ width: 13, height: 13 }} />
          项目仓库
        </a>
      </div>
    </footer>
  )
}
