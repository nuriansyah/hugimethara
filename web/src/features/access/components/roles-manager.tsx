"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Lock, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { fieldAria, FormField } from "@/components/form-field";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@hugimethara/ui/components/dialog";
import { Input } from "@hugimethara/ui/components/input";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { ApiError } from "@hugimethara/api-client";
import { toastError } from "@/lib/errors";
import { applyApiError, keepOpen } from "@/lib/forms";
import { useCan, useResourceTypes, useRoleMutations, useRoles, type Role } from "../api";
import { PermissionMatrix } from "./permission-matrix";

export function RolesManager() {
  const can = useCan();
  const roles = useRoles();
  const [editing, setEditing] = useState<Role | "new" | null>(null);
  const { remove } = useRoleMutations();

  return (
    <div className="flex flex-col gap-4">
      {can("role:create") && (
        <div className="flex justify-end">
          <Button onClick={() => setEditing("new")}><Plus aria-hidden /> New role</Button>
        </div>
      )}
      {roles.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : roles.error ? (
        <QueryError error={roles.error} onRetry={() => roles.refetch()} title="Could not load roles" />
      ) : (
        <div className="grid gap-3 md:grid-cols-2">
          {roles.data.map((r) => (
            <Card key={r.id}>
              <CardHeader>
                <CardTitle className="flex items-center justify-between gap-2">
                  <span className="flex items-center gap-2">
                    {r.name}
                    {r.immutable && <Lock className="size-3.5 text-muted-foreground" aria-label="Permissions locked" />}
                  </span>
                  <span className="flex gap-1">
                    {can("role:update") && !r.immutable && (
                      <Button variant="ghost" size="icon" aria-label={`Edit ${r.name}`} onClick={() => setEditing(r)}><Pencil /></Button>
                    )}
                    {can("role:delete") && !r.is_system && (
                      <ConfirmDialog
                        trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${r.name}`}><Trash2 /></Button>}
                        title={`Delete “${r.name}”?`}
                        description="Only roles that are not assigned to anyone can be deleted."
                        confirmLabel="Delete role"
                        onConfirm={async () => {
                          try {
                            await remove.mutateAsync(r.id);
                            toast.success("Role deleted.");
                          } catch (err) {
                            toastError(err);
                            throw err;
                          }
                        }}
                      />
                    )}
                  </span>
                </CardTitle>
                <CardDescription className="flex flex-wrap items-center gap-2">
                  <span className="font-mono">{r.key}</span>
                  {r.is_system ? <Badge variant="secondary">Built-in</Badge> : <Badge variant="outline">Custom</Badge>}
                  <span>{r.binding_count} assignment{r.binding_count === 1 ? "" : "s"}</span>
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-2">
                {r.description && <p className="text-sm text-muted-foreground">{r.description}</p>}
                <div className="flex flex-wrap gap-1">
                  {r.permissions.length === 0 ? (
                    <span className="text-xs text-muted-foreground">No permissions</span>
                  ) : (
                    r.permissions.map((p) => <Badge key={p} variant="outline" className="font-mono text-[11px]">{p}</Badge>)
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
      {editing && <RoleDialog role={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
    </div>
  );
}

const schema = z.object({
  key: z.string().regex(/^[a-z0-9_]{2,40}$/, "2–40 lowercase letters, digits or underscores."),
  name: z.string().trim().min(1, "Enter a name.").max(100),
  description: z.string().max(500),
});
type Values = z.infer<typeof schema>;

function RoleDialog({ role, onClose }: { role: Role | null; onClose: () => void }) {
  const types = useResourceTypes();
  const { create, update } = useRoleMutations();
  const [perms, setPerms] = useState<string[]>(role?.permissions ?? []);
  const [formError, setFormError] = useState<string>();
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { key: role?.key ?? "", name: role?.name ?? "", description: role?.description ?? "" },
  });
  const { errors, isSubmitting } = form.formState;

  async function submit(v: Values) {
    setFormError(undefined);
    try {
      if (role) await update.mutateAsync({ id: role.id, body: { name: v.name, description: v.description, permissions: perms } });
      else await create.mutateAsync({ key: v.key, name: v.name, description: v.description, permissions: perms });
      toast.success(role ? "Role saved." : "Role created.");
      onClose();
    } catch (err) {
      if (err instanceof ApiError && err.code === "privilege_escalation") {
        setFormError(err.message);
        return;
      }
      setFormError(applyApiError(err, form.setError, ["key", "name", "description"]));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-3xl" onInteractOutside={keepOpen}>
        <form noValidate onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>{role ? `Edit ${role.name}` : "New role"}</DialogTitle>
            <DialogDescription>
              Changes apply to everyone with this role on their next request. You can only add permissions you hold yourself.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField id="role-name" label="Name" error={errors.name?.message}>
              <Input {...fieldAria("role-name", errors.name?.message)} {...form.register("name")} />
            </FormField>
            <FormField id="role-key" label="Key" error={errors.key?.message} hint={role ? "Cannot be changed." : "Used in integrations and logs."}>
              <Input className="font-mono" disabled={!!role} {...fieldAria("role-key", errors.key?.message, "hint")} {...form.register("key")} />
            </FormField>
          </div>
          <FormField id="role-desc" label="Description" error={errors.description?.message}>
            <Textarea rows={2} {...fieldAria("role-desc", errors.description?.message)} {...form.register("description")} />
          </FormField>
          {types.isPending ? (
            <Skeleton className="h-48 w-full" />
          ) : types.error ? (
            <QueryError error={types.error} onRetry={() => types.refetch()} title="Could not load permissions" />
          ) : (
            <PermissionMatrix types={types.data} value={perms} onChange={setPerms} />
          )}
          {formError && <Alert variant="destructive"><AlertDescription>{formError}</AlertDescription></Alert>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={isSubmitting}>{isSubmitting ? "Saving…" : role ? "Save role" : "Create role"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
