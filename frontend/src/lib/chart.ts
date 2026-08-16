// Path helpers for the line / area charts, copied from the hi-fi design
// (designs/fluxa-console-overview/data.jsx). Both return SVG path strings
// in a 0..w by 0..h box, so each chart sizes its own viewBox.

export function linePath(values: number[], w: number, h: number, pad = 0): string {
  const max = Math.max(...values)
  const min = Math.min(...values)
  const span = max - min || 1
  const step = w / (values.length - 1 || 1)
  return values
    .map((v, i) => {
      const x = i * step
      const y = h - pad - ((v - min) / span) * (h - pad * 2)
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(" ")
}

export function areaPath(values: number[], w: number, h: number, pad = 0): string {
  return `${linePath(values, w, h, pad)} L${w} ${h} L0 ${h} Z`
}

// Smooth (Catmull-Rom -> cubic) variant for the softer console look.
export function smoothPath(values: number[], w: number, h: number, pad = 0): string {
  const max = Math.max(...values)
  const min = Math.min(...values)
  const span = max - min || 1
  const step = w / (values.length - 1 || 1)
  const pts = values.map((v, i) => [i * step, h - pad - ((v - min) / span) * (h - pad * 2)])
  if (pts.length === 0) return ""
  let d = `M${pts[0][0].toFixed(1)} ${pts[0][1].toFixed(1)}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] || pts[i]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[i + 2] || p2
    const c1x = p1[0] + (p2[0] - p0[0]) / 6
    const c1y = p1[1] + (p2[1] - p0[1]) / 6
    const c2x = p2[0] - (p3[0] - p1[0]) / 6
    const c2y = p2[1] - (p3[1] - p1[1]) / 6
    d += ` C${c1x.toFixed(1)} ${c1y.toFixed(1)}, ${c2x.toFixed(1)} ${c2y.toFixed(1)}, ${p2[0].toFixed(1)} ${p2[1].toFixed(1)}`
  }
  return d
}

// Same curve, but against a caller-supplied ceiling instead of the
// series' own min/max. Two series drawn on one chart have to share a
// scale, otherwise the shorter one is silently stretched to fill the box
// and the comparison the chart exists for is a lie.
export function smoothPathScaled(values: number[], max: number, w: number, h: number, pad = 0): string {
  if (values.length === 0) return ""
  const step = w / (values.length - 1 || 1)
  const top = max || 1
  const pts = values.map((v, i) => [i * step, h - pad - (v / top) * (h - pad * 2)])
  let d = `M${pts[0][0].toFixed(1)} ${pts[0][1].toFixed(1)}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] || pts[i]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[i + 2] || p2
    const c1x = p1[0] + (p2[0] - p0[0]) / 6
    const c1y = p1[1] + (p2[1] - p0[1]) / 6
    const c2x = p2[0] - (p3[0] - p1[0]) / 6
    const c2y = p2[1] - (p3[1] - p1[1]) / 6
    d += ` C${c1x.toFixed(1)} ${c1y.toFixed(1)}, ${c2x.toFixed(1)} ${c2y.toFixed(1)}, ${p2[0].toFixed(1)} ${p2[1].toFixed(1)}`
  }
  return d
}

// Bucket anything with a timestamp into the last `days` calendar days.
// Every page that draws a trend needs the same walk, and doing it once
// keeps the empty days in (a gap-free x-axis) instead of collapsing them.
export function dailyBuckets<T>(
  rows: T[],
  days: number,
  at: (row: T) => string,
  value: (row: T) => number,
): { day: Date; total: number }[] {
  const out: { day: Date; total: number }[] = []
  const index = new Map<string, number>()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date()
    d.setHours(0, 0, 0, 0)
    d.setDate(d.getDate() - i)
    index.set(d.toDateString(), out.length)
    out.push({ day: d, total: 0 })
  }
  for (const row of rows) {
    const d = new Date(at(row))
    if (Number.isNaN(d.getTime())) continue
    const slot = index.get(d.toDateString())
    if (slot === undefined) continue
    out[slot].total += value(row)
  }
  return out
}
