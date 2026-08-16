import { useState, type ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { Icon } from "@/components/console/icon"
import { Brand } from "@/components/console/brand"
import { Card, PageHead } from "@/components/console/ui"
import { useApiQuery } from "@/hooks/use-api-query"
import type { CallLog, Model, Provider, ProviderHealth, VirtualKey } from "@/lib/types"

// 快速接入 -- the docs page type. The whole pitch of the gateway is that
// it speaks the OpenAI protocol, so the page is mostly one code block and
// the three facts you need to paste into it.

const REPO_URL = "https://github.com/amigoer/fluxa"

export function QuickstartPage() {
  const navigate = useNavigate()
  const [lang, setLang] = useState<"curl" | "python" | "node">("curl")

  const { data: providers } = useApiQuery<Provider[]>("/api/providers")
  const { data: health } = useApiQuery<ProviderHealth[]>("/api/provider-health")
  const { data: models } = useApiQuery<Model[]>("/api/models/published")
  const { data: keys } = useApiQuery<VirtualKey[]>("/api/virtual-keys")
  const { data: calls } = useApiQuery<CallLog[]>("/api/call-logs")

  const baseUrl = `${window.location.origin}/v1`
  const firstModel = (models ?? [])[0]?.ModelIdentifier ?? "gpt-4o"
  const healthy = (health ?? []).filter((h) => h.State === "normal").length
  const hasProviders = (providers ?? []).length > 0
  const hasKeys = (keys ?? []).length > 0
  const hasCalls = (calls ?? []).length > 0

  const copy = async (text: string, what: string) => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success(`已复制${what}`)
    } catch {
      toast.error("复制失败，请手动选中复制")
    }
  }

  return (
    <div className="cn-page">
      <PageHead title="快速接入" sub="兼容 OpenAI 协议：把 base_url 换成 Fluxa，其余代码不用改">
        <a className="cn-btn" href={REPO_URL} target="_blank" rel="noreferrer">
          <Icon name="book" size={14} />
          完整 API 文档
        </a>
        <button className="cn-btn cn-btn-pri" onClick={() => navigate("/admin/keys")}>
          <Icon name="key" size={14} />
          创建我的第一个 Key
        </button>
      </PageHead>

      <div className="cn-docs">
        <div style={{ display: "flex", flexDirection: "column", gap: 14, minWidth: 0 }}>
          <Card title="接入信息">
            <div className="cn-kv">
              <span className="cn-kv-k">Base URL</span>
              <span className="cn-kv-v">{baseUrl}</span>
              <button className="cn-copy" onClick={() => void copy(baseUrl, " Base URL")}>
                <Icon name="copy" size={12} />
                复制
              </button>
            </div>
            <div className="cn-kv">
              <span className="cn-kv-k">认证方式</span>
              <span className="cn-kv-v">Authorization: Bearer &lt;FLUXA_API_KEY&gt;</span>
            </div>
            <div className="cn-kv">
              <span className="cn-kv-k">兼容接口</span>
              <span className="cn-kv-v">/chat/completions · /embeddings · /models</span>
            </div>
          </Card>

          <div className="cn-card">
            <div className="cn-code-head">
              <span className="cn-card-title">代码示例</span>
              <div className="cn-tabs" style={{ marginLeft: 8 }}>
                {(["curl", "python", "node"] as const).map((l) => (
                  <button key={l} data-on={l === lang} onClick={() => setLang(l)}>
                    {l === "node" ? "Node.js" : l === "python" ? "Python" : "cURL"}
                  </button>
                ))}
              </div>
              <button className="cn-copy" onClick={() => void copy(sampleText(lang, baseUrl, firstModel), "代码")}>
                <Icon name="copy" size={12} />
                复制
              </button>
            </div>
            <pre className="cn-code">{sample(lang, baseUrl, firstModel)}</pre>
          </div>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <Card title="三步接入">
            <div className="cn-steps">
              <Step
                n={1}
                done={hasProviders}
                title="管理员已配置供应商"
                desc={
                  hasProviders
                    ? `当前 ${(providers ?? []).length} 个供应商，${healthy} 个健康。`
                    : "还没有接入任何上游账号，网关暂时无法转发请求。"
                }
              />
              <Step
                n={2}
                done={hasKeys}
                title="创建一个虚拟 Key"
                desc="Key 决定可用模型范围和预算上限，不直接暴露供应商的真实凭证。"
              />
              <Step
                n={3}
                done={hasCalls}
                title="替换 base_url 并发起调用"
                desc="用左侧示例验证，随后在 Playground 里确认路由是否按预期命中。"
              />
            </div>
          </Card>

          <Card
            title="可用模型"
            note={`${(models ?? []).length} 个已发布`}
            link="模型与路由"
            onLink={() => navigate("/admin/models-routing")}
          >
            <div className="cn-rows">
              {(models ?? []).slice(0, 5).map((m) => (
                <div key={m.ID} className="cn-kv" style={{ padding: "9px 16px" }}>
                  <Brand kind={m.ProviderKind} size={14} />
                  <span className="cn-kv-v" style={{ fontSize: 12.5 }}>
                    {m.ModelIdentifier}
                  </span>
                </div>
              ))}
              {(models ?? []).length === 0 && (
                <div className="cn-kv" style={{ color: "var(--ink-3)" }}>
                  还没有已发布的模型。
                </div>
              )}
            </div>
          </Card>
        </div>
      </div>
    </div>
  )
}

