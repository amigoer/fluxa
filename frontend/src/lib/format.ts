// Money is always integer minor units (fen) end to end, on the backend
// and here -- see DESIGN.md 12 -- so formatting is just a division, no
// floating point anywhere in the pipeline.
//
// Two decimals, always: the design fixed this deliberately (see the note
// in designs/fluxa-console-overview/data.jsx). Trimming them turns
// ¥72,140.20 into "¥72,140.2" and the decimal points in a money column
// stop lining up, which is glaring in a table.
export function fmt(cents: number): string {
  return `¥${(cents / 100).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

// Compact form for axis labels and dense tiles: ¥18.4万 / ¥6,842.
export function fmtShort(cents: number): string {
  const yuan = cents / 100
  if (yuan >= 10000) return `¥${(yuan / 10000).toFixed(1)}万`
  return `¥${Math.round(yuan).toLocaleString("zh-CN")}`
}

// A column has to keep one unit throughout. Deciding it from the column's
// own maximum avoids ¥6.8万 sitting next to ¥7,719, which makes the reader
// convert between two magnitudes to compare two adjacent rows.
export function makeMoneyFmt(maxCents: number): (cents: number) => string {
  const useWan = maxCents / 100 >= 10000
  return (cents) =>
    useWan ? `¥${(cents / 1000000).toFixed(1)}万` : `¥${Math.round(cents / 100).toLocaleString("zh-CN")}`
}

export function fmtNum(n: number): string {
  return n.toLocaleString("zh-CN")
}

// Dashes, not slashes, and always zero-padded: these land in monospace
// tabular columns next to Request IDs, and the design uses the same
// 08-14 / 08-16 14:32 shape throughout.
const pad = (n: number) => String(n).padStart(2, "0")

export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// Prose form for the places the design writes a date into a sentence:
// "预计 8 月 27 日 触顶", "2026 年 8 月 1 日".
export function formatDateCN(iso: string, withYear = false): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const md = `${d.getMonth() + 1} 月 ${d.getDate()} 日`
  return withYear ? `${d.getFullYear()} 年 ${md}` : md
}

// yyyy-MM-dd, for the log tables that show a full date.
export function formatDay(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function formatTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleTimeString("zh-CN", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" })
}

// "18 分钟前" / "昨天" -- the drawer and the request timeline read better
// in elapsed time than in wall-clock time.
export function formatAgo(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return iso
  const mins = Math.floor((Date.now() - then) / 60000)
  if (mins < 1) return "刚刚"
  if (mins < 60) return `${mins} 分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  if (days === 1) return "昨天"
  if (days < 30) return `${days} 天前`
  return formatDate(iso)
}
