import type { CSSProperties, ReactElement } from "react"

// The console's stroke icon set, copied path-for-path from the hi-fi
// design (designs/fluxa-console-overview/icons.jsx): 24x24, 1.5px stroke,
// round cap/join, no fills. Shipping the design's own geometry rather
// than swapping in a look-alike from an icon package is what keeps the
// running app and the mockup pixel-identical.
const ICON_PATHS: Record<string, ReactElement> = {
  "layout-grid": (
    <g>
      <rect x="3" y="3" width="7" height="7" rx="1.2" />
      <rect x="14" y="3" width="7" height="7" rx="1.2" />
      <rect x="14" y="14" width="7" height="7" rx="1.2" />
      <rect x="3" y="14" width="7" height="7" rx="1.2" />
    </g>
  ),
  rocket: (
    <g>
      <path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91 0z" />
      <path d="M12 15l-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" />
      <path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0" />
      <path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5" />
    </g>
  ),
  server: (
    <g>
      <rect x="2" y="3" width="20" height="7" rx="2" />
      <rect x="2" y="14" width="20" height="7" rx="2" />
      <path d="M6 6.5h.01M6 17.5h.01" />
    </g>
  ),
  waypoints: (
    <g>
      <circle cx="12" cy="4.5" r="2.4" />
      <circle cx="4.8" cy="19.4" r="2.4" />
      <circle cx="19.2" cy="19.4" r="2.4" />
      <path d="M10.4 6.6 6.4 17.2M13.6 6.6l4 10.6M7.2 19.4h9.6" />
    </g>
  ),
  flask: (
    <g>
      <path d="M10 2.5v6.9L4.4 19.2A1.5 1.5 0 0 0 5.7 21.5h12.6a1.5 1.5 0 0 0 1.3-2.3L14 9.4V2.5" />
      <path d="M8.4 2.5h7.2M6.8 15h10.4" />
    </g>
  ),
  "package-plus": (
    <g>
      <path d="M16.5 18.5h5M19 16v5" />
      <path d="M20.5 11V7.6a1 1 0 0 0-.5-.87l-7.5-4.2a1 1 0 0 0-1 0l-7.5 4.2a1 1 0 0 0-.5.87v8.8a1 1 0 0 0 .5.87l7.5 4.2a1 1 0 0 0 1 0l.6-.34" />
      <path d="m4 7.2 8 4.5 8-4.5M12 21.3v-9.6" />
    </g>
  ),
  users: (
    <g>
      <path d="M15.5 21v-1.8a3.8 3.8 0 0 0-3.8-3.8H6.3a3.8 3.8 0 0 0-3.8 3.8V21" />
      <circle cx="9" cy="7.2" r="3.7" />
      <path d="M21.5 21v-1.8a3.8 3.8 0 0 0-2.9-3.68M15.8 3.7a3.8 3.8 0 0 1 0 7.1" />
    </g>
  ),
  "shield-check": (
    <g>
      <path d="M19.5 12.6c0 4.8-3.35 7.2-7.14 8.55a1 1 0 0 1-.72 0C7.85 19.8 4.5 17.4 4.5 12.6V6.2a1 1 0 0 1 1-1c1.9 0 4.3-1.14 5.95-2.58a1.14 1.14 0 0 1 1.1 0C14.2 4.06 16.6 5.2 18.5 5.2a1 1 0 0 1 1 1z" />
      <path d="m9.4 12.2 2 2 3.6-3.8" />
    </g>
  ),
  key: (
    <g>
      <path d="M2.9 17.3a2 2 0 0 0-.4 1.2v2a.9.9 0 0 0 .9.9h2.7a.9.9 0 0 0 .9-.9v-.9a.9.9 0 0 1 .9-.9h.9a.9.9 0 0 0 .9-.9v-.9a.9.9 0 0 1 .9-.9h.2a2 2 0 0 0 1.35-.55l.74-.74a6.2 6.2 0 1 0-3.8-3.8z" />
      <circle cx="16.4" cy="7.6" r=".9" />
    </g>
  ),
  fingerprint: (
    <g>
      <path d="M12 10.2a2 2 0 0 0-2 2c0 1-.1 2.5-.26 4" />
      <path d="M13.9 13.1c0 2.4 0 6.4-1 8.9" />
      <path d="M17.2 21c.12-.6.43-2.3.5-3" />
      <path d="M2.6 12a9.5 9.5 0 0 1 17-5.7" />
      <path d="M21.6 16c.2-2 .13-5.35 0-6" />
      <path d="M5.3 19.4C5.8 18 6.3 15 6.3 12c0-.7.12-1.37.34-2" />
      <path d="M8.8 21.9c.2-.66.44-1.32.56-2" />
      <path d="M9.2 6.9a5.9 5.9 0 0 1 8.8 5.1V14" />
    </g>
  ),
  mail: (
    <g>
      <rect x="2.5" y="4.5" width="19" height="15" rx="2" />
      <path d="m21 7.5-8.45 5.4a1.9 1.9 0 0 1-2.1 0L3 7.5" />
    </g>
  ),
  "shield-alert": (
    <g>
      <path d="M19.5 12.6c0 4.8-3.35 7.2-7.14 8.55a1 1 0 0 1-.72 0C7.85 19.8 4.5 17.4 4.5 12.6V6.2a1 1 0 0 1 1-1c1.9 0 4.3-1.14 5.95-2.58a1.14 1.14 0 0 1 1.1 0C14.2 4.06 16.6 5.2 18.5 5.2a1 1 0 0 1 1 1z" />
      <path d="M12 8.4v3.8M12 15.6h.01" />
    </g>
  ),
  "alert-triangle": (
    <g>
      <path d="m21.2 18.1-8-13.9a1.9 1.9 0 0 0-3.3 0l-8 13.9A1.9 1.9 0 0 0 3.5 21h17a1.9 1.9 0 0 0 1.7-2.9" />
      <path d="M12 9.4v4M12 17.2h.01" />
    </g>
  ),
  "scroll-text": (
    <g>
      <path d="M14.5 11.5H10M14.5 8H10" />
      <path d="M18.5 16.5V5a2 2 0 0 0-2-2H4.5" />
      <path d="M8 21h11a2 2 0 0 0 2-2v-.8a.9.9 0 0 0-.9-.9H9.9a.9.9 0 0 0-.9.9v.8a2 2 0 1 1-4 0V5a2 2 0 1 0-4 0v1.9a.9.9 0 0 0 .9.9H5" />
    </g>
  ),
  "clipboard-list": (
    <g>
      <rect x="8.5" y="2.5" width="7" height="4" rx="1.2" />
      <path d="M15.5 4.5h2a2 2 0 0 1 2 2v13a2 2 0 0 1-2 2h-11a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2h2" />
      <path d="M12 11h4M12 15.5h4M8.4 11h.01M8.4 15.5h.01" />
    </g>
  ),
  bell: (
    <g>
      <path d="M10.3 20.5a2 2 0 0 0 3.4 0" />
      <path d="M3.7 15.2A1 1 0 0 0 4.4 17h15.2a1 1 0 0 0 .7-1.75C19 13.9 17.7 12.5 17.7 8.4a5.7 5.7 0 0 0-11.4 0c0 4.1-1.3 5.5-2.6 6.8" />
    </g>
  ),
  "chevron-down": <path d="m6.5 9.5 5.5 5.5 5.5-5.5" />,
  "chevron-right": <path d="m9.5 5.5 6 6.5-6 6.5" />,
  "chevron-up-down": <path d="m8 10 4-4 4 4M8 14l4 4 4-4" />,
  search: (
    <g>
      <circle cx="11" cy="11" r="7" />
      <path d="m20.5 20.5-4.2-4.2" />
    </g>
  ),
  "arrow-up-right": <path d="M7.5 16.5 16.5 7.5M9 7.5h7.5V15" />,
  "arrow-right": <path d="M4.5 12h14M13 6.5l5.5 5.5-5.5 5.5" />,
  "trending-up": (
    <g>
      <path d="m21.5 7.5-8 8-4-4-7 7" />
      <path d="M16 7.5h5.5V13" />
    </g>
  ),
  "trending-down": (
    <g>
      <path d="m21.5 16.5-8-8-4 4-7-7" />
      <path d="M16 16.5h5.5V11" />
    </g>
  ),
  plus: <path d="M12 5.5v13M5.5 12h13" />,
  activity: <path d="M22 12h-4.2l-3 8.5-6-17-3 8.5H2" />,
  zap: <path d="M13.2 2.5 4.4 13.2a.7.7 0 0 0 .54 1.15H10l-1.2 7.15 8.8-10.7a.7.7 0 0 0-.54-1.15H12z" />,
  command: <path d="M15 6.5v11a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3v-11a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3" />,
  "external-link": (
    <g>
      <path d="M14.5 3.5h6v6M10.5 13.5l10-10" />
      <path d="M18.5 14v5.5a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5V7A1.5 1.5 0 0 1 5 5.5h5.5" />
    </g>
  ),
  more: (
    <g>
      <circle cx="12" cy="12" r="1.1" />
      <circle cx="19" cy="12" r="1.1" />
      <circle cx="5" cy="12" r="1.1" />
    </g>
  ),
  check: <path d="m20 6.5-10.5 11L4 12" />,
  x: <path d="m6 6 12 12M18 6 6 18" />,
  filter: <path d="M3.5 5.5h17l-6.5 7.6v5.6l-4 2.3v-7.9z" />,
  download: <path d="M12 3.5v11m0 0 4-4m-4 4-4-4M4 18.5v1a1.5 1.5 0 0 0 1.5 1.5h13a1.5 1.5 0 0 0 1.5-1.5v-1" />,
  clock: (
    <g>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 1.8" />
    </g>
  ),
  gauge: (
    <g>
      <path d="M3.2 15.5a9 9 0 1 1 17.6 0" />
      <path d="m14.5 10-3.4 3.4a1.6 1.6 0 1 0 2.3 2.3z" />
    </g>
  ),
  wallet: (
    <g>
      <path d="M3.5 7.5a2 2 0 0 1 2-2h11a2 2 0 0 1 2 2" />
      <rect x="2.5" y="7.5" width="19" height="12" rx="2" />
      <path d="M17 13.5h.01" />
    </g>
  ),
  layers: (
    <g>
      <path d="m12 2.7 9 4.6-9 4.6-9-4.6z" />
      <path d="m3 12.2 9 4.6 9-4.6M3 16.9l9 4.6 9-4.6" />
    </g>
  ),
  sliders: (
    <g>
      <path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h9M17 18h3" />
      <circle cx="15" cy="6" r="2" />
      <circle cx="9" cy="12" r="2" />
      <circle cx="15" cy="18" r="2" />
    </g>
  ),
  sun: (
    <g>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.2 5.2l1.4 1.4M17.4 17.4l1.4 1.4M18.8 5.2l-1.4 1.4M6.6 17.4l-1.4 1.4" />
    </g>
  ),
  moon: <path d="M20.5 14.3A8.6 8.6 0 0 1 9.7 3.5a8.6 8.6 0 1 0 10.8 10.8z" />,
  copy: (
    <g>
      <rect x="9" y="9" width="12" height="12" rx="2" />
      <path d="M5 15H4.5A2 2 0 0 1 2.5 13V4.5a2 2 0 0 1 2-2H13a2 2 0 0 1 2 2V5" />
    </g>
  ),
  edit: (
    <g>
      <path d="M16.8 3.3a2.7 2.7 0 0 1 3.9 3.9L7.6 20.3 2.5 21.5l1.2-5.1z" />
      <path d="m14.6 5.5 3.9 3.9" />
    </g>
  ),
  trash: (
    <g>
      <path d="M3.5 6h17M8.5 6V4.2a1.2 1.2 0 0 1 1.2-1.2h4.6a1.2 1.2 0 0 1 1.2 1.2V6" />
      <path d="M18.5 6v13.3a2 2 0 0 1-2 2h-9a2 2 0 0 1-2-2V6" />
      <path d="M10.2 10.5v6.5M13.8 10.5v6.5" />
    </g>
  ),
  send: (
    <g>
      <path d="M21 3 10.5 13.5" />
      <path d="M21 3l-6.6 18-3.9-8.1L2.4 9z" />
    </g>
  ),
  refresh: (
    <g>
      <path d="M20.5 12a8.5 8.5 0 1 1-2.5-6" />
      <path d="M20.5 3.5v6h-6" />
    </g>
  ),
  book: (
    <g>
      <path d="M4 18.8A2.3 2.3 0 0 1 6.3 16.5H20" />
      <path d="M6.3 2.5H20v19H6.3A2.3 2.3 0 0 1 4 19.2V4.8a2.3 2.3 0 0 1 2.3-2.3z" />
    </g>
  ),
  terminal: <path d="m4.5 16.5 5.5-4.5-5.5-4.5M12.5 18h7" />,
  "arrow-left": <path d="M19.5 12h-14M11 6.5 5.5 12l5.5 5.5" />,
  smartphone: (
    <g>
      <rect x="6.2" y="2.5" width="11.6" height="19" rx="2.4" />
      <path d="M11 18.2h2" />
    </g>
  ),
  lock: (
    <g>
      <rect x="4" y="10.4" width="16" height="11.1" rx="2" />
      <path d="M8 10.4V7a4 4 0 0 1 8 0v3.4" />
    </g>
  ),
  "user-plus": (
    <g>
      <path d="M14 21v-1.8a3.8 3.8 0 0 0-3.8-3.8H5.8A3.8 3.8 0 0 0 2 19.2V21" />
      <circle cx="8" cy="7.2" r="3.7" />
      <path d="M19 8.5v6M22 11.5h-6" />
    </g>
  ),
  eye: (
    <g>
      <path d="M2.5 12s3.6-6.8 9.5-6.8S21.5 12 21.5 12s-3.6 6.8-9.5 6.8S2.5 12 2.5 12z" />
      <circle cx="12" cy="12" r="2.9" />
    </g>
  ),
  link: (
    <g>
      <path d="M10.2 13.3a4.6 4.6 0 0 0 6.9.5l2.7-2.7a4.6 4.6 0 0 0-6.5-6.5L11.7 6.2" />
      <path d="M13.8 10.7a4.6 4.6 0 0 0-6.9-.5l-2.7 2.7a4.6 4.6 0 0 0 6.5 6.5l1.6-1.6" />
    </g>
  ),
  inbox: (
    <g>
      <path d="M21.5 12.5h-5.3l-1.8 2.8H9.6l-1.8-2.8H2.5" />
      <path d="M5.7 5.1 2.5 12.5v5.4a2 2 0 0 0 2 2h15a2 2 0 0 0 2-2v-5.4l-3.2-7.4a2 2 0 0 0-1.8-1.1H7.5a2 2 0 0 0-1.8 1.1z" />
    </g>
  ),
  "circle-check": (
    <g>
      <circle cx="12" cy="12" r="8.6" />
      <path d="m8.3 12.2 2.4 2.4 5-5" />
    </g>
  ),
  "sidebar-collapse": (
    <g>
      <rect x="3" y="3.5" width="18" height="17" rx="2.2" />
      <path d="M9 3.5v17" />
      <path d="m16.2 9.2-2.8 2.8 2.8 2.8" />
    </g>
  ),
  "sidebar-expand": (
    <g>
      <rect x="3" y="3.5" width="18" height="17" rx="2.2" />
      <path d="M9 3.5v17" />
      <path d="m13.4 9.2 2.8 2.8-2.8 2.8" />
    </g>
  ),
  "log-out": (
    <g>
      <path d="M9.5 21H5.5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <path d="m16 16.5 4.5-4.5L16 7.5M20.5 12h-11" />
    </g>
  ),
}

export type IconName = keyof typeof ICON_PATHS

export function Icon({
  name,
  size = 16,
  className,
  style,
  stroke = 1.5,
}: {
  name: IconName | string
  size?: number
  className?: string
  style?: CSSProperties
  stroke?: number
}) {
  const body = ICON_PATHS[name]
  if (!body) return null
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={stroke}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      style={{ flex: "none", ...style }}
      aria-hidden="true"
    >
      {body}
    </svg>
  )
}
