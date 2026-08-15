import { useMemo, useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/shared/page-header"
import { StatusPill } from "@/components/shared/status-pill"
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
import type { Department, Member } from "@/lib/types"

const statusLabel = { active: "在职", pending_review: "待审批", disabled: "已停用" } as const
const statusTone = { active: "ok", pending_review: "warn", disabled: "bad" } as const

export function MembersPage() {
  const { data: departments, refetch: refetchDepartments } = useApiQuery<Department[]>("/api/departments")
  const [selected, setSelected] = useState<string | null>(null)
  const { data: members, refetch: refetchMembers } = useApiQuery<Member[]>(
    selected ? `/api/members?departmentId=${selected}` : "/api/members",
    [selected],
  )

  const [open, setOpen] = useState(false)
  const [deptName, setDeptName] = useState("")

  const counts = useMemo(() => {
    const map = new Map<string, number>()
    for (const m of members ?? []) {
      const key = m.DepartmentID ?? "none"
      map.set(key, (map.get(key) ?? 0) + 1)
    }
    return map
  }, [members])

  const createDepartment = async () => {
    try {
      await api.post("/api/departments", { Name: deptName })
      setOpen(false)
      setDeptName("")
      refetchDepartments()
      toast.success("部门已创建")
    } catch {
      toast.error("创建失败")
    }
  }

  const approve = async (id: string) => {
    await api.post(`/api/members/${id}/approve`)
    refetchMembers()
    toast.success("已通过")
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="成员与部门"
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>添加部门</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>添加部门</DialogTitle>
              </DialogHeader>
              <div>
                <Label className="mb-1.5 text-xs">部门名称</Label>
                <Input value={deptName} onChange={(e) => setDeptName(e.target.value)} />
              </div>
              <DialogFooter>
                <Button disabled={!deptName} onClick={() => void createDepartment()}>
                  创建
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        }
      />

      <div className="flex flex-col gap-4 lg:flex-row">
        <div className="flex w-full flex-none flex-col gap-0.5 lg:w-[180px]">
          <button
            className={cn(
              "flex justify-between rounded-md px-2.5 py-2 text-[12.5px] text-muted-foreground",
              !selected && "bg-accent font-semibold text-accent-foreground",
            )}
            onClick={() => setSelected(null)}
          >
            全部成员
          </button>
          {(departments ?? []).map((d) => (
            <button
              key={d.ID}
              className={cn(
                "flex justify-between rounded-md px-2.5 py-2 text-[12.5px] text-muted-foreground",
                selected === d.ID && "bg-accent font-semibold text-accent-foreground",
              )}
              onClick={() => setSelected(d.ID)}
            >
              {d.Name}
              <span className="text-[11px] text-muted-foreground">{counts.get(d.ID) ?? 0}</span>
            </button>
          ))}
        </div>

        <div className="flex-1 overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
          <table className="w-full text-[11.5px]">
            <thead>
              <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
                <th className="p-3 font-semibold">姓名</th>
                <th className="p-3 font-semibold">邮箱 / 手机号</th>
                <th className="p-3 font-semibold">状态</th>
                <th className="p-3 font-semibold">操作</th>
              </tr>
            </thead>
            <tbody>
              {(members ?? []).map((m) => (
                <tr key={m.ID} className="border-t border-border">
                  <td className="p-3 text-foreground">{m.Name}</td>
                  <td className="p-3 text-muted-foreground">{m.Email ?? m.Phone ?? "—"}</td>
                  <td className="p-3">
                    <StatusPill tone={statusTone[m.Status]}>{statusLabel[m.Status]}</StatusPill>
                  </td>
                  <td className="p-3">
                    {m.Status === "pending_review" ? (
                      <button className="font-semibold text-primary" onClick={() => void approve(m.ID)}>
                        审批
                      </button>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {(members ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无成员</p>}
        </div>
      </div>
    </div>
  )
}
