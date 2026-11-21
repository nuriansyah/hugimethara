"use client";

import { ArrowDownToLine, ChevronDown, ChevronRight, Code2, Plus, Play, Trash2 } from "lucide-react";
import { useMemo, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
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
import { Textarea } from "@hugimethara/ui/components/textarea";
import { useCan } from "@/features/access/api";
import { useEnvironments, useWorkspaces } from "@/features/workspaces/api";
import { errorMessage, toastError } from "@/lib/errors";
import { useConnectors } from "../api";
import {
  useServiceEndpoints,
  useServiceMutations,
  useServices,
  type OpenAPIImportResult,
  type Service,
  type ServiceEndpoint,
  type ServiceTryResult,
} from "../api";

export function ServiceRegistryManager() {
  const can = useCan();
  const workspaces = useWorkspaces(false);
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState("");
  const workspaceOptions = workspaces.data ?? [];
  const activeWorkspace = workspaceOptions.find((item) => item.id === selectedWorkspaceId) ?? workspaceOptions[0];
  const workspaceId = activeWorkspace?.id ?? "";
  const services = useServices(workspaceId);
  const mutations = useServiceMutations(workspaceId);
  const [creating, setCreating] = useState(false);

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Service registry" description="Keep API contracts, environment URLs, and runnable endpoints in one place." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle>Services</CardTitle>
            <CardDescription>Import an OpenAPI document to review endpoint changes and power the API explorer.</CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Select value={workspaceId} onValueChange={setSelectedWorkspaceId}>
              <SelectTrigger aria-label="Workspace" className="w-52"><SelectValue placeholder="Choose workspace" /></SelectTrigger>
              <SelectContent>{workspaceOptions.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
            {activeWorkspace && can("service:create", activeWorkspace.id) && <Button onClick={() => setCreating(true)}><Plus aria-hidden /> Add service</Button>}
          </div>
        </CardHeader>
        <CardContent>
          {workspaces.isPending ? (
            <Skeleton className="h-28 w-full" />
          ) : workspaces.error ? (
            <QueryError error={workspaces.error} onRetry={() => workspaces.refetch()} title="Could not load workspaces" />
          ) : !workspaceId ? (
            <div className="rounded-lg border border-dashed p-9 text-center text-sm text-muted-foreground">
              Choose a workspace to view its service registry.
            </div>
          ) : services.isPending ? (
            <Skeleton className="h-28 w-full" />
          ) : services.error ? (
            <QueryError error={services.error} onRetry={() => services.refetch()} title="Could not load services" />
          ) : services.data.length === 0 ? (
            <div className="rounded-lg border border-dashed p-9 text-center">
              <Code2 className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden />
              <p className="text-sm font-medium">No services registered</p>
              <p className="mt-1 text-sm text-muted-foreground">Start with a service name, then import its API contract.</p>
            </div>
          ) : (
            <div className="space-y-3">
              {services.data.map((service) => (
                <ServiceCard
                  key={service.id}
                  service={service}
                  canUpdate={can("service:update", service.workspace_id)}
                  canDelete={can("service:delete", service.workspace_id)}
                  canInvoke={can("service:invoke", service.workspace_id)}
                  onRemove={async () => {
                    try {
                      await mutations.remove.mutateAsync(service.id);
                      toast.success("Service removed.");
                    } catch (error) {
                      toastError(error);
                      throw error;
                    }
                  }}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
      <ServiceCreateDialog
        open={creating}
        workspaceId={workspaceId}
        onOpenChange={setCreating}
      />
    </div>
  );
}

function ServiceCard({
  service,
  canUpdate,
  canDelete,
  canInvoke,
  onRemove,
}: {
  service: Service;
  canUpdate: boolean;
  canDelete: boolean;
  canInvoke: boolean;
  onRemove: () => Promise<void>;
}) {
  const [expanded, setExpanded] = useState(false);
  const [importing, setImporting] = useState(false);
  const [binding, setBinding] = useState(false);
  const endpoints = useServiceEndpoints(service.id, expanded);
  const environments = useEnvironments();
  const mutations = useServiceMutations(service.workspace_id);
  const environmentNames = useMemo(() => new Map(environments.data?.map((e) => [e.id, e.name])), [environments.data]);

  return (
    <section className="rounded-xl border">
      <div className="flex flex-wrap items-center justify-between gap-3 p-4">
        <button type="button" className="flex min-w-0 flex-1 items-start gap-3 text-left focus-visible:rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => setExpanded(!expanded)} aria-expanded={expanded}>
          {expanded ? <ChevronDown className="mt-0.5 size-4 shrink-0" aria-hidden /> : <ChevronRight className="mt-0.5 size-4 shrink-0" aria-hidden />}
          <span className="min-w-0">
            <span className="flex flex-wrap items-center gap-2 text-sm font-semibold">
              {service.name}<Badge variant="outline">{service.protocol}</Badge>
            </span>
            <span className="mt-1 block text-xs text-muted-foreground">
              {service.owner || "No owner"} · {service.endpoint_count} endpoints · {service.environments.length} environments
            </span>
            {service.description && <span className="mt-1 block text-sm text-muted-foreground">{service.description}</span>}
          </span>
        </button>
        <div className="flex items-center gap-1">
          {canUpdate && <Button variant="outline" size="sm" onClick={() => setBinding(true)}>Environments</Button>}
          {canUpdate && <Button variant="outline" size="sm" onClick={() => setImporting(true)}><ArrowDownToLine aria-hidden /> Import API</Button>}
          {canDelete && <ConfirmDialog
            trigger={<Button variant="ghost" size="icon" aria-label={`Remove ${service.name}`}><Trash2 /></Button>}
            title={`Remove “${service.name}”?`}
            description="Its endpoint catalog and environment bindings will be removed."
            confirmLabel="Remove service"
            onConfirm={onRemove}
          />}
        </div>
      </div>
      {expanded && (
        <div className="border-t bg-muted/20 p-4">
          <div className="mb-4 flex flex-wrap gap-2">
            {service.environments.map((environment) => (
              <Badge key={environment.environment_id} variant="outline">
                {environmentNames.get(environment.environment_id) ?? "Environment"} · {environment.base_url}
              </Badge>
            ))}
            {!service.environments.length && <p className="text-sm text-muted-foreground">No environment URLs configured yet.</p>}
          </div>
            {environments.error && <QueryError error={environments.error} onRetry={() => environments.refetch()} title="Could not load environment names" />}
          {endpoints.isPending ? <Skeleton className="h-20 w-full" /> : endpoints.error ? (
            <QueryError error={endpoints.error} onRetry={() => endpoints.refetch()} title="Could not load endpoints" />
          ) : endpoints.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">Import an OpenAPI document to populate the endpoint catalog.</p>
          ) : (
            <ul className="divide-y">
              {endpoints.data.map((endpoint) => (
                <EndpointRow key={endpoint.id} endpoint={endpoint} service={service} canInvoke={canInvoke} />
              ))}
            </ul>
          )}
        </div>
      )}
      <ServiceImportDialog
        service={service}
        open={importing}
        onOpenChange={setImporting}
        onImport={(body, dryRun) => mutations.importOpenAPI.mutateAsync({ id: service.id, body, dryRun })}
      />
      <EnvironmentBindingsDialog service={service} open={binding} onOpenChange={setBinding} />
    </section>
  );
}

function EndpointRow({ endpoint, service, canInvoke }: { endpoint: ServiceEndpoint; service: Service; canInvoke: boolean }) {
  const [trying, setTrying] = useState(false);
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 py-3">
      <div className="min-w-0">
        <p className="flex flex-wrap items-center gap-2 text-sm">
          <Badge className="font-mono">{endpoint.method.toUpperCase()}</Badge>
          <code className="font-mono text-xs">{endpoint.path}</code>
          {endpoint.deprecated && <Badge variant="outline">deprecated</Badge>}
        </p>
        <p className="mt-1 text-xs text-muted-foreground">{endpoint.summary || endpoint.operation_id}</p>
      </div>
      {canInvoke && service.environments.length > 0 && (
        <Button size="sm" variant="outline" onClick={() => setTrying(true)}><Play aria-hidden /> Try</Button>
      )}
      <TryEndpointDialog endpoint={endpoint} service={service} open={trying} onOpenChange={setTrying} />
    </li>
  );
}

function ServiceCreateDialog({
  open,
  workspaceId,
  onOpenChange,
}: {
  open: boolean;
  workspaceId: string;
  onOpenChange: (open: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [owner, setOwner] = useState("");
  const [description, setDescription] = useState("");
  const [protocol, setProtocol] = useState<"rest" | "graphql" | "grpc">("rest");
  const [error, setError] = useState("");
  const { create } = useServiceMutations(workspaceId);
  const close = () => {
    setName(""); setOwner(""); setDescription(""); setProtocol("rest"); setError(""); onOpenChange(false);
  };
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    try {
      await create.mutateAsync({ workspace_id: workspaceId, name: name.trim(), owner: owner.trim(), description: description.trim(), protocol });
      toast.success("Service registered.");
      close();
    } catch (err) {
      setError(errorMessage(err));
    }
  };
  return (
    <Dialog open={open} onOpenChange={(next) => !create.isPending && (next ? onOpenChange(true) : close())}>
      <DialogContent>
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader><DialogTitle>Register a service</DialogTitle><DialogDescription>Give the API a stable name before importing its contract.</DialogDescription></DialogHeader>
          <Field label="Service name"><Input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120} /></Field>
          <Field label="Owner"><Input value={owner} onChange={(event) => setOwner(event.target.value)} maxLength={120} placeholder="Platform team" /></Field>
          <Field label="Protocol">
            <Select value={protocol} onValueChange={(value) => {
              if (value === "rest" || value === "graphql" || value === "grpc") setProtocol(value);
            }}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="rest">REST</SelectItem><SelectItem value="graphql">GraphQL</SelectItem><SelectItem value="grpc">gRPC</SelectItem></SelectContent>
            </Select>
          </Field>
          <Field label="Description"><Textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={1000} /></Field>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter><Button type="button" variant="outline" onClick={close}>Cancel</Button><Button type="submit" disabled={!workspaceId || !name.trim() || create.isPending}>Register service</Button></DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function EnvironmentBindingsDialog({ service, open, onOpenChange }: { service: Service; open: boolean; onOpenChange: (open: boolean) => void }) {
  const environments = useEnvironments();
  const connectors = useConnectors();
  const mutations = useServiceMutations(service.workspace_id);
  const [environmentId, setEnvironmentId] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [connectorId, setConnectorId] = useState("none");
  const [error, setError] = useState("");
  const options = environments.data ?? [];
  const connectorOptions = connectors.data?.filter((item) => item.workspace_id === service.workspace_id) ?? [];
  const close = () => { setError(""); setEnvironmentId(""); setBaseUrl(""); setConnectorId("none"); onOpenChange(false); };
  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    try {
      await mutations.putEnvironment.mutateAsync({
        id: service.id,
        environmentId,
        body: { base_url: baseUrl.trim(), connector_instance_id: connectorId === "none" ? undefined : connectorId },
      });
      toast.success("Environment URL saved.");
      close();
    } catch (err) { setError(errorMessage(err)); }
  };
  return (
    <Dialog open={open} onOpenChange={(next) => !mutations.putEnvironment.isPending && (next ? onOpenChange(true) : close())}>
      <DialogContent>
        <form onSubmit={save} className="space-y-4">
          <DialogHeader><DialogTitle>Configure environment</DialogTitle><DialogDescription>Bind a base URL and optional connector for this service.</DialogDescription></DialogHeader>
          <Field label="Environment">
            <Select value={environmentId} onValueChange={(value) => {
              setEnvironmentId(value);
              const existing = service.environments.find((entry) => entry.environment_id === value);
              setBaseUrl(existing?.base_url ?? "");
              setConnectorId(existing?.connector_instance_id ?? "none");
            }}>
              <SelectTrigger className="w-full"><SelectValue placeholder="Choose environment" /></SelectTrigger>
              <SelectContent>{options.map((environment) => <SelectItem key={environment.id} value={environment.id}>{environment.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Base URL"><Input type="url" required value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} placeholder="https://api.example.com" /></Field>
          <Field label="Connector">
            <Select value={connectorId} onValueChange={setConnectorId}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="none">No connector</SelectItem>{connectorOptions.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          {environments.error && <QueryError error={environments.error} onRetry={() => environments.refetch()} title="Could not load environments" />}
          {connectors.error && <QueryError error={connectors.error} onRetry={() => connectors.refetch()} title="Could not load connectors" />}
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter><Button type="button" variant="outline" onClick={close}>Cancel</Button><Button type="submit" disabled={!environmentId || mutations.putEnvironment.isPending}>Save environment</Button></DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ServiceImportDialog({
  service,
  open,
  onOpenChange,
  onImport,
}: {
  service: Service;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImport: (body: { spec?: string; url?: string; version_label?: string }, dryRun: boolean) => Promise<OpenAPIImportResult>;
}) {
  const [source, setSource] = useState<"spec" | "url">("spec");
  const [document, setDocument] = useState("");
  const [version, setVersion] = useState("");
  const [preview, setPreview] = useState<OpenAPIImportResult>();
  const [error, setError] = useState("");
  const mutations = useServiceMutations(service.workspace_id);
  const pending = mutations.importOpenAPI.isPending;
  const close = () => { setSource("spec"); setDocument(""); setVersion(""); setPreview(undefined); setError(""); onOpenChange(false); };
  const body = () => source === "spec" ? { spec: document, ...(version.trim() ? { version_label: version.trim() } : {}) } : { url: document, ...(version.trim() ? { version_label: version.trim() } : {}) };
  const run = async (dryRun: boolean) => {
    setError("");
    try {
      const result = await onImport(body(), dryRun);
      setPreview(result);
      if (!dryRun) {
        toast.success(`API contract imported: ${result.endpoint_count} endpoints.`);
        close();
      }
    } catch (err) { setError(errorMessage(err)); }
  };
  return (
    <Dialog open={open} onOpenChange={(next) => !pending && (next ? onOpenChange(true) : close())}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader><DialogTitle>Import API contract</DialogTitle><DialogDescription>Preview endpoint additions, removals, and breaking changes before applying the contract.</DialogDescription></DialogHeader>
        <div className="space-y-4">
          <Field label="Source">
            <Select value={source} onValueChange={(value) => { setSource(value as typeof source); setPreview(undefined); }}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="spec">Paste OpenAPI</SelectItem><SelectItem value="url">Fetch from URL</SelectItem></SelectContent>
            </Select>
          </Field>
          {source === "spec" ? (
            <Field label="OpenAPI document" hint="Paste JSON or YAML. Remote references are not fetched.">
              <Textarea value={document} onChange={(event) => { setDocument(event.target.value); setPreview(undefined); }} className="min-h-48 font-mono text-xs" spellCheck={false} />
            </Field>
          ) : <Field label="Document URL"><Input type="url" value={document} onChange={(event) => { setDocument(event.target.value); setPreview(undefined); }} placeholder="https://example.com/openapi.yaml" /></Field>}
          <Field label="Version label (optional)"><Input value={version} onChange={(event) => setVersion(event.target.value)} maxLength={100} placeholder="2025-03" /></Field>
          {preview && <ImportPreview result={preview} />}
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={close}>Cancel</Button>
          {!preview ? (
            <Button onClick={() => void run(true)} disabled={!document.trim() || pending}>Preview changes</Button>
          ) : (
            <Button onClick={() => void run(false)} disabled={pending}>{pending ? "Importing…" : "Apply contract"}</Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ImportPreview({ result }: { result: OpenAPIImportResult }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-sm font-medium">{result.endpoint_count} endpoints found</p>
      {result.changes.length ? <ul className="mt-2 max-h-40 divide-y overflow-auto text-xs">
        {result.changes.map((change, index) => (
          <li key={`${change.operation_id}-${index}`} className="flex flex-wrap items-center gap-2 py-2">
            <Badge variant={change.breaking ? "destructive" : "outline"}>{change.kind}</Badge>
            <span className="font-mono">{change.method} {change.path}</span>
            <span className="text-muted-foreground">{change.operation_id}</span>
          </li>
        ))}
      </ul> : <p className="mt-1 text-xs text-muted-foreground">No endpoint changes detected.</p>}
    </div>
  );
}

function TryEndpointDialog({ endpoint, service, open, onOpenChange }: {
  endpoint: ServiceEndpoint;
  service: Service;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const mutations = useServiceMutations(service.workspace_id);
  const environments = useEnvironments();
  const [environmentId, setEnvironmentId] = useState(service.environments[0]?.environment_id ?? "");
  const [pathValues, setPathValues] = useState("{}");
  const [query, setQuery] = useState("{}");
  const [headers, setHeaders] = useState("{}");
  const [body, setBody] = useState("{}");
  const [result, setResult] = useState<ServiceTryResult>();
  const [error, setError] = useState("");
  const environmentOptions = service.environments.map((binding) => ({
    ...binding,
    name: environments.data?.find((item) => item.id === binding.environment_id)?.name ?? binding.environment_id,
  }));
  const close = () => { setResult(undefined); setError(""); onOpenChange(false); };
  const run = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    try {
      const parseObject = (value: string, field: string): Record<string, string> => {
        const parsed: unknown = JSON.parse(value);
        if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object" ||
            Object.values(parsed).some((item) => typeof item !== "string")) {
          throw new Error(`${field} must be a JSON object with string values.`);
        }
        return parsed as Record<string, string>;
      };
      let parsedBody: unknown;
      try { parsedBody = JSON.parse(body); } catch { throw new Error("Request body must be valid JSON."); }
      const response = await mutations.tryEndpoint.mutateAsync({
        endpointId: endpoint.id,
        body: {
          environment_id: environmentId,
          path_values: parseObject(pathValues, "Path values"),
          query: parseObject(query, "Query"),
          headers: parseObject(headers, "Headers"),
          body: parsedBody,
        },
      });
      setResult(response);
    } catch (err) { setError(errorMessage(err)); }
  };
  return (
    <Dialog open={open} onOpenChange={(next) => !mutations.tryEndpoint.isPending && (next ? onOpenChange(true) : close())}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <form onSubmit={run} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Try endpoint</DialogTitle>
            <DialogDescription><span className="font-mono">{endpoint.method.toUpperCase()} {endpoint.path}</span> · Calls are sent through guarded egress.</DialogDescription>
          </DialogHeader>
          <Field label="Environment">
            <Select value={environmentId} onValueChange={setEnvironmentId}>
              <SelectTrigger className="w-full"><SelectValue placeholder="Choose environment" /></SelectTrigger>
              <SelectContent>{environmentOptions.map((item) => <SelectItem key={item.environment_id} value={item.environment_id}>{item.name} · {item.base_url}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          {environments.error && <QueryError error={environments.error} onRetry={() => environments.refetch()} title="Could not load environments" />}
          <div className="grid gap-3 sm:grid-cols-2">
            <JsonInput label="Path values" value={pathValues} onChange={setPathValues} />
            <JsonInput label="Query parameters" value={query} onChange={setQuery} />
            <JsonInput label="Headers" value={headers} onChange={setHeaders} />
            <JsonInput label="Request body" value={body} onChange={setBody} />
          </div>
          {result && <TryResult result={result} />}
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>Close</Button>
            <Button type="submit" disabled={!environmentId || mutations.tryEndpoint.isPending}>{mutations.tryEndpoint.isPending ? "Sending…" : "Send request"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function TryResult({ result }: { result: ServiceTryResult }) {
  return <div className="rounded-lg border p-3">
    <p className="text-sm font-medium">HTTP {result.status} <span className="ml-2 text-xs font-normal text-muted-foreground">{result.duration_ms} ms</span></p>
    <pre className="mt-2 max-h-56 overflow-auto whitespace-pre-wrap break-all rounded bg-muted p-3 text-xs">{JSON.stringify(result.body, null, 2)}</pre>
  </div>;
}

function JsonInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <Field label={label}>
    <Textarea value={value} onChange={(event) => onChange(event.target.value)} className="min-h-24 font-mono text-xs" spellCheck={false} />
  </Field>;
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <div className="space-y-1.5">
    <Label>{label}</Label>
    {children}
    {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
  </div>;
}
