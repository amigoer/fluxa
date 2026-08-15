import { useState } from "react"
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
import { useApiQuery } from "@/hooks/use-api-query"
import { api } from "@/lib/api"
import { formatCents, formatDate } from "@/lib/format"
import type { VirtualKey } from "@/lib/types"

export function KeysPage() {
  const { data: keys, refetch } = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [budget, setBudget] = useState("")
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)

  const create = async () => {
    try {
      const res = await api.post<{ secret: string }>("/api/virtual-keys", {
        Name: name,
        OwnerType: "member",
        BudgetCents: Math.round(Number(budget) * 100),
      })
      setCreatedSecret(res.secret)
      setName("")
      setBudget("")
      refetch()
    } catch {
      toast.error("创建失败")
    }
  }

  const revoke = async (id: string) => {
    await api.post(`/api/virtual-keys/${id}/revoke`)
    refetch()
    toast.success("已吊销")
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="Key 管理" />

      <div className="flex items-center gap-2">
        <Input placeholder="搜索 Key 名称…" className="max-w-[260px]" />
        <div className="flex-1" />
        <Dialog
          open={open}
          onOpenChange={(v) => {
            setOpen(v)
            if (!v) setCreatedSecret(null)
          }}
        >
          <DialogTrigger asChild>
            <Button>创建 Key</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>创建 Key</DialogTitle>
            </DialogHeader>
            {createdSecret ? (
              <div className="flex flex-col gap-2.5">
                <p className="text-xs text-warn">请立即复制保存，关闭后将无法再次查看完整 Key</p>
                <code className="break-all rounded-md bg-muted p-3 text-[11.5px]">{createdSecret}</code>
                <Button
                  variant="outline"
                  onClick={() => {
                    void navigator.clipboard.writeText(createdSecret)
                    toast.success("已复制")
                  }}
                >
                  复制
                </Button>
              </div>
            ) : (
              <>
                <div className="flex flex-col gap-3.5">
                  <div>
                    <Label className="mb-1.5 text-xs">名称</Label>
                    <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：客服助手项目" />
                  </div>
                  <div>
                    <Label className="mb-1.5 text-xs">额度（¥/月）</Label>
                    <Input value={budget} onChange={(e) => setBudget(e.target.value)} placeholder="2000" />
                  </div>
                </div>
                <DialogFooter>
                  <Button disabled={!name || !budget} onClick={() => void create()}>
                    创建
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>
      </div>

      <div className="overflow-x-auto rounded-lg border border-border bg-card shadow-[var(--shadow-card)]">
        <table className="w-full text-[11.5px]">
          <thead>
            <tr className="text-left text-[10.5px] font-semibold text-muted-foreground">
              <th className="p-3 font-semibold">名称</th>
              <th className="p-3 font-semibold">归属</th>
              <th className="p-3 font-semibold">额度</th>
              <th className="p-3 font-semibold">状态</th>
              <th className="p-3 font-semibold">创建时间</th>
              <th className="p-3 font-semibold">操作</th>
            </tr>
          </thead>
          <tbody>
            {(keys ?? []).map((k) => (
              <tr key={k.ID} className="border-t border-border">
                <td className="p-3 text-foreground">{k.Name}</td>
                <td className="p-3 text-muted-foreground">{k.OwnerType === "department" ? "部门池" : "个人"}</td>
                <td className="p-3 text-muted-foreground">
                  {formatCents(k.SpentCents)} / {formatCents(k.BudgetCents)}
                </td>
                <td className="p-3">
                  <StatusPill tone={k.Status === "active" ? "ok" : "bad"}>{k.Status === "active" ? "正常" : "已吊销"}</StatusPill>
                </td>
                <td className="p-3 text-muted-foreground">{formatDate(k.CreatedAt)}</td>
                <td className="p-3">
                  {k.Status === "active" ? (
                    <button className="text-muted-foreground" onClick={() => void revoke(k.ID)}>
                      吊销
                    </button>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {(keys ?? []).length === 0 && <p className="p-4 text-xs text-muted-foreground">暂无 Key</p>}
      </div>
    </div>
  )
}
