import { useEffect, useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { cn } from "@/lib/utils"
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { permissionCatalog } from "@/lib/permission-catalog"
import type { Role } from "@/lib/types"

export function RolesPage() {
  const { data: roles, refetch: refetchRoles } = useApiQuery<Role[]>("/api/roles")
  const [selected, setSelected] = useState<string | null>(null)
  const [granted, setGranted] = useState<Set<string>>(new Set())
  const [loadingPerms, setLoadingPerms] = useState(false)

  const active = selected ?? roles?.[0]?.ID ?? null

  useEffect(() => {
    if (!active) return
    setLoadingPerms(true)
    api
      .get<string[]>(`/api/roles/${active}/permissions`)
      .then((codes) => setGranted(new Set(codes)))
      .finally(() => setLoadingPerms(false))
  }, [active])

  const toggle = (code: string) => {
    setGranted((prev) => {
      const next = new Set(prev)
      if (next.has(code)) next.delete(code)
      else next.add(code)
      return next
    })
  }

  const save = async () => {
    if (!active) return
    try {
      await api.put(`/api/roles/${active}/permissions`, { permissions: Array.from(granted) })
      toast.success("权限已保存")
    } catch {
      toast.error("保存失败")
    }
  }

  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")

  const createRole = async () => {
    try {
      await api.post("/api/roles", { name, permissions: [] })
      setOpen(false)
      setName("")
      refetchRoles()
      toast.success("角色已创建")
    } catch {
      toast.error("创建失败")
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="角色权限"
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>新建角色</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>新建角色</DialogTitle>
              </DialogHeader>
              <div>
                <Label className="mb-1.5 text-xs">角色名称</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：Finance" />
              </div>
              <DialogFooter>
                <Button disabled={!name} onClick={() => void createRole()}>
                  创建
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        }
      />

      <div className="flex flex-col gap-4 lg:flex-row">
        <div className="flex w-full flex-none flex-col gap-0.5 lg:w-[180px]">
          {(roles ?? []).map((r) => (
            <button
              key={r.ID}
              className={cn(
                "rounded-md px-2.5 py-2 text-left text-[12.5px] text-muted-foreground",
                active === r.ID && "bg-accent font-semibold text-accent-foreground",
              )}
              onClick={() => setSelected(r.ID)}
            >
              {r.Name}
              {r.IsBuiltin && <span className="ml-1 text-[10px] text-muted-foreground">内置</span>}
            </button>
          ))}
        </div>

        <div className="flex-1 rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
          {loadingPerms ? (
            <p className="text-xs text-muted-foreground">加载中…</p>
          ) : (
            <>
              {permissionCatalog.map((g) => (
                <div key={g.group} className="mb-4 last:mb-0">
                  <p className="mb-2.5 text-[11px] font-bold uppercase tracking-wide text-muted-foreground">{g.group}</p>
                  <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                    {g.items.map((item) => (
                      <label key={item.code} className="flex items-center gap-2.5 text-[12.5px] text-foreground">
                        <input
                          type="checkbox"
                          className="size-4 rounded border-border accent-primary"
                          checked={granted.has(item.code)}
                          onChange={() => toggle(item.code)}
                        />
                        {item.label}
                      </label>
                    ))}
                  </div>
                </div>
              ))}
              <Button className="mt-2" onClick={() => void save()}>
                保存
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
