"use client";

import { KeyRound, Plus, Trash2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { CopyField } from "@/components/copy-field";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@hugimethara/ui/components/dialog";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Switch } from "@hugimethara/ui/components/switch";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { useCan, useRoleBindings } from "@/features/access/api";
import { ApiError } from "@hugimethara/api-client";
import { errorMessage, toastError } from "@/lib/errors";
import { keepOpen } from "@/lib/forms";
import { useApiKeys, useServiceAccountMutations, useServiceAccounts, type ApiKeyCreated, type ServiceAccount } from "../api";

export function ServiceAccountsManager() {
  const can = useCan();
  const q = useServiceAccounts();
  const [creating, setCreating] = useState(false);
  const [keysFor, setKeysFor] = useState<ServiceAccount>();
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Service accounts</CardTitle>
          <CardDescription>
            Identities for scripts and integrations. They call the API with{" "}
            <code className="font-mono text-xs">Authorization: Bearer &lt;key&gt;</code> and hold only the roles you assign in{" "}
            <Link className="underline underline-offset-2" href="/governance/access">Access</Link>.
          </CardDescription>
        </div>
        {can("service_account:create") && <Button onClick={() => setCreating(true)}><Plus aria-hidden /> New service account</Button>}
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load service accounts" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No service accounts yet.</p>
        ) : (
          <ul className="divide-y">
            {q.data.map((sa) => (
              <AccountRow key={sa.id} sa={sa} onKeys={() => setKeysFor(sa)}
                canUpdate={can("service_account:update")} canDelete={can("service_account:delete")} />
            ))}
          </ul>
        )}
      </CardContent>
      {creating && <CreateDialog onClose={() => setCreating(false)} />}
      {keysFor && <KeysDialog sa={keysFor} canManage={can("service_account:manage_keys")} onClose={() => setKeysFor(undefined)} />}
    </Card>
  );
}

