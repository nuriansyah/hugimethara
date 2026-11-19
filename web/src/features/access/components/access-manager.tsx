"use client";

import { CheckCircle2, Plus, Trash2, XCircle } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useOrgUsers, useWorkspaces } from "@/features/workspaces/api";
import { errorMessage, toastError } from "@/lib/errors";
import { keepOpen } from "@/lib/forms";
import {
  useBindingMutations,
  useCan,
  useDecisions,
  useEvaluate,
  useResourceTypes,
  useRoleBindings,
  useRoles,
  type RoleBinding,
} from "../api";

export function AccessManager() {
  const can = useCan();
  return (
    <div className="flex flex-col gap-6">
      <Assignments canBind={can("role:bind")} />
      {can("authorization:simulate") && <CheckAccess />}
      {can("authorization:read_decisions") && <RecentDenials />}
    </div>
  );
}

function scopeLabel(b: RoleBinding) {
  if (b.scope.type === "org") return "Whole organization";
  if (b.scope.type === "workspace") return `Workspace · ${b.workspace_name ?? b.workspace_slug ?? "?"}`;
  return `${b.scope.resource_type} · ${b.scope.resource_id}`;
}

function Assignments({ canBind }: { canBind: boolean }) {
  const q = useRoleBindings();
  const { remove } = useBindingMutations();
  const [assigning, setAssigning] = useState(false);
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Role assignments</CardTitle>
          <CardDescription>Who holds which role, and where. Workspace roles can also be managed from each workspace.</CardDescription>
        </div>
        {canBind && <Button onClick={() => setAssigning(true)}><Plus aria-hidden /> Assign role</Button>}
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-40 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load assignments" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No assignments.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Member</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Scope</TableHead>
                  <TableHead>Since</TableHead>
                  <TableHead className="w-12"><span className="sr-only">Actions</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data.map((b) => (
                  <TableRow key={b.id}>
                    <TableCell>
                      <span className="block text-sm font-medium">{b.user.display_name}</span>
                      <span className="block text-xs text-muted-foreground">{b.user.email}</span>
                    </TableCell>
                    <TableCell><Badge variant={b.role.key === "org_owner" ? "default" : "outline"}>{b.role.name}</Badge></TableCell>
                    <TableCell className="text-sm">{scopeLabel(b)}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(b.created_at).toLocaleDateString()}
                      {b.expires_at && ` · until ${new Date(b.expires_at).toLocaleDateString()}`}
                    </TableCell>
                    <TableCell>
                      {canBind && (
                        <ConfirmDialog
                          trigger={<Button variant="ghost" size="icon" aria-label={`Remove ${b.role.name} from ${b.user.display_name}`}><Trash2 /></Button>}
                          title="Remove this assignment?"
                          description={`${b.user.display_name} loses the ${b.role.name} role (${scopeLabel(b)}) immediately.`}
                          confirmLabel="Remove"
                          onConfirm={async () => {
                            try {
                              await remove.mutateAsync(b.id);
                              toast.success("Assignment removed.");
                            } catch (err) {
                              toastError(err);
                              throw err;
                            }
                          }}
                        />
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
      {assigning && <AssignDialog onClose={() => setAssigning(false)} />}
    </Card>
  );
}

function AssignDialog({ onClose }: { onClose: () => void }) {
  const users = useOrgUsers(true);
  const roles = useRoles();
  const workspaces = useWorkspaces(false);
  const { assign } = useBindingMutations();
  const [userId, setUserId] = useState("");
  const [roleId, setRoleId] = useState("");
  const [scope, setScope] = useState<"org" | "workspace">("org");
  const [workspaceId, setWorkspaceId] = useState("");
  const [error, setError] = useState<string>();
  const ready = userId && roleId && (scope === "org" || workspaceId);

  async function submit() {
    setError(undefined);
    try {
      await assign.mutateAsync({
        user_id: userId,
        role_id: roleId,
        scope: scope === "org" ? { type: "org" } : { type: "workspace", workspace_id: workspaceId },
      });
      toast.success("Role assigned.");
      onClose();
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen}>
        <DialogHeader>
          <DialogTitle>Assign a role</DialogTitle>
          <DialogDescription>You can only assign roles whose permissions you hold yourself at that scope.</DialogDescription>
        </DialogHeader>
        {users.error || roles.error ? (
          <QueryError error={(users.error ?? roles.error)!} onRetry={() => { users.refetch(); roles.refetch(); }} title="Could not load members or roles" />
        ) : (
          <div className="flex flex-col gap-4">
            <Picker id="as-user" label="Member" value={userId} onChange={setUserId} loading={users.isPending}
              options={(users.data ?? []).map((u) => ({ value: u.id, label: `${u.display_name} · ${u.email}` }))} />
            <Picker id="as-role" label="Role" value={roleId} onChange={setRoleId} loading={roles.isPending}
              options={(roles.data ?? []).map((r) => ({ value: r.id, label: r.name }))} />
            <Picker id="as-scope" label="Where" value={scope} onChange={(v) => setScope(v as "org" | "workspace")}
              options={[{ value: "org", label: "Whole organization" }, { value: "workspace", label: "One workspace" }]} />
            {scope === "workspace" && (
              <Picker id="as-ws" label="Workspace" value={workspaceId} onChange={setWorkspaceId} loading={workspaces.isPending}
                options={(workspaces.data ?? []).map((w) => ({ value: w.id, label: w.name }))} />
            )}
            {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          </div>
        )}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button disabled={!ready || assign.isPending} onClick={submit}>{assign.isPending ? "Assigning…" : "Assign"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Picker({ id, label, value, onChange, options, loading }: {
  id: string; label: string; value: string; onChange: (v: string) => void;
  options: { value: string; label: string }[]; loading?: boolean;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange} disabled={loading}>
        <SelectTrigger id={id} className="w-full"><SelectValue placeholder={loading ? "Loading…" : "Choose"} /></SelectTrigger>
        <SelectContent>
          {options.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  );
}

function CheckAccess() {
  const users = useOrgUsers(true);
  const types = useResourceTypes();
  const workspaces = useWorkspaces(false);
  const evaluate = useEvaluate();
  const [userId, setUserId] = useState("");
  const [type, setType] = useState("");
  const [action, setAction] = useState("");
  const [workspaceId, setWorkspaceId] = useState("");
  const actions = types.data?.find((t) => t.type === type)?.actions ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Check access</CardTitle>
        <CardDescription>Ask whether a member could do something, and which assignment would allow it. Checks are not logged as real decisions.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Picker id="ck-user" label="Member" value={userId} onChange={setUserId} loading={users.isPending}
            options={(users.data ?? []).map((u) => ({ value: u.id, label: u.display_name }))} />
          <Picker id="ck-type" label="Resource" value={type} onChange={(v) => { setType(v); setAction(""); }} loading={types.isPending}
            options={(types.data ?? []).map((t) => ({ value: t.type, label: t.type }))} />
          <Picker id="ck-action" label="Action" value={action} onChange={setAction}
            options={actions.map((a) => ({ value: a, label: a }))} />
          <Picker id="ck-ws" label="In workspace (optional)" value={workspaceId} onChange={setWorkspaceId} loading={workspaces.isPending}
            options={[{ value: "none", label: "— none —" }, ...(workspaces.data ?? []).map((w) => ({ value: w.id, label: w.name }))]} />
        </div>
        <div>
          <Button disabled={!userId || !type || !action || evaluate.isPending}
            onClick={() => evaluate.mutate({ user_id: userId, resource_type: type, action,
              workspace_id: workspaceId && workspaceId !== "none" ? workspaceId : undefined })}>
            {evaluate.isPending ? "Checking…" : "Check"}
          </Button>
        </div>
        {evaluate.error && <Alert variant="destructive"><AlertDescription>{errorMessage(evaluate.error)}</AlertDescription></Alert>}
        {evaluate.data && (
          <div className="flex items-start gap-2 rounded-md border p-3 text-sm" role="status">
            {evaluate.data.allowed
              ? <CheckCircle2 className="mt-0.5 size-4 text-emerald-600" aria-hidden />
              : <XCircle className="mt-0.5 size-4 text-destructive" aria-hidden />}
            <div>
              <p className="font-medium">{evaluate.data.allowed ? "Allowed" : "Denied"} <span className="font-normal text-muted-foreground">({evaluate.data.reason})</span></p>
              {evaluate.data.matched_rules.length > 0 && (
                <p className="font-mono text-xs text-muted-foreground">{evaluate.data.matched_rules.join(", ")}</p>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function RecentDenials() {
  const q = useDecisions(false);
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent denials</CardTitle>
        <CardDescription>Requests refused for lack of permission. Useful when someone reports “I can’t do X”.</CardDescription>
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-32 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load decisions" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No denied requests.</p>
        ) : (
          <ul className="divide-y text-sm">
            {q.data.map((d) => (
              <li key={d.id} className="flex flex-wrap items-center justify-between gap-2 py-2">
                <span>
                  <span className="font-medium">{d.principal_email ?? d.principal_id}</span>{" "}
                  <span className="font-mono text-xs">{d.resource_type}:{d.action}</span>
                </span>
                <span className="text-xs text-muted-foreground">{new Date(d.occurred_at).toLocaleString()}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
