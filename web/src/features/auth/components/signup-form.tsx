"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { fieldAria, FormField } from "@/components/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { applyApiError } from "@/lib/forms";
import { useSignup } from "../api";

// Mirrors the API's validation so users get instant feedback; the API remains
// the authority and its field errors are mapped back onto the form.
const schema = z.object({
  organization_name: z.string().trim().min(2, "Use at least 2 characters.").max(100, "Use at most 100 characters."),
  organization_slug: z
    .string()
    .regex(/^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$/, "3–40 lowercase letters, digits or hyphens; no leading or trailing hyphen."),
  display_name: z.string().trim().min(1, "Enter your name.").max(100),
  email: z.email("Enter a valid email address."),
  password: z.string().min(12, "Use at least 12 characters.").max(128, "Use at most 128 characters."),
});
type Values = z.infer<typeof schema>;
const FIELDS = ["organization_name", "organization_slug", "display_name", "email", "password"] as const;

function slugify(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40)
    .replace(/-+$/, "");
}

export function SignupForm() {
  const router = useRouter();
  const signup = useSignup();
  const [formError, setFormError] = useState<string>();
  const [slugTouched, setSlugTouched] = useState(false);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { organization_name: "", organization_slug: "", display_name: "", email: "", password: "" },
  });
  const { errors, isSubmitting } = form.formState;

  const orgName = form.register("organization_name", {
    onChange: (e) => {
      if (!slugTouched) form.setValue("organization_slug", slugify(e.target.value));
    },
  });
  const slug = form.register("organization_slug", { onChange: () => setSlugTouched(true) });

  async function submit(values: Values) {
    setFormError(undefined);
    try {
      await signup.mutateAsync(values);
      router.replace("/");
    } catch (err) {
      setFormError(applyApiError(err, form.setError, FIELDS));
    }
  }

  return (
    <form method="post" noValidate onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
      <FormField id="organization_name" label="Organization name" error={errors.organization_name?.message}>
        <Input autoComplete="organization" autoFocus {...fieldAria("organization_name", errors.organization_name?.message)} {...orgName} />
      </FormField>
      <FormField
        id="organization_slug"
        label="Organization address"
        error={errors.organization_slug?.message}
        hint="Used to identify your organization when signing in. Cannot be changed later."
      >
        <Input
          autoCapitalize="none"
          spellCheck={false}
          className="font-mono"
          {...fieldAria("organization_slug", errors.organization_slug?.message, "hint")}
          {...slug}
        />
      </FormField>
      <FormField id="display_name" label="Your name" error={errors.display_name?.message}>
        <Input autoComplete="name" {...fieldAria("display_name", errors.display_name?.message)} {...form.register("display_name")} />
      </FormField>
      <FormField id="email" label="Work email" error={errors.email?.message}>
        <Input type="email" autoComplete="email" {...fieldAria("email", errors.email?.message)} {...form.register("email")} />
      </FormField>
      <FormField id="password" label="Password" error={errors.password?.message} hint="At least 12 characters. A passphrase works well.">
        <Input type="password" autoComplete="new-password" {...fieldAria("password", errors.password?.message, "hint")} {...form.register("password")} />
      </FormField>
      {formError && (
        <Alert variant="destructive">
          <AlertDescription>{formError}</AlertDescription>
        </Alert>
      )}
      <Button type="submit" disabled={isSubmitting}>
        {isSubmitting ? "Creating organization…" : "Create organization"}
      </Button>
      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{" "}
        <Link href="/login" className="font-medium text-foreground underline underline-offset-4">
          Sign in
        </Link>
      </p>
    </form>
  );
}
