import { useState } from "react";
import { PencilIcon, PlusIcon, Trash2Icon, XIcon } from "lucide-react";
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
import { useResource } from "@/hooks/use-resource";
import { api, type VirtualModel, type VirtualModelRoute } from "@/lib/api";

const EMPTY_TARGET: VirtualModelRoute = {
  weight: 1,
  target_type: "real",
  target_model: "",
  provider: "",
  enabled: true,
};

export function VirtualModelsPage() {
  const models = useResource(() => api.listVirtualModels());
  const providers = useResource(() => api.listProviders());
  const [editing, setEditing] = useState<VirtualModel | null>(null);
  const [isNew, setIsNew] = useState(false);

  return (
    <>
      <PageHeader
        title="Virtual Models"
        description="One caller-facing name that fans out to weighted targets. Weights drive a random pick per request."
        action={
          <Button
            onClick={() => {
              setEditing({ name: "", description: "", enabled: true, routes: [{ ...EMPTY_TARGET }] });
              setIsNew(true);
            }}
          >
            <PlusIcon />
            Add virtual model
          </Button>
        }
      />

      <DataState
        loading={models.loading}
        error={models.error}
        empty={(models.data ?? []).length === 0}
        emptyMessage="No virtual models yet."
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Targets</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-24 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(models.data ?? []).map((model) => {
                  const total = model.routes.reduce((sum, route) => sum + route.weight, 0) || 1;
                  return (
                    <TableRow key={model.name}>
                      <TableCell>
                        <div className="font-medium">{model.name}</div>
                        {model.description ? (
                          <div className="text-muted-foreground text-xs">{model.description}</div>
                        ) : null}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {model.routes.map((route) => (
                            <Badge
                              key={route.id ?? `${route.target_model}-${route.position}`}
                              variant="outline"
                            >
                              {route.target_model}
                              <span className="text-muted-foreground ml-1">
                                {Math.round((route.weight / total) * 100)}%
                              </span>
                            </Badge>
                          ))}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={model.enabled === false ? "outline" : "secondary"}>
                          {model.enabled === false ? "disabled" : "enabled"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => {
                              setEditing(structuredClone(model));
                              setIsNew(false);
                            }}
                          >
                            <PencilIcon />
                            <span className="sr-only">Edit {model.name}</span>
                          </Button>
                          <ConfirmButton
                            title={`Delete ${model.name}?`}
                            description="Requests using this alias will stop resolving."
                            successMessage={`Deleted ${model.name}`}
                            onConfirm={async () => {
                              await api.deleteVirtualModel(model.name);
                              models.reload();
                            }}
                            trigger={
                              <Button variant="ghost" size="icon">
                                <Trash2Icon />
                                <span className="sr-only">Delete {model.name}</span>
                              </Button>
                            }
                          />
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </DataState>

      {editing ? (
        <VirtualModelDialog
          key={isNew ? "__new__" : editing.name}
          model={editing}
          isNew={isNew}
          providers={(providers.data ?? []).map((p) => p.name)}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            models.reload();
          }}
        />
      ) : null}
    </>
  );
}

function VirtualModelDialog({
  model,
  isNew,
  providers,
  onClose,
  onSaved,
}: {
  model: VirtualModel;
  isNew: boolean;
  providers: string[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState<VirtualModel>(model);
  const [busy, setBusy] = useState(false);

  const patchTarget = (index: number, patch: Partial<VirtualModelRoute>) =>
    setDraft({
      ...draft,
      routes: draft.routes.map((route, i) => (i === index ? { ...route, ...patch } : route)),
    });

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await api.upsertVirtualModel(draft);
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
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={save}>
          <DialogHeader>
            <DialogTitle>{isNew ? "Add virtual model" : `Edit ${draft.name}`}</DialogTitle>
            <DialogDescription>
              A target of type "real" needs a provider; "virtual" points at another alias and is
              resolved recursively.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="vm-name">Name</Label>
                <Input
                  id="vm-name"
                  placeholder="qwen-latest"
                  value={draft.name}
                  disabled={!isNew}
                  onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="vm-description">Description</Label>
                <Input
                  id="vm-description"
                  value={draft.description ?? ""}
                  onChange={(event) => setDraft({ ...draft, description: event.target.value })}
                />
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Switch
                id="vm-enabled"
                checked={draft.enabled !== false}
                onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })}
              />
              <Label htmlFor="vm-enabled">Enabled</Label>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>Targets</Label>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setDraft({ ...draft, routes: [...draft.routes, { ...EMPTY_TARGET }] })}
                >
                  <PlusIcon />
                  Add target
                </Button>
              </div>

              <div className="space-y-2">
                {draft.routes.map((route, index) => (
                  <div
                    key={index}
                    className="grid items-end gap-2 rounded-md border p-3 sm:grid-cols-[7rem_1fr_1fr_5rem_auto]"
                  >
                    <div className="space-y-1">
                      <Label className="text-xs">Type</Label>
                      <Select
                        value={route.target_type}
                        onValueChange={(value) =>
                          patchTarget(index, { target_type: value as "real" | "virtual" })
                        }
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="real">real</SelectItem>
                          <SelectItem value="virtual">virtual</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-1">
                      <Label className="text-xs">Target model</Label>
                      <Input
                        value={route.target_model}
                        onChange={(event) =>
                          patchTarget(index, { target_model: event.target.value })
                        }
                        required
                      />
                    </div>

                    <div className="space-y-1">
                      <Label className="text-xs">Provider</Label>
                      <Select
                        value={route.provider || "__none__"}
                        onValueChange={(value) =>
                          patchTarget(index, { provider: value === "__none__" ? "" : value })
                        }
                        disabled={route.target_type === "virtual"}
                      >
                        <SelectTrigger className="w-full">
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

                    <div className="space-y-1">
                      <Label className="text-xs">Weight</Label>
                      <Input
                        type="number"
                        min={1}
                        value={route.weight}
                        onChange={(event) =>
                          patchTarget(index, { weight: Math.max(1, Number(event.target.value)) })
                        }
                      />
                    </div>

                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      disabled={draft.routes.length === 1}
                      onClick={() =>
                        setDraft({ ...draft, routes: draft.routes.filter((_, i) => i !== index) })
                      }
                    >
                      <XIcon />
                      <span className="sr-only">Remove target</span>
                    </Button>
                  </div>
                ))}
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
