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

// The Fluxa mark: geometric F in a rounded square (DESIGN.md 6.2).
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
    <span
      style={{
        width: size,
        height: size,
        borderRadius: radius,
        background: bg,
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        flex: "none",
      }}
    >
      <svg viewBox="0 0 24 24" fill={fg} width={size * 0.58} height={size * 0.58} aria-hidden="true">
        <rect x="7" y="5" width="2.6" height="14" rx="1" />
        <rect x="7" y="5" width="10" height="2.6" rx="1" />
        <rect x="7" y="11.2" width="7.5" height="2.6" rx="1" />
      </svg>
    </span>
  )
}
