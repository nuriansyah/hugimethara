"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Plus, Users } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { toast } from "sonner";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@hugimethara/ui/components/dialog";
import { Label } from "@hugimethara/ui/components/label";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Switch } from "@hugimethara/ui/components/switch";
import { useCan } from "@/features/access/api";
import { planError, useEntitlements } from "@/features/billing/api";
import { PlanNotice } from "@/features/billing/components/plan-notice";
import { applyApiError, keepOpen } from "@/lib/forms";
import { slugify } from "@/lib/slug";
import { useCreateWorkspace, useWorkspaces } from "../api";
import {
  departmentPayload,
  NO_DEPARTMENT,
  WORKSPACE_FIELDS,
  WorkspaceFields,
  workspaceSchema,
  type WorkspaceValues,
} from "./workspace-form-fields";

export function WorkspaceList() {
  const can = useCan();
  const canCreate = can("workspace:create");
  const [archived, setArchived] = useState(false);
  const q = useWorkspaces(archived);
  const ent = useEntitlements();
  const quota = ent.data?.features.find((f) => f.key === "workspaces.max");

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Switch id="show-archived" checked={archived} onCheckedChange={setArchived} />
          <Label htmlFor="show-archived">Show archived</Label>
        </div>
        <div className="flex items-center gap-3">
          {quota?.limit !== undefined && quota.used !== undefined && (
            <span className="text-sm text-muted-foreground">
              {quota.used} of {quota.limit} workspaces on your plan
            </span>
          )}
          {canCreate && <CreateWorkspaceDialog />}
        </div>
      </div>

      {q.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }, (_, i) => <Skeleton key={i} className="h-28" />)}
        </div>
      ) : q.error ? (
        <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load workspaces" />
      ) : q.data.length === 0 ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          {canCreate
            ? "No workspaces yet. Create one for each team, such as Marketing or Finance."
            : "You are not a member of any workspace yet. Ask an administrator to add you."}
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {q.data.map((w) => (
            <Link key={w.id} href={`/workspaces/${w.slug}`} className="rounded-xl focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none">
              <Card className="h-full transition-colors hover:bg-muted/50">
                <CardHeader>
                  <CardTitle className="flex items-center justify-between gap-2">
                    <span className="truncate">{w.name}</span>
                    {w.archived_at && <Badge variant="outline">Archived</Badge>}
                  </CardTitle>
                  <CardDescription className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <span className="font-mono">{w.slug}</span>
                    {w.department_name && <span>{w.department_name}</span>}
                    <span className="flex items-center gap-1">
                      <Users className="size-3" aria-hidden /> {w.member_count}
                    </span>
                  </CardDescription>
                  {w.description && <p className="line-clamp-2 text-sm text-muted-foreground">{w.description}</p>}
                </CardHeader>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function CreateWorkspaceDialog() {
  const [open, setOpen] = useState(false);
  const [formError, setFormError] = useState<string>();
  const [plan, setPlan] = useState<string>();
  const [slugTouched, setSlugTouched] = useState(false);
  const create = useCreateWorkspace();
  const form = useForm<WorkspaceValues>({
    resolver: zodResolver(workspaceSchema),
    defaultValues: { slug: "", name: "", description: "", department_id: NO_DEPARTMENT },
  });

  const name = useWatch({ control: form.control, name: "name" });
  useEffect(() => {
    if (!slugTouched) form.setValue("slug", slugify(name));
  }, [name, slugTouched, form]);

  function reset() {
    form.reset();
    setSlugTouched(false);
    setFormError(undefined);
    setPlan(undefined);
  }

  async function submit(v: WorkspaceValues) {
    setFormError(undefined);
    setPlan(undefined);
    try {
      const w = await create.mutateAsync({ ...v, department_id: departmentPayload(v.department_id) });
      toast.success(`Workspace “${w.name}” created.`);
      setOpen(false);
      reset();
    } catch (err) {
      const pe = planError(err);
      if (pe) setPlan(pe.message);
      else setFormError(applyApiError(err, form.setError, WORKSPACE_FIELDS));
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (!o) reset(); }}>
      <DialogTrigger asChild>
        <Button>
          <Plus aria-hidden /> New workspace
        </Button>
      </DialogTrigger>
      <DialogContent onInteractOutside={keepOpen}>
        <form noValidate onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4"
          onChange={(e) => { if ((e.target as HTMLElement).id === "ws-slug") setSlugTouched(true); }}>
          <DialogHeader>
            <DialogTitle>New workspace</DialogTitle>
            <DialogDescription>A space for one team. You will be its first member.</DialogDescription>
          </DialogHeader>
          <WorkspaceFields form={form} showSlug />
          {plan && <PlanNotice message={plan} />}
          {formError && (
            <Alert variant="destructive">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting ? "Creating…" : "Create workspace"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
