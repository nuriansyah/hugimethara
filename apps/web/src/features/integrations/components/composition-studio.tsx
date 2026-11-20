"use client";

import { ArrowDown, GitFork, Play, Plus, Rocket, Save, ShieldCheck, Trash2 } from "lucide-react";
import { useMemo, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useCan } from "@/features/access/api";
import { useEnvironments, useWorkspaces } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import {
  useApiCompositionExecutions,
  useApiCompositionMutations,
  useApiCompositions,
  useApiCompositionVersions,
  useMappingVersions,
  useMappings,
  useServiceEndpoints,
  useServices,
  type ApiComposition,
  type ApiCompositionExecution,
} from "../api";

const emptyDefinition = JSON.stringify({
  steps: [],
  response: {},
  limits: { timeout_ms: 5000, max_concurrency: 4 },
}, null, 2);
const sampleRequest = JSON.stringify({
  path: { id: "order-123" },
  query: {},
  body: {},
}, null, 2);

type GraphStep = {
  id?: string;
  depends_on?: string[];
  when?: string;
  on_error?: string;
  call?: { service_id?: string; endpoint_id?: string };
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readDefinition(raw: string): { steps: GraphStep[]; response?: Record<string, unknown> } | null {
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!isRecord(parsed)) return null;
    return {
      steps: Array.isArray(parsed.steps) ? parsed.steps.filter(isRecord) as GraphStep[] : [],
      response: isRecord(parsed.response) ? parsed.response : undefined,
    };
  } catch {
    return null;
  }
}

export function CompositionStudio() {
  const can = useCan();
  const workspaces = useWorkspaces(false);
  const environments = useEnvironments();
  const [workspaceId, setWorkspaceId] = useState("");
  const activeWorkspace = workspaces.data?.find((item) => item.id === workspaceId) ?? workspaces.data?.[0];
  const resolvedWorkspaceId = activeWorkspace?.id ?? "";
  const compositions = useApiCompositions(resolvedWorkspaceId);
  const [selectedId, setSelectedId] = useState("");
  const selected = compositions.data?.find((item) => item.id === selectedId) ?? compositions.data?.[0];
  const [creating, setCreating] = useState(false);

  return (
    <div className="mx-auto flex max-w-7xl flex-col gap-6">
      <PageHeader
        title="API compositions"
        description="Compose registered endpoints into bounded, versioned APIs. Validate the graph, publish a snapshot, then run and inspect it."
      />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Compositions</CardTitle>
            <CardDescription>Each published version is immutable and every execution pins its version.</CardDescription>
          </div>
          <div className="flex gap-2">
            <Select value={resolvedWorkspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger aria-label="Workspace" className="w-52"><SelectValue placeholder="Choose workspace" /></SelectTrigger>
              <SelectContent>
                {workspaces.data?.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}
              </SelectContent>
            </Select>
            {activeWorkspace && can("api_composition:create", activeWorkspace.id) && (
              <Button onClick={() => setCreating(true)}><Plus aria-hidden /> New composition</Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {workspaces.isPending ? <Skeleton className="h-32 w-full" /> : workspaces.error ? (
            <QueryError error={workspaces.error} onRetry={() => workspaces.refetch()} title="Could not load workspaces" />
          ) : !resolvedWorkspaceId ? (
            <div className="rounded-lg border border-dashed p-9 text-center text-sm text-muted-foreground">Choose a workspace to open its compositions.</div>
          ) : compositions.isPending ? <Skeleton className="h-32 w-full" /> : compositions.error ? (
            <QueryError error={compositions.error} onRetry={() => compositions.refetch()} title="Could not load compositions" />
          ) : compositions.data.length === 0 ? (
            <div className="rounded-lg border border-dashed p-9 text-center">
              <GitFork className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden />
              <p className="text-sm font-medium">No compositions yet</p>
              <p className="mt-1 text-sm text-muted-foreground">Create a draft to connect services and shape a unified response.</p>
            </div>
          ) : (
            <div className="grid gap-4 lg:grid-cols-[230px_minmax(0,1fr)]">
              <nav aria-label="Compositions" className="space-y-2">
                {compositions.data.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSelectedId(item.id)}
                    aria-current={selected?.id === item.id ? "page" : undefined}
                    className={`w-full rounded-lg border p-3 text-left text-sm ${selected?.id === item.id ? "border-primary bg-primary/5" : "hover:bg-muted/50"}`}
                  >
                    <span className="block font-medium">{item.name}</span>
                    <span className="font-mono text-xs text-muted-foreground">/{item.slug}</span>
                  </button>
                ))}
              </nav>
              {selected && activeWorkspace && (
                <CompositionEditor
                  key={selected.id}
                  composition={selected}
                  workspaceId={resolvedWorkspaceId}
                  environments={environments.data ?? []}
                  canUpdate={can("api_composition:update", resolvedWorkspaceId)}
                  canPublish={can("api_composition:publish", resolvedWorkspaceId)}
                  canExecute={can("api_composition:execute", resolvedWorkspaceId)}
                  canDelete={can("api_composition:delete", resolvedWorkspaceId)}
                />
              )}
            </div>
          )}
        </CardContent>
      </Card>
      {creating && <CreateCompositionDialog workspaceId={resolvedWorkspaceId} onOpenChange={setCreating} onCreated={setSelectedId} />}
    </div>
  );
}

