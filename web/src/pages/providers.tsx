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
import { api, type Provider } from "@/lib/api";
import { parseList } from "@/lib/format";

const EMPTY: Provider = { name: "", kind: "", enabled: true };

export function ProvidersPage() {
  const providers = useResource(() => api.listProviders());
  const [editing, setEditing] = useState<Provider | null>(null);
  // A provider is keyed by name, so renaming would create a second row
  // rather than moving one. The field is locked while editing.
  const [isNew, setIsNew] = useState(false);

  const openCreate = () => {
    setEditing({ ...EMPTY });
    setIsNew(true);
  };
  const openEdit = (provider: Provider) => {
    setEditing({ ...provider });
    setIsNew(false);
  };

  return (
    <>
      <PageHeader
        title="Providers"
        description="Upstream vendors the gateway can forward to. Credentials never leave the server."
        action={
          <Button onClick={openCreate}>
            <PlusIcon />
            Add provider
          </Button>
        }
      />

      <DataState
        loading={providers.loading}
        error={providers.error}
        empty={(providers.data ?? []).length === 0}
        emptyMessage="No providers yet. Add one to bring the data plane online."
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Kind</TableHead>
                  <TableHead>Base URL</TableHead>
                  <TableHead>Models</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-24 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(providers.data ?? []).map((provider) => (
                  <TableRow key={provider.name}>
                    <TableCell className="font-medium">{provider.name}</TableCell>
                    <TableCell className="text-muted-foreground">{provider.kind}</TableCell>
                    <TableCell className="text-muted-foreground max-w-[22rem] truncate">
                      {provider.base_url || "default"}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {provider.models?.length ? `${provider.models.length} listed` : "any"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={provider.enabled === false ? "outline" : "secondary"}>
                        {provider.enabled === false ? "disabled" : "enabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => openEdit(provider)}>
                          <PencilIcon />
                          <span className="sr-only">Edit {provider.name}</span>
                        </Button>
                        <ConfirmButton
                          title={`Delete ${provider.name}?`}
                          description="Routes pointing at this provider will stop resolving until they are repointed."
                          successMessage={`Deleted ${provider.name}`}
                          onConfirm={async () => {
                            await api.deleteProvider(provider.name);
                            providers.reload();
                          }}
                          trigger={
                            <Button variant="ghost" size="icon">
                              <Trash2Icon />
                              <span className="sr-only">Delete {provider.name}</span>
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

      {/* Keyed so opening a different row remounts the form with fresh
          state instead of leaking the previous row's edits. */}
      {editing ? (
        <ProviderDialog
          key={isNew ? "__new__" : editing.name}
          provider={editing}
          isNew={isNew}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            providers.reload();
          }}
        />
      ) : null}
    </>
  );
}

function ProviderDialog({
  provider,
  isNew,
  onClose,
  onSaved,
}: {
  provider: Provider;
  isNew: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<Provider>(provider);
  const [modelsText, setModelsText] = useState((provider.models ?? []).join("\n"));
  const [busy, setBusy] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await api.upsertProvider({
        ...draft,
        kind: draft.kind || draft.name,
        models: parseList(modelsText),
      });
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
            <DialogTitle>{isNew ? "Add provider" : `Edit ${draft.name}`}</DialogTitle>
            <DialogDescription>
              Anything left blank falls back to the vendor default baked into the router.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  value={draft.name}
                  disabled={!isNew}
                  onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="kind">Kind</Label>
                <Input
                  id="kind"
                  placeholder="defaults to name"
                  value={draft.kind}
                  onChange={(event) => setDraft({ ...draft, kind: event.target.value })}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="api_key">API key</Label>
              <Input
                id="api_key"
                type="password"
                autoComplete="off"
                value={draft.api_key ?? ""}
                onChange={(event) => setDraft({ ...draft, api_key: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="base_url">Base URL</Label>
              <Input
                id="base_url"
                placeholder="https://api.vendor.com/v1"
                value={draft.base_url ?? ""}
                onChange={(event) => setDraft({ ...draft, base_url: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="models">Models</Label>
              <Textarea
                id="models"
                rows={3}
                placeholder="One per line. Leave empty to allow any model."
                value={modelsText}
                onChange={(event) => setModelsText(event.target.value)}
              />
            </div>

            <div className="grid grid-cols-2 items-end gap-4">
              <div className="space-y-2">
                <Label htmlFor="timeout">Timeout (seconds)</Label>
                <Input
                  id="timeout"
                  type="number"
                  min={0}
                  value={draft.timeout_sec ?? 0}
                  onChange={(event) =>
                    setDraft({ ...draft, timeout_sec: Number(event.target.value) })
                  }
                />
              </div>
              <div className="flex items-center gap-2 pb-2">
                <Switch
                  id="enabled"
                  checked={draft.enabled !== false}
                  onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })}
                />
                <Label htmlFor="enabled">Enabled</Label>
              </div>
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
