"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { fieldAria, FormField } from "@/components/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { applyApiError, safeNext } from "@/lib/forms";
import { organizationChoices, useLogin, type OrganizationChoice } from "../api";

const schema = z.object({
  email: z.email("Enter a valid email address."),
  password: z.string().min(1, "Enter your password."),
});
type Values = z.infer<typeof schema>;

export function LoginForm() {
  const router = useRouter();
  const next = safeNext(useSearchParams().get("next"));
  const login = useLogin();
  const [formError, setFormError] = useState<string>();
  const [choices, setChoices] = useState<OrganizationChoice[]>();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { email: "", password: "" } });
  const { errors, isSubmitting } = form.formState;

  async function submit(values: Values, organizationSlug?: string) {
    setFormError(undefined);
    try {
      await login.mutateAsync({ ...values, organization_slug: organizationSlug });
      router.replace(next);
    } catch (err) {
      const orgs = organizationChoices(err);
      if (orgs) {
        setChoices(orgs);
        return;
      }
      setChoices(undefined);
      setFormError(applyApiError(err, form.setError, ["email", "password"]));
    }
  }

  if (choices) {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-sm text-muted-foreground">Your account belongs to several organizations. Choose one:</p>
        <ul className="flex flex-col gap-2">
          {choices.map((o) => (
            <li key={o.slug}>
              <Button
                variant="outline"
                className="w-full justify-between"
                disabled={login.isPending}
                onClick={() => submit(form.getValues(), o.slug)}
              >
                <span>{o.name}</span>
                <span className="font-mono text-xs text-muted-foreground">{o.slug}</span>
              </Button>
            </li>
          ))}
        </ul>
        {formError && (
          <Alert variant="destructive">
            <AlertDescription>{formError}</AlertDescription>
          </Alert>
        )}
        <Button variant="ghost" onClick={() => setChoices(undefined)}>
          Use a different account
        </Button>
      </div>
    );
  }

  return (
    <form noValidate onSubmit={form.handleSubmit((v) => submit(v))} className="flex flex-col gap-4">
      <FormField id="email" label="Email" error={errors.email?.message}>
        <Input type="email" autoComplete="username" autoFocus {...fieldAria("email", errors.email?.message)} {...form.register("email")} />
      </FormField>
      <FormField id="password" label="Password" error={errors.password?.message}>
        <Input type="password" autoComplete="current-password" {...fieldAria("password", errors.password?.message)} {...form.register("password")} />
      </FormField>
      {formError && (
        <Alert variant="destructive">
          <AlertDescription>{formError}</AlertDescription>
        </Alert>
      )}
      <Button type="submit" disabled={isSubmitting}>
        {isSubmitting ? "Signing in…" : "Sign in"}
      </Button>
      <p className="text-center text-sm text-muted-foreground">
        New to Hugimethara?{" "}
        <Link href="/signup" className="font-medium text-foreground underline underline-offset-4">
          Create an organization
        </Link>
      </p>
    </form>
  );
}
