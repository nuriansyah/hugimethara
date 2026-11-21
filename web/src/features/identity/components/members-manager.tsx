"use client";

import { MoreHorizontal, Plus, Trash2, UserPlus, X } from "lucide-react";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@hugimethara/ui/components/dropdown-menu";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { useCan, useRoles } from "@/features/access/api";
import { useCurrentUser } from "@/features/auth/components/auth-gate";
import { useWorkspaces } from "@/features/workspaces/api";
import { ApiError } from "@hugimethara/api-client";
import { errorMessage, toastError } from "@/lib/errors";
import { keepOpen } from "@/lib/forms";
import {
  useInvitationMutations,
  useInvitations,
  useMemberMutations,
  useMembers,
  type InvitationCreated,
  type Member,
} from "../api";

export function MembersManager() {
  const can = useCan();
  const canInvite = can("user:invite");
  const [inviting, setInviting] = useState(false);
  const [created, setCreated] = useState<InvitationCreated>();
  return (
    <div className="flex flex-col gap-6">
      <MembersCard onInvite={canInvite ? () => setInviting(true) : undefined} />
      {canInvite && <InvitationsCard />}
      {inviting && (
        <InviteDialog
          onClose={() => setInviting(false)}
          onCreated={(c) => {
            setInviting(false);
            setCreated(c);
          }}
        />
      )}
      {created && <InviteLinkDialog created={created} onClose={() => setCreated(undefined)} />}
    </div>
  );
}

