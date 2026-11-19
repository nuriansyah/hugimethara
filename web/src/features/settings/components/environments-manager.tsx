"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { fieldAria, FormField } from "@/components/form-field";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useCan } from "@/features/access/api";
import { planError, useEntitlements, useFeature } from "@/features/billing/api";
import { PlanNotice } from "@/features/billing/components/plan-notice";
import { useEnvironmentMutations, useEnvironments, type Environment } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import { applyApiError, keepOpen } from "@/lib/forms";
import { SHORT_SLUG, SHORT_SLUG_MESSAGE, slugify } from "@/lib/slug";

const schema = z.object({
  name: z.string().trim().min(1, "Enter a name.").max(100),
  slug: z.string().regex(SHORT_SLUG, SHORT_SLUG_MESSAGE),
  is_production: z.boolean(),
  allowlist: z.string().max(6400),
});
type Values = z.infer<typeof schema>;
const FIELDS = ["name", "slug"] as const;

const parseAllowlist = (s: string) => s.split(/[\s,]+/).map((x) => x.trim()).filter(Boolean);

export function EnvironmentsManager() {
  const can = useCan();
  const q = useEnvironments();
  const ent = useEntitlements();
  const quota = ent.data?.features.find((f) => f.key === "environments.max");
  const [editing, setEditing] = useState<Environment | "new" | null>(null);
  const { remove } = useEnvironmentMutations();
  const canEdit = can("environment:update");

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Environments</CardTitle>
          <CardDescription>
            Stages your integrations run in. Connections and secrets are configured per environment.
            {quota?.limit !== undefined && quota.used !== undefined && ` ${quota.used} of ${quota.limit} used on your plan.`}
          </CardDescription>
        </div>
        {can("environment:create") && (
          <Button onClick={() => setEditing("new")}>
            <Plus aria-hidden /> New environment
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-32 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load environments" />
        ) : (
          <ul className="divide-y">
            {q.data.map((e) => (
              <li key={e.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                <div className="min-w-0">
                  <p className="flex items-center gap-2 text-sm font-medium">
                    {e.name} <span className="font-mono text-xs text-muted-foreground">{e.slug}</span>
                    {e.is_production && <Badge>Production</Badge>}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {e.egress_allowlist.length
                      ? `Private network access: ${e.egress_allowlist.join(", ")}`
                      : "Outbound calls limited to public internet addresses"}
                  </p>
                </div>
                {canEdit && (
                  <div className="flex gap-1">
                    <Button variant="ghost" size="icon" aria-label={`Edit ${e.name}`} onClick={() => setEditing(e)}><Pencil /></Button>
                    <ConfirmDialog
                      trigger={<Button variant="ghost" size="icon" aria-label={`Delete ${e.name}`} disabled={q.data.length <= 1}><Trash2 /></Button>}
                      title={`Delete “${e.name}”?`}
                      description="Resources configured for this environment will stop working there. This cannot be undone."
                      confirmLabel="Delete environment"
                      onConfirm={async () => {
                        try {
                          await remove.mutateAsync(e.id);
                          toast.success("Environment deleted.");
                        } catch (err) {
                          toastError(err);
                          throw err;
                        }
                      }}
                    />
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
      {editing && <EnvironmentDialog env={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
    </Card>
  );
}

function EnvironmentDialog({ env, onClose }: { env: Environment | null; onClose: () => void }) {
  const egressEnabled = useFeature("environments.egress_allowlist");
  const { create, update } = useEnvironmentMutations();
  const [formError, setFormError] = useState<string>();
  const [plan, setPlan] = useState<string>();
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: env
      ? { name: env.name, slug: env.slug, is_production: env.is_production, allowlist: env.egress_allowlist.join("\n") }
      : { name: "", slug: "", is_production: false, allowlist: "" },
  });
  const { errors, isSubmitting } = form.formState;
  const isProduction = useWatch({ control: form.control, name: "is_production" });
  const name = form.register("name", {
    onChange: (e) => {
      if (!env && !form.getFieldState("slug").isDirty) form.setValue("slug", slugify(e.target.value));
    },
  });

  async function submit(v: Values) {
    setFormError(undefined);
    setPlan(undefined);
    const allow = parseAllowlist(v.allowlist);
    try {
      if (env) await update.mutateAsync({ id: env.id, body: { name: v.name, is_production: v.is_production, egress_allowlist: allow } });
      else await create.mutateAsync({ name: v.name, slug: v.slug, is_production: v.is_production, egress_allowlist: allow });
      toast.success(env ? "Environment saved." : "Environment created.");
      onClose();
    } catch (err) {
      const pe = planError(err);
      if (pe) {
        setPlan(pe.message);
        return;
      }
      setFormError(applyApiError(err, form.setError, FIELDS));
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen}>
        <form noValidate onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>{env ? `Edit ${env.name}` : "New environment"}</DialogTitle>
            <DialogDescription>Production environments are highlighted and will require extra confirmation for risky changes.</DialogDescription>
          </DialogHeader>
          <FormField id="env-name" label="Name" error={errors.name?.message}>
            <Input {...fieldAria("env-name", errors.name?.message)} {...name} />
          </FormField>
          {!env && (
            <FormField id="env-slug" label="Address" error={errors.slug?.message}>
              <Input className="font-mono" {...fieldAria("env-slug", errors.slug?.message)} {...form.register("slug")} />
            </FormField>
          )}
          <div className="flex items-center gap-2">
            <Switch id="env-prod" checked={isProduction}
              onCheckedChange={(v) => form.setValue("is_production", v, { shouldDirty: true })} />
            <Label htmlFor="env-prod">Production environment</Label>
          </div>
          <FormField id="env-allow" label="Private network access (IP ranges)"
            hint={egressEnabled ? "One CIDR or IP per line, e.g. 10.20.0.0/16. Outbound calls may reach these internal addresses." : "Available on Pro and Enterprise plans."}>
            <Textarea rows={3} className="font-mono" disabled={egressEnabled === false && !form.getValues("allowlist")}
              {...fieldAria("env-allow", undefined, "hint")} {...form.register("allowlist")} />
          </FormField>
          {plan && <PlanNotice message={plan} />}
          {formError && <Alert variant="destructive"><AlertDescription>{formError}</AlertDescription></Alert>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={isSubmitting}>{isSubmitting ? "Saving…" : env ? "Save" : "Create environment"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
