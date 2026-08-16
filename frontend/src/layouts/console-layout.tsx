import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom"
import { toast } from "sonner"
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
import { adminNav, employeeNav, filterNav, routeLabel } from "@/layouts/nav-config"

const VERSION = "v1.0.0"
const REPO_URL = "https://github.com/amigoer/fluxa"

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
        <SheetHeader className="cn-drawer-head block space-y-0 p-0">
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

// ---- top-bar search ---------------------------------------------------
// The mockup's placeholder promises members, keys and request IDs, so the
// box does exactly those three: it jumps to a page, to a member, to a key,
// or hands the raw string to the call log as a filter.

function useOnOutsideClick(ref: { current: HTMLElement | null }, onOut: () => void, active: boolean) {
  useEffect(() => {
    if (!active) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onOut()
    }
    window.addEventListener("mousedown", onDown)
    return () => window.removeEventListener("mousedown", onDown)
  }, [ref, onOut, active])
}

function OmniSearch({ pages, logs }: { pages: { to: string; label: string }[]; logs: boolean }) {
  const navigate = useNavigate()
  const { permissions } = useAuth()
  const [q, setQ] = useState("")
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  useOnOutsideClick(box, () => setOpen(false), open)

  const wantsMembers = open && permissions.has(Permission.OrgManageMembers)
  const members = useApiQuery<Member[]>(wantsMembers ? "/api/members" : null)
  const keys = useApiQuery<VirtualKey[]>(open ? "/api/virtual-keys" : null)

  const needle = q.trim().toLowerCase()
  const hitPages = needle ? pages.filter((p) => p.label.toLowerCase().includes(needle)).slice(0, 4) : []
  const hitMembers = needle
    ? (members.data ?? [])
        .filter((m) => `${m.Name}${m.Email ?? ""}${m.Phone ?? ""}`.toLowerCase().includes(needle))
        .slice(0, 4)
    : []
  const hitKeys = needle
    ? (keys.data ?? [])
        .filter((k) => `${k.Name}${k.SecretPrefix}`.toLowerCase().includes(needle))
        .slice(0, 4)
    : []

  const go = (to: string) => {
    setOpen(false)
    setQ("")
    navigate(to)
  }

  return (
    <div className="cn-search" ref={box}>
      <Icon name="search" size={14} />
      <input
        value={q}
        placeholder="搜索成员、Key、请求 ID…"
        aria-label="搜索"
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQ(e.target.value)
          setOpen(true)
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") setOpen(false)
          if (e.key === "Enter" && needle && logs) go(`/admin/call-logs?q=${encodeURIComponent(q.trim())}`)
        }}
      />
      {open && needle && (
        <div className="cn-omni">
          {hitPages.length > 0 && <div className="cn-omni-group">页面</div>}
          {hitPages.map((p) => (
            <button key={p.to} className="cn-omni-item" onClick={() => go(p.to)}>
              <Icon name="layout-grid" size={14} />
              <b>{p.label}</b>
            </button>
          ))}

          {hitMembers.length > 0 && <div className="cn-omni-group">成员</div>}
          {hitMembers.map((m) => (
            <button key={m.ID} className="cn-omni-item" onClick={() => go("/admin/members")}>
              <Icon name="users" size={14} />
              <b>{m.Name}</b>
              <span>{m.Email ?? m.Phone ?? ""}</span>
            </button>
          ))}

          {hitKeys.length > 0 && <div className="cn-omni-group">Key</div>}
          {hitKeys.map((k) => (
            <button key={k.ID} className="cn-omni-item" onClick={() => go("/admin/keys")}>
              <Icon name="key" size={14} />
              <b>{k.Name}</b>
              <span>{k.SecretPrefix}</span>
            </button>
          ))}

          {logs && (
            <>
              <div className="cn-omni-group">调用日志</div>
              <button className="cn-omni-item" onClick={() => go(`/admin/call-logs?q=${encodeURIComponent(q.trim())}`)}>
                <Icon name="scroll-text" size={14} />
                <b>按 “{q.trim()}” 查请求</b>
              </button>
            </>
          )}
          {!logs && hitPages.length + hitMembers.length + hitKeys.length === 0 && (
            <div className="cn-omni-empty">没有匹配的结果</div>
          )}
        </div>
      )}
    </div>
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
      <div className="screen cn" data-side={collapsed ? "collapsed" : "expanded"}>
        <div className="cn-side">
          <div className="cn-brand">
            <FluxaLogo size={30} radius={8} bg="var(--brand)" />
            <div className="cn-brand-text">
              <div className="cn-brand-name">Fluxa</div>
              <div className="cn-brand-sub">{orgName || "企业 AI 网关"}</div>
            </div>
            <button
              className="cn-side-toggle"
              onClick={() => setCollapsed((v) => !v)}
              title={collapsed ? "展开侧边栏" : "收起侧边栏"}
              aria-label={collapsed ? "展开侧边栏" : "收起侧边栏"}
            >
              <Icon name={collapsed ? "sidebar-expand" : "sidebar-collapse"} size={17} />
            </button>
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

        <div className="cn-body">
          <div className="cn-top">
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

          <ConsoleFooter orgName={orgName} note="数据延迟 < 1 分钟" />
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

export function ConsoleFooter({ orgName, note }: { orgName?: string; note?: ReactNode }) {
  return (
    <div className="cn-foot">
      <span>Fluxa {VERSION}</span>
      <span className="cn-foot-sep">·</span>
      <span>© {new Date().getFullYear()} {orgName || "Fluxa"}</span>
      <span className="cn-foot-sep">·</span>
      <a href={REPO_URL} target="_blank" rel="noreferrer">
        项目仓库 <Icon name="external-link" size={11} style={{ verticalAlign: -1, display: "inline" }} />
      </a>
      {note && <span style={{ marginLeft: "auto" }}>{note}</span>}
    </div>
  )
}
