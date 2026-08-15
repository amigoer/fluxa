import type { ComponentType } from "react"
import {
  OpenAIIcon,
  AnthropicIcon,
  AzureIcon,
  GeminiIcon,
  BedrockIcon,
} from "@/components/shared/brand-icons"

// Real vendor mark keyed off the provider's Kind (matches
// internal/provider/types.ProviderKind on the backend) wherever we know
// it; falls back to the old letter-in-a-circle only for a kind this
// map doesn't cover, or when no kind is known at all (e.g. a
// provider.Kind of "openai_compatible" pointed at some other vendor's
// compatible endpoint, where there is no single real logo to show).
const KIND_ICON: Record<string, ComponentType<{ className?: string }>> = {
  openai_compatible: OpenAIIcon,
  anthropic: AnthropicIcon,
  azure_openai: AzureIcon,
  gemini: GeminiIcon,
  bedrock: BedrockIcon,
}

export function ProviderAvatar({ name, kind }: { name: string; kind?: string }) {
  const Icon = kind ? KIND_ICON[kind] : undefined
  if (Icon) {
    return (
      <span className="mr-2 inline-flex size-[22px] flex-none items-center justify-center rounded-full bg-card ring-1 ring-inset ring-border">
        <Icon className="size-3.5" />
      </span>
    )
  }
  return (
    <span className="mr-2 inline-flex size-[22px] flex-none items-center justify-center rounded-full bg-accent text-[9.5px] font-bold text-accent-foreground">
      {name.slice(0, 1).toUpperCase()}
    </span>
  )
}
