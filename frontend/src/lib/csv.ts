// Client-side CSV export. Every log and ledger page offers one, and none
// of them has a server-side export endpoint -- the rows are already in
// the browser, so writing the file here is both simpler and honest about
// exporting exactly what the operator is looking at.

function cell(v: unknown): string {
  const s = v === null || v === undefined ? "" : String(v)
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

export function downloadCsv(filename: string, header: string[], rows: unknown[][]) {
  const body = [header.map(cell).join(","), ...rows.map((r) => r.map(cell).join(","))].join("\n")
  // Excel on Windows needs the BOM to read UTF-8, and these files are
  // full of Chinese member and department names.
  const blob = new Blob([`﻿${body}`], { type: "text/csv;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