function Step({ n, done, title, desc }: { n: number; done: boolean; title: string; desc: string }) {
  return (
    <div className="cn-step" data-done={done}>
      <span className="cn-step-n">{done ? <Icon name="check" size={12} /> : n}</span>
      <div>
        <div className="cn-step-t">{title}</div>
        <div className="cn-step-d">{desc}</div>
      </div>
    </div>
  )
}

// The samples exist twice: once as highlighted JSX for the page, once as
// plain text for the clipboard. Keeping them side by side is the only way
// the copy button is guaranteed to hand over what the reader is looking at.
function sample(lang: "curl" | "python" | "node", baseUrl: string, model: string): ReactNode {
  if (lang === "curl") {
    return (
      <>
        <span className="c"># 和调用 OpenAI 完全一样，只换 base_url 和 key</span>
        {`\ncurl ${baseUrl}/chat/completions \\\n  -H `}
        <span className="s">"Content-Type: application/json"</span>
        {` \\\n  -H `}
        <span className="s">"Authorization: Bearer $FLUXA_API_KEY"</span>
        {` \\\n  -d '{\n    "model": "${model}",\n    "messages": [{"role": "user", "content": "你好"}]\n  }'`}
      </>
    )
  }
  if (lang === "python") {
    return (
      <>
        <span className="k">from</span>
        {" openai "}
        <span className="k">import</span>
        {" OpenAI\n\nclient = OpenAI(\n    base_url="}
        <span className="s">"{baseUrl}"</span>
        {",\n    api_key=os.environ["}
        <span className="s">"FLUXA_API_KEY"</span>
        {`],\n)\n\nresp = client.chat.completions.create(\n    model=`}
        <span className="s">"{model}"</span>
        {`,\n    messages=[{"role": "user", "content": "你好"}],\n)\nprint(resp.choices[0].message.content)`}
      </>
    )
  }
  return (
    <>
      <span className="k">import</span>
      {" OpenAI "}
      <span className="k">from</span>
      {" "}
      <span className="s">"openai"</span>
      {`\n\nconst client = new OpenAI({\n  baseURL: `}
      <span className="s">"{baseUrl}"</span>
      {`,\n  apiKey: process.env.FLUXA_API_KEY,\n})\n\nconst resp = await client.chat.completions.create({\n  model: `}
      <span className="s">"{model}"</span>
      {`,\n  messages: [{ role: "user", content: "你好" }],\n})\nconsole.log(resp.choices[0].message.content)`}
    </>
  )
}

function sampleText(lang: "curl" | "python" | "node", baseUrl: string, model: string): string {
  if (lang === "curl") {
    return `curl ${baseUrl}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $FLUXA_API_KEY" \\
  -d '{
    "model": "${model}",
    "messages": [{"role": "user", "content": "你好"}]
  }'`
  }
  if (lang === "python") {
    return `from openai import OpenAI

client = OpenAI(
    base_url="${baseUrl}",
    api_key=os.environ["FLUXA_API_KEY"],
)

resp = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)`
  }
  return `import OpenAI from "openai"

const client = new OpenAI({
  baseURL: "${baseUrl}",
  apiKey: process.env.FLUXA_API_KEY,
})

const resp = await client.chat.completions.create({
  model: "${model}",
  messages: [{ role: "user", content: "你好" }],
})
console.log(resp.choices[0].message.content)`
}
