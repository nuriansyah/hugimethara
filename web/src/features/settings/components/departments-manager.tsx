"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { fieldAria, FormField } from "@/components/form-field";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useCan } from "@/features/access/api";
import { planError, useFeature } from "@/features/billing/api";
import { PlanNotice } from "@/features/billing/components/plan-notice";
import { useDepartmentMutations, useDepartments, type Department } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import { applyApiError, keepOpen } from "@/lib/forms";
import { SHORT_SLUG, SHORT_SLUG_MESSAGE, slugify } from "@/lib/slug";

const schema = z.object({
  name: z.string().trim().min(1, "Enter a name.").max(100),
  slug: z.string().regex(SHORT_SLUG, SHORT_SLUG_MESSAGE),
  description: z.string().max(500),
});
type Values = z.infer<typeof schema>;

export function DepartmentsManager() {
  const can = useCan();
  const enabled = useFeature("departments");
  const q = useDepartments();
  const canEdit = can("department:update") || can("department:create");

  return (
    <div className="flex flex-col gap-4">
      {enabled === false && (
        <PlanNotice message="Departments are part of the Pro and Enterprise plans. Existing departments stay visible, but new ones cannot be created on your plan." />
      )}
      {can("department:create") && enabled && <CreateDepartmentCard />}
      <Card>
        <CardHeader>
          <CardTitle>Departments</CardTitle>
          <CardDescription>Group workspaces by the part of the business they serve.</CardDescription>
        </CardHeader>
        <CardContent>
          {q.isPending ? (
            <Skeleton className="h-24 w-full" />
          ) : q.error ? (
            <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load departments" />
          ) : q.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">No departments yet.</p>
          ) : (
            <ul className="divide-y">
              {q.data.map((d) => (
                <DepartmentRow key={d.id} d={d} canEdit={canEdit} />
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function CreateDepartmentCard() {
  const { create } = useDepartmentMutations();
  const [formError, setFormError] = useState<string>();
  const [plan, setPlan] = useState<string>();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { name: "", slug: "", description: "" } });
  const { errors, isSubmitting } = form.formState;
  const name = form.register("name", {
    onChange: (e) => {
      if (!form.getFieldState("slug").isDirty) form.setValue("slug", slugify(e.target.value));
    },
  });

  async function submit(v: Values) {
    setFormError(undefined);
    setPlan(undefined);
    try {
      await create.mutateAsync(v);
      toast.success(`Department “${v.name}” created.`);
      form.reset();
    } catch (err) {
      const pe = planError(err);
      if (pe) setPlan(pe.message);
      else setFormError(applyApiError(err, form.setError, ["name", "slug", "description"]));
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>New department</CardTitle>
      </CardHeader>
      <CardContent>
        <form noValidate onSubmit={form.handleSubmit(submit)} className="grid gap-4 sm:grid-cols-2">
          <FormField id="d-name" label="Name" error={errors.name?.message}>
            <Input {...fieldAria("d-name", errors.name?.message)} {...name} />
          </FormField>
          <FormField id="d-slug" label="Address" error={errors.slug?.message}>
            <Input className="font-mono" {...fieldAria("d-slug", errors.slug?.message)} {...form.register("slug")} />
          </FormField>
          <div className="sm:col-span-2">
            <FormField id="d-desc" label="Description" error={errors.description?.message}>
              <Textarea rows={2} {...fieldAria("d-desc", errors.description?.message)} {...form.register("description")} />
            </FormField>
          </div>
          {plan && <div className="sm:col-span-2"><PlanNotice message={plan} /></div>}
          {formError && (
            <Alert variant="destructive" className="sm:col-span-2">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
          <div className="sm:col-span-2 flex justify-end">
            <Button type="submit" disabled={isSubmitting}>{isSubmitting ? "Creating…" : "Create department"}</Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function DepartmentRow({ d, canEdit }: { d: Department; canEdit: boolean }) {
  const { update, remove } = useDepartmentMutations();
  const [editing, setEditing] = useState(false);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { name: d.name, slug: d.slug, description: d.description } });
  const { errors, isSubmitting } = form.formState;
  const [formError, setFormError] = useState<string>();

  async function save(v: Values) {
    setFormError(undefined);
    try {
      await update.mutateAsync({ id: d.id, body: { name: v.name, description: v.description } });
      toast.success("Department saved.");
      setEditing(false);
    } catch (err) {
      setFormError(applyApiError(err, form.setError, ["name", "description"]));
    }
  }

  return (
    <li className="flex flex-wrap items-center justify-between gap-2 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium">{d.name} <span className="font-mono text-xs text-muted-foreground">{d.slug}</span></p>
        <p className="text-xs text-muted-foreground">
          {d.workspace_count} active workspace{d.workspace_count === 1 ? "" : "s"}
          {d.description && ` · ${d.description}`}
        </p>
      </div>
      {canEdit && (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" aria-label={`Edit ${d.name}`} onClick={() => { form.reset({ name: d.name, slug: d.slug, description: d.description }); setEditing(true); }}>
            <Pencil />
          </Button>
          <ConfirmDialog
            trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${d.name}`}><Trash2 /></Button>}
            title={`Delete “${d.name}”?`}
            description={`Its ${d.workspace_count} workspace(s) stay but will no longer belong to a department.`}
            confirmLabel="Delete department"
            onConfirm={async () => {
              try {
                await remove.mutateAsync(d.id);
                toast.success("Department deleted.");
              } catch (err) {
                toastError(err);
                throw err;
              }
            }}
          />
        </div>
      )}
      <Dialog open={editing} onOpenChange={setEditing}>
        <DialogContent onInteractOutside={keepOpen}>
          <form noValidate onSubmit={form.handleSubmit(save)} className="flex flex-col gap-4">
            <DialogHeader><DialogTitle>Edit department</DialogTitle></DialogHeader>
            <FormField id={`e-${d.id}-name`} label="Name" error={errors.name?.message}>
              <Input {...fieldAria(`e-${d.id}-name`, errors.name?.message)} {...form.register("name")} />
            </FormField>
            <FormField id={`e-${d.id}-desc`} label="Description" error={errors.description?.message}>
              <Textarea rows={2} {...fieldAria(`e-${d.id}-desc`, errors.description?.message)} {...form.register("description")} />
            </FormField>
            {formError && <Alert variant="destructive"><AlertDescription>{formError}</AlertDescription></Alert>}
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setEditing(false)}>Cancel</Button>
              <Button type="submit" disabled={isSubmitting}>{isSubmitting ? "Saving…" : "Save"}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </li>
  );
}
