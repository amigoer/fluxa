import { useState } from "react";
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { ConfirmButton } from "@/components/confirm-button";
import { DataState, PageHeader } from "@/components/page";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useResource } from "@/hooks/use-resource";
import { api, type RegexModel } from "@/lib/api";

const EMPTY: RegexModel = {
  pattern: "",
  priority: 100,
  target_type: "real",
  target_model: "",
  provider: "",
  description: "",
  enabled: true,
};

export function RegexModelsPage() {
  const rules = useResource(() => api.listRegexModels());
  const providers = useResource(() => api.listProviders());
  const [editing, setEditing] = useState<RegexModel | null>(null);

  return (
    <>
      <PageHeader
        title="Regex Models"
        description="Pattern-based interception. Lower priority runs first and the first match wins."
        action={
          <Button onClick={() => setEditing({ ...EMPTY })}>
            <PlusIcon />
            Add rule
          </Button>
        }
      />

      <ResolveTester />

      <DataState
        loading={rules.loading}
        error={rules.error}
        empty={(rules.data ?? []).length === 0}
        emptyMessage="No pattern rules yet."
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-20">Priority</TableHead>
                  <TableHead>Pattern</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-24 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(rules.data ?? []).map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell className="tabular-nums">{rule.priority}</TableCell>
                    <TableCell>
                      <code className="bg-muted rounded px-1.5 py-0.5 text-xs">{rule.pattern}</code>
                      {rule.description ? (
                        <div className="text-muted-foreground mt-1 text-xs">{rule.description}</div>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Badge variant="outline">{rule.target_type}</Badge>
                        <span className="text-sm">{rule.target_model}</span>
                        {rule.provider ? (
                          <span className="text-muted-foreground text-xs">via {rule.provider}</span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={rule.enabled === false ? "outline" : "secondary"}>
                        {rule.enabled === false ? "disabled" : "enabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => setEditing({ ...rule })}>
                          <PencilIcon />
                          <span className="sr-only">Edit rule</span>
                        </Button>
                        <ConfirmButton
                          title="Delete this rule?"
                          description={`Requests matching ${rule.pattern} will no longer be redirected.`}
                          successMessage="Rule deleted"
                          onConfirm={async () => {
                            await api.deleteRegexModel(rule.id!);
                            rules.reload();
                          }}
                          trigger={
                            <Button variant="ghost" size="icon">
                              <Trash2Icon />
                              <span className="sr-only">Delete rule</span>
                            </Button>
                          }
                        />
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </DataState>

      {editing ? (
        <RegexDialog
          key={editing.id ?? "__new__"}
          rule={editing}
          providers={(providers.data ?? []).map((p) => p.name)}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            rules.reload();
          }}
        />
      ) : null}
    </>
  );
}

/** Runs a model name through the live resolver so a rule can be checked
    against the real chain rather than by eyeballing the regex. */
function ResolveTester() {
  const [model, setModel] = useState("");
  const [result, setResult] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const run = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      setResult(JSON.stringify(await api.resolveModel(model), null, 2));
    } catch (err) {
      setResult(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Resolve tester</CardTitle>
        <CardDescription>
          Ask the gateway what a model name resolves to right now.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <form onSubmit={run} className="flex gap-2">
          <Input
            placeholder="gpt-4o-2024-08-06"
            value={model}
            onChange={(event) => setModel(event.target.value)}
            required
          />
          <Button type="submit" variant="secondary" disabled={busy || !model}>
            {busy ? "Resolving…" : "Resolve"}
          </Button>
        </form>
        {result ? (
          <pre className="bg-muted max-h-56 overflow-auto rounded-md p-3 text-xs">{result}</pre>
        ) : null}
      </CardContent>
    </Card>
  );
}

function RegexDialog({
  rule,
  providers,
  onClose,
  onSaved,
}: {
  rule: RegexModel;
  providers: string[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<RegexModel>(rule);
  const [busy, setBusy] = useState(false);

  // Compiling in the browser catches a bad pattern before the round trip;
  // the server validates again on write.
  let patternError: string | null = null;
  try {
    if (draft.pattern) new RegExp(draft.pattern);
  } catch (err) {
    patternError = err instanceof Error ? err.message : String(err);
  }

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      if (draft.id) await api.updateRegexModel(draft);
      else await api.createRegexModel(draft);
      toast.success("Rule saved");
      onSaved();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={save}>
          <DialogHeader>
            <DialogTitle>{draft.id ? "Edit rule" : "Add rule"}</DialogTitle>
            <DialogDescription>
              Patterns use Go's RE2 syntax; anchors like ^ and $ behave as expected.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="pattern">Pattern</Label>
              <Input
                id="pattern"
                placeholder="^gpt-4.*"
                value={draft.pattern}
                onChange={(event) => setDraft({ ...draft, pattern: event.target.value })}
                aria-invalid={patternError !== null}
                required
              />
              {patternError ? (
                <p className="text-destructive text-xs">{patternError}</p>
              ) : null}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="target-type">Target type</Label>
                <Select
                  value={draft.target_type}
                  onValueChange={(value) =>
                    setDraft({ ...draft, target_type: value as "real" | "virtual" })
                  }
                >
                  <SelectTrigger id="target-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="real">real</SelectItem>
                    <SelectItem value="virtual">virtual</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="priority">Priority</Label>
                <Input
                  id="priority"
                  type="number"
                  value={draft.priority}
                  onChange={(event) => setDraft({ ...draft, priority: Number(event.target.value) })}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="target-model">Target model</Label>
                <Input
                  id="target-model"
                  value={draft.target_model}
                  onChange={(event) => setDraft({ ...draft, target_model: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="regex-provider">Provider</Label>
                <Select
                  value={draft.provider || "__none__"}
                  onValueChange={(value) =>
                    setDraft({ ...draft, provider: value === "__none__" ? "" : value })
                  }
                  disabled={draft.target_type === "virtual"}
                >
                  <SelectTrigger id="regex-provider" className="w-full">
                    <SelectValue placeholder="—" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">—</SelectItem>
                    {providers.map((name) => (
                      <SelectItem key={name} value={name}>
                        {name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="regex-description">Description</Label>
              <Input
                id="regex-description"
                value={draft.description ?? ""}
                onChange={(event) => setDraft({ ...draft, description: event.target.value })}
              />
            </div>

            <div className="flex items-center gap-2">
              <Switch
                id="regex-enabled"
                checked={draft.enabled !== false}
                onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })}
              />
              <Label htmlFor="regex-enabled">Enabled</Label>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || patternError !== null}>
              {busy ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