function CreateCompositionDialog({ workspaceId, onOpenChange, onCreated }: {
  workspaceId: string;
  onOpenChange: (open: boolean) => void;
  onCreated: (id: string) => void;
}) {
  const mutations = useApiCompositionMutations(workspaceId);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      const created = await mutations.create.mutateAsync({ workspace_id: workspaceId, name, slug, draft: JSON.parse(emptyDefinition) });
      onCreated(created.id);
      toast.success("Composition draft created.");
      onOpenChange(false);
    } catch (reason) {
      if (reason instanceof SyntaxError) setError("The starter definition is invalid.");
      else {
        setError("The composition could not be created.");
        toastError(reason);
      }
    }
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>New composition</DialogTitle>
            <DialogDescription>Create a draft, then add endpoint steps and response mapping.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="composition-name">Name</Label>
            <Input id="composition-name" value={name} onChange={(event) => {
              setName(event.target.value);
              if (!slug) setSlug(event.target.value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""));
            }} required maxLength={100} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="composition-slug">Slug</Label>
            <Input id="composition-slug" value={slug} onChange={(event) => setSlug(event.target.value)} required pattern="[a-z0-9][a-z0-9-]{0,62}" />
          </div>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={mutations.create.isPending}>Create draft</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function CompositionEditor({
  composition,
  workspaceId,
  environments,
  canUpdate,
  canPublish,
  canExecute,
  canDelete,
}: {
  composition: ApiComposition;
  workspaceId: string;
  environments: { id: string; name: string; slug: string }[];
  canUpdate: boolean;
  canPublish: boolean;
  canExecute: boolean;
  canDelete: boolean;
}) {
  const mutations = useApiCompositionMutations(workspaceId);
  const versions = useApiCompositionVersions(composition.id);
  const executions = useApiCompositionExecutions(workspaceId, composition.id);
  const [definition, setDefinition] = useState(JSON.stringify(composition.draft, null, 2));
  const [request, setRequest] = useState(sampleRequest);
  const [environmentId, setEnvironmentId] = useState(environments[0]?.id ?? "");
  const resolvedEnvironmentId = environmentId || environments[0]?.id || "";
  const [selectedExecutionId, setSelectedExecutionId] = useState("");
  const graph = useMemo(() => readDefinition(definition), [definition]);
  const draftChanged = definition !== JSON.stringify(composition.draft, null, 2);
  const selectedExecution = executions.data?.find((item) => item.id === selectedExecutionId) ?? executions.data?.[0];

  async function saveDraft(): Promise<boolean> {
    try {
      const saved = await mutations.update.mutateAsync({ id: composition.id, body: { draft: JSON.parse(definition) } });
      setDefinition(JSON.stringify(saved.draft, null, 2));
      toast.success("Composition draft saved.");
      return true;
    } catch (error) {
      toastError(error);
      return false;
    }
  }

  async function validate() {
    try {
      if (draftChanged && !(await saveDraft())) return;
      await mutations.validate.mutateAsync(composition.id);
      toast.success("Composition is valid.");
    } catch (error) {
      toastError(error);
    }
  }

  async function publish() {
    try {
      if (draftChanged && !(await saveDraft())) return;
      await mutations.publish.mutateAsync(composition.id);
      toast.success("Composition published.");
    } catch (error) {
      toastError(error);
    }
  }

  async function execute() {
    try {
      const input = JSON.parse(request) as Record<string, unknown>;
      const result = await mutations.execute.mutateAsync({
        id: composition.id,
        body: { environment_id: resolvedEnvironmentId, request: input },
      });
      setSelectedExecutionId(result.id);
      toast.success(result.status === "succeeded" ? "Composition completed." : "Composition run failed. Inspect the steps below.");
    } catch (error) {
      toastError(error);
    }
  }

  return (
    <div className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold">{composition.name}</h2>
          <p className="text-xs text-muted-foreground">/{composition.slug} · Updated {new Date(composition.updated_at).toLocaleString()}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {canUpdate && <Button size="sm" variant="outline" onClick={saveDraft} disabled={!draftChanged || mutations.update.isPending}><Save aria-hidden /> Save</Button>}
          {canUpdate && <Button size="sm" variant="outline" onClick={validate} disabled={mutations.validate.isPending}><ShieldCheck aria-hidden /> Validate</Button>}
          {canPublish && <Button size="sm" onClick={publish} disabled={mutations.publish.isPending}><Rocket aria-hidden /> Publish</Button>}
          {canDelete && <ConfirmDialog
            trigger={<Button size="icon" variant="ghost" aria-label={`Delete ${composition.name}`}><Trash2 /></Button>}
            title={`Delete “${composition.name}”?`}
            description="The draft, published versions, and execution history will be removed."
            confirmLabel="Delete composition"
            onConfirm={async () => {
              try { await mutations.remove.mutateAsync(composition.id); toast.success("Composition deleted."); }
              catch (error) { toastError(error); throw error; }
            }}
          />}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Step graph</CardTitle>
          <CardDescription>Parallel-ready steps run together after their dependencies. Select a step in the definition to configure its endpoint and mappings.</CardDescription>
        </CardHeader>
        <CardContent>
          {!graph ? (
            <p role="alert" className="text-sm text-destructive">Definition is not valid JSON.</p>
          ) : graph.steps.length === 0 ? (
            <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">Add steps to the definition below to build this graph.</div>
          ) : (
            <ol className="flex flex-wrap items-stretch gap-3">
              {graph.steps.map((step, index) => (
                <li key={`${step.id ?? "step"}-${index}`} className="flex min-w-48 max-w-full items-center gap-3">
                  <div className="min-w-48 rounded-lg border bg-card p-3">
                    <p className="font-mono text-sm font-medium">{step.id || `Step ${index + 1}`}</p>
                    <p className="mt-1 break-all text-xs text-muted-foreground">
                      {step.call?.service_id ? `Service ${step.call.service_id}` : "No service selected"}
                    </p>
                    <p className="break-all text-xs text-muted-foreground">
                      {step.call?.endpoint_id ? `Endpoint ${step.call.endpoint_id}` : "No endpoint selected"}
                    </p>
                    {step.depends_on?.length ? <p className="mt-2 text-xs">After: {step.depends_on.join(", ")}</p> : null}
                    {step.when && <p className="mt-1 text-xs text-muted-foreground">When: {step.when}</p>}
                  </div>
                  {index < graph.steps.length - 1 && <ArrowDown className="size-4 shrink-0 text-muted-foreground sm:rotate-[-90deg]" aria-hidden />}
                </li>
              ))}
            </ol>
          )}
          <div className="mt-4 rounded-lg bg-muted/50 p-3 text-sm">
            <p className="font-medium">Response mapping</p>
            <p className="mt-1 font-mono text-xs text-muted-foreground">
              {graph?.response?.mapping_id
                ? `Mapping ${String(graph.response.mapping_id)} · version ${String(graph.response.mapping_version ?? "unspecified")}`
                : "No mapping selected; the response includes request and step results."}
            </p>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Composition definition</CardTitle>
            <CardDescription>References use registered service and endpoint IDs. Use JSONPath strings such as $.request.path.id or $.steps.order.body.customer_id for call inputs.</CardDescription>
          </CardHeader>
          <CardContent>
            <Textarea
              aria-label="Composition definition JSON"
              className="min-h-96 font-mono text-xs"
              value={definition}
              onChange={(event) => setDefinition(event.target.value)}
              disabled={!canUpdate}
              spellCheck={false}
            />
            <ReferenceCatalog workspaceId={workspaceId} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Run composition</CardTitle>
            <CardDescription>Execution uses the latest published version and records step output for inspection.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="composition-environment">Environment</Label>
              <Select value={resolvedEnvironmentId} onValueChange={setEnvironmentId}>
                <SelectTrigger id="composition-environment"><SelectValue placeholder="Choose environment" /></SelectTrigger>
                <SelectContent>{environments.map((env) => <SelectItem key={env.id} value={env.id}>{env.name} · {env.slug}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="composition-request">Request JSON</Label>
              <Textarea id="composition-request" className="min-h-44 font-mono text-xs" value={request} onChange={(event) => setRequest(event.target.value)} spellCheck={false} />
            </div>
            {canExecute && (
              <Button onClick={execute} disabled={!resolvedEnvironmentId || mutations.execute.isPending}>
                <Play aria-hidden /> Run published version
              </Button>
            )}
            {versions.error ? <QueryError error={versions.error} onRetry={() => versions.refetch()} title="Could not load versions" /> : (
              <div className="border-t pt-3">
                <p className="mb-2 text-sm font-medium">Published versions</p>
                {versions.isPending ? <Skeleton className="h-10 w-full" /> : versions.data?.length ? (
                  <ul className="space-y-1 text-xs text-muted-foreground">
                    {versions.data.map((version) => <li key={version.id}>v{version.version} · {version.checksum.slice(0, 12)} · {new Date(version.published_at).toLocaleString()}</li>)}
                  </ul>
                ) : <p className="text-xs text-muted-foreground">No published versions yet.</p>}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <ExecutionInspector
        executions={executions.data ?? []}
        selected={selectedExecution}
        selectedId={selectedExecution?.id ?? ""}
        onSelect={setSelectedExecutionId}
        loading={executions.isPending}
        error={executions.error}
        onRetry={() => executions.refetch()}
      />
    </div>
  );
}

function ReferenceCatalog({ workspaceId }: { workspaceId: string }) {
  const services = useServices(workspaceId);
  const mappings = useMappings(workspaceId);
  return (
    <details className="rounded-lg border p-3 text-xs">
      <summary className="cursor-pointer font-medium">Available service endpoints and response mappings</summary>
      <div className="mt-3 space-y-4">
        <div>
          <p className="mb-2 font-medium">Endpoints</p>
          {services.isPending ? <Skeleton className="h-8 w-full" /> : services.error ? (
            <QueryError error={services.error} onRetry={() => services.refetch()} title="Could not load services" />
          ) : services.data.length === 0 ? <p className="text-muted-foreground">No registered services.</p> : (
            <div className="space-y-2">
              {services.data.map((service) => <ServiceEndpointReferences key={service.id} service={service} />)}
            </div>
          )}
        </div>
        <div className="border-t pt-3">
          <p className="mb-2 font-medium">Published response mappings</p>
          {mappings.isPending ? <Skeleton className="h-8 w-full" /> : mappings.error ? (
            <QueryError error={mappings.error} onRetry={() => mappings.refetch()} title="Could not load mappings" />
          ) : mappings.data.length === 0 ? <p className="text-muted-foreground">No mappings in this workspace.</p> : (
            <div className="space-y-2">
              {mappings.data.map((mapping) => <MappingVersionReferences key={mapping.id} mapping={mapping} />)}
            </div>
          )}
        </div>
      </div>
    </details>
  );
}

function ServiceEndpointReferences({ service }: { service: { id: string; name: string } }) {
  const endpoints = useServiceEndpoints(service.id);
  return (
    <div className="rounded-md bg-muted/50 p-2">
      <p className="font-medium">{service.name} <span className="font-mono text-muted-foreground">{service.id}</span></p>
      {endpoints.isPending ? <p className="mt-1 text-muted-foreground">Loading endpoints…</p> : endpoints.error ? (
        <QueryError error={endpoints.error} onRetry={() => endpoints.refetch()} title={`Could not load ${service.name} endpoints`} />
      ) : endpoints.data.length === 0 ? <p className="mt-1 text-muted-foreground">No endpoints.</p> : (
        <ul className="mt-1 space-y-1">
          {endpoints.data.map((endpoint) => (
            <li key={endpoint.id} className="break-all font-mono">
              {endpoint.method} {endpoint.path} · {endpoint.operation_id} · {endpoint.id}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function MappingVersionReferences({ mapping }: { mapping: { id: string; name: string } }) {
  const versions = useMappingVersions(mapping.id);
  return (
    <div className="rounded-md bg-muted/50 p-2">
      <p className="font-medium">{mapping.name} <span className="font-mono text-muted-foreground">{mapping.id}</span></p>
      {versions.isPending ? <p className="mt-1 text-muted-foreground">Loading versions…</p> : versions.error ? (
        <QueryError error={versions.error} onRetry={() => versions.refetch()} title={`Could not load ${mapping.name} versions`} />
      ) : versions.data.length === 0 ? <p className="mt-1 text-muted-foreground">No published versions.</p> : (
        <ul className="mt-1 space-y-1 font-mono">
          {versions.data.map((version) => <li key={version.id}>v{version.version} · {version.id}</li>)}
        </ul>
      )}
    </div>
  );
}

function ExecutionInspector({
  executions,
  selected,
  selectedId,
  onSelect,
  loading,
  error,
  onRetry,
}: {
  executions: ApiCompositionExecution[];
  selected?: ApiCompositionExecution;
  selectedId: string;
  onSelect: (id: string) => void;
  loading: boolean;
  error: Error | null;
  onRetry: () => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Execution history</CardTitle>
        <CardDescription>Recent runs pinned to the exact published version, with per-step results.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading ? <Skeleton className="h-24 w-full" /> : error ? (
          <QueryError error={error} onRetry={onRetry} title="Could not load execution history" />
        ) : executions.length === 0 ? (
          <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">No runs yet.</p>
        ) : (
          <>
            <div className="flex flex-wrap gap-2" aria-label="Recent executions">
              {executions.map((run) => (
                <button
                  key={run.id}
                  type="button"
                  aria-pressed={selectedId === run.id}
                  onClick={() => onSelect(run.id)}
                  className={`rounded-lg border px-3 py-2 text-left text-xs ${selectedId === run.id ? "border-primary bg-primary/5" : "hover:bg-muted/50"}`}
                >
                  <span className="flex items-center gap-2 font-medium">
                    <Badge variant={run.status === "succeeded" ? "default" : "destructive"}>{run.status}</Badge>
                    v{run.version_id.slice(0, 8)}
                  </span>
                  <span className="mt-1 block text-muted-foreground">{new Date(run.started_at).toLocaleString()} · {run.duration_ms} ms</span>
                </button>
              ))}
            </div>
            {selected && (
              <div className="grid gap-4 xl:grid-cols-2">
                <JSONPanel title="Step results" value={selected.step_summary} />
                <JSONPanel title={selected.error ? "Execution error" : "Response"} value={selected.error ? { error: selected.error } : selected.output} />
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

function JSONPanel({ title, value }: { title: string; value: unknown }) {
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">{title}</h3>
      <pre className="max-h-96 overflow-auto rounded-lg bg-muted p-3 text-xs leading-relaxed">{JSON.stringify(value, null, 2)}</pre>
    </div>
  );
}
