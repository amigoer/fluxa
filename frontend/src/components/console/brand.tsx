import type { CSSProperties, ReactElement } from "react"
import type { IconProps } from "@/components/shared/brand-icons"
import {
  AlibabaCloudIcon,
  AnthropicIcon,
  AzureIcon,
  BedrockIcon,
  DingTalkIcon,
  FeishuIcon,
  GeminiIcon,
  OpenAIIcon,
} from "@/components/shared/brand-icons"

// Vendor marks keyed by the provider `Kind` the backend stores, so a row
// can render its logo straight from the record without a lookup table at
// every call site. Anything without a real mark renders nothing -- the
// design never falls back to a letter avatar for a vendor (DESIGN.md 6.2).
const MARKS: Record<string, (p: IconProps) => ReactElement> = {
  openai_compatible: OpenAIIcon,
  openai: OpenAIIcon,
  anthropic: AnthropicIcon,
  azure_openai: AzureIcon,
  gemini: GeminiIcon,
  bedrock: BedrockIcon,
  alibaba: AlibabaCloudIcon,
  alibaba_cloud: AlibabaCloudIcon,
  feishu: FeishuIcon,
  dingtalk: DingTalkIcon,
}

// Feishu and DingTalk ship as full app icons with their own white plate,
// so they get the rounded-square treatment instead of being drawn as a
// bare glyph -- the same thing every "sign in with X" button does.
const PLATED = new Set(["feishu", "dingtalk"])

export function Brand({ kind, size = 14, style }: { kind?: string | null; size?: number; style?: CSSProperties }) {
  if (!kind) return null
  const Mark = MARKS[kind]
  if (!Mark) return null
  const base: CSSProperties = { width: size, height: size, display: "block", flex: "none", ...style }
  if (PLATED.has(kind)) base.borderRadius = Math.max(3, size * 0.23)
  return <Mark style={base} />
}

// The Fluxa mark: a jellyfish on a rounded plate (DESIGN.md 6.2).
//
// Drawn as one SVG rather than a CSS plate with a glyph inside it, so the
// plate and the creature scale together and the whole thing can be handed
// to a favicon or a README at any size without re-deriving the geometry.
//
// The composition is the designer's own and is deliberately not centred:
// the bell sits high and the tentacles trail past the middle, which is
// what keeps it reading as drifting rather than as a logo pinned to a
// grid. Re-centring it would flatten exactly that.
export function FluxaLogo({
  size = 26,
  radius = 7,
  bg = "var(--brand)",
  fg = "#fff",
}: {
  size?: number
  radius?: number
  bg?: string
  fg?: string
}) {
  return (
    <svg
      viewBox="0 0 120 120"
      width={size}
      height={size}
      style={{ display: "block", flex: "none" }}
      aria-hidden="true"
    >
      {/* The prop is in rendered pixels, the canvas is 120 wide, so the
          corner keeps the same visual radius at every size. */}
      <rect width="120" height="120" rx={(radius * 120) / size} fill={bg} />
      <g transform="rotate(-4 60 58) scale(0.62) translate(37 37)">
        <path
          fill={fg}
          fillRule="evenodd"
          clipRule="evenodd"
          d="M13 52C13 26 34 6 60 6C86 6 107 26 107 52C99 62 90 48 82 54C75 59 67 47 60 53C53 59 45 47 38 54C30 60 21 62 13 52ZM45 29a5 5 0 1 0 .1 0zM75 29a5 5 0 1 0 .1 0z"
        />
        <g stroke={fg} strokeWidth="7" strokeLinecap="round" fill="none">
          <path d="M26 60C23 76 15 81 19 97" />
          <path d="M43 62C40 82 32 92 39 108" />
          <path d="M60 63C56 86 66 96 57 117" />
          <path d="M77 62C80 82 89 90 81 105" />
          <path d="M94 60C97 74 105 78 100 93" />
        </g>
      </g>
    </svg>
  )
}
