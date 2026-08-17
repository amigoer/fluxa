import type { CSSProperties } from "react"

// One member's picture, wherever a member is shown.
//
// The letter tile is not a fallback branch -- it is always rendered, and
// the photo is laid over it. That way a URL that fails, or one that
// simply never resolves (a CDN unreachable from inside the customer's
// network, which is the likely failure for a self-hosted deployment),
// leaves the letter showing rather than a blank circle. An onError
// handler would only have covered the first of those two.
export function Avatar({
  name,
  src,
  size = 28,
  style,
}: {
  name: string
  src?: string | null
  size?: number
  style?: CSSProperties
}) {
  return (
    <span
      className="cn-av"
      style={{ width: size, height: size, fontSize: Math.round(size * 0.43), ...style }}
      title={name}
    >
      {name.slice(0, 1)}
      {src && <img className="cn-av-img" src={src} alt="" referrerPolicy="no-referrer" />}
    </span>
  )
}
