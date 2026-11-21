"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowLeft, UserMinus } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { useCurrentUser } from "@/features/auth/components/auth-gate";
import { useCan } from "@/features/access/api";
import { planError } from "@/features/billing/api";
import { PlanNotice } from "@/features/billing/components/plan-notice";
import { ApiError } from "@hugimethara/api-client";
import { errorMessage, toastError } from "@/lib/errors";
import { applyApiError } from "@/lib/forms";
import { useChangeMember, useOrgUsers, useSetArchived, useUpdateWorkspace, useWorkspace, useWorkspaceMembers, type Workspace } from "../api";
import { departmentPayload, NO_DEPARTMENT, WORKSPACE_FIELDS, WorkspaceFields, workspaceSchema, type WorkspaceValues } from "./workspace-form-fields";

export function WorkspaceDetail({ slug }: { slug: string }) {
  const q = useWorkspace(slug);
  if (q.isPending) return <Skeleton className="h-72 w-full" />;
  if (q.error) {
    if (q.error instanceof ApiError && q.error.status === 404) {
      return (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          This workspace does not exist or you are not a member of it.{" "}
          <Link href="/workspaces" className="underline underline-offset-4">Back to workspaces</Link>
        </div>
      );
    }
    return <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load workspace" />;
  }
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <Link href="/workspaces" className="mb-1 flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="size-4" aria-hidden /> Workspaces
          </Link>
          <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight">
            {q.data.name}
            {q.data.archived_at && <Badge variant="outline">Archived</Badge>}
          </h1>
          <p className="font-mono text-sm text-muted-foreground">{q.data.slug}</p>
        </div>
        <ArchiveControl ws={q.data} />
      </div>
      <DetailsCard ws={q.data} />
      <MembersCard ws={q.data} />
    </div>
  );
}

function ArchiveControl({ ws }: { ws: Workspace }) {
  const can = useCan();
  const setArchived = useSetArchived(ws.slug);
  if (!can("workspace:archive", ws.id)) return null;
  if (ws.archived_at) {
    return (
      <Button
        variant="outline"
        disabled={setArchived.isPending}
        onClick={async () => {
          try {
            await setArchived.mutateAsync(false);
            toast.success("Workspace restored.");
          } catch (err) {
            toastError(err);
          }
        }}
      >
        Restore workspace
      </Button>
    );
  }
  return (
    <ConfirmDialog
      trigger={<Button variant="outline">Archive workspace</Button>}
      title={`Archive “${ws.name}”?`}
      description="Archived workspaces are read-only, hidden from lists and do not count toward your plan's workspace limit. You can restore it later."
      confirmLabel="Archive"
      onConfirm={async () => {
        try {
          await setArchived.mutateAsync(true);
          toast.success("Workspace archived.");
        } catch (err) {
          toastError(err);
          throw err;
        }
      }}
    />
  );
}

const WORKSPACE_ROLES = [
  { key: "viewer", label: "Viewer" },
  { key: "editor", label: "Editor" },
  { key: "workspace_admin", label: "Workspace admin" },
];

const toValues = (w: Workspace): WorkspaceValues => ({
  slug: w.slug,
  name: w.name,
  description: w.description,
  department_id: w.department_id ?? NO_DEPARTMENT,
});

