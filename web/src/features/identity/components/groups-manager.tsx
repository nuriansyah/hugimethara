"use client";

import { Pencil, Plus, Trash2, Users, X } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@hugimethara/ui/components/dialog";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { useCan, useRoleBindings } from "@/features/access/api";
import { useOrgUsers } from "@/features/workspaces/api";
import { ApiError } from "@hugimethara/api-client";
import { errorMessage, toastError } from "@/lib/errors";
import { keepOpen } from "@/lib/forms";
import { useGroupMembers, useGroupMutations, useGroups, type Group } from "../api";

export function GroupsManager() {
  const can = useCan();
  const q = useGroups();
  const [editing, setEditing] = useState<Group | "new">();
  const [members, setMembers] = useState<Group>();
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Groups</CardTitle>
          <CardDescription>
            Give a role to a group once and everyone in it holds it. Assign roles to groups in{" "}
            <Link className="underline underline-offset-2" href="/governance/access">Access</Link>.
          </CardDescription>
        </div>
        {can("group:create") && <Button onClick={() => setEditing("new")}><Plus aria-hidden /> New group</Button>}
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load groups" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No groups yet.</p>
        ) : (
          <ul className="divide-y">
            {q.data.map((g) => (
              <li key={g.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium">
                    {g.name} {g.source === "idp" && <Badge variant="outline" className="ml-1">Identity provider</Badge>}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {g.member_count} member{g.member_count === 1 ? "" : "s"}
                    {g.description && ` · ${g.description}`}
                  </p>
                </div>
                <div className="flex gap-1">
                  <Button variant="outline" size="sm" onClick={() => setMembers(g)}><Users aria-hidden /> Members</Button>
                  {can("group:update") && (
                    <Button variant="ghost" size="icon" aria-label={`Edit ${g.name}`} onClick={() => setEditing(g)}><Pencil /></Button>
                  )}
                  {can("group:delete") && <DeleteGroup g={g} />}
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
      {editing && <GroupDialog group={editing === "new" ? undefined : editing} onClose={() => setEditing(undefined)} />}
      {members && <MembersDialog group={members} canManage={can("group:manage_members") && members.source === "internal"} onClose={() => setMembers(undefined)} />}
    </Card>
  );
}

function DeleteGroup({ g }: { g: Group }) {
  const { remove } = useGroupMutations();
  const bindings = useRoleBindings({ group_id: g.id });
  const n = bindings.data?.length;
  return (
    <ConfirmDialog
      trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${g.name}`}><Trash2 /></Button>}
      title={`Delete “${g.name}”?`}
      description={`Its ${g.member_count} member(s) lose every role the group gives them${n ? ` (${n} assignment${n === 1 ? "" : "s"})` : ""}. Their own roles are not affected.`}
      confirmLabel="Delete group"
      onConfirm={async () => {
        try {
          await remove.mutateAsync(g.id);
          toast.success("Group deleted.");
        } catch (err) {
          toastError(err);
          throw err;
        }
      }}
    />
  );
}

function GroupDialog({ group, onClose }: { group?: Group; onClose: () => void }) {
  const { create, update } = useGroupMutations();
  const [name, setName] = useState(group?.name ?? "");
  const [description, setDescription] = useState(group?.description ?? "");
  const [error, setError] = useState<string>();
  const [nameError, setNameError] = useState<string>();
  const pending = create.isPending || update.isPending;

  async function save() {
    setError(undefined);
    setNameError(undefined);
    try {
      if (group) await update.mutateAsync({ id: group.id, body: { name, description } });
      else await create.mutateAsync({ name, description });
      toast.success(group ? "Group saved." : `Group “${name.trim()}” created.`);
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
          <DialogHeader><DialogTitle>{group ? "Edit group" : "New group"}</DialogTitle></DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="g-name">Name</Label>
            <Input id="g-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)}
              aria-invalid={nameError ? true : undefined} aria-describedby={nameError ? "g-name-error" : undefined} />
            {nameError && <p id="g-name-error" role="alert" className="text-sm text-destructive">{nameError}</p>}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="g-desc">Description</Label>
            <Textarea id="g-desc" rows={2} maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={!name.trim() || pending}>{pending ? "Saving…" : group ? "Save" : "Create group"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function MembersDialog({ group, canManage, onClose }: { group: Group; canManage: boolean; onClose: () => void }) {
  const q = useGroupMembers(group.id);
  const users = useOrgUsers(canManage);
  const { addMember, removeMember } = useGroupMutations();
  const [userId, setUserId] = useState("");
  const [error, setError] = useState<string>();
  const inGroup = new Set((q.data ?? []).map((m) => m.id));
  const candidates = (users.data ?? []).filter((u) => !inGroup.has(u.id));

  async function add() {
    setError(undefined);
    try {
      await addMember.mutateAsync({ id: group.id, userId });
      setUserId("");
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{group.name}</DialogTitle>
          <DialogDescription>
            {group.source === "idp"
              ? "Members come from your identity provider."
              : "Adding someone gives them every role this group holds; you can only do that if you could grant those roles yourself."}
          </DialogDescription>
        </DialogHeader>
        {canManage && (
          <div className="flex gap-2">
            <Select value={userId} onValueChange={setUserId} disabled={users.isPending}>
              <SelectTrigger className="flex-1" aria-label="Member to add">
                <SelectValue placeholder={users.isPending ? "Loading…" : "Choose a member"} />
              </SelectTrigger>
              <SelectContent>
                {candidates.map((u) => <SelectItem key={u.id} value={u.id}>{u.display_name} · {u.email}</SelectItem>)}
              </SelectContent>
            </Select>
            <Button disabled={!userId || addMember.isPending} onClick={add}>{addMember.isPending ? "Adding…" : "Add"}</Button>
          </div>
        )}
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        {q.isPending ? (
          <Skeleton className="h-20 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load members" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nobody is in this group yet.</p>
        ) : (
          <ul className="max-h-72 divide-y overflow-y-auto">
            {q.data.map((m) => (
              <li key={m.id} className="flex items-center justify-between gap-2 py-2">
                <span className="min-w-0">
                  <span className="block truncate text-sm">{m.display_name}</span>
                  <span className="block truncate text-xs text-muted-foreground">{m.email}</span>
                </span>
                {canManage && (
                  <Button variant="ghost" size="icon" aria-label={`Remove ${m.display_name} from ${group.name}`}
                    disabled={removeMember.isPending}
                    onClick={async () => {
                      try {
                        await removeMember.mutateAsync({ id: group.id, userId: m.id });
                      } catch (err) {
                        toastError(err);
                      }
                    }}><X /></Button>
                )}
              </li>
            ))}
          </ul>
        )}
        <DialogFooter><Button variant="ghost" onClick={onClose}>Close</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
