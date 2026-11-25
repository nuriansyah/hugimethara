"use client";

import { Database, GitBranch, Play, Plus, Send, Trash2 } from "lucide-react";
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
import { useWorkspaces } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import {
  useDatasetLineage,
  useDatasetMutations,
  useDatasetPolicies,
  useDatasetVersions,
  useDatasets,
  type Dataset,
  type DatasetCreate,
  type DatasetPreview,
} from "../api";

export function DatasetsManager() {
  const can = useCan();
  const workspaces = useWorkspaces(false);
  const [workspaceId, setWorkspaceId] = useState("");
  const [creating, setCreating] = useState(false);
  const [selectedId, setSelectedId] = useState<string>();
  const datasets = useDatasets(workspaceId);
  const mutations = useDatasetMutations(workspaceId);
  const selected = datasets.data?.find((dataset) => dataset.id === selectedId);

  useEffect(() => {
    if (!workspaceId && workspaces.data?.[0]) setWorkspaceId(workspaces.data[0].id);
  }, [workspaceId, workspaces.data]);

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Datasets" description="Versioned SQL models over raw tables, with previews, lineage and governance policies." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div><CardTitle>Models</CardTitle><CardDescription>Publishing creates a ClickHouse view used by the workbench, metrics and dashboards.</CardDescription></div>
          <div className="flex items-center gap-2">
            <Select value={workspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger className="w-52"><SelectValue placeholder="Workspace" /></SelectTrigger>
              <SelectContent>{(workspaces.data ?? []).map((w) => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}</SelectContent>
            </Select>
            {can("dataset:create", workspaceId) && <Button onClick={() => setCreating(true)} disabled={!workspaceId}><Plus aria-hidden /> New dataset</Button>}
          </div>
        </CardHeader>
        <CardContent>
          {!workspaceId ? <p className="text-sm text-muted-foreground">Choose a workspace.</p> : datasets.isPending ? <Skeleton className="h-28 w-full" /> : datasets.error ? (
            <QueryError error={datasets.error} onRetry={() => datasets.refetch()} title="Could not load datasets" />
          ) : datasets.data.length === 0 ? (
            <div className="rounded-lg border border-dashed p-9 text-center">
              <Database className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden />
              <p className="text-sm font-medium">No datasets yet</p>
              <p className="mt-1 text-sm text-muted-foreground">Create a model over an ingested raw table, then publish it.</p>
            </div>
          ) : (
            <Table>
              <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Owner</TableHead><TableHead>Status</TableHead><TableHead>Updated</TableHead><TableHead className="text-right">Actions</TableHead></TableRow></TableHeader>
              <TableBody>
                {datasets.data.map((dataset) => (
                  <TableRow key={dataset.id} className="cursor-pointer" onClick={() => setSelectedId(dataset.id)}>
                    <TableCell><span className="font-medium">{dataset.name}</span><p className="font-mono text-xs text-muted-foreground">{dataset.slug}</p></TableCell>
                    <TableCell>{dataset.owner || "—"}</TableCell>
                    <TableCell><Badge variant={dataset.status === "published" ? "default" : "outline"}>{dataset.status}</Badge></TableCell>
                    <TableCell className="text-muted-foreground">{new Date(dataset.updated_at ?? "").toLocaleString()}</TableCell>
                    <TableCell className="text-right">
                      {can("dataset:delete", dataset.workspace_id) && (
                        <Button size="sm" variant="ghost" onClick={async (event) => {
                          event.stopPropagation();
                          if (!confirm(`Delete ${dataset.name}?`)) return;
                          try { await mutations.remove.mutateAsync(dataset.id); toast.success("Dataset removed."); if (selectedId === dataset.id) setSelectedId(undefined); }
                          catch (error) { toastError(error); }
                        }}><Trash2 aria-hidden /></Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      {selected && (
        <DatasetDetail
          key={selected.id}
          dataset={selected}
          mutations={mutations}
          canEdit={can("dataset:update", selected.workspace_id)}
          canPublish={can("dataset:update", selected.workspace_id)}
        />
      )}
      <CreateDatasetDialog
        open={creating}
        onOpenChange={setCreating}
        workspaceId={workspaceId}
        onCreate={async (body) => { await mutations.create.mutateAsync(body); toast.success("Dataset created."); setCreating(false); }}
      />
    </div>
  );
}

function DatasetDetail({ dataset, mutations, canEdit, canPublish }: {
  dataset: Dataset;
  mutations: ReturnType<typeof useDatasetMutations>;
  canEdit: boolean;
  canPublish: boolean;
}) {
  const versions = useDatasetVersions(dataset.id);
  const lineage = useDatasetLineage(dataset.id);
  const policies = useDatasetPolicies(dataset.id);
  const [sql, setSql] = useState(dataset.draft_sql ?? "");
  const [preview, setPreview] = useState<DatasetPreview>();
  const [column, setColumn] = useState("");
  const [classification, setClassification] = useState("pii");
  const [mask, setMask] = useState("email");
  const [roleKey, setRoleKey] = useState("");
  const [rowFilter, setRowFilter] = useState("");

  return (
    <Card>
      <CardHeader><CardTitle>{dataset.name}</CardTitle><CardDescription>{dataset.description || "No description."}</CardDescription></CardHeader>
      <CardContent className="grid gap-6">
        <div className="grid gap-2">
          <Label htmlFor="dataset-sql">SQL</Label>
          <Textarea id="dataset-sql" rows={10} className="font-mono text-xs" value={sql} onChange={(e) => setSql(e.target.value)} disabled={!canEdit} />
          <div className="flex flex-wrap gap-2">
            {canEdit && <Button size="sm" variant="outline" onClick={async () => {
              try { await mutations.update.mutateAsync({ id: dataset.id, body: { sql } }); toast.success("Draft saved."); }
              catch (error) { toastError(error); }
            }}>Save draft</Button>}
            {canPublish && <Button size="sm" onClick={async () => {
              try { await mutations.publish.mutateAsync(dataset.id); toast.success("Dataset published."); }
              catch (error) { toastError(error); }
            }}><Send aria-hidden /> Publish</Button>}
            <Button size="sm" variant="outline" onClick={async () => {
              try { setPreview(await mutations.preview.mutateAsync({ id: dataset.id, sql })); }
              catch (error) { toastError(error); }
            }}><Play aria-hidden /> Preview</Button>
          </div>
        </div>
        {preview && (
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader><TableRow>{preview.columns.map((column) => <TableHead key={column}>{column}</TableHead>)}</TableRow></TableHeader>
              <TableBody>
                {preview.rows.map((row, index) => (
                  <TableRow key={index}>{row.map((value, cell) => <TableCell key={cell} className="font-mono text-xs">{value == null ? "—" : String(value)}</TableCell>)}</TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        <div className="grid gap-2">
          <h3 className="text-sm font-medium">Versions</h3>
          {versions.isPending ? <Skeleton className="h-12 w-full" /> : versions.data?.length ? (
            <div className="flex flex-wrap gap-2">{versions.data.map((version) => <Badge key={version.id} variant="outline">v{version.version} · {new Date(version.published_at ?? "").toLocaleDateString()}</Badge>)}</div>
          ) : <p className="text-sm text-muted-foreground">Not published yet.</p>}
        </div>
        <div className="grid gap-2">
          <h3 className="flex items-center gap-2 text-sm font-medium"><GitBranch aria-hidden className="size-4" /> Lineage</h3>
          {lineage.data?.length ? <ul className="space-y-1 text-sm">{lineage.data.map((edge) => <li key={`${edge.upstream_kind}:${edge.upstream_id}`} className="font-mono text-xs">{edge.upstream_kind} · {edge.upstream_id} ({edge.columns?.join(", ")})</li>)}</ul> : <p className="text-sm text-muted-foreground">No upstream edges recorded.</p>}
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          <div className="grid gap-2">
            <h3 className="text-sm font-medium">Column policies</h3>
            {policies.columns.data?.length ? <ul className="space-y-1 text-sm">{policies.columns.data.map((policy) => <li key={policy.column_name} className="flex items-center justify-between gap-2"><span className="font-mono text-xs">{policy.column_name}</span><span className="text-muted-foreground">{policy.classification}{policy.mask ? ` · ${policy.mask}` : ""}</span></li>)}</ul> : <p className="text-sm text-muted-foreground">No column policies.</p>}
            {canEdit && (
              <div className="grid gap-2 rounded-md border p-3">
                <Input placeholder="column_name" value={column} onChange={(e) => setColumn(e.target.value)} />
                <Input placeholder="classification" value={classification} onChange={(e) => setClassification(e.target.value)} />
                <Input placeholder="mask (email, phone, last4, redact, hash)" value={mask} onChange={(e) => setMask(e.target.value)} />
                <Button size="sm" variant="outline" disabled={!column} onClick={async () => {
                  try { await mutations.putColumnPolicy.mutateAsync({ id: dataset.id, column, body: { classification, mask } }); toast.success("Column policy saved."); setColumn(""); }
                  catch (error) { toastError(error); }
                }}>Add policy</Button>
              </div>
            )}
          </div>
          <div className="grid gap-2">
            <h3 className="text-sm font-medium">Row policies</h3>
            {policies.rows.data?.length ? <ul className="space-y-1 text-sm">{policies.rows.data.map((policy) => <li key={policy.role_key} className="font-mono text-xs">{policy.role_key}: {policy.row_filter || "all rows"}</li>)}</ul> : <p className="text-sm text-muted-foreground">No row policies.</p>}
            {canEdit && (
              <div className="grid gap-2 rounded-md border p-3">
                <Input placeholder="role key (e.g. analyst)" value={roleKey} onChange={(e) => setRoleKey(e.target.value)} />
                <Input placeholder="row filter (e.g. region = 'EU')" value={rowFilter} onChange={(e) => setRowFilter(e.target.value)} />
                <Button size="sm" variant="outline" disabled={!roleKey} onClick={async () => {
                  try { await mutations.putRowPolicy.mutateAsync({ id: dataset.id, roleKey, body: { row_filter: rowFilter } }); toast.success("Row policy saved."); setRoleKey(""); setRowFilter(""); }
                  catch (error) { toastError(error); }
                }}>Add policy</Button>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function CreateDatasetDialog({ open, onOpenChange, workspaceId, onCreate }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  onCreate: (body: DatasetCreate) => Promise<void>;
}) {
  const [form, setForm] = useState({ name: "", slug: "", description: "", sql: "" });
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof form) => (value: string) => setForm((prev) => ({ ...prev, [key]: value }));
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="grid gap-4" onSubmit={async (event) => {
          event.preventDefault(); setBusy(true);
          try { await onCreate({ workspace_id: workspaceId, ...form }); }
          catch (error) { toastError(error); } finally { setBusy(false); }
        }}>
          <DialogHeader><DialogTitle>New dataset</DialogTitle><DialogDescription>Write a read-only SELECT over raw tables or published datasets.</DialogDescription></DialogHeader>
          <div className="grid gap-1.5"><Label htmlFor="dataset-name">Name</Label><Input id="dataset-name" value={form.name} onChange={(e) => set("name")(e.target.value)} required /></div>
          <div className="grid gap-1.5"><Label htmlFor="dataset-slug">Slug (optional)</Label><Input id="dataset-slug" value={form.slug} onChange={(e) => set("slug")(e.target.value)} placeholder="orders-clean" /></div>
          <div className="grid gap-1.5"><Label htmlFor="dataset-description">Description</Label><Input id="dataset-description" value={form.description} onChange={(e) => set("description")(e.target.value)} /></div>
          <div className="grid gap-1.5"><Label htmlFor="dataset-draft">SQL</Label><Textarea id="dataset-draft" rows={8} className="font-mono text-xs" value={form.sql} onChange={(e) => set("sql")(e.target.value)} placeholder="SELECT * FROM raw_orders" /></div>
          <DialogFooter><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button><Button type="submit" disabled={busy || !form.name}>Create</Button></DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