function DetailsCard({ ws }: { ws: Workspace }) {
  const can = useCan();
  const canEdit = can("workspace:update", ws.id) && !ws.archived_at; // UX only; the API decides
  const update = useUpdateWorkspace(ws.slug);
  const [formError, setFormError] = useState<string>();
  const [plan, setPlan] = useState<string>();
  const form = useForm<WorkspaceValues>({ resolver: zodResolver(workspaceSchema), defaultValues: toValues(ws) });
  useEffect(() => form.reset(toValues(ws)), [ws, form]);
  const { isDirty, isSubmitting } = form.formState;

  async function submit(v: WorkspaceValues) {
    setFormError(undefined);
    setPlan(undefined);
    try {
      await update.mutateAsync({ name: v.name, description: v.description, department_id: departmentPayload(v.department_id) });
      toast.success("Workspace saved.");
    } catch (err) {
      const pe = planError(err);
      if (pe) setPlan(pe.message);
      else setFormError(applyApiError(err, form.setError, WORKSPACE_FIELDS));
    }
  }

  return (
    <form noValidate onSubmit={form.handleSubmit(submit)}>
      <Card>
        <CardHeader>
          <CardTitle>Details</CardTitle>
          <CardDescription>Created {new Date(ws.created_at).toLocaleDateString()}</CardDescription>
        </CardHeader>
        <CardContent>
          <fieldset disabled={!canEdit} className="flex flex-col gap-4">
            <WorkspaceFields form={form} showSlug={false} />
          </fieldset>
          {plan && <div className="mt-4"><PlanNotice message={plan} /></div>}
          {formError && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
        </CardContent>
        {canEdit && (
          <CardFooter className="justify-end gap-2">
            <Button type="button" variant="ghost" disabled={!isDirty || isSubmitting} onClick={() => form.reset(toValues(ws))}>
              Discard
            </Button>
            <Button type="submit" disabled={!isDirty || isSubmitting}>
              {isSubmitting ? "Saving…" : "Save changes"}
            </Button>
          </CardFooter>
        )}
      </Card>
    </form>
  );
}

function MembersCard({ ws }: { ws: Workspace }) {
  const me = useCurrentUser();
  const can = useCan();
  const canManage = can("workspace:manage_members", ws.id) && !ws.archived_at;
  const members = useWorkspaceMembers(ws.slug);
  const users = useOrgUsers(canManage && can("user:read"));
  const change = useChangeMember(ws.slug);
  const [pick, setPick] = useState<string>("");
  const [role, setRole] = useState<string>("editor");
  const memberIds = new Set((members.data ?? []).map((m) => m.id));
  const candidates = (users.data ?? []).filter((u) => !memberIds.has(u.id) && u.status === "active");

  async function add() {
    if (!pick) return;
    try {
      await change.mutateAsync({ userId: pick, add: true, role });
      setPick("");
      toast.success("Member added.");
    } catch (err) {
      toastError(err);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Members</CardTitle>
        <CardDescription>People who can work in this workspace.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {canManage && (
          <div className="flex flex-wrap items-center gap-2">
            <Select value={pick} onValueChange={setPick} disabled={users.isPending || candidates.length === 0}>
              <SelectTrigger className="w-72" aria-label="Organization member to add">
                <SelectValue placeholder={users.isPending ? "Loading people…" : candidates.length ? "Choose a person" : "Everyone is already a member"} />
              </SelectTrigger>
              <SelectContent>
                {candidates.map((u) => (
                  <SelectItem key={u.id} value={u.id}>
                    {u.display_name} · {u.email}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={role} onValueChange={setRole}>
              <SelectTrigger className="w-44" aria-label="Role in this workspace">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {WORKSPACE_ROLES.map((r) => (
                  <SelectItem key={r.key} value={r.key}>{r.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={add} disabled={!pick || change.isPending}>Add member</Button>
            {users.error && <span role="alert" className="text-sm text-destructive">{errorMessage(users.error)}</span>}
          </div>
        )}
        {members.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : members.error ? (
          <QueryError error={members.error} onRetry={() => members.refetch()} title="Could not load members" />
        ) : members.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No members.</p>
        ) : (
          <ul className="divide-y">
            {members.data.map((m) => (
              <li key={m.id} className="flex items-center justify-between gap-2 py-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">
                    {m.display_name} {m.id === me.user.id && <span className="text-muted-foreground">(you)</span>}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">{m.email}</p>
                </div>
                <div className="flex items-center gap-2">
                  {(m.roles ?? []).map((r) => (
                    <Badge key={r} variant="outline">{WORKSPACE_ROLES.find((x) => x.key === r)?.label ?? r}</Badge>
                  ))}
                  {m.is_owner && <Badge variant="secondary">Org owner</Badge>}
                  {canManage && (
                    <ConfirmDialog
                      trigger={<Button variant="ghost" size="icon" aria-label={`Remove ${m.display_name}`}><UserMinus /></Button>}
                      title={`Remove ${m.display_name}?`}
                      description="They lose access to this workspace immediately. They stay in the organization."
                      confirmLabel="Remove"
                      onConfirm={async () => {
                        try {
                          await change.mutateAsync({ userId: m.id, add: false });
                          toast.success("Member removed.");
                        } catch (err) {
                          toastError(err);
                          throw err;
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
  );
}
