// Money is integer minor units end to end, on the backend and here --
// see DESIGN.md 12 -- so formatting is just a division, no floating
// point anywhere in the pipeline.
//
// That minor unit is the micro-cent: a cent was too coarse to hold one
// call's cost, which is derived from per-million-token pricing and is
// routinely a fraction of a cent. Rounding it to whole cents billed most
// ordinary traffic as free. MICRO_CENTS_PER_YUAN is what turns the
// stored integer back into the number a person reads.
const MICRO_CENTS_PER_YUAN = 1_000_000

// Two decimals, always: the design fixed this deliberately (see the note
// in designs/fluxa-console-overview/data.jsx). Trimming them turns
// ¥72,140.20 into "¥72,140.2" and the decimal points in a money column
// stop lining up, which is glaring in a table.
export function fmt(microCents: number): string {
  return `¥${(microCents / MICRO_CENTS_PER_YUAN).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

// Compact form for axis labels and dense tiles: ¥18.4万 / ¥6,842.
export function fmtShort(microCents: number): string {
  const yuan = microCents / MICRO_CENTS_PER_YUAN
  if (yuan >= 10000) return `¥${(yuan / 10000).toFixed(1)}万`
  return `¥${Math.round(yuan).toLocaleString("zh-CN")}`
}

// A column has to keep one unit throughout. Deciding it from the column's
// own maximum avoids ¥6.8万 sitting next to ¥7,719, which makes the reader
// convert between two magnitudes to compare two adjacent rows.
export function makeMoneyFmt(maxMicroCents: number): (microCents: number) => string {
  const useWan = maxMicroCents / MICRO_CENTS_PER_YUAN >= 10000
  return (microCents) =>
    useWan
      ? `¥${(microCents / (MICRO_CENTS_PER_YUAN * 10000)).toFixed(1)}万`
      : `¥${Math.round(microCents / MICRO_CENTS_PER_YUAN).toLocaleString("zh-CN")}`
}

// Model pricing is the one money figure that is NOT in micro-cents: it
// is quoted in cents per million tokens, which already carries six
// digits of headroom, so it never needed rescaling and did not get it.
export function fmtPrice(centsPerMillionTokens: number): string {
  return `¥${(centsPerMillionTokens / 100).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

// yuanToMicroCents converts what someone typed into a form into the
// stored unit. Rounding matters: ¥0.07 is 70000 micro-cents exactly, but
// float multiplication lands a hair either side of it.
export function yuanToMicroCents(yuan: number): number {
  return Math.round(yuan * MICRO_CENTS_PER_YUAN)
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
