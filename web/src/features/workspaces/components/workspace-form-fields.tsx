"use client";

import { useWatch, type UseFormReturn } from "react-hook-form";
import { z } from "zod";
import { fieldAria, FormField } from "@/components/form-field";
import { Input } from "@hugimethara/ui/components/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { useFeature } from "@/features/billing/api";
import { SHORT_SLUG, SHORT_SLUG_MESSAGE } from "@/lib/slug";
import { useDepartments } from "../api";

export const NO_DEPARTMENT = "__none__";

export const workspaceSchema = z.object({
  slug: z.string().regex(SHORT_SLUG, SHORT_SLUG_MESSAGE),
  name: z.string().trim().min(1, "Enter a name.").max(100),
  description: z.string().max(500, "Use at most 500 characters."),
  department_id: z.string(),
});
export type WorkspaceValues = z.infer<typeof workspaceSchema>;
export const WORKSPACE_FIELDS = ["slug", "name", "description", "department_id"] as const;

/** Name, description and (when the plan includes departments) department. */
export function WorkspaceFields({ form, showSlug }: { form: UseFormReturn<WorkspaceValues>; showSlug: boolean }) {
  const { errors } = form.formState;
  const departmentsEnabled = useFeature("departments");
  const departments = useDepartments();
  const deptValue = useWatch({ control: form.control, name: "department_id" });
  return (
    <>
      <FormField id="ws-name" label="Name" error={errors.name?.message}>
        <Input {...fieldAria("ws-name", errors.name?.message)} {...form.register("name")} />
      </FormField>
      {showSlug && (
        <FormField id="ws-slug" label="Address" error={errors.slug?.message} hint="Used in links. Cannot be changed later.">
          <Input className="font-mono" autoCapitalize="none" spellCheck={false}
            {...fieldAria("ws-slug", errors.slug?.message, "hint")} {...form.register("slug")} />
        </FormField>
      )}
      <FormField id="ws-description" label="Description" error={errors.description?.message}>
        <Textarea rows={3} {...fieldAria("ws-description", errors.description?.message)} {...form.register("description")} />
      </FormField>
      {(departmentsEnabled || deptValue !== NO_DEPARTMENT) && (
        <FormField id="ws-department" label="Department" error={errors.department_id?.message}>
          <Select value={deptValue} onValueChange={(v) => form.setValue("department_id", v, { shouldDirty: true })}>
            <SelectTrigger id="ws-department" className="w-full">
              <SelectValue placeholder="No department" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_DEPARTMENT}>No department</SelectItem>
              {(departments.data ?? []).map((d) => (
                <SelectItem key={d.id} value={d.id}>
                  {d.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormField>
      )}
    </>
  );
}

export function departmentPayload(v: string) {
  return v === NO_DEPARTMENT ? undefined : v;
}
