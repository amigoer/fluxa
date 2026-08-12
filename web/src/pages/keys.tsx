import { useState } from "react";
import { CopyIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { useResource } from "@/hooks/use-resource";
import { api, type VirtualKey } from "@/lib/api";
import { formatDateTime, formatNumber, formatUSD, parseList } from "@/lib/format";

const EMPTY: VirtualKey = { name: "", description: "", enabled: true };

export function KeysPage() {
  const keys = useResource(() => api.listKeys());
  const [editing, setEditing] = useState<VirtualKey | null>(null);

  const copy = async (id: string) => {
    try {
      await navigator.clipboard.writeText(id);
      toast.success("Key copied to clipboard");
    } catch {
      toast.error("Clipboard unavailable — select the value and copy manually");
    }
  };

  return (
    <>
      <PageHeader
        title="Virtual Keys"
        description="Per-project credentials with their own budgets and limits. Real provider keys stay on the server."
        action={
          <Button onClick={() => setEditing({ ...EMPTY })}>
            <PlusIcon />
            Create key
          </Button>
        }
      />

      <DataState
        loading={keys.loading}
        error={keys.error}
        empty={(keys.data ?? []).length === 0}
        emptyMessage="No virtual keys yet."
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Limits</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-28 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(keys.data ?? []).map((key) => (
                  <TableRow key={key.id}>
                    <TableCell>
                      <div className="font-medium">{key.name}</div>
                      {key.description ? (
                        <div className="text-muted-foreground text-xs">{key.description}</div>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <code className="bg-muted rounded px-1.5 py-0.5 text-xs">{key.id}</code>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      <div>
                        {key.rpm_limit ? `${formatNumber(key.rpm_limit)} rpm` : "unlimited rpm"}
                      </div>
                      <div>
                        {key.budget_usd_daily
                          ? `${formatUSD(key.budget_usd_daily)} / day`
                          : "no daily budget"}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {key.expires_at ? formatDateTime(key.expires_at) : "never"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={key.enabled === false ? "outline" : "secondary"}>
                        {key.enabled === false ? "disabled" : "enabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => void copy(key.id!)}>
                          <CopyIcon />
                          <span className="sr-only">Copy key</span>
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => setEditing({ ...key })}>
                          <PencilIcon />
                          <span className="sr-only">Edit {key.name}</span>
                        </Button>
                        <ConfirmButton
                          title={`Delete ${key.name}?`}
                          description="Applications using this key will start receiving 401s immediately. Usage history is removed with it."
                          successMessage={`Deleted ${key.name}`}
                          onConfirm={async () => {
                            await api.deleteKey(key.id!);
                            keys.reload();
                          }}
                          trigger={
                            <Button variant="ghost" size="icon">
                              <Trash2Icon />
                              <span className="sr-only">Delete {key.name}</span>
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
        <KeyDialog
          key={editing.id ?? "__new__"}
          entry={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            keys.reload();
          }}
        />
      ) : null}
    </>
  );
}

function KeyDialog({
  entry,
  onClose,
  onSaved,
}: {
  entry: VirtualKey;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<VirtualKey>(entry);
  const [modelsText, setModelsText] = useState((entry.models ?? []).join("\n"));
  const [ipsText, setIpsText] = useState((entry.ip_allowlist ?? []).join("\n"));
  const [busy, setBusy] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      const payload: VirtualKey = {
        ...draft,
        models: parseList(modelsText),
        ip_allowlist: parseList(ipsText),
      };
      const saved = draft.id ? await api.updateKey(payload) : await api.createKey(payload);
      toast.success(draft.id ? `Saved ${saved.name}` : `Created ${saved.id}`);
      onSaved();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const numberField = (
    id: keyof VirtualKey,
    label: string,
    hint: string,
    step?: string,
  ) => (
    <div className="space-y-2">
      <Label htmlFor={String(id)}>{label}</Label>
      <Input
        id={String(id)}
        type="number"
        min={0}
        step={step}
        value={(draft[id] as number | undefined) ?? 0}
        onChange={(event) => setDraft({ ...draft, [id]: Number(event.target.value) })}
      />
      <p className="text-muted-foreground text-xs">{hint}</p>
    </div>
  );

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={save}>
          <DialogHeader>
            <DialogTitle>{draft.id ? `Edit ${draft.name}` : "Create virtual key"}</DialogTitle>
            <DialogDescription>
              Zero means unlimited for every budget and rate field.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="key-name">Name</Label>
                <Input
                  id="key-name"
                  value={draft.name}
                  onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="key-description">Description</Label>
                <Input
                  id="key-description"
                  value={draft.description ?? ""}
                  onChange={(event) => setDraft({ ...draft, description: event.target.value })}
                />
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="key-models">Allowed models</Label>
                <Textarea
                  id="key-models"
                  rows={3}
                  placeholder="One per line. Empty allows every model."
                  value={modelsText}
                  onChange={(event) => setModelsText(event.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="key-ips">IP allowlist</Label>
                <Textarea
                  id="key-ips"
                  rows={3}
                  placeholder="One address per line. Empty allows any IP."
                  value={ipsText}
                  onChange={(event) => setIpsText(event.target.value)}
                />
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {numberField("rpm_limit", "Requests / minute", "0 = unlimited")}
              {numberField("budget_tokens_daily", "Daily tokens", "0 = unlimited")}
              {numberField("budget_tokens_monthly", "Monthly tokens", "0 = unlimited")}
              {numberField("budget_usd_daily", "Daily budget (USD)", "0 = unlimited", "0.01")}
              {numberField("budget_usd_monthly", "Monthly budget (USD)", "0 = unlimited", "0.01")}
              <div className="space-y-2">
                <Label htmlFor="key-expires">Expires at</Label>
                <Input
                  id="key-expires"
                  type="datetime-local"
                  value={toLocalInput(draft.expires_at)}
                  onChange={(event) =>
                    setDraft({
                      ...draft,
                      expires_at: event.target.value
                        ? new Date(event.target.value).toISOString()
                        : null,
                    })
                  }
                />
                <p className="text-muted-foreground text-xs">Empty = never expires</p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Switch
                id="key-enabled"
                checked={draft.enabled !== false}
                onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })}
              />
              <Label htmlFor="key-enabled">Enabled</Label>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/** datetime-local wants "YYYY-MM-DDTHH:mm" in local time, not an ISO string. */
function toLocalInput(iso?: string | null) {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
