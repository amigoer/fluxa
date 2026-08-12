import { useState } from "react";
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { ConfirmButton } from "@/components/confirm-button";
import { DataState, PageHeader } from "@/components/page";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResource } from "@/hooks/use-resource";
import { api, type DLPRule } from "@/lib/api";
import { formatDateTime, formatNumber } from "@/lib/format";

const EMPTY: DLPRule = {
  name: "",
  pattern: "",
  pattern_type: "keyword",
  scope: "both",
  action: "block",
  priority: 100,
  model_pattern: "",
  description: "",
  enabled: true,
};

export function DLPPage() {
  const [editing, setEditing] = useState<DLPRule | null>(null);

  return (
    <>
      <PageHeader
        title="Data Loss Prevention"
        description="Content rules applied to request and response bodies before they leave the network."
        action={
          <Button onClick={() => setEditing({ ...EMPTY })}>
            <PlusIcon />
            Add rule
          </Button>
        }
      />

      <Tabs defaultValue="rules">
        <TabsList>
          <TabsTrigger value="rules">Rules</TabsTrigger>
          <TabsTrigger value="violations">Violations</TabsTrigger>
        </TabsList>
        <TabsContent value="rules" className="mt-4">
          <RulesTab onEdit={setEditing} editing={editing} onCloseEdit={() => setEditing(null)} />
        </TabsContent>
        <TabsContent value="violations" className="mt-4">
          <ViolationsTab />
        </TabsContent>
      </Tabs>
    </>
  );
}

function RulesTab({
  editing,
  onEdit,
  onCloseEdit,
}: {
  editing: DLPRule | null;
  onEdit: (rule: DLPRule) => void;
  onCloseEdit: () => void;
}) {
  const rules = useResource(() => api.listDLPRules());

  return (
    <>
      <DataState
        loading={rules.loading}
        error={rules.error}
        empty={(rules.data ?? []).length === 0}
        emptyMessage="No DLP rules configured — nothing is being inspected."
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-20">Priority</TableHead>
                  <TableHead>Rule</TableHead>
                  <TableHead>Scope</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-24 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(rules.data ?? []).map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell className="tabular-nums">{rule.priority}</TableCell>
                    <TableCell>
                      <div className="font-medium">{rule.name}</div>
                      <code className="text-muted-foreground text-xs">{rule.pattern}</code>
                      <Badge variant="outline" className="ml-2">
                        {rule.pattern_type}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{rule.scope}</TableCell>
                    <TableCell>
                      <Badge variant={rule.action === "block" ? "destructive" : "secondary"}>
                        {rule.action}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={rule.enabled === false ? "outline" : "secondary"}>
                        {rule.enabled === false ? "disabled" : "enabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => onEdit({ ...rule })}>
                          <PencilIcon />
                          <span className="sr-only">Edit {rule.name}</span>
                        </Button>
                        <ConfirmButton
                          title={`Delete ${rule.name}?`}
                          description="Traffic will no longer be inspected against this pattern. Past violations stay in the audit log."
                          successMessage={`Deleted ${rule.name}`}
                          onConfirm={async () => {
                            await api.deleteDLPRule(rule.id!);
                            rules.reload();
                          }}
                          trigger={
                            <Button variant="ghost" size="icon">
                              <Trash2Icon />
                              <span className="sr-only">Delete {rule.name}</span>
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
        <RuleDialog
          key={editing.id ?? "__new__"}
          rule={editing}
          onClose={onCloseEdit}
          onSaved={() => {
            onCloseEdit();
            rules.reload();
          }}
        />
      ) : null}
    </>
  );
}

