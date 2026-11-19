"use client";

import { Activity, Pencil, PlugZap, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { CopyField } from "@/components/copy-field";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useCan } from "@/features/access/api";
import { ApiError } from "@/lib/api/client";
import { errorMessage, toastError } from "@/lib/errors";
import { keepOpen } from "@/lib/forms";
import {
  useGroupMappings,
  useGroups,
  useIdentityProviderMutations,
  useIdentityProviders,
  type GroupMappingCreate,
  type IdentityProvider,
  type IdentityProviderCreate,
  type IdentityProviderUpdate,
} from "../api";

const PRESETS = ["entra", "google", "keycloak", "generic"] as const;
type Preset = (typeof PRESETS)[number];
const PRESET_LABEL: Record<Preset, string> = {
  entra: "Microsoft Entra ID",
  google: "Google Workspace",
  keycloak: "Keycloak",
  generic: "Generic OIDC",
};

function list(value: string): string[] {
  return value.split(/[\n,]+/).map((v) => v.trim()).filter(Boolean);
}

export function IdentityProvidersManager() {
  const can = useCan();
  const q = useIdentityProviders();
  const [editing, setEditing] = useState<IdentityProvider | "new">();
  const [mappingsFor, setMappingsFor] = useState<IdentityProvider>();
  return (
    <>
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Identity providers</CardTitle>
            <CardDescription>
              Let members sign in with your identity provider over OIDC. Members are created on first
              sign-in when just-in-time provisioning is on; otherwise invite them first.
            </CardDescription>
          </div>
          {can("identity_provider:create") && (
            <Button onClick={() => setEditing("new")}><Plus aria-hidden /> New provider</Button>
          )}
        </CardHeader>
        <CardContent>
          {q.isPending ? (
            <Skeleton className="h-24 w-full" />
          ) : q.error ? (
            <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load identity providers" />
          ) : q.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">No identity providers yet.</p>
          ) : (
            <ul className="divide-y">
              {q.data.map((p) => (
                <ProviderRow
                  key={p.id}
                  provider={p}
                  onEdit={() => setEditing(p)}
                  onMappings={() => setMappingsFor(p)}
                  canUpdate={can("identity_provider:update")}
                  canDelete={can("identity_provider:delete")}
                  canTest={can("identity_provider:test")}
                />
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
      {editing && (
        <ProviderDialog provider={editing === "new" ? undefined : editing} onClose={() => setEditing(undefined)} />
      )}
      {mappingsFor && <MappingsDialog provider={mappingsFor} onClose={() => setMappingsFor(undefined)} />}
    </>
  );
}

function ProviderRow({
  provider: p,
  onEdit,
  onMappings,
  canUpdate,
  canDelete,
  canTest,
}: {
  provider: IdentityProvider;
  onEdit: () => void;
  onMappings: () => void;
  canUpdate: boolean;
  canDelete: boolean;
  canTest: boolean;
}) {
  const { test, remove } = useIdentityProviderMutations();
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium">
          {p.name}{" "}
          <Badge variant="outline" className="ml-1">{PRESET_LABEL[p.preset]}</Badge>{" "}
          {!p.enabled && <Badge variant="destructive">Disabled</Badge>}
          {p.has_client_secret && <Badge variant="outline" className="ml-1">Client secret</Badge>}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {p.issuer} · {p.group_mapping_count} group mapping{p.group_mapping_count === 1 ? "" : "s"}
          {p.allowed_domains.length > 0 && ` · ${p.allowed_domains.join(", ")}`}
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-1">
        {canTest && (
          <Button
            variant="outline"
            size="sm"
            disabled={test.isPending}
            onClick={async () => {
              try {
                const r = await test.mutateAsync(p.id);
                if (r.ok) toast.success(`Discovery OK in ${r.latency_ms} ms.`);
                else toast.error(r.error ?? "Discovery failed.");
              } catch (err) {
                toastError(err);
              }
            }}
          >
            <Activity aria-hidden /> {test.isPending ? "Testing…" : "Test"}
          </Button>
        )}
        <Button variant="outline" size="sm" onClick={onMappings}>
          <PlugZap aria-hidden /> Groups
        </Button>
        {canUpdate && (
          <Button variant="ghost" size="icon" aria-label={`Edit ${p.name}`} onClick={onEdit}><Pencil /></Button>
        )}
        {canDelete && (
          <ConfirmDialog
            trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${p.name}`}><Trash2 /></Button>}
            title={`Delete “${p.name}”?`}
            description="Its sign-in links and group mappings are removed. Members stay, but can no longer sign in through this provider."
            confirmLabel="Delete provider"
            onConfirm={async () => {
              try {
                await remove.mutateAsync(p.id);
                toast.success("Identity provider deleted.");
              } catch (err) {
                toastError(err);
                throw err;
              }
            }}
          />
        )}
      </div>
    </li>
  );
}

function ProviderDialog({ provider, onClose }: { provider?: IdentityProvider; onClose: () => void }) {
  const { create, update } = useIdentityProviderMutations();
  const [name, setName] = useState(provider?.name ?? "");
  const [preset, setPreset] = useState<Preset>(provider?.preset ?? "generic");
  const [issuer, setIssuer] = useState(provider?.issuer ?? "");
  const [clientId, setClientId] = useState(provider?.client_id ?? "");
  const [clientSecret, setClientSecret] = useState("");
  const [clearSecret, setClearSecret] = useState(false);
  const [scopes, setScopes] = useState((provider?.scopes ?? ["openid", "email", "profile"]).join(", "));
  const [emailClaim, setEmailClaim] = useState(provider?.claim_mapping.email ?? "email");
  const [nameClaim, setNameClaim] = useState(provider?.claim_mapping.name ?? "name");
  const [groupsClaim, setGroupsClaim] = useState(provider?.claim_mapping.groups ?? "groups");
  const [jit, setJit] = useState(provider?.jit_provisioning ?? true);
  const [domains, setDomains] = useState((provider?.allowed_domains ?? []).join("\n"));
  const [enabled, setEnabled] = useState(provider?.enabled ?? true);
  const [error, setError] = useState<string>();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const pending = create.isPending || update.isPending;

  async function save() {
    setError(undefined);
    setFieldErrors({});
    const body: IdentityProviderCreate = {
      name: name.trim(),
      preset,
      issuer: issuer.trim(),
      client_id: clientId.trim(),
      scopes: list(scopes),
      claim_mapping: {
        email: emailClaim.trim(),
        name: nameClaim.trim(),
        groups: groupsClaim.trim() || undefined,
      },
      jit_provisioning: jit,
      allowed_domains: list(domains),
      enabled,
    };
    try {
      if (!provider) {
        await create.mutateAsync({ ...body, client_secret: clientSecret || undefined });
        toast.success(`Identity provider “${body.name}” created.`);
      } else {
        const patch: IdentityProviderUpdate = { ...body };
        if (clearSecret) patch.client_secret = "";
        else if (clientSecret) patch.client_secret = clientSecret;
        await update.mutateAsync({ id: provider.id, body: patch });
        toast.success("Identity provider saved.");
      }
      onClose();
    } catch (err) {
      if (err instanceof ApiError) {
        const errors: Record<string, string> = {};
        for (const fe of err.fieldErrors ?? []) errors[fe.field] = fe.message;
        setFieldErrors(errors);
        if (Object.keys(errors).length === 0) setError(errorMessage(err));
      } else {
        setError(errorMessage(err));
      }
    }
  }

  const fieldError = (field: string) =>
    fieldErrors[field] && <p role="alert" className="text-sm text-destructive">{fieldErrors[field]}</p>;

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg" onInteractOutside={keepOpen}>
        <form noValidate className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); save(); }}>
          <DialogHeader>
            <DialogTitle>{provider ? "Edit identity provider" : "New identity provider"}</DialogTitle>
            <DialogDescription>
              {provider
                ? "Omit the client secret to keep the current one."
                : "Discovery is read from the issuer's /.well-known/openid-configuration."}
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-name">Display name</Label>
            <Input id="idp-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
            {fieldError("name")}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-preset">Provider</Label>
            <Select value={preset} onValueChange={(v) => setPreset(v as Preset)}>
              <SelectTrigger id="idp-preset"><SelectValue /></SelectTrigger>
              <SelectContent>
                {PRESETS.map((p) => <SelectItem key={p} value={p}>{PRESET_LABEL[p]}</SelectItem>)}
              </SelectContent>
            </Select>
            {fieldError("preset")}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-issuer">Issuer URL</Label>
            <Input id="idp-issuer" value={issuer} placeholder="https://login.example.com/realms/acme" onChange={(e) => setIssuer(e.target.value)} />
            {fieldError("issuer")}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-client">Client ID</Label>
            <Input id="idp-client" value={clientId} onChange={(e) => setClientId(e.target.value)} />
            {fieldError("client_id")}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-secret">Client secret</Label>
            <Input id="idp-secret" type="password" autoComplete="new-password" value={clientSecret}
              placeholder={provider?.has_client_secret && !clearSecret ? "Leave blank to keep the current secret" : "Optional for public clients using PKCE"}
              onChange={(e) => setClientSecret(e.target.value)} />
            {provider?.has_client_secret && (
              <label className="flex items-center gap-2 text-sm text-muted-foreground">
                <input type="checkbox" checked={clearSecret} onChange={(e) => setClearSecret(e.target.checked)} />
                Remove the stored secret
              </label>
            )}
            {fieldError("client_secret")}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-scopes">Scopes</Label>
            <Input id="idp-scopes" value={scopes} onChange={(e) => setScopes(e.target.value)} />
            <p className="text-xs text-muted-foreground">Comma-separated. <code className="font-mono">openid</code> is always requested.</p>
            {fieldError("scopes")}
          </div>

          <fieldset className="flex flex-col gap-3">
            <legend className="text-sm font-medium">Claims</legend>
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-claim-email">Email</Label>
                <Input id="idp-claim-email" value={emailClaim} onChange={(e) => setEmailClaim(e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-claim-name">Name</Label>
                <Input id="idp-claim-name" value={nameClaim} onChange={(e) => setNameClaim(e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-claim-groups">Groups</Label>
                <Input id="idp-claim-groups" value={groupsClaim} onChange={(e) => setGroupsClaim(e.target.value)} />
              </div>
            </div>
            {fieldError("claim_mapping")}
            {fieldError("claim_mapping.email")}
            {fieldError("claim_mapping.name")}
            {fieldError("claim_mapping.groups")}
            <p className="text-xs text-muted-foreground">Leave Groups empty to disable group mapping.</p>
          </fieldset>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="idp-domains">Allowed email domains</Label>
            <Textarea id="idp-domains" rows={2} value={domains} placeholder="acme.com\nacme.co.id" onChange={(e) => setDomains(e.target.value)} />
            <p className="text-xs text-muted-foreground">One per line. Empty allows every domain the provider asserts.</p>
            {fieldError("allowed_domains")}
          </div>

          <label className="flex items-center justify-between gap-3 text-sm">
            <span>
              <span className="block font-medium">Just-in-time provisioning</span>
              <span className="block text-xs text-muted-foreground">Create members on first sign-in. Off: only existing members can link.</span>
            </span>
            <Switch checked={jit} onCheckedChange={setJit} aria-label="Just-in-time provisioning" />
          </label>

          <label className="flex items-center justify-between gap-3 text-sm">
            <span>
              <span className="block font-medium">Enabled</span>
              <span className="block text-xs text-muted-foreground">Disabled providers cannot start a sign-in.</span>
            </span>
            <Switch checked={enabled} onCheckedChange={setEnabled} aria-label="Enabled" />
          </label>

          {provider && (
            <CopyField id="idp-redirect" label="Redirect URI to register at the provider" value={provider.redirect_uri} />
          )}

          {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={!name.trim() || !issuer.trim() || !clientId.trim() || pending}>
              {pending ? "Saving…" : provider ? "Save" : "Create provider"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function MappingsDialog({ provider, onClose }: { provider: IdentityProvider; onClose: () => void }) {
  const q = useGroupMappings(provider.id);
  const groups = useGroups();
  const { createMapping, removeMapping } = useIdentityProviderMutations();
  const [external, setExternal] = useState("");
  const [target, setTarget] = useState("__new__");
  const [error, setError] = useState<string>();

  async function add() {
    setError(undefined);
    const body: GroupMappingCreate = {
      external_group: external.trim(),
      group_id: target === "__new__" ? undefined : target,
    };
    try {
      const created = await createMapping.mutateAsync({ id: provider.id, body });
      toast.success(
        target === "__new__"
          ? `Group “${created.group_name}” created and mapped.`
          : `Mapped “${created.external_group}” to ${created.group_name}.`,
      );
      setExternal("");
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Group mappings · {provider.name}</DialogTitle>
          <DialogDescription>
            Group names asserted by the identity provider are matched at each sign-in. Mapped groups then
            grant the roles assigned to them in Access.
          </DialogDescription>
        </DialogHeader>

        {provider.enabled || (
          <Alert>
            <AlertDescription>This provider is disabled, so mappings will not be synced until it is enabled.</AlertDescription>
          </Alert>
        )}

        <div className="flex flex-col gap-2 rounded-md border p-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="mapping-external">Group name from the identity provider</Label>
            <Input id="mapping-external" value={external} placeholder="engineering" onChange={(e) => setExternal(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="mapping-target">Hugimethara group</Label>
            <Select value={target} onValueChange={setTarget} disabled={groups.isPending}>
              <SelectTrigger id="mapping-target"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="__new__">Create an IdP-managed group named after it</SelectItem>
                {(groups.data ?? []).map((g) => (
                  <SelectItem key={g.id} value={g.id}>{g.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          <Button
            className="self-start"
            disabled={!external.trim() || createMapping.isPending}
            onClick={add}
          >
            {createMapping.isPending ? "Adding…" : "Add mapping"}
          </Button>
        </div>

        {q.isPending ? (
          <Skeleton className="h-20 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load mappings" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No mappings yet.</p>
        ) : (
          <ul className="max-h-72 divide-y overflow-y-auto">
            {q.data.map((m) => (
              <li key={m.id} className="flex items-center justify-between gap-2 py-2">
                <span className="min-w-0">
                  <span className="block truncate text-sm">{m.external_group} → {m.group_name}</span>
                  <span className="block text-xs text-muted-foreground">
                    {m.group_source === "idp" ? "IdP-managed group" : "Internal group"}
                  </span>
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Remove mapping for ${m.external_group}`}
                  disabled={removeMapping.isPending}
                  onClick={async () => {
                    try {
                      await removeMapping.mutateAsync({ id: provider.id, mappingId: m.id });
                    } catch (err) {
                      toastError(err);
                    }
                  }}
                >
                  <Trash2 />
                </Button>
              </li>
            ))}
          </ul>
        )}
        <DialogFooter><Button variant="ghost" onClick={onClose}>Close</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
