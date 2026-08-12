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
import { useT } from "@/lib/i18n";
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
  const t = useT();
  const [editing, setEditing] = useState<DLPRule | null>(null);

  return (
    <>
      <PageHeader
        title={t("dlp.title")}
        description={t("dlp.description")}
        action={
          <Button onClick={() => setEditing({ ...EMPTY })}>
            <PlusIcon />
            {t("dlp.add")}
          </Button>
        }
      />

      <Tabs defaultValue="rules">
        <TabsList>
          <TabsTrigger value="rules">{t("dlp.tabRules")}</TabsTrigger>
          <TabsTrigger value="violations">{t("dlp.tabViolations")}</TabsTrigger>
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
  const t = useT();
  const rules = useResource(() => api.listDLPRules());

  return (
    <>
      <DataState
        loading={rules.loading}
        error={rules.error}
        empty={(rules.data ?? []).length === 0}
        emptyMessage={t("dlp.rulesEmpty")}
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-20">{t("common.priority")}</TableHead>
                  <TableHead>{t("dlp.colRule")}</TableHead>
                  <TableHead>{t("dlp.colScope")}</TableHead>
                  <TableHead>{t("dlp.colAction")}</TableHead>
                  <TableHead>{t("common.status")}</TableHead>
                  <TableHead className="w-24 text-right">{t("common.actions")}</TableHead>
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
                        {rule.enabled === false ? t("common.disabled") : t("common.enabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => onEdit({ ...rule })}>
                          <PencilIcon />
                          <span className="sr-only">{t("common.editSr", { name: rule.name })}</span>
                        </Button>
                        <ConfirmButton
                          title={t("dlp.deleteTitle", { name: rule.name })}
                          description={t("dlp.deleteDescription")}
                          successMessage={t("dlp.deleted", { name: rule.name })}
                          onConfirm={async () => {
                            await api.deleteDLPRule(rule.id!);
                            rules.reload();
                          }}
                          trigger={
                            <Button variant="ghost" size="icon">
                              <Trash2Icon />
                              <span className="sr-only">{t("common.deleteSr", { name: rule.name })}</span>
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
  const t = useT();
  const violations = useResource(() => api.listDLPViolations(50, 0));

  return (
    <DataState
      loading={violations.loading}
      error={violations.error}
      empty={(violations.data?.data ?? []).length === 0}
      emptyMessage={t("dlp.violationsEmpty")}
    >
      <Card>
        <CardContent>
          <p className="text-muted-foreground mb-3 text-sm">
            {t("dlp.violationsTotal", { count: formatNumber(violations.data?.total ?? 0) })}
          </p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("dlp.colWhen")}</TableHead>
                <TableHead>{t("dlp.colRule")}</TableHead>
                <TableHead>{t("common.model")}</TableHead>
                <TableHead>{t("dlp.colDirection")}</TableHead>
                <TableHead>{t("dlp.colMatched")}</TableHead>
                <TableHead>{t("dlp.colAction")}</TableHead>
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
  const t = useT();
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
      toast.success(t("dlp.saved", { name: draft.name }));
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
            <DialogTitle>
              {draft.id ? t("dlp.dialogEdit", { name: draft.name }) : t("dlp.dialogAdd")}
            </DialogTitle>
<DialogDescription>{t("dlp.dialogDescription")}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="dlp-name">{t("common.name")}</Label>
                <Input
                  id="dlp-name"
                  value={draft.name}
                  onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="dlp-priority">{t("common.priority")}</Label>
                <Input
                  id="dlp-priority"
                  type="number"
                  value={draft.priority}
                  onChange={(event) => setDraft({ ...draft, priority: Number(event.target.value) })}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="dlp-pattern">{t("dlp.pattern")}</Label>
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
                <Label htmlFor="dlp-type">{t("dlp.type")}</Label>
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
                <Label htmlFor="dlp-scope">{t("dlp.scope")}</Label>
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
                <Label htmlFor="dlp-action">{t("dlp.action")}</Label>
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
              <Label htmlFor="dlp-model">{t("dlp.modelScope")}</Label>
              <Input
                id="dlp-model"
                placeholder={t("dlp.modelScopePlaceholder")}
                value={draft.model_pattern ?? ""}
                onChange={(event) => setDraft({ ...draft, model_pattern: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="dlp-description">{t("common.description")}</Label>
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
              <Label htmlFor="dlp-enabled">{t("common.enabled")}</Label>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={busy || patternError !== null}>
              {busy ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