function AccountRow({ sa, onKeys, canUpdate, canDelete }: { sa: ServiceAccount; onKeys: () => void; canUpdate: boolean; canDelete: boolean }) {
  const { update, remove } = useServiceAccountMutations();
  const roles = useRoleBindings({ service_account_id: sa.id });
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium">
          {sa.name} {sa.disabled && <Badge variant="destructive" className="ml-1">Disabled</Badge>}
        </p>
        <p className="text-xs text-muted-foreground">
          {sa.active_key_count} active key{sa.active_key_count === 1 ? "" : "s"}
          {" · "}
          {roles.data
            ? roles.data.length === 0
              ? "no roles"
              : roles.data.map((b) => b.role.name + (b.scope.type === "workspace" ? ` (${b.workspace_name})` : "")).join(", ")
            : "…"}
          {sa.description && ` · ${sa.description}`}
        </p>
      </div>
      <div className="flex items-center gap-2">
        {canUpdate && (
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={!sa.disabled} disabled={update.isPending} aria-label={`${sa.name} enabled`}
              onCheckedChange={async (on) => {
                try {
                  await update.mutateAsync({ id: sa.id, body: { name: sa.name, description: sa.description, disabled: !on } });
                  toast.success(on ? `${sa.name} enabled.` : `${sa.name} disabled; its keys stop working now.`);
                } catch (err) {
                  toastError(err);
                }
              }} />
            Enabled
          </label>
        )}
        <Button variant="outline" size="sm" onClick={onKeys}><KeyRound aria-hidden /> Keys</Button>
        {canDelete && (
          <ConfirmDialog
            trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${sa.name}`}><Trash2 /></Button>}
            title={`Delete “${sa.name}”?`}
            description="Its keys stop working immediately and its role assignments are removed. This cannot be undone."
            confirmLabel="Delete service account"
            onConfirm={async () => {
              try {
                await remove.mutateAsync(sa.id);
                toast.success("Service account deleted.");
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

function CreateDialog({ onClose }: { onClose: () => void }) {
  const { create } = useServiceAccountMutations();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string>();
  const [nameError, setNameError] = useState<string>();

  async function save() {
    setError(undefined);
    setNameError(undefined);
    try {
      await create.mutateAsync({ name, description });
      toast.success(`Service account “${name.trim()}” created. Assign it roles in Access, then create a key.`);
      onClose();
    } catch (err) {
      const fe = err instanceof ApiError ? err.fieldErrors?.find((f) => f.field === "name") : undefined;
      if (fe) setNameError(fe.message);
      else if (err instanceof ApiError && err.code === "name_taken") setNameError(err.message);
      else setError(errorMessage(err));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen}>
        <form noValidate className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); save(); }}>
          <DialogHeader>
            <DialogTitle>New service account</DialogTitle>
            <DialogDescription>It starts with no access at all.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sa-name">Name</Label>
            <Input id="sa-name" value={name} maxLength={100} placeholder="e.g. CI deployer" onChange={(e) => setName(e.target.value)}
              aria-invalid={nameError ? true : undefined} aria-describedby={nameError ? "sa-name-error" : undefined} />
            {nameError && <p id="sa-name-error" role="alert" className="text-sm text-destructive">{nameError}</p>}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sa-desc">Description</Label>
            <Textarea id="sa-desc" rows={2} maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={!name.trim() || create.isPending}>{create.isPending ? "Creating…" : "Create"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function keyState(k: { revoked_at?: string; expires_at?: string }) {
  if (k.revoked_at) return "Revoked";
  if (k.expires_at && new Date(k.expires_at) <= new Date()) return "Expired";
  return undefined;
}

function KeysDialog({ sa, canManage, onClose }: { sa: ServiceAccount; canManage: boolean; onClose: () => void }) {
  const q = useApiKeys(sa.id);
  const { createKey, revokeKey } = useServiceAccountMutations();
  const [name, setName] = useState("");
  const [expires, setExpires] = useState("");
  const [created, setCreated] = useState<ApiKeyCreated>();
  const [error, setError] = useState<string>();

  async function create() {
    setError(undefined);
    try {
      const res = await createKey.mutateAsync({ id: sa.id, body: {
        name, expires_at: expires ? new Date(`${expires}T23:59:59`).toISOString() : undefined } });
      setCreated(res);
      setName("");
      setExpires("");
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen} className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Keys for {sa.name}</DialogTitle>
          <DialogDescription>Keep at most two active keys while rotating. Revoked keys stop working on the next request.</DialogDescription>
        </DialogHeader>
        {created && (
          <Alert>
            <AlertDescription className="flex flex-col gap-3">
              <span>Copy the key now — it will not be shown again.</span>
              <CopyField id="new-key" label={`Key “${created.key.name}”`} value={created.secret} />
            </AlertDescription>
          </Alert>
        )}
        {canManage && !sa.disabled && (
          <form className="flex flex-wrap items-end gap-2" onSubmit={(e) => { e.preventDefault(); create(); }}>
            <div className="flex min-w-40 flex-1 flex-col gap-1.5">
              <Label htmlFor="k-name">New key name</Label>
              <Input id="k-name" value={name} maxLength={100} placeholder="e.g. production" onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="k-exp">Expires (optional)</Label>
              <Input id="k-exp" type="date" value={expires} min={new Date().toISOString().slice(0, 10)} onChange={(e) => setExpires(e.target.value)} />
            </div>
            <Button type="submit" disabled={!name.trim() || createKey.isPending}>{createKey.isPending ? "Creating…" : "Create key"}</Button>
          </form>
        )}
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        {q.isPending ? (
          <Skeleton className="h-20 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load keys" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No keys yet.</p>
        ) : (
          <ul className="max-h-72 divide-y overflow-y-auto">
            {q.data.map((k) => {
              const state = keyState(k);
              return (
                <li key={k.id} className="flex items-center justify-between gap-2 py-2">
                  <span className="min-w-0">
                    <span className="block text-sm">
                      {k.name} <code className="font-mono text-xs text-muted-foreground">hugi_{k.prefix}_…</code>
                      {state && <Badge variant="outline" className="ml-1">{state}</Badge>}
                    </span>
                    <span className="block text-xs text-muted-foreground">
                      Created {new Date(k.created_at).toLocaleDateString()}
                      {k.expires_at && ` · expires ${new Date(k.expires_at).toLocaleDateString()}`}
                      {` · ${k.last_used_at ? `last used ${new Date(k.last_used_at).toLocaleString()}` : "never used"}`}
                    </span>
                  </span>
                  {canManage && !state && (
                    <ConfirmDialog
                      trigger={<Button variant="ghost" size="sm">Revoke</Button>}
                      title={`Revoke “${k.name}”?`}
                      description="Anything using this key loses access on its next request."
                      confirmLabel="Revoke key"
                      onConfirm={async () => {
                        try {
                          await revokeKey.mutateAsync({ id: sa.id, keyId: k.id });
                          toast.success("Key revoked.");
                        } catch (err) {
                          toastError(err);
                          throw err;
                        }
                      }}
                    />
                  )}
                </li>
              );
            })}
          </ul>
        )}
        <DialogFooter><Button variant="ghost" onClick={onClose}>Close</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
