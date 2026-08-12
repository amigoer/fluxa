import { useState } from "react";
import { PencilIcon, PlusIcon, SearchIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { ConfirmButton } from "@/components/confirm-button";
import { ProviderLogo } from "@/components/provider-logo";
import { DataState, PageHeader } from "@/components/page";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { useT } from "@/lib/i18n";
import { api, type Provider } from "@/lib/api";
import { parseList } from "@/lib/format";
import { cn } from "@/lib/utils";

const EMPTY: Provider = { name: "", kind: "", enabled: true };

export function ProvidersPage() {
  const t = useT();
  const providers = useResource(() => api.listProviders());
  const [search, setSearch] = useState("");
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

  // Filtering client-side: the provider list is tens of rows, not
  // thousands, so a round trip per keystroke would be slower than this.
  const all = providers.data ?? [];
  const needle = search.trim().toLowerCase();
  const rows = needle
    ? all.filter((p) =>
        [p.name, p.kind, p.base_url ?? ""].some((field) =>
          field.toLowerCase().includes(needle),
        ),
      )
    : all;

  return (
    <>
      <PageHeader
        eyebrow={t("nav.group.gateway")}
        title={t("providers.title")}
        description={t("providers.description")}
        action={
          <Button onClick={openCreate}>
            <PlusIcon />
            {t("providers.add")}
          </Button>
        }
      />

      <DataState
        loading={providers.loading}
        error={providers.error}
        empty={all.length === 0}
        emptyMessage={t("providers.empty")}
      >
        <Card className="gap-0 py-0">
          {/* A sunken strip separates "what you can do to this table" from
              the table itself; without it controls and data blur together. */}
          <CardHeader className="bg-surface-sunken gap-0 rounded-t-xl border-b px-4 py-3">
            <div className="flex flex-wrap items-center gap-3">
              <CardTitle className="text-sm">{t("providers.listTitle")}</CardTitle>
              <span className="bg-background text-muted-foreground rounded-full border px-2 py-0.5 text-xs tabular-nums">
                {t("providers.count", { count: all.length })}
              </span>
              <div className="relative ml-auto w-full max-w-xs">
                <SearchIcon className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                <Input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={t("providers.searchPlaceholder")}
                  className="bg-background pl-8"
                />
              </div>
            </div>
          </CardHeader>
          <CardContent className="px-0 pb-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("common.name")}</TableHead>
                  <TableHead>{t("providers.colKind")}</TableHead>
                  <TableHead>{t("providers.colBaseUrl")}</TableHead>
                  <TableHead>{t("providers.colModels")}</TableHead>
                  <TableHead>{t("common.status")}</TableHead>
                  <TableHead className="w-36 text-right">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((provider) => (
                  <TableRow key={provider.name}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2.5">
                        <ProviderLogo
                          kind={provider.kind}
                          name={provider.name}
                          className={cn(provider.enabled === false && "opacity-45 grayscale")}
                        />
                        {provider.name}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{provider.kind}</TableCell>
                    <TableCell className="text-muted-foreground max-w-[22rem] truncate font-mono text-xs">
                      {provider.base_url || t("providers.baseUrlDefault")}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {provider.models?.length
                        ? t("providers.modelsListed", { count: provider.models.length })
                        : t("common.any")}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="secondary"
                        className={cn(
                          "font-medium",
                          provider.enabled === false
                            ? "text-muted-foreground"
                            : "bg-success-subtle text-success-emphasis",
                        )}
                      >
                        {provider.enabled === false ? t("common.disabled") : t("common.enabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="outline" size="sm" onClick={() => openEdit(provider)}>
                          <PencilIcon />
                          {t("common.edit")}
                        </Button>
                        <ConfirmButton
                          title={t("providers.deleteTitle", { name: provider.name })}
                          description={t("providers.deleteDescription")}
                          successMessage={t("providers.deleted", { name: provider.name })}
                          onConfirm={async () => {
                            await api.deleteProvider(provider.name);
                            providers.reload();
                          }}
                          trigger={
                            <Button
                              variant="ghost"
                              size="icon"
                              className="text-muted-foreground hover:text-destructive"
                            >
                              <Trash2Icon />
                              <span className="sr-only">
                                {t("common.deleteSr", { name: provider.name })}
                              </span>
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
  const t = useT();
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
      toast.success(t("providers.saved", { name: draft.name }));
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
              {isNew ? t("providers.dialogAdd") : t("providers.dialogEdit", { name: draft.name })}
            </DialogTitle>
<DialogDescription>{t("providers.dialogDescription")}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="name">{t("common.name")}</Label>
                <Input
                  id="name"
                  value={draft.name}
                  disabled={!isNew}
                  onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="kind">{t("providers.kind")}</Label>
                <Input
                  id="kind"
                  placeholder={t("providers.kindPlaceholder")}
                  value={draft.kind}
                  onChange={(event) => setDraft({ ...draft, kind: event.target.value })}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="api_key">{t("providers.apiKey")}</Label>
              <Input
                id="api_key"
                type="password"
                autoComplete="off"
                value={draft.api_key ?? ""}
                onChange={(event) => setDraft({ ...draft, api_key: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="base_url">{t("providers.baseUrl")}</Label>
              <Input
                id="base_url"
                placeholder="https://api.vendor.com/v1"
                value={draft.base_url ?? ""}
                onChange={(event) => setDraft({ ...draft, base_url: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="models">{t("providers.models")}</Label>
              <Textarea
                id="models"
                rows={3}
                placeholder={t("providers.modelsPlaceholder")}
                value={modelsText}
                onChange={(event) => setModelsText(event.target.value)}
              />
            </div>

            <div className="grid grid-cols-2 items-end gap-4">
              <div className="space-y-2">
                <Label htmlFor="timeout">{t("providers.timeout")}</Label>
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
                <Label htmlFor="enabled">{t("common.enabled")}</Label>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
