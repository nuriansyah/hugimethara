"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { fieldAria, FormField } from "@/components/form-field";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useCurrentUser } from "@/features/auth/components/auth-gate";
import { applyApiError } from "@/lib/forms";
import { useCurrentOrganization, useUpdateOrganization, type Organization } from "../api";

const schema = z
  .object({
    name: z.string().trim().min(2, "Use at least 2 characters.").max(100),
    idle: z.number({ error: "Enter a number." }).int().min(5, "Minimum 5 minutes.").max(1440, "Maximum 1440 minutes."),
    absolute: z.number({ error: "Enter a number." }).int().min(1, "Minimum 1 hour.").max(720, "Maximum 720 hours."),
  })
  .refine((v) => v.idle <= v.absolute * 60, {
    path: ["idle"],
    message: "Idle timeout cannot be longer than the absolute session lifetime.",
  });
type Values = z.infer<typeof schema>;

const toValues = (o: Organization): Values => ({
  name: o.name,
  idle: o.settings.session_idle_timeout_minutes,
  absolute: o.settings.session_absolute_timeout_hours,
});

export function OrganizationSettingsForm() {
  const { data, error, isPending, refetch } = useCurrentOrganization();
  if (isPending) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-3 pt-6" aria-busy="true" aria-label="Loading">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-1/2" />
          <Skeleton className="h-9 w-1/2" />
        </CardContent>
      </Card>
    );
  }
  if (error) return <QueryError error={error} onRetry={() => refetch()} title="Could not load organization" />;
  return <SettingsForm org={data} />;
}

function SettingsForm({ org }: { org: Organization }) {
  const me = useCurrentUser();
  const canEdit = me.membership.is_owner; // UX only; the API enforces organization:update
  const update = useUpdateOrganization();
  const [formError, setFormError] = useState<string>();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: toValues(org) });
  const { errors, isSubmitting, isDirty } = form.formState;

  useEffect(() => form.reset(toValues(org)), [org, form]);

  async function submit(v: Values) {
    setFormError(undefined);
    try {
      await update.mutateAsync({
        name: v.name,
        settings: { session_idle_timeout_minutes: v.idle, session_absolute_timeout_hours: v.absolute },
      });
      toast.success("Organization settings saved.");
    } catch (err) {
      const msg = applyApiError(err, form.setError, ["name"]);
      // Map nested settings field names returned by the API.
      const fe = (err as { fieldErrors?: { field: string; message: string }[] }).fieldErrors ?? [];
      for (const e of fe) {
        if (e.field === "settings.session_idle_timeout_minutes") form.setError("idle", { message: e.message });
        if (e.field === "settings.session_absolute_timeout_hours") form.setError("absolute", { message: e.message });
      }
      setFormError(fe.length ? undefined : msg);
    }
  }

  return (
    <form noValidate onSubmit={form.handleSubmit(submit)}>
      <Card>
        <CardHeader>
          <CardTitle>Organization</CardTitle>
          <CardDescription>
            Address <span className="font-mono">{org.slug}</span> · created {new Date(org.created_at).toLocaleDateString()}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {!canEdit && (
            <Alert>
              <AlertDescription>Only organization owners can change these settings.</AlertDescription>
            </Alert>
          )}
          <fieldset disabled={!canEdit} className="flex flex-col gap-4">
            <FormField id="name" label="Name" error={errors.name?.message}>
              <Input {...fieldAria("name", errors.name?.message)} {...form.register("name")} />
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField id="idle" label="Idle session timeout (minutes)" error={errors.idle?.message} hint="Sign out after this much inactivity.">
                <Input type="number" inputMode="numeric" {...fieldAria("idle", errors.idle?.message, "hint")} {...form.register("idle", { valueAsNumber: true })} />
              </FormField>
              <FormField id="absolute" label="Maximum session lifetime (hours)" error={errors.absolute?.message} hint="Applies to new sessions.">
                <Input type="number" inputMode="numeric" {...fieldAria("absolute", errors.absolute?.message, "hint")} {...form.register("absolute", { valueAsNumber: true })} />
              </FormField>
            </div>
          </fieldset>
          {formError && (
            <Alert variant="destructive">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
        </CardContent>
        {canEdit && (
          <CardFooter className="justify-end gap-2">
            <Button type="button" variant="ghost" disabled={!isDirty || isSubmitting} onClick={() => form.reset(toValues(org))}>
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
