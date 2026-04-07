import { useState } from "react";
import { Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { Resolver, type ResolveResult } from "@/lib/api";
import { useT } from "@/lib/i18n";

// ResolveTesterPage is the dashboard's "what would happen if I sent
// model X right now" probe. It calls /admin/resolve-model which runs
// the same pre-resolver as the data plane and returns the full trace
// without ever touching an upstream provider — so operators can edit
// virtual models or regex routes and validate the result immediately.
export function ResolveTesterPage() {
  const { t } = useT();
  const [model, setModel] = useState("");
  const [result, setResult] = useState<ResolveResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(e: React.FormEvent) {
    e.preventDefault();
    if (!model.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const res = await Resolver.test(model.trim());
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
      setResult(null);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("resolve.title")}
        </h1>
        <p className="text-sm text-muted-foreground">{t("resolve.subtitle")}</p>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <Card>
        <CardContent className="p-6">
          <form onSubmit={run} className="flex items-end gap-3">
            <div className="flex-1 space-y-2">
              <Label>{t("resolve.input")}</Label>
              <Input
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder={t("resolve.placeholder")}
                autoFocus
                className="font-mono"
              />
            </div>
            <Button type="submit" disabled={loading || !model.trim()}>
              <Play className="h-4 w-4" />
              {loading ? t("common.saving") : t("resolve.run")}
            </Button>
          </form>
        </CardContent>
      </Card>

      {result && (
        <Card>
          <CardContent className="p-6 space-y-4">
            {result.error && (
              <div className="space-y-1">
                <div className="text-xs uppercase tracking-wide text-muted-foreground">
                  {t("resolve.errorTitle")}
                </div>
                <div className="text-sm text-destructive font-mono">
                  {result.error}
                </div>
              </div>
            )}

            {result.passthrough && !result.error && (
              <div className="rounded-md border border-border/60 bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
                {t("resolve.passthrough")}
              </div>
            )}

            {result.target && (
              <div className="space-y-1">
                <div className="text-xs uppercase tracking-wide text-muted-foreground">
                  {t("resolve.target")}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <div className="text-xs text-muted-foreground">
                      {t("resolve.targetProvider")}
                    </div>
                    <div className="font-mono text-sm">
                      {result.target.provider || "—"}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground">
                      {t("resolve.targetModel")}
                    </div>
                    <div className="font-mono text-sm">
                      {result.target.model}
                    </div>
                  </div>
                </div>
              </div>
            )}

            <div className="space-y-2">
              <div className="text-xs uppercase tracking-wide text-muted-foreground">
                {t("resolve.trace")}
              </div>
              {result.trace.length === 0 ? (
                <div className="text-sm text-muted-foreground">
                  {t("resolve.traceEmpty")}
                </div>
              ) : (
                <ol className="space-y-1.5">
                  {result.trace.map((step, idx) => (
                    <li
                      key={idx}
                      className="rounded-md border border-border/60 bg-background px-3 py-2 text-xs font-mono"
                      style={{ marginLeft: `${step.depth * 1.5}rem` }}
                    >
                      <span className="text-muted-foreground">
                        [{step.depth}]
                      </span>{" "}
                      <span className="font-semibold">{step.type}</span>
                      {step.pattern && (
                        <span className="text-muted-foreground">
                          {" "}
                          /{step.pattern}/
                        </span>
                      )}
                      {step.name && (
                        <span className="text-muted-foreground">
                          {" "}
                          {step.name}
                        </span>
                      )}
                      {step.target && (
                        <span> → {step.target}</span>
                      )}
                      {step.provider && (
                        <span className="text-muted-foreground">
                          @{step.provider}
                        </span>
                      )}
                      {step.weight_picked != null &&
                        step.weight_picked > 0 && (
                          <span className="text-muted-foreground">
                            {" "}
                            (w={step.weight_picked})
                          </span>
                        )}
                    </li>
                  ))}
                </ol>
              )}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