function ViolationsTab() {
  const violations = useResource(() => api.listDLPViolations(50, 0));

  return (
    <DataState
      loading={violations.loading}
      error={violations.error}
      empty={(violations.data?.data ?? []).length === 0}
      emptyMessage="No violations recorded."
    >
      <Card>
        <CardContent>
          <p className="text-muted-foreground mb-3 text-sm">
            {formatNumber(violations.data?.total ?? 0)} total violations
          </p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>When</TableHead>
                <TableHead>Rule</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Direction</TableHead>
                <TableHead>Matched</TableHead>
                <TableHead>Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(violations.data?.data ?? []).map((violation) => (
                <TableRow key={violation.id}>
                  <TableCell className="text-muted-foreground whitespace-nowrap">
                    {formatDateTime(violation.created_at)}
                  </TableCell>
                  <TableCell className="font-medium">{violation.rule_name}</TableCell>
                  <TableCell className="text-muted-foreground">{violation.model || "—"}</TableCell>
                  <TableCell className="text-muted-foreground">{violation.direction}</TableCell>
                  <TableCell className="max-w-xs truncate">
                    <code className="text-xs">{violation.matched_text}</code>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={violation.action_taken === "block" ? "destructive" : "secondary"}
                    >
                      {violation.action_taken}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </DataState>
  );
}

function RuleDialog({
  rule,
  onClose,
  onSaved,
}: {
  rule: DLPRule;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<DLPRule>(rule);
  const [busy, setBusy] = useState(false);

  let patternError: string | null = null;
  if (draft.pattern_type === "regex" && draft.pattern) {
    try {
      new RegExp(draft.pattern);
    } catch (err) {
      patternError = err instanceof Error ? err.message : String(err);
    }
  }

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      if (draft.id) await api.updateDLPRule(draft);
      else await api.createDLPRule(draft);
      toast.success(`Saved ${draft.name}`);
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
            <DialogTitle>{draft.id ? `Edit ${draft.name}` : "Add DLP rule"}</DialogTitle>
            <DialogDescription>
              block rejects the call, mask replaces the match, log records it and lets it through.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="dlp-name">Name</Label>
                <Input
                  id="dlp-name"
                  value={draft.name}
                  onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="dlp-priority">Priority</Label>
                <Input
                  id="dlp-priority"
                  type="number"
                  value={draft.priority}
                  onChange={(event) => setDraft({ ...draft, priority: Number(event.target.value) })}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="dlp-pattern">Pattern</Label>
              <Input
                id="dlp-pattern"
                value={draft.pattern}
                onChange={(event) => setDraft({ ...draft, pattern: event.target.value })}
                aria-invalid={patternError !== null}
                required
              />
              {patternError ? <p className="text-destructive text-xs">{patternError}</p> : null}
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <Label htmlFor="dlp-type">Type</Label>
                <Select
                  value={draft.pattern_type}
                  onValueChange={(value) =>
                    setDraft({ ...draft, pattern_type: value as DLPRule["pattern_type"] })
                  }
                >
                  <SelectTrigger id="dlp-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="keyword">keyword</SelectItem>
                    <SelectItem value="regex">regex</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="dlp-scope">Scope</Label>
                <Select
                  value={draft.scope}
                  onValueChange={(value) =>
                    setDraft({ ...draft, scope: value as DLPRule["scope"] })
                  }
                >
                  <SelectTrigger id="dlp-scope" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="request">request</SelectItem>
                    <SelectItem value="response">response</SelectItem>
                    <SelectItem value="both">both</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="dlp-action">Action</Label>
                <Select
                  value={draft.action}
                  onValueChange={(value) =>
                    setDraft({ ...draft, action: value as DLPRule["action"] })
                  }
                >
                  <SelectTrigger id="dlp-action" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="block">block</SelectItem>
                    <SelectItem value="mask">mask</SelectItem>
                    <SelectItem value="log">log</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="dlp-model">Model scope</Label>
              <Input
                id="dlp-model"
                placeholder="Optional regex — empty applies the rule to every model"
                value={draft.model_pattern ?? ""}
                onChange={(event) => setDraft({ ...draft, model_pattern: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="dlp-description">Description</Label>
              <Input
                id="dlp-description"
                value={draft.description ?? ""}
                onChange={(event) => setDraft({ ...draft, description: event.target.value })}
              />
            </div>

            <div className="flex items-center gap-2">
              <Switch
                id="dlp-enabled"
                checked={draft.enabled !== false}
                onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })}
              />
              <Label htmlFor="dlp-enabled">Enabled</Label>
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
