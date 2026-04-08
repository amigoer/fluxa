// ProviderCreatePanel — minimal create form for a new upstream
// Provider, mounted inside the route graph side panel when the
// operator clicks "+ 供应商" on the toolbar.
//
// Scope: we only expose the common-case fields every provider
// backend needs (name, kind, api_key, base_url, models). Advanced
// kinds that need extra credentials or deployments (AWS Bedrock,
// Azure OpenAI) still live on the dedicated Providers page — the
// form there covers ~15 fields and would overflow this 340px
// drawer. The hint line in the panel header tells the operator
// where to go for those.
//
// Unlike the regex / virtual-model create flows, we do NOT insert
// a draft node on the canvas: a provider record is a config blob,
// not a (provider, model) tuple. Instead we rely on the enhanced
// buildGraph, which now iterates `provider.models[]` and emits a
// standalone ProviderNode per entry. The new provider therefore
// appears on the canvas immediately on the next load() — one node
// per model the operator listed.

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Providers, type Provider } from "@/lib/api";
import { useT } from "@/lib/i18n";

interface Props {
  onDirty?: () => void;
  onChange: () => void | Promise<void>;
  onClose: () => void;
}

// KIND_OPTIONS are the common-case backend kinds the panel lets the
// operator pick. The gateway actually supports more under the hood
// (azure, bedrock, etc.) but those need extra credential fields we
// don't host here — the Providers page is still the right place
// for those today.
const KIND_OPTIONS = [
  "openai",
  "anthropic",
  "qwen",
  "google",
  "deepseek",
  "custom",
];

const EMPTY_PROVIDER: Provider = {
  name: "",
  kind: "openai",
  api_key: "",
  base_url: "",
  models: [],
  enabled: true,
};

export function ProviderCreatePanel({ onDirty, onChange, onClose }: Props) {
  const { t } = useT();
  const [rawForm, setRawForm] = useState<Provider>(EMPTY_PROVIDER);
  const form = rawForm;
  const setForm = (next: Provider) => {
    onDirty?.();
    setRawForm(next);
  };
  // modelsText is a comma-joined string backing the textarea so the
  // operator can type naturally; we split it on save. Tracking it as
  // a separate local state (rather than re-serialising form.models
  // on every render) keeps the cursor position sane while editing.
  const [modelsText, setModelsText] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    if (!form.name.trim()) {
      setError(t("graph.errors.save"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await Providers.upsert({
        ...form,
        name: form.name.trim(),
        kind: form.kind.trim(),
        api_key: form.api_key ?? "",
        base_url: form.base_url?.trim() || undefined,
        models: modelsText
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      });
      await onChange();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("graph.errors.save"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.name")}</Label>
        <Input
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          placeholder="qwen"
          autoFocus
        />
      </div>

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.kind")}</Label>
        <select
          className="flex h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm"
          value={form.kind}
          onChange={(e) => setForm({ ...form, kind: e.target.value })}
        >
          {KIND_OPTIONS.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </select>
      </div>

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.baseUrl")}</Label>
        <Input
          value={form.base_url ?? ""}
          onChange={(e) => setForm({ ...form, base_url: e.target.value })}
          placeholder="https://dashscope.aliyuncs.com/compatible-mode/v1"
        />
      </div>

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.apiKey")}</Label>
        <Input
          type="password"
          value={form.api_key ?? ""}
          onChange={(e) => setForm({ ...form, api_key: e.target.value })}
          placeholder="sk-..."
          autoComplete="off"
        />
      </div>

      <div className="space-y-2">
        <Label className="text-xs">{t("graph.field.models")}</Label>
        <Input
          value={modelsText}
          onChange={(e) => {
            onDirty?.();
            setModelsText(e.target.value);
          }}
          placeholder="qwen3-32b, qwen3-72b-instruct"
        />
        <p className="text-[10px] text-muted-foreground">
          {t("graph.field.modelsHint")}
        </p>
      </div>

      <div className="flex gap-2 pt-2">
        <Button onClick={save} disabled={saving} size="sm" className="flex-1">
          {saving ? t("graph.action.saving") : t("graph.action.create")}
        </Button>
      </div>
    </div>
  );
}
