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
import { api, type Route } from "@/lib/api";
import { parseList } from "@/lib/format";

export function RoutesPage() {
  const t = useT();
  const routes = useResource(() => api.listRoutes());
  const providers = useResource(() => api.listProviders());
  const [editing, setEditing] = useState<Route | null>(null);
  const [isNew, setIsNew] = useState(false);

  const providerNames = (providers.data ?? []).map((p) => p.name);

  return (
    <>
      <PageHeader
        title={t("routes.title")}
        description={t("routes.description")}
        action={
          <Button
            onClick={() => {
              setEditing({ model: "", provider: providerNames[0] ?? "", fallback: [] });
              setIsNew(true);
            }}
            disabled={providerNames.length === 0}
          >
            <PlusIcon />
            {t("routes.add")}
          </Button>
        }
      />

      {providerNames.length === 0 && !providers.loading ? (
<p className="text-muted-foreground text-sm">{t("routes.needProvider")}</p>
      ) : null}

      <DataState
        loading={routes.loading}
        error={routes.error}
        empty={(routes.data ?? []).length === 0}
        emptyMessage={t("routes.empty")}
      >
        <Card>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("common.model")}</TableHead>
                  <TableHead>{t("routes.colPrimary")}</TableHead>
                  <TableHead>{t("routes.colFallback")}</TableHead>
                  <TableHead className="w-24 text-right">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(routes.data ?? []).map((route) => (
                  <TableRow key={route.model}>
                    <TableCell className="font-medium">{route.model}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{route.provider}</Badge>
                    </TableCell>
                    <TableCell>
                      {route.fallback?.length ? (
                        <div className="flex flex-wrap items-center gap-1">
                          {route.fallback.map((name) => (
                            <Badge key={name} variant="outline">
                              {name}
                            </Badge>
                          ))}
                        </div>
                      ) : (
                        <span className="text-muted-foreground">{t("common.none")}</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => {
                            setEditing({ ...route });
                            setIsNew(false);
                          }}
                        >
                          <PencilIcon />
                          <span className="sr-only">{t("common.editSr", { name: route.model })}</span>
                        </Button>
                        <ConfirmButton
                          title={t("routes.deleteTitle", { model: route.model })}
                          description={t("routes.deleteDescription")}
                          successMessage={t("routes.deleted", { model: route.model })}
                          onConfirm={async () => {
                            await api.deleteRoute(route.model);
                            routes.reload();
                          }}
                          trigger={
                            <Button variant="ghost" size="icon">
                              <Trash2Icon />
                              <span className="sr-only">{t("common.deleteSr", { name: route.model })}</span>
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
        <RouteDialog
          key={isNew ? "__new__" : editing.model}
          route={editing}
          isNew={isNew}
          providers={providerNames}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            routes.reload();
          }}
        />
      ) : null}
    </>
  );
}

function RouteDialog({
  route,
  isNew,
  providers,
  onClose,
  onSaved,
}: {
  route: Route;
  isNew: boolean;
  providers: string[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const t = useT();
  const [draft, setDraft] = useState<Route>(route);
  const [fallbackText, setFallbackText] = useState((route.fallback ?? []).join("\n"));
  const [busy, setBusy] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      await api.upsertRoute({ ...draft, fallback: parseList(fallbackText) });
      toast.success(t("routes.saved", { model: draft.model }));
      onSaved();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <form onSubmit={save}>
          <DialogHeader>
            <DialogTitle>
              {isNew ? t("routes.dialogAdd") : t("routes.dialogEdit", { model: draft.model })}
            </DialogTitle>
<DialogDescription>{t("routes.dialogDescription")}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="model">{t("common.model")}</Label>
              <Input
                id="model"
                placeholder="gpt-4o"
                value={draft.model}
                disabled={!isNew}
                onChange={(event) => setDraft({ ...draft, model: event.target.value })}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="provider">{t("routes.colPrimary")}</Label>
              <Select
                value={draft.provider}
                onValueChange={(value) => setDraft({ ...draft, provider: value })}
              >
                <SelectTrigger id="provider" className="w-full">
                  <SelectValue placeholder={t("routes.selectProvider")} />
                </SelectTrigger>
                <SelectContent>
                  {providers.map((name) => (
                    <SelectItem key={name} value={name}>
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="fallback">{t("routes.fallbackLabel")}</Label>
              <Textarea
                id="fallback"
                rows={3}
                placeholder={t("routes.fallbackPlaceholder")}
                value={fallbackText}
                onChange={(event) => setFallbackText(event.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={busy || !draft.provider}>
              {busy ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
