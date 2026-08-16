import { useState } from "react"
import { toast } from "sonner"
import { Field, Modal } from "@/components/console/ui"
import { api } from "@/lib/api"
import type { Provider } from "@/lib/types"

// Recording a top-up is reachable from two places (the overview's primary
// action and the procurement page), so the form lives in one component
// rather than being written twice with two slightly different validations.
export function ProcurementModal({
  open,
  providers,
  onClose,
  onDone,
}: {
  open: boolean
  providers: Provider[]
  onClose: () => void
  onDone: () => void
}) {
  const [providerId, setProviderId] = useState("")
  const [amount, setAmount] = useState("")
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    const yuan = Number(amount)
    if (!providerId || !Number.isFinite(yuan) || yuan <= 0) {
      toast.error("请选择供应商并填写正确的金额")
      return
    }
    setBusy(true)
    try {
      await api.post("/api/procurement", {
        ProviderID: providerId,
        AmountCents: Math.round(yuan * 100),
        Note: note,
      })
      toast.success("已登记入库")
      setProviderId("")
      setAmount("")
      setNote("")
      onDone()
      onClose()
    } catch {
      toast.error("登记失败，请确认你有入库登记权限")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      title="登记入库"
      sub="采购充值流水，决定各供应商的可用余额"
      onClose={onClose}
      footer={
        <>
          <button className="cn-btn" onClick={onClose}>
            取消
          </button>
          <button className="cn-btn cn-btn-pri" disabled={busy} onClick={() => void submit()}>
            {busy ? "提交中…" : "确认登记"}
          </button>
        </>
      }
    >
      <div className="cn-form">
        <Field label="供应商" optional="必填">
          <select className="cn-input" value={providerId} onChange={(e) => setProviderId(e.target.value)}>
            <option value="">选择供应商</option>
            {providers.map((p) => (
              <option key={p.ID} value={p.ID}>
                {p.Name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="金额" optional="必填" hint="单位为元，支持两位小数；系统内部按分存储。">
          <input
            className="cn-input"
            inputMode="decimal"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="50000"
          />
        </Field>
        <Field label="备注" hint="写清采购批次或合同号，对账时能省很多事。">
          <textarea
            className="cn-input"
            rows={3}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="季度采购充值"
          />
        </Field>
      </div>
    </Modal>
  )
}
