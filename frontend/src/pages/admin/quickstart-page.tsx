import { useState } from "react"
import { PageHeader } from "@/components/shared/page-header"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"

function CodeBlock({ children }: { children: string }) {
  return (
    <div className="relative overflow-x-auto rounded-lg bg-[#15161A] p-4 font-mono text-[11.5px] leading-relaxed text-[#E4E5E9]">
      <pre className="whitespace-pre-wrap">{children}</pre>
      <button
        className="absolute right-2.5 top-2.5 rounded border border-[#33353B] bg-[#1E1F24] px-2 py-1 text-[10px] font-semibold text-[#B7BAC2]"
        onClick={() => {
          void navigator.clipboard.writeText(children)
          toast.success("已复制")
        }}
      >
        复制
      </button>
    </div>
  )
}

export function QuickstartPage() {
  const [origin] = useState(window.location.origin)

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="快速接入" />

      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">Base URL</p>
        <CodeBlock>{`${origin}/v1`}</CodeBlock>
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">认证方式</p>
        <CodeBlock>{"Authorization: Bearer vk-xxxxxxxxxxxxxxxx"}</CodeBlock>
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <p className="mb-3.5 text-[12.5px] font-semibold text-foreground">代码示例</p>
        <CodeBlock>{`curl ${origin}/v1/chat/completions \\
  -H "Authorization: Bearer vk-xxxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "gpt-4o", "messages": [{"role":"user","content":"你好"}]}'`}</CodeBlock>
      </div>

      <div className="flex items-center justify-between gap-4 rounded-lg border border-border bg-card p-4 shadow-[var(--shadow-card)]">
        <span className="text-[12.5px] text-foreground">
          还没有虚拟 Key？
          <span className="mt-0.5 block text-[11.5px] text-muted-foreground">创建一个 Key 才能实际调用接口</span>
        </span>
        <Button asChild>
          <a href="/admin/keys">创建我的第一个 Key</a>
        </Button>
      </div>
    </div>
  )
}