function MembersCard({ onInvite }: { onInvite?: () => void }) {
  const q = useMembers();
  const can = useCan();
  const me = useCurrentUser();
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Members</CardTitle>
          <CardDescription>People in this organization and their organization-wide roles.</CardDescription>
        </div>
        {onInvite && <Button onClick={onInvite}><UserPlus aria-hidden /> Invite</Button>}
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-40 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load members" />
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Member</TableHead>
                  <TableHead>Roles</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Joined</TableHead>
                  <TableHead className="w-12"><span className="sr-only">Actions</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data.map((m) => (
                  <MemberRow key={m.id} m={m} self={m.id === me.user.id}
                    canUpdate={can("user:update")} canRemove={can("user:remove")} />
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function MemberRow({ m, self, canUpdate, canRemove }: { m: Member; self: boolean; canUpdate: boolean; canRemove: boolean }) {
  const { setStatus, remove } = useMemberMutations();
  const [removing, setRemoving] = useState(false);
  const suspended = m.membership_status === "suspended";

  async function toggle() {
    try {
      await setStatus.mutateAsync({ id: m.id, status: suspended ? "active" : "suspended" });
      toast.success(suspended ? `${m.display_name} can sign in again.` : `${m.display_name} is suspended and signed out.`);
    } catch (err) {
      toastError(err);
    }
  }

  return (
    <TableRow>
      <TableCell>
        <span className="block text-sm font-medium">{m.display_name}{self && <span className="text-muted-foreground"> (you)</span>}</span>
        <span className="block text-xs text-muted-foreground">{m.email}</span>
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap gap-1">
          {(m.roles ?? []).map((r) => (
            <Badge key={r} variant={r === "org_owner" ? "default" : "outline"}>{r}</Badge>
          ))}
        </div>
      </TableCell>
      <TableCell>
        {suspended ? <Badge variant="destructive">Suspended</Badge> : <span className="text-sm text-muted-foreground">Active</span>}
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">{m.joined_at && new Date(m.joined_at).toLocaleDateString()}</TableCell>
      <TableCell>
        {!self && (canUpdate || canRemove) && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={`Actions for ${m.display_name}`}><MoreHorizontal /></Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {canUpdate && (
                <DropdownMenuItem onSelect={toggle} disabled={setStatus.isPending}>
                  {suspended ? "Reactivate" : "Suspend"}
                </DropdownMenuItem>
              )}
              {canRemove && (
                <DropdownMenuItem variant="destructive" onSelect={() => setRemoving(true)}>
                  <Trash2 aria-hidden /> Remove from organization
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        <Dialog open={removing} onOpenChange={setRemoving}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Remove {m.display_name}?</DialogTitle>
              <DialogDescription>
                They lose every role, workspace and group here and are signed out. Their account stays, so you can invite them again later.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setRemoving(false)}>Cancel</Button>
              <Button variant="destructive" disabled={remove.isPending} onClick={async () => {
                try {
                  await remove.mutateAsync(m.id);
                  toast.success(`${m.display_name} was removed.`);
                  setRemoving(false);
                } catch (err) {
                  toastError(err);
                }
              }}>{remove.isPending ? "Removing…" : "Remove"}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </TableCell>
    </TableRow>
  );
}

function InvitationsCard() {
  const q = useInvitations(true);
  const { revoke } = useInvitationMutations();
  return (
    <Card>
      <CardHeader>
        <CardTitle>Pending invitations</CardTitle>
        <CardDescription>Links that have not been used yet. Re-inviting an address replaces its old link.</CardDescription>
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-20 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load invitations" />
        ) : q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No pending invitations.</p>
        ) : (
          <ul className="divide-y">
            {q.data.map((inv) => (
              <li key={inv.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium">{inv.email}</p>
                  <p className="text-xs text-muted-foreground">
                    {[inv.org_role?.name, ...inv.workspaces.map((w) => `${w.workspace_name} (${w.role_name})`)].filter(Boolean).join(" · ") || "Member"}
                    {" · "}expires {new Date(inv.expires_at).toLocaleDateString()}
                    {inv.invited_by_name && ` · by ${inv.invited_by_name}`}
                  </p>
                </div>
                <ConfirmDialog
                  trigger={<Button variant="ghost" size="icon" aria-label={`Revoke invitation for ${inv.email}`}><X /></Button>}
                  title="Revoke this invitation?"
                  description={`The link sent to ${inv.email} stops working.`}
                  confirmLabel="Revoke"
                  onConfirm={async () => {
                    try {
                      await revoke.mutateAsync(inv.id);
                      toast.success("Invitation revoked.");
                    } catch (err) {
                      toastError(err);
                      throw err;
                    }
                  }}
                />
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

type WsGrant = { workspaceId: string; roleId: string };
const MEMBER_ONLY = "member-only";

function InviteDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (c: InvitationCreated) => void }) {
  const can = useCan();
  const roles = useRoles();
  const workspaces = useWorkspaces(false);
  const { create } = useInvitationMutations();
  const [email, setEmail] = useState("");
  const [orgRole, setOrgRole] = useState(MEMBER_ONLY);
  const [days, setDays] = useState("7");
  const [grants, setGrants] = useState<WsGrant[]>([]);
  const [error, setError] = useState<string>();
  const [emailError, setEmailError] = useState<string>();

  const editorId = roles.data?.find((r) => r.key === "editor")?.id ?? "";
  const orgRoles = (roles.data ?? []).filter((r) => r.key !== "member" && !["workspace_admin", "editor", "viewer"].includes(r.key));
  const wsRoles = (roles.data ?? []).filter((r) => ["workspace_admin", "editor", "viewer"].includes(r.key) || !r.is_system);
  const usedWs = new Set(grants.map((g) => g.workspaceId));
  const freeWs = (workspaces.data ?? []).filter((w) => !usedWs.has(w.id));

  async function submit() {
    setError(undefined);
    setEmailError(undefined);
    try {
      const created = await create.mutateAsync({
        email: email.trim(),
        org_role_id: orgRole === MEMBER_ONLY ? undefined : orgRole,
        expires_in_days: Number(days),
        workspaces: grants.filter((g) => g.workspaceId && g.roleId).map((g) => ({ workspace_id: g.workspaceId, role_id: g.roleId })),
      });
      onCreated(created);
    } catch (err) {
      const fe = err instanceof ApiError ? err.fieldErrors?.find((f) => f.field === "email") : undefined;
      if (fe) setEmailError(fe.message);
      else setError(errorMessage(err));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen} className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Invite someone</DialogTitle>
          <DialogDescription>They get the Member role, plus anything you add here. You can only hand out access you hold yourself.</DialogDescription>
        </DialogHeader>
        {roles.error || workspaces.error ? (
          <QueryError error={(roles.error ?? workspaces.error)!} onRetry={() => { roles.refetch(); workspaces.refetch(); }} title="Could not load roles or workspaces" />
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="inv-email">Email</Label>
              <Input id="inv-email" type="email" autoComplete="off" value={email} onChange={(e) => setEmail(e.target.value)}
                aria-invalid={emailError ? true : undefined} aria-describedby={emailError ? "inv-email-error" : undefined} />
              {emailError && <p id="inv-email-error" role="alert" className="text-sm text-destructive">{emailError}</p>}
            </div>
            {can("role:bind") && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="inv-role">Organization role</Label>
                <Select value={orgRole} onValueChange={setOrgRole} disabled={roles.isPending}>
                  <SelectTrigger id="inv-role" className="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value={MEMBER_ONLY}>Member only</SelectItem>
                    {orgRoles.map((r) => <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
            )}
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-1.5 text-sm font-medium">Workspaces</legend>
              {grants.length === 0 && <p className="text-xs text-muted-foreground">No workspace access. They can be added to workspaces later.</p>}
              {grants.map((g, i) => (
                <div key={i} className="flex gap-2">
                  <Select value={g.workspaceId} onValueChange={(v) => setGrants(grants.map((x, j) => (j === i ? { ...x, workspaceId: v } : x)))}>
                    <SelectTrigger className="flex-1" aria-label={`Workspace ${i + 1}`}><SelectValue placeholder="Workspace" /></SelectTrigger>
                    <SelectContent>
                      {(workspaces.data ?? []).filter((w) => w.id === g.workspaceId || !usedWs.has(w.id)).map((w) => (
                        <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Select value={g.roleId} onValueChange={(v) => setGrants(grants.map((x, j) => (j === i ? { ...x, roleId: v } : x)))}>
                    <SelectTrigger className="w-40" aria-label={`Role in workspace ${i + 1}`}><SelectValue placeholder="Role" /></SelectTrigger>
                    <SelectContent>
                      {wsRoles.map((r) => <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>)}
                    </SelectContent>
                  </Select>
                  <Button type="button" variant="ghost" size="icon" aria-label={`Remove workspace ${i + 1}`}
                    onClick={() => setGrants(grants.filter((_, j) => j !== i))}><X /></Button>
                </div>
              ))}
              {freeWs.length > 0 && (
                <Button type="button" variant="outline" size="sm" className="self-start"
                  onClick={() => setGrants([...grants, { workspaceId: "", roleId: editorId }])}>
                  <Plus aria-hidden /> Add workspace
                </Button>
              )}
            </fieldset>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="inv-days">Link expires after</Label>
              <Select value={days} onValueChange={setDays}>
                <SelectTrigger id="inv-days" className="w-40"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {["1", "7", "14", "30"].map((d) => <SelectItem key={d} value={d}>{d} day{d === "1" ? "" : "s"}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          </div>
        )}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button disabled={!email.trim() || create.isPending || grants.some((g) => !g.workspaceId || !g.roleId)} onClick={submit}>
            {create.isPending ? "Creating…" : "Create invitation"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function InviteLinkDialog({ created, onClose }: { created: InvitationCreated; onClose: () => void }) {
  const link = typeof window === "undefined" ? created.accept_path : `${window.location.origin}${created.accept_path}`;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen}>
        <DialogHeader>
          <DialogTitle>Invitation ready</DialogTitle>
          <DialogDescription>
            Send this link to {created.invitation.email}. It is shown only once and works until{" "}
            {new Date(created.invitation.expires_at).toLocaleDateString()}. Anyone with the link can join as that address.
          </DialogDescription>
        </DialogHeader>
        <CopyField id="invite-link" label="Invitation link" value={link} />
        <DialogFooter>
          <Button onClick={onClose}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
