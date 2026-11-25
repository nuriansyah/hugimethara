"use client";

import { Play, Plus, Trash2 } from "lucide-react";
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
import { useCan } from "@/features/access/api";
import { useWorkspaces } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import { useDataSources, usePipelineMutations, usePipelineRuns, usePipelines, type PipelineCreate } from "../api";

export function PipelinesManager() {
  const can = useCan();
  const workspaces = useWorkspaces(false);
  const [workspaceId, setWorkspaceId] = useState("");
  const [creating, setCreating] = useState(false);
  const [selected, setSelected] = useState<string>();
  const pipelines = usePipelines(workspaceId);
  const sources = useDataSources(workspaceId);
  const mutations = usePipelineMutations(workspaceId);

  useEffect(() => {
    if (!workspaceId && workspaces.data?.[0]) setWorkspaceId(workspaces.data[0].id);
  }, [workspaceId, workspaces.data]);

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Ingestion pipelines" description="Copy source objects into the analytical store on a schedule or on demand." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div><CardTitle>Pipelines</CardTitle><CardDescription>Full refresh or an incremental watermark into a ClickHouse raw table.</CardDescription></div>
          <div className="flex items-center gap-2">
            <Select value={workspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger className="w-52"><SelectValue placeholder="Workspace" /></SelectTrigger>
              <SelectContent>{(workspaces.data ?? []).map((w) => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}</SelectContent>
            </Select>
            {can("datasource:create", workspaceId) && <Button onClick={() => setCreating(true)} disabled={!workspaceId}><Plus aria-hidden /> Add pipeline</Button>}
          </div>
        </CardHeader>
        <CardContent>
          {!workspaceId ? <p className="text-sm text-muted-foreground">Choose a workspace.</p> : pipelines.isPending ? <Skeleton className="h-28 w-full" /> : pipelines.error ? (
            <QueryError error={pipelines.error} onRetry={() => pipelines.refetch()} title="Could not load pipelines" />
          ) : pipelines.data.length === 0 ? (
            <p className="rounded-lg border border-dashed p-9 text-center text-sm text-muted-foreground">No pipelines yet. Add a data source first, then create a pipeline.</p>
          ) : (
            <Table>
              <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Target</TableHead><TableHead>Mode</TableHead><TableHead>Schedule</TableHead><TableHead>Last run</TableHead><TableHead className="text-right">Actions</TableHead></TableRow></TableHeader>
              <TableBody>
                {pipelines.data.map((pipeline) => (
                  <TableRow key={pipeline.id} className="cursor-pointer" onClick={() => setSelected(pipeline.id)}>
                    <TableCell><span className="font-medium">{pipeline.name}</span><p className="font-mono text-xs text-muted-foreground">{pipeline.slug}</p></TableCell>
                    <TableCell className="font-mono text-xs">{pipeline.target_table}</TableCell>
                    <TableCell><Badge variant="secondary">{pipeline.mode}</Badge></TableCell>
                    <TableCell className="font-mono text-xs">{pipeline.schedule_cron || "manual"}</TableCell>
                    <TableCell className="text-muted-foreground">{pipeline.last_run_at ? new Date(pipeline.last_run_at).toLocaleString() : "never"}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button size="sm" variant="outline" onClick={async (event) => {
                          event.stopPropagation();
                          try { await mutations.run.mutateAsync(pipeline.id); toast.success("Pipeline run started."); setSelected(pipeline.id); }
                          catch (error) { toastError(error); }
                        }}><Play aria-hidden /> Run</Button>
                        {can("datasource:delete", pipeline.workspace_id) && (
                          <Button size="sm" variant="ghost" onClick={async (event) => {
                            event.stopPropagation();
                            if (!confirm(`Delete ${pipeline.name}?`)) return;
                            try { await mutations.remove.mutateAsync(pipeline.id); toast.success("Pipeline removed."); }
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
      {selected && <RunHistory pipelineId={selected} />}
      <CreatePipelineDialog
        open={creating}
        onOpenChange={setCreating}
        workspaceId={workspaceId}
        sources={(sources.data ?? []).map((s) => ({ id: s.id, name: s.name }))}
        onCreate={async (body) => {
          await mutations.create.mutateAsync(body);
          toast.success("Pipeline created.");
          setCreating(false);
        }}
      />
    </div>
  );
}

function RunHistory({ pipelineId }: { pipelineId: string }) {
  const runs = usePipelineRuns(pipelineId);
  return (
    <Card>
      <CardHeader><CardTitle>Recent runs</CardTitle><CardDescription>Newest first; refresh after a manual run.</CardDescription></CardHeader>
      <CardContent>
        {runs.isPending ? <Skeleton className="h-20 w-full" /> : runs.error ? <QueryError error={runs.error} onRetry={() => runs.refetch()} title="Could not load runs" /> : runs.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No runs yet.</p>
        ) : (
          <Table>
            <TableHeader><TableRow><TableHead>Status</TableHead><TableHead>Mode</TableHead><TableHead>Read</TableHead><TableHead>Written</TableHead><TableHead>Started</TableHead></TableRow></TableHeader>
            <TableBody>
              {runs.data.map((run) => (
                <TableRow key={run.id}>
                  <TableCell><Badge variant={run.status === "succeeded" ? "default" : run.status === "failed" ? "destructive" : "secondary"}>{run.status}</Badge></TableCell>
                  <TableCell>{run.mode}</TableCell>
                  <TableCell>{run.rows_read}</TableCell>
                  <TableCell>{run.rows_written}</TableCell>
                  <TableCell className="text-muted-foreground">{run.started_at ? new Date(run.started_at).toLocaleString() : "queued"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function CreatePipelineDialog({ open, onOpenChange, workspaceId, sources, onCreate }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  sources: { id: string; name: string }[];
  onCreate: (body: PipelineCreate) => Promise<void>;
}) {
  const [form, setForm] = useState({ data_source_id: "", name: "", target_table: "", mode: "full", source_object: "", watermark_column: "", watermark_type: "timestamp", schedule_cron: "" });
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof form) => (value: string) => setForm((prev) => ({ ...prev, [key]: value }));

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    try {
      await onCreate({
        workspace_id: workspaceId,
        data_source_id: form.data_source_id,
        name: form.name,
        target_table: form.target_table,
        mode: form.mode,
        source_object: form.source_object,
        watermark_column: form.watermark_column,
        watermark_type: form.watermark_type,
        schedule_cron: form.schedule_cron,
      });
    } catch (error) { toastError(error); }
    finally { setBusy(false); }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader><DialogTitle>Add pipeline</DialogTitle><DialogDescription>The target table is created on first run with the discovered columns.</DialogDescription></DialogHeader>
          <div className="grid gap-1.5">
            <Label>Data source</Label>
            <Select value={form.data_source_id} onValueChange={set("data_source_id")}>
              <SelectTrigger><SelectValue placeholder="Source" /></SelectTrigger>
              <SelectContent>{sources.map((s) => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}</SelectContent>
            </Select>
          </div>
          <div className="grid gap-1.5"><Label htmlFor="pipeline-name">Name</Label><Input id="pipeline-name" value={form.name} onChange={(e) => set("name")(e.target.value)} required /></div>
          <div className="grid gap-1.5"><Label htmlFor="pipeline-target">Target table</Label><Input id="pipeline-target" value={form.target_table} onChange={(e) => set("target_table")(e.target.value)} placeholder="raw_orders" required /></div>
          <div className="grid gap-1.5"><Label htmlFor="pipeline-object">Source object</Label><Input id="pipeline-object" value={form.source_object} onChange={(e) => set("source_object")(e.target.value)} placeholder="public.orders" required /></div>
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label>Mode</Label>
              <Select value={form.mode} onValueChange={set("mode")}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem value="full">Full refresh</SelectItem><SelectItem value="incremental">Incremental</SelectItem></SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label>Watermark type</Label>
              <Select value={form.watermark_type} onValueChange={set("watermark_type")}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem value="timestamp">Timestamp</SelectItem><SelectItem value="integer">Integer</SelectItem></SelectContent>
              </Select>
            </div>
          </div>
          {form.mode === "incremental" && <div className="grid gap-1.5"><Label htmlFor="pipeline-watermark">Watermark column</Label><Input id="pipeline-watermark" value={form.watermark_column} onChange={(e) => set("watermark_column")(e.target.value)} placeholder="updated_at" /></div>}
          <div className="grid gap-1.5"><Label htmlFor="pipeline-cron">Schedule (cron)</Label><Input id="pipeline-cron" value={form.schedule_cron} onChange={(e) => set("schedule_cron")(e.target.value)} placeholder="0 * * * * (hourly)" /></div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={busy || !form.data_source_id || !form.name || !form.target_table || !form.source_object}>Create pipeline</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
