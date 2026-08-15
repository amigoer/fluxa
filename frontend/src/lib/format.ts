// Money is always integer minor units (fen) end to end, on the backend
// and here -- see DESIGN.md 12 -- so formatting is just a division, no
// floating point anywhere in the pipeline.
export function formatCents(cents: number): string {
  return `¥${(cents / 100).toLocaleString("zh-CN", { minimumFractionDigits: 0, maximumFractionDigits: 2 })}`
}

export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit" })
}
