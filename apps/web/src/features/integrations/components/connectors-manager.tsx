"use client";

import { Activity, PlugZap, Plus, Trash2 } from "lucide-react";
import { useMemo, useState, type FormEvent, type ReactNode } from "react";
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
import { Switch } from "@/components/ui/switch";
import { useCan } from "@/features/access/api";
import { useCredentials } from "@/features/secrets/api";
import { useEnvironments, useWorkspaces } from "@/features/workspaces/api";
import { errorMessage, toastError } from "@/lib/errors";
import {
  useConnectorDefinitions,
  useConnectorHealth,
  useConnectorMutations,
  useConnectorVersions,
  useConnectors,
  type ConnectorInstance,
} from "../api";

export function ConnectorsManager() {
  const can = useCan();
  const q = useConnectors();
  const mutations = useConnectorMutations();
  const [creating, setCreating] = useState(false);
  const [testing, setTesting] = useState<string>();

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Connectors" description="Connect workspaces to external systems without exposing stored credentials." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Configured connections</CardTitle>
            <CardDescription>Each connection is scoped to a workspace and environment.</CardDescription>
          </div>
          {can("connector:create") && <Button onClick={() => setCreating(true)}><Plus aria-hidden /> Add connector</Button>}
        </CardHeader>
        <CardContent>
          {q.isPending ? <Skeleton className="h-28 w-full" /> : q.error ? (
            <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load connectors" />
          ) : q.data.length === 0 ? (
            <div className="rounded-lg border border-dashed p-9 text-center">
              <PlugZap className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden />
              <p className="text-sm font-medium">No connectors configured</p>
              <p className="mt-1 text-sm text-muted-foreground">Add a connector to give a workspace a safe route to an external system.</p>
            </div>
          ) : (
            <ul className="divide-y">
              {q.data.map((instance) => (
                <ConnectorRow
                  key={instance.id}
                  instance={instance}
                  canTest={can("connector:test", instance.workspace_id)}
                  canUpdate={can("connector:update", instance.workspace_id)}
                  canDelete={can("connector:delete", instance.workspace_id)}
                  testing={testing === instance.id}
                  onTest={() => setTesting(instance.id)}
                  onTestDone={() => setTesting(undefined)}
                  onToggle={async () => {
                    try {
                      await mutations.update.mutateAsync({
                        id: instance.id,
                        body: { status: instance.status === "enabled" ? "disabled" : "enabled" },
                      });
                      toast.success(`Connector ${instance.status === "enabled" ? "disabled" : "enabled"}.`);
                    } catch (error) {
                      toastError(error);
                    }
                  }}
                  onRemove={async () => {
                    try {
                      await mutations.remove.mutateAsync(instance.id);
                      toast.success("Connector removed.");
                    } catch (error) {
                      toastError(error);
                      throw error;
                    }
                  }}
                />
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
      <ConnectorCreateDialog open={creating} onOpenChange={setCreating} />
    </div>
  );
}

function ConnectorRow({
  instance,
  canTest,
  canUpdate,
  canDelete,
  testing,
  onTest,
  onTestDone,
  onToggle,
  onRemove,
}: {
  instance: ConnectorInstance;
  canTest: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  testing: boolean;
  onTest: () => void;
  onTestDone: () => void;
  onToggle: () => Promise<void>;
  onRemove: () => Promise<void>;
}) {
  const health = useConnectorHealth(instance.id, true);
  const test = useConnectorMutations().test;
  const runTest = async () => {
    onTest();
    try {
      const result = await test.mutateAsync(instance.id);
      if (result.ok) toast.success(`Connection healthy (${result.latency_ms} ms).`);
      else toast.error(result.message);
    } catch (error) {
      toastError(error);
    } finally {
      onTestDone();
    }
  };
  const latest = health.data?.[0];
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 py-4">
      <div className="min-w-0">
        <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
          {instance.name}
          <Badge variant={instance.status === "enabled" ? "default" : "outline"}>{instance.status}</Badge>
          {instance.credential_configured && <Badge variant="outline">credential linked</Badge>}
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          {instance.definition_key} · {instance.ref_key} · {instance.version} · {instance.auth_method || "no auth"}
        </p>
        {latest && (
          <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
            <Activity className="size-3" aria-hidden />
            Last check {latest.ok ? `healthy in ${latest.latency_ms} ms` : latest.message}
          </p>
        )}
      </div>
      <div className="flex items-center gap-1">
        {canTest && <Button variant="outline" size="sm" onClick={runTest} disabled={testing}>
          {testing ? "Testing…" : "Test connection"}
        </Button>}
        {canUpdate && <Button variant="ghost" size="sm" onClick={() => void onToggle()}>
          {instance.status === "enabled" ? "Disable" : "Enable"}
        </Button>}
        {canDelete && <ConfirmDialog
          trigger={<Button variant="ghost" size="icon" aria-label={`Remove ${instance.name}`}><Trash2 /></Button>}
          title={`Remove “${instance.name}”?`}
          description="Workflows using this connection will no longer be able to call it."
          confirmLabel="Remove connector"
          onConfirm={onRemove}
        />}
      </div>
    </li>
  );
}

function ConnectorCreateDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const workspaces = useWorkspaces(false);
  const environments = useEnvironments();
  const definitions = useConnectorDefinitions();
  const can = useCan();
  const credentials = useCredentials(can("secret:read"));
  const { create } = useConnectorMutations();
  const [workspaceId, setWorkspaceId] = useState("");
  const [environmentId, setEnvironmentId] = useState("");
  const [definitionId, setDefinitionId] = useState("");
  const [versionId, setVersionId] = useState("");
  const [refKey, setRefKey] = useState("");
  const [authMethod, setAuthMethod] = useState("");
  const [secretId, setSecretId] = useState("");
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [error, setError] = useState("");
  const versions = useConnectorVersions(definitionId);
  const selectedVersion = versions.data?.find((version) => version.id === versionId);
  const configProperties = asRecord(selectedVersion?.config_schema.properties) ?? {};
  const requiredConfig = new Set(
    Array.isArray(selectedVersion?.config_schema.required)
      ? selectedVersion.config_schema.required.filter((value): value is string => typeof value === "string")
      : [],
  );
  const environmentOptions = environments.data ?? [];
  const credentialOptions = useMemo(
    () => credentials.data?.filter((item) => !item.environment_id || item.environment_id === environmentId) ?? [],
    [credentials.data, environmentId],
  );
  const optionErrors = [workspaces.error, environments.error, definitions.error, credentials.error, versions.error].filter(Boolean);

  const close = () => {
    setWorkspaceId("");
    setEnvironmentId("");
    setDefinitionId("");
    setVersionId("");
    setRefKey("");
    setAuthMethod("");
    setSecretId("");
    setConfig({});
    setError("");
    onOpenChange(false);
  };
  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    try {
      await create.mutateAsync({
        workspace_id: workspaceId,
        environment_id: environmentId,
        ref_key: refKey.trim(),
        connector_version_id: versionId,
        config,
        ...(authMethod ? { auth_method: authMethod } : {}),
        ...(secretId ? { secret_id: secretId } : {}),
      });
      toast.success("Connector added.");
      close();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !create.isPending && (next ? onOpenChange(true) : close())}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <form onSubmit={save} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Add a connector</DialogTitle>
            <DialogDescription>Choose its scope and version, then provide non-secret configuration and a vault credential.</DialogDescription>
          </DialogHeader>
          {optionErrors.length > 0 && (
            <p role="alert" className="text-sm text-destructive">
              Could not load connector options: {errorMessage(optionErrors[0])}
            </p>
          )}
          <Field label="Workspace">
            <Select value={workspaceId} onValueChange={(value) => { setWorkspaceId(value); setEnvironmentId(""); }}>
              <SelectTrigger className="w-full"><SelectValue placeholder="Choose workspace" /></SelectTrigger>
              <SelectContent>{workspaces.data?.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Environment">
            <Select value={environmentId} onValueChange={setEnvironmentId} disabled={!workspaceId}>
              <SelectTrigger className="w-full"><SelectValue placeholder="Choose environment" /></SelectTrigger>
              <SelectContent>{environmentOptions.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Connector">
            <Select value={definitionId} onValueChange={(value) => { setDefinitionId(value); setVersionId(""); setAuthMethod(""); setConfig({}); }}>
              <SelectTrigger className="w-full"><SelectValue placeholder="Choose connector" /></SelectTrigger>
              <SelectContent>{definitions.data?.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Version">
            <Select value={versionId} onValueChange={(value) => { setVersionId(value); setAuthMethod(""); }}>
              <SelectTrigger className="w-full"><SelectValue placeholder="Choose version" /></SelectTrigger>
              <SelectContent>{versions.data?.map((item) => <SelectItem key={item.id} value={item.id}>{item.version}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field label="Connection key" hint="A stable, short name used by workflows.">
            <Input value={refKey} onChange={(event) => setRefKey(event.target.value)} required maxLength={100} placeholder="salesforce-prod" />
          </Field>
          {selectedVersion?.auth_methods.length ? (
            <Field label="Authentication">
              <Select value={authMethod} onValueChange={setAuthMethod}>
                <SelectTrigger className="w-full"><SelectValue placeholder="Choose authentication" /></SelectTrigger>
                <SelectContent>{selectedVersion.auth_methods.map((method) => <SelectItem key={method} value={method}>{method}</SelectItem>)}</SelectContent>
              </Select>
            </Field>
          ) : null}
          <Field label="Vault credential" hint="Secret values are never shown here.">
            <Select value={secretId || "none"} onValueChange={(value) => setSecretId(value === "none" ? "" : value)}>
              <SelectTrigger className="w-full"><SelectValue placeholder="No credential" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="none">No credential</SelectItem>
                {credentialOptions.map((item) => <SelectItem key={item.id} value={item.id}>{item.name} · {item.kind}</SelectItem>)}
              </SelectContent>
            </Select>
          </Field>
          {selectedVersion && (
            <div className="space-y-3">
              <div>
                <p className="text-sm font-medium">Configuration</p>
                <p className="text-xs text-muted-foreground">Fields follow this connector version’s schema. Keep secret values in the vault.</p>
              </div>
              {Object.entries(configProperties).map(([key, rawSchema]) => {
                const schema = asRecord(rawSchema) ?? {};
                const title = typeof schema.title === "string" ? schema.title : key.replaceAll("_", " ");
                const fieldType = typeof schema.type === "string" ? schema.type : "string";
                const current = config[key] ?? schema.default;
                const enumOptions = Array.isArray(schema.enum)
                  ? schema.enum.filter((item): item is string => typeof item === "string")
                  : [];
                return (
                  <Field key={key} label={`${title}${requiredConfig.has(key) ? " *" : ""}`} hint={typeof schema.description === "string" ? schema.description : undefined}>
                    {enumOptions.length ? (
                      <Select
                        value={typeof current === "string" ? current : "__unset"}
                        onValueChange={(value) => setConfig((old) => ({ ...old, [key]: value === "__unset" ? undefined : value }))}
                      >
                        <SelectTrigger className="w-full"><SelectValue placeholder="Choose a value" /></SelectTrigger>
                        <SelectContent>
                          {!requiredConfig.has(key) && <SelectItem value="__unset">Not set</SelectItem>}
                          {enumOptions.map((option) => <SelectItem key={option} value={option}>{option}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    ) : fieldType === "boolean" ? (
                      <Switch
                        checked={typeof current === "boolean" ? current : false}
                        onCheckedChange={(checked) => setConfig((old) => ({ ...old, [key]: checked }))}
                        aria-label={title}
                      />
                    ) : (
                      <Input
                        type={fieldType === "integer" || fieldType === "number" ? "number" : "text"}
                        step={fieldType === "integer" ? "1" : fieldType === "number" ? "any" : undefined}
                        value={current === undefined || current === null ? "" : String(current)}
                        onChange={(event) => {
                          const raw = event.target.value;
                          setConfig((old) => ({
                            ...old,
                            [key]: raw === "" ? undefined :
                              fieldType === "integer" ? Number.parseInt(raw, 10) :
                                fieldType === "number" ? Number(raw) : raw,
                          }));
                        }}
                        required={requiredConfig.has(key)}
                        maxLength={fieldType === "integer" || fieldType === "number" ? undefined : 2000}
                      />
                    )}
                  </Field>
                );
              })}
              {Object.keys(configProperties).length === 0 && <p className="text-sm text-muted-foreground">This connector has no configuration fields.</p>}
            </div>
          )}
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close} disabled={create.isPending}>Cancel</Button>
            <Button type="submit" disabled={create.isPending || optionErrors.length > 0 || !workspaceId || !environmentId || !versionId || !refKey.trim()}>
              {create.isPending ? "Adding…" : "Add connector"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <div className="space-y-1.5">
    <Label>{label}</Label>
    {children}
    {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
  </div>;
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : undefined;
}
