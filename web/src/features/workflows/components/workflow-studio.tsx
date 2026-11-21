"use client";

import { CheckCircle2, Play, Plus, Rocket, Save, ShieldCheck, Trash2, Workflow as WorkflowIcon } from "lucide-react";
import { useMemo, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@hugimethara/ui/components/dialog";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Switch } from "@hugimethara/ui/components/switch";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { useCan } from "@/features/access/api";
import { useEnvironments, useWorkspaces } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import {
  useNodeTypes,
  useWorkflowDeployments,
  useWorkflowExecutions,
  useWorkflowMutations,
  useWorkflows,
  useWorkflowVersions,
  type NodeType,
  type Workflow,
  type WorkflowDefinition,
  type WorkflowIssue,
} from "../api";
import { ExecutionDetail, StatusBadge } from "./execution-detail";
import { NodeInspector } from "./node-inspector";
import { WorkflowCanvas } from "./workflow-canvas";

function nextNodeId(def: WorkflowDefinition, type: string) {
  const base = type.split(".").pop()!.replace(/[^a-z]/gi, "") || "node";
  let i = 1;
  while (def.nodes.some((n) => n.id === `${base}${i}`)) i++;
  return `${base}${i}`;
}

function slugify(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 63);
}

export function WorkflowStudio() {
  const workspaces = useWorkspaces(false);
  const [workspaceId, setWorkspaceId] = useState("");
  const resolvedWorkspaceId = workspaceId || workspaces.data?.[0]?.id || "";
  const workflows = useWorkflows(resolvedWorkspaceId);
  const catalog = useNodeTypes();
  const [selectedId, setSelectedId] = useState("");
  const selected = workflows.data?.find((w) => w.id === selectedId) ?? workflows.data?.[0];
  const [creating, setCreating] = useState(false);

  if (workspaces.isError) {
    return <QueryError error={workspaces.error} onRetry={() => workspaces.refetch()} title="Workspaces unavailable" />;
  }
  return (
    <div className="mx-auto flex max-w-7xl flex-col gap-6">
      <PageHeader
        title="Workflows"
        description="Design a workflow on the canvas, validate and publish an immutable version, deploy it to an environment and run it. Every step's input, output and error is kept for inspection."
      />
      <Card>
        <CardContent className="flex flex-wrap items-end gap-3 pt-6">
          <div className="flex min-w-48 flex-col gap-1.5">
            <Label htmlFor="wf-workspace">Workspace</Label>
            <Select value={resolvedWorkspaceId} onValueChange={(v) => { setWorkspaceId(v); setSelectedId(""); }}>
              <SelectTrigger id="wf-workspace" className="w-full">
                <SelectValue placeholder="Choose a workspace" />
              </SelectTrigger>
              <SelectContent>
                {(workspaces.data ?? []).map((w) => (
                  <SelectItem key={w.id} value={w.id}>
                    {w.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex min-w-64 flex-col gap-1.5">
            <Label htmlFor="wf-select">Workflow</Label>
            <Select value={selected?.id ?? ""} onValueChange={setSelectedId} disabled={!workflows.data?.length}>
              <SelectTrigger id="wf-select" className="w-full">
                <SelectValue placeholder={workflows.isPending ? "Loading…" : "No workflows yet"} />
              </SelectTrigger>
              <SelectContent>
                {(workflows.data ?? []).map((w) => (
                  <SelectItem key={w.id} value={w.id}>
                    {w.name} {w.latest_version ? `· v${w.latest_version}` : "· draft"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button onClick={() => setCreating(true)} disabled={!resolvedWorkspaceId}>
            <Plus aria-hidden /> New workflow
          </Button>
        </CardContent>
      </Card>
      <CreateWorkflowDialog
        open={creating}
        onOpenChange={setCreating}
        workspaceId={resolvedWorkspaceId}
        onCreated={(w) => setSelectedId(w.id)}
      />
      {workflows.isError && <QueryError error={workflows.error} onRetry={() => workflows.refetch()} title="Workflows unavailable" />}
      {(workflows.isPending || catalog.isPending) && resolvedWorkspaceId && <Skeleton className="h-[520px] w-full" />}
      {catalog.isError && <QueryError error={catalog.error} onRetry={() => catalog.refetch()} title="Node catalog unavailable" />}
      {selected && catalog.data && (
        <Designer key={selected.id} workflow={selected} catalog={catalog.data} workspaceId={resolvedWorkspaceId} />
      )}
      {workflows.data && workflows.data.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <WorkflowIcon className="size-8 text-muted-foreground" aria-hidden />
            <p className="font-medium">No workflows in this workspace</p>
            <p className="text-sm text-muted-foreground">Create one to start designing on the canvas.</p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function CreateWorkflowDialog({
  open,
  onOpenChange,
  workspaceId,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  onCreated: (w: Workflow) => void;
}) {
  const { create } = useWorkflowMutations(workspaceId);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  async function submit(e: FormEvent) {
    e.preventDefault();
    try {
      const w = await create.mutateAsync({ workspace_id: workspaceId, name, slug: slug || slugify(name) });
      toast.success("Workflow created.");
      onCreated(w);
      onOpenChange(false);
      setName("");
      setSlug("");
    } catch (err) {
      toastError(err);
    }
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>New workflow</DialogTitle>
            <DialogDescription>It starts as a draft with a manual trigger.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-wf-name">Name</Label>
            <Input id="new-wf-name" required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-wf-slug">Slug</Label>
            <Input
              id="new-wf-slug"
              pattern="[a-z0-9][a-z0-9-]{0,62}"
              placeholder={slugify(name) || "order-approval"}
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={create.isPending || !name.trim()}>
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function Designer({ workflow, catalog, workspaceId }: { workflow: Workflow; catalog: NodeType[]; workspaceId: string }) {
  const can = useCan();
  const canEdit = can("workflow:update");
  const m = useWorkflowMutations(workspaceId);
  const [localDraft, setDraft] = useState<WorkflowDefinition>(workflow.draft);
  const [dirty, setDirty] = useState(false);
  // While nothing is edited the server copy is the source of truth.
  const draft = dirty ? localDraft : workflow.draft;
  const [selectedNode, setSelectedNode] = useState<string>();
  const [issues, setIssues] = useState<WorkflowIssue[] | null>(null);
  const [executionId, setExecutionId] = useState("");
  const versions = useWorkflowVersions(workflow.id);
  const executions = useWorkflowExecutions(workspaceId, workflow.id);
  const palette = useMemo(() => catalog.filter((t) => t.kind !== "trigger"), [catalog]);

  function change(next: WorkflowDefinition) {
    setDraft(next);
    setDirty(true);
    setIssues(null);
  }

  async function save() {
    try {
      await m.update.mutateAsync({ id: workflow.id, body: { draft } });
      setDirty(false);
      toast.success("Draft saved.");
    } catch (err) {
      toastError(err);
    }
  }

  async function validate() {
    try {
      if (dirty) await save();
      const res = await m.validate.mutateAsync(workflow.id);
      setIssues(res.issues);
      if (res.valid) toast.success("The workflow is valid and can be published.");
    } catch (err) {
      toastError(err);
    }
  }

  async function publish() {
    try {
      if (dirty) await save();
      const v = await m.publish.mutateAsync(workflow.id);
      setIssues([]);
      toast.success(`Published version ${v.version}.`);
    } catch (err) {
      toastError(err);
    }
  }

  const versionsById = new Map((versions.data ?? []).map((v) => [v.id, v]));
  const selectedExecution = executions.data?.find((e) => e.id === executionId);

  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-6 lg:grid-cols-[1fr_340px]">
        <Card className="min-w-0">
          <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
            <div>
              <CardTitle className="flex items-center gap-2">
                {workflow.name}
                <Badge variant="outline" className="capitalize">
                  {workflow.status}
                </Badge>
                {dirty && <Badge variant="secondary">Unsaved</Badge>}
              </CardTitle>
              <CardDescription>
                {workflow.latest_version ? `Latest published version: v${workflow.latest_version}` : "Not published yet"}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" onClick={save} disabled={!canEdit || !dirty || m.update.isPending}>
                <Save aria-hidden /> Save draft
              </Button>
              <Button variant="outline" size="sm" onClick={validate} disabled={m.validate.isPending}>
                <ShieldCheck aria-hidden /> Validate
              </Button>
              <Button size="sm" onClick={publish} disabled={!can("workflow:publish") || m.publish.isPending}>
                <CheckCircle2 aria-hidden /> Publish
              </Button>
              <ConfirmDialog
                trigger={
                  <Button variant="ghost" size="sm" disabled={!can("workflow:delete")} aria-label="Delete workflow">
                    <Trash2 aria-hidden />
                  </Button>
                }
                title="Delete this workflow?"
                description="Its versions, deployments and execution history are deleted too. This cannot be undone."
                confirmLabel="Delete workflow"
                onConfirm={async () => {
                  try {
                    await m.remove.mutateAsync(workflow.id);
                    toast.success("Workflow deleted.");
                  } catch (err) {
                    toastError(err);
                  }
                }}
              />
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {canEdit && (
              <div className="flex flex-wrap gap-2" role="toolbar" aria-label="Add node">
                {palette.map((t) => (
                  <Button
                    key={t.type}
                    variant="secondary"
                    size="sm"
                    disabled={!t.available}
                    title={t.available ? t.description : t.unavailable_reason}
                    onClick={() => {
                      const id = nextNodeId(draft, t.type);
                      change({ ...draft, nodes: [...draft.nodes, { id, type: t.type, config: {} }] });
                      setSelectedNode(id);
                    }}
                  >
                    <Plus aria-hidden /> {t.label}
                  </Button>
                ))}
              </div>
            )}
            <WorkflowCanvas
              definition={draft}
              catalog={catalog}
              onChange={change}
              selectedId={selectedNode}
              onSelect={setSelectedNode}
              readOnly={!canEdit}
            />
            {issues && issues.length > 0 && (
              <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/5 p-3">
                <p className="mb-1 text-sm font-medium">Fix these before publishing</p>
                <ul className="list-disc pl-5 text-sm">
                  {issues.map((is, i) => (
                    <li key={i}>
                      <span className="font-mono text-xs text-muted-foreground">{is.path}</span> — {is.message}{" "}
                      <span className="font-mono text-xs text-muted-foreground">({is.code})</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </CardContent>
        </Card>
        <div className="flex flex-col gap-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Inspector</CardTitle>
            </CardHeader>
            <CardContent>
              <NodeInspector
                definition={draft}
                catalog={catalog}
                selectedId={selectedNode}
                onChange={change}
                onSelect={setSelectedNode}
                readOnly={!canEdit}
              />
            </CardContent>
          </Card>
          <ReleasePanel workflow={workflow} workspaceId={workspaceId} onExecuted={setExecutionId} />
        </div>
      </div>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Executions</CardTitle>
          <CardDescription>Most recent first; running executions refresh automatically.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-6 lg:grid-cols-[280px_1fr]">
          <ul className="flex flex-col gap-1" aria-label="Executions">
            {executions.isPending && <Skeleton className="h-24 w-full" />}
            {executions.data?.length === 0 && <li className="text-sm text-muted-foreground">No executions yet.</li>}
            {(executions.data ?? []).map((e) => (
              <li key={e.id}>
                <button
                  type="button"
                  onClick={() => setExecutionId(e.id)}
                  aria-current={e.id === executionId ? "true" : undefined}
                  className="flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent aria-[current]:bg-accent"
                >
                  <span className="truncate">
                    v{e.version} · {new Date(e.created_at).toLocaleString()}
                  </span>
                  <StatusBadge status={e.status} />
                </button>
              </li>
            ))}
          </ul>
          <div className="min-w-0">
            {executionId ? (
              <ExecutionDetail
                executionId={executionId}
                definition={selectedExecution ? versionsById.get(selectedExecution.version_id)?.definition : undefined}
              />
            ) : (
              <p className="text-sm text-muted-foreground">Select an execution to inspect each step.</p>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function ReleasePanel({
  workflow,
  workspaceId,
  onExecuted,
}: {
  workflow: Workflow;
  workspaceId: string;
  onExecuted: (executionId: string) => void;
}) {
  const can = useCan();
  const m = useWorkflowMutations(workspaceId);
  const environments = useEnvironments();
  const versions = useWorkflowVersions(workflow.id);
  const deployments = useWorkflowDeployments(workflow.id);
  const [environmentId, setEnvironmentId] = useState("");
  const envId = environmentId || environments.data?.[0]?.id || "";
  const deployed = deployments.data?.find((d) => d.environment_id === envId);
  const [versionId, setVersionId] = useState("");
  const chosenVersion = versionId || versions.data?.[0]?.id || "";
  const [enabled, setEnabled] = useState(true);
  const [input, setInput] = useState('{\n  "amount": 250\n}');
  const [running, setRunning] = useState(false);

  async function deploy() {
    try {
      const d = await m.deploy.mutateAsync({ id: workflow.id, environmentId: envId, body: { version_id: chosenVersion, enabled } });
      toast.success(`Version ${d.version} ${d.enabled ? "deployed" : "disabled"}.`);
    } catch (err) {
      toastError(err);
    }
  }

  async function run() {
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(input || "{}");
    } catch {
      toast.error("The input must be a JSON object.");
      return;
    }
    try {
      const e = await m.execute.mutateAsync({ id: workflow.id, body: { environment_id: envId, input: parsed } });
      onExecuted(e.id);
      setRunning(false);
      toast.success("Execution started.");
    } catch (err) {
      toastError(err);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Deploy & run</CardTitle>
        <CardDescription>
          {deployed ? `v${deployed.version} is ${deployed.enabled ? "live" : "disabled"} here.` : "Not deployed to this environment."}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="rel-env">Environment</Label>
          <Select value={envId} onValueChange={setEnvironmentId}>
            <SelectTrigger id="rel-env" className="w-full">
              <SelectValue placeholder="Choose…" />
            </SelectTrigger>
            <SelectContent>
              {(environments.data ?? []).map((env) => (
                <SelectItem key={env.id} value={env.id}>
                  {env.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="rel-version">Version</Label>
          <Select value={chosenVersion} onValueChange={setVersionId} disabled={!versions.data?.length}>
            <SelectTrigger id="rel-version" className="w-full">
              <SelectValue placeholder="Publish first" />
            </SelectTrigger>
            <SelectContent>
              {(versions.data ?? []).map((v) => (
                <SelectItem key={v.id} value={v.id}>
                  v{v.version} · {new Date(v.published_at).toLocaleDateString()}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <Switch id="rel-enabled" checked={enabled} onCheckedChange={setEnabled} />
          <Label htmlFor="rel-enabled">Enabled</Label>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" onClick={deploy} disabled={!can("workflow:deploy") || !chosenVersion || !envId || m.deploy.isPending}>
            <Rocket aria-hidden /> Deploy
          </Button>
          <Button size="sm" onClick={() => setRunning(true)} disabled={!can("workflow:execute") || !deployed?.enabled}>
            <Play aria-hidden /> Run
          </Button>
        </div>
      </CardContent>
      <Dialog open={running} onOpenChange={setRunning}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Run {workflow.name}</DialogTitle>
            <DialogDescription>The input is a JSON object available to nodes as trigger (and as input to entry nodes).</DialogDescription>
          </DialogHeader>
          <Textarea className="min-h-40 font-mono text-xs" value={input} onChange={(e) => setInput(e.target.value)} aria-label="Execution input" />
          <DialogFooter>
            <Button onClick={run} disabled={m.execute.isPending}>
              <Play aria-hidden /> Start execution
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
