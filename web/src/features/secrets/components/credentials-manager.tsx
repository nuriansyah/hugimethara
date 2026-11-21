"use client";

import { KeyRound, Plus, RotateCw, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";
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
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { useCan } from "@/features/access/api";
import { errorMessage, toastError } from "@/lib/errors";
import { useCredentialMutations, useCredentials, type Credential } from "../api";

export function CredentialsManager() {
  const can = useCan();
  const q = useCredentials();
  const [creating, setCreating] = useState(false);
  const [rotating, setRotating] = useState<Credential>();
  const { remove } = useCredentialMutations();
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader title="Credentials" description="Encrypted credentials for integrations. Secret values are write-only." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Secret vault</CardTitle>
            <CardDescription>
              Values are encrypted with an organization data key. They can be replaced, but never read back.
            </CardDescription>
          </div>
          {can("secret:create") && <Button onClick={() => setCreating(true)}><Plus aria-hidden /> New credential</Button>}
        </CardHeader>
        <CardContent>
          {q.isPending ? <Skeleton className="h-24 w-full" /> : q.error ? (
            <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load credentials" />
          ) : q.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">No credentials stored.</p>
          ) : (
            <ul className="divide-y">
              {q.data.map((credential) => (
                <li key={credential.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                  <div className="min-w-0">
                    <p className="text-sm font-medium">
                      {credential.name} <Badge variant="outline" className="ml-1">{credential.kind}</Badge>
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {credential.environment_id ? "Environment-scoped" : "Organization-wide"} · {credential.hint ? `ending ${credential.hint}` : "value stored"} · v{credential.version}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    {can("secret:update") && (
                      <Button variant="outline" size="sm" onClick={() => setRotating(credential)}>
                        <RotateCw aria-hidden /> Rotate
                      </Button>
                    )}
                    {can("secret:delete") && (
                      <ConfirmDialog
                        trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${credential.name}`}><Trash2 /></Button>}
                        title={`Delete “${credential.name}”?`}
                        description="Any integration that references this credential will no longer be able to authenticate."
                        confirmLabel="Delete credential"
                        onConfirm={async () => {
                          try {
                            await remove.mutateAsync(credential.id);
                            toast.success("Credential deleted.");
                          } catch (error) {
                            toastError(error);
                          }
                        }}
                      />
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
      <p className="flex items-center gap-2 text-xs text-muted-foreground">
        <KeyRound aria-hidden className="size-4" /> Only the final four characters are shown as a hint; plaintext is never returned by the API.
      </p>
      <CredentialDialog open={creating} onOpenChange={setCreating} />
      {rotating && <CredentialDialog credential={rotating} open onOpenChange={(open) => !open && setRotating(undefined)} />}
    </div>
  );
}

function CredentialDialog({
  credential,
  open,
  onOpenChange,
}: {
  credential?: Credential;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { create, update } = useCredentialMutations();
  const [name, setName] = useState(credential?.name ?? "");
  const [kind, setKind] = useState(credential?.kind ?? "token");
  const [value, setValue] = useState("");
  const [error, setError] = useState("");
  const pending = create.isPending || update.isPending;
  const close = () => {
    setName("");
    setKind("token");
    setValue("");
    setError("");
    onOpenChange(false);
  };
  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    try {
      if (credential) {
        await update.mutateAsync({ id: credential.id, body: { value, ...(name !== credential.name ? { name } : {}) } });
        toast.success("Credential rotated.");
      } else {
        await create.mutateAsync({ name, kind, value });
        toast.success("Credential encrypted and stored.");
      }
      close();
    } catch (err) {
      setError(errorMessage(err));
    }
  };
  return (
    <Dialog open={open} onOpenChange={(next) => !pending && (next ? onOpenChange(true) : close())}>
      <DialogContent>
        <form onSubmit={save} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{credential ? "Rotate credential" : "New credential"}</DialogTitle>
            <DialogDescription>
              The secret value is encrypted immediately and is not shown again after saving.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="credential-name">Name</Label>
            <Input id="credential-name" value={name} onChange={(e) => setName(e.target.value)} required maxLength={100} />
          </div>
          {!credential && (
            <div className="space-y-2">
              <Label htmlFor="credential-kind">Kind</Label>
              <Input id="credential-kind" value={kind} onChange={(e) => setKind(e.target.value)} required maxLength={64} />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="credential-value">Secret value</Label>
            <Input id="credential-value" type="password" autoComplete="new-password" value={value}
              onChange={(e) => setValue(e.target.value)} required minLength={1} maxLength={65536} />
          </div>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close} disabled={pending}>Cancel</Button>
            <Button type="submit" disabled={pending || !name.trim() || !kind.trim() || !value}>
              {pending ? "Saving…" : credential ? "Rotate value" : "Encrypt and save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
