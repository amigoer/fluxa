import { Cpu } from "lucide-react";

// Brand SVGs come from @lobehub/icons-static-svg, a zero-dependency
// bundle of AI-provider logos. We import the color variant for each
// kind via Vite's `?url` loader so the bundler emits a hashed asset
// and serves a real <img src="…">. This sidesteps the Iconify CDN
// entirely (lobe-icons is not a public Iconify collection, which is
// why the previous `lobe-icons:*` names rendered blank).
//
// Keys mirror the provider kinds enumerated in
// internal/router/router.go (`openaiCompatibleDefaults` plus the
// hand-rolled adapters). Add a new kind in both places to keep the
// picker, the adapter factory, and the icon map in sync.

import openaiUrl from "@lobehub/icons-static-svg/icons/openai.svg?url";
import anthropicUrl from "@lobehub/icons-static-svg/icons/claude-color.svg?url";
import azureUrl from "@lobehub/icons-static-svg/icons/azure-color.svg?url";
import geminiUrl from "@lobehub/icons-static-svg/icons/gemini-color.svg?url";
import bedrockUrl from "@lobehub/icons-static-svg/icons/bedrock-color.svg?url";
import awsUrl from "@lobehub/icons-static-svg/icons/aws-color.svg?url";
import ollamaUrl from "@lobehub/icons-static-svg/icons/ollama.svg?url";

import qwenUrl from "@lobehub/icons-static-svg/icons/qwen-color.svg?url";
import deepseekUrl from "@lobehub/icons-static-svg/icons/deepseek-color.svg?url";
// Kimi's color variant has a white-on-transparent body with only a
// tiny blue accent — unusable on a light card. The monochrome
// `moonshot.svg` uses currentColor and renders correctly instead.
import moonshotUrl from "@lobehub/icons-static-svg/icons/moonshot.svg?url";
import zhipuUrl from "@lobehub/icons-static-svg/icons/zhipu-color.svg?url";
import doubaoUrl from "@lobehub/icons-static-svg/icons/doubao-color.svg?url";
import wenxinUrl from "@lobehub/icons-static-svg/icons/wenxin-color.svg?url";
import siliconcloudUrl from "@lobehub/icons-static-svg/icons/siliconcloud-color.svg?url";
import minimaxUrl from "@lobehub/icons-static-svg/icons/minimax-color.svg?url";
import baichuanUrl from "@lobehub/icons-static-svg/icons/baichuan-color.svg?url";
import stepfunUrl from "@lobehub/icons-static-svg/icons/stepfun-color.svg?url";
import sparkUrl from "@lobehub/icons-static-svg/icons/spark-color.svg?url";
import yiUrl from "@lobehub/icons-static-svg/icons/yi-color.svg?url";
import hunyuanUrl from "@lobehub/icons-static-svg/icons/hunyuan-color.svg?url";

import mistralUrl from "@lobehub/icons-static-svg/icons/mistral-color.svg?url";
import groqUrl from "@lobehub/icons-static-svg/icons/groq.svg?url";
import grokUrl from "@lobehub/icons-static-svg/icons/grok.svg?url";
import perplexityUrl from "@lobehub/icons-static-svg/icons/perplexity-color.svg?url";
import togetherUrl from "@lobehub/icons-static-svg/icons/together-color.svg?url";
import fireworksUrl from "@lobehub/icons-static-svg/icons/fireworks-color.svg?url";
import openrouterUrl from "@lobehub/icons-static-svg/icons/openrouter.svg?url";
import cohereUrl from "@lobehub/icons-static-svg/icons/cohere-color.svg?url";
import nvidiaUrl from "@lobehub/icons-static-svg/icons/nvidia-color.svg?url";

const ICON_MAP: Record<string, string> = {
  // Core adapters
  openai: openaiUrl,
  anthropic: anthropicUrl,
  claude: anthropicUrl,
  azure: azureUrl,
  microsoft: azureUrl,
  gemini: geminiUrl,
  google: geminiUrl,
  bedrock: bedrockUrl,
  aws: awsUrl,
  ollama: ollamaUrl,

  // Chinese OpenAI-compatible vendors
  qwen: qwenUrl,
  aliyun: qwenUrl,
  dashscope: qwenUrl,
  deepseek: deepseekUrl,
  moonshot: moonshotUrl,
  zhipu: zhipuUrl,
  doubao: doubaoUrl,
  ernie: wenxinUrl,
  baidu: wenxinUrl,
  siliconflow: siliconcloudUrl,
  minimax: minimaxUrl,
  baichuan: baichuanUrl,
  stepfun: stepfunUrl,
  spark: sparkUrl,
  "zero-one": yiUrl,
  tencent: hunyuanUrl,

  // Western OpenAI-compatible vendors
  mistral: mistralUrl,
  groq: groqUrl,
  xai: grokUrl,
  perplexity: perplexityUrl,
  together: togetherUrl,
  fireworks: fireworksUrl,
  openrouter: openrouterUrl,
  cohere: cohereUrl,
  nvidia: nvidiaUrl,
};

export function ProviderIcon({ kind, className = "" }: { kind: string; className?: string }) {
  const normalized = kind.toLowerCase().trim();
  const url = ICON_MAP[normalized];
  if (url) {
    return <img src={url} alt={normalized} className={className} draggable={false} />;
  }
  return <Cpu className={className} />;
}
