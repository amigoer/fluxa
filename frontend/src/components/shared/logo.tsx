// Fluxa's one and only mark (DESIGN.md 6.2): a rounded-square badge with
// a geometric "F" built from rectangles, not a grabbed font glyph. Not
// user-replaceable -- there is no logo-upload setting anywhere in the
// product.
export function Logo({ size = 26 }: { size?: number }) {
  return (
    <span
      className="flex flex-none items-center justify-center rounded-[7px] bg-primary"
      style={{ width: size, height: size }}
    >
      <svg viewBox="0 0 24 24" fill="#fff" width={size * 0.58} height={size * 0.58}>
        <rect x="7" y="5" width="2.6" height="14" rx="1" />
        <rect x="7" y="5" width="10" height="2.6" rx="1" />
        <rect x="7" y="11.2" width="7.5" height="2.6" rx="1" />
      </svg>
    </span>
  )
}
