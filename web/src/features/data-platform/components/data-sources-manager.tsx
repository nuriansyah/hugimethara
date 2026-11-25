"use client";

import { Database, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@hugimethara/ui/components/dialog";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { useCan } from "@/features/access/api";
import { useEnvironments, useWorkspaces } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import {
  useDataSources,
  useDataSourceKinds,
  useDataSourceMutations,
  type DataSource,
  type DataSourceCreate,
} from "../api";

export function DataSourcesManager() {
  const can = useCan();
  const workspaces = useWorkspaces(false);
  const environments = useEnvironments();
  const [workspaceId, setWorkspaceId] = useState("");
  const [creating, setCreating] = useState(false);
  const sources = useDataSources(workspaceId);
  const mutations = useDataSourceMutations(workspaceId);

  useEffect(() => {
    if (!workspaceId && workspaces.data?.[0]) setWorkspaceId(workspaces.data[0].id);
  }, [workspaceId, workspaces.data]);

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Data sources" description="Register external systems and discover their schemas before ingesting." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Registered sources</CardTitle>
            <CardDescription>Credentials stay in the secret vault; source config never contains a password.</CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Select value={workspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger className="w-52"><SelectValue placeholder="Workspace" /></SelectTrigger>
              <SelectContent>
                {(workspaces.data ?? []).map((w) => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}
              </SelectContent>
            </Select>
            {can("datasource:create", workspaceId) && (
              <Button onClick={() => setCreating(true)} disabled={!workspaceId}><Plus aria-hidden /> Add source</Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {!workspaceId ? <p className="text-sm text-muted-foreground">Choose a workspace.</p> : sources.isPending ? (
            <Skeleton className="h-28 w-full" />
          ) : sources.error ? (
            <QueryError error={sources.error} onRetry={() => sources.refetch()} title="Could not load data sources" />
          ) : sources.data.length === 0 ? (
            <div className="rounded-lg border border-dashed p-9 text-center">
              <Database className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden />
              <p className="text-sm font-medium">No data sources yet</p>
              <p className="mt-1 text-sm text-muted-foreground">Add PostgreSQL, REST or a CSV/JSON file to start ingesting.</p>
            </div>
          ) : (
            <Table>
              <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Kind</TableHead><TableHead>Status</TableHead><TableHead>Updated</TableHead><TableHead className="text-right">Actions</TableHead></TableRow></TableHeader>
              <TableBody>
                {sources.data.map((source) => (
                  <TableRow key={source.id}>
                    <TableCell><span className="font-medium">{source.name}</span><p className="font-mono text-xs text-muted-foreground">{source.slug}</p></TableCell>
                    <TableCell><Badge variant="secondary">{source.kind}</Badge></TableCell>
                    <TableCell><Badge variant={source.status === "enabled" ? "default" : "outline"}>{source.status}</Badge></TableCell>
                    <TableCell className="text-muted-foreground">{new Date(source.updated_at ?? "").toLocaleString()}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button size="sm" variant="outline" onClick={async () => {
                          try {
                            const result = await mutations.test.mutateAsync(source.id);
                            toast.success(result.ok ? `Connected in ${result.latency_ms ?? 0} ms.` : (result.message ?? "Connection failed."));
                          } catch (error) { toastError(error); }
                        }}>Test</Button>
                        <Button size="sm" variant="outline" onClick={async () => {
                          try {
                            const result = await mutations.discover.mutateAsync(source.id);
                            toast.success(`Discovered ${result.schema?.objects?.length ?? 0} objects${result.drift?.length ? `, ${result.drift.length} drift notes` : ""}.`);
                          } catch (error) { toastError(error); }
                        }}><RefreshCw aria-hidden /> Discover</Button>
                        {can("datasource:delete", source.workspace_id) && (
                          <Button size="sm" variant="ghost" onClick={async () => {
                            if (!confirm(`Delete ${source.name}?`)) return;
                            try { await mutations.remove.mutateAsync(source.id); toast.success("Data source removed."); }
                            catch (error) { toastError(error); }
                          }}><Trash2 aria-hidden /></Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <CreateSourceDialog
        open={creating}
        onOpenChange={setCreating}
        workspaceId={workspaceId}
        environments={environments.data ?? []}
        onCreate={async (body) => {
          await mutations.create.mutateAsync(body);
          toast.success("Data source created.");
          setCreating(false);
        }}
      />
    </div>
  );
}

function CreateSourceDialog({ open, onOpenChange, workspaceId, environments, onCreate }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  environments: { id: string; name: string }[];
  onCreate: (body: DataSourceCreate) => Promise<void>;
}) {
  const kinds = useDataSourceKinds();
  const [kind, setKind] = useState("postgresql");
  const [name, setName] = useState("");
  const [environmentId, setEnvironmentId] = useState("");
  const [config, setConfig] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!environmentId && environments[0]) setEnvironmentId(environments[0].id);
  }, [environmentId, environments]);

  const field = (key: string, label: string, placeholder = "", type = "text") => (
    <div className="grid gap-1.5">
      <Label htmlFor={key}>{label}</Label>
      <Input id={key} type={type} placeholder={placeholder} value={config[key] ?? ""}
        onChange={(e) => setConfig((prev) => ({ ...prev, [key]: e.target.value }))} />
    </div>
  );

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      const values: Record<string, unknown> = { ...config };
      if (kind === "postgresql") values.port = Number(config.port || 5432);
      await onCreate({ workspace_id: workspaceId, environment_id: environmentId, name, kind, config: values });
    } catch (error) { toastError(error); }
    finally { setBusy(false); }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Add data source</DialogTitle>
            <DialogDescription>Connection secrets belong in Settings → Credentials; config here is metadata only.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-1.5">
            <Label>Workspace</Label>
            <Input value={workspaceId} disabled />
          </div>
          <div className="grid gap-1.5">
            <Label>Kind</Label>
            <Select value={kind} onValueChange={(value) => { setKind(value); setConfig({}); }}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(kinds.data ?? []).map((k) => <SelectItem key={k.key} value={k.key} disabled={!k.available}>{k.name}{k.available ? "" : " (soon)"}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="source-name">Name</Label>
            <Input id="source-name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="grid gap-1.5">
            <Label>Environment</Label>
            <Select value={environmentId} onValueChange={setEnvironmentId}>
              <SelectTrigger><SelectValue placeholder="Environment" /></SelectTrigger>
              <SelectContent>{environments.map((e) => <SelectItem key={e.id} value={e.id}>{e.name}</SelectItem>)}</SelectContent>
            </Select>
          </div>
          {kind === "postgresql" && (
            <>
              {field("host", "Host", "db.internal")}
              {field("port", "Port", "5432", "number")}
              {field("database", "Database", "analytics")}
              {field("user", "User", "readonly")}
              {field("sslmode", "SSL mode", "require")}
            </>
          )}
          {kind === "rest" && (
            <>
              {field("base_url", "Base URL", "https://api.example.com")}
              {field("path", "Path", "/v1/records")}
              {field("records_path", "Records path", "data.items")}
            </>
          )}
          {kind === "file" && (
            <>
              <div className="grid gap-1.5">
                <Label>Format</Label>
                <Select value={config.format ?? "csv"} onValueChange={(value) => setConfig((prev) => ({ ...prev, format: value }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent><SelectItem value="csv">CSV</SelectItem><SelectItem value="json">JSON</SelectItem></SelectContent>
                </Select>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="file-content">Content</Label>
                <Textarea id="file-content" rows={6} value={config.content ?? ""}
                  onChange={(e) => setConfig((prev) => ({ ...prev, content: e.target.value }))} />
              </div>
            </>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={busy || !name || !environmentId || !workspaceId}>Create source</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
