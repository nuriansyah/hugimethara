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
import { api } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";
import { applyApiError, safeNext } from "@/lib/forms";
import { organizationChoices, useLogin, type OrganizationChoice } from "../api";

const SSO_ERRORS: Record<string, string> = {
  provider_unavailable: "Single sign-on is temporarily unavailable. Try again or use your password.",
  state_missing: "Your sign-in attempt expired. Start again.",
  state_invalid: "Your sign-in attempt could not be verified. Start again.",
  state_expired: "Your sign-in attempt expired. Start again.",
  access_denied: "Sign-in was cancelled at the identity provider.",
  idp_error: "The identity provider reported an error.",
  invalid_token: "The identity provider's response could not be verified.",
  email_unverified: "Your identity provider has not verified this email address.",
  email_missing: "The identity provider did not return an email address.",
  email_domain_not_allowed: "Your email domain is not allowed for this organization.",
  account_disabled: "Your account is disabled.",
  membership_suspended: "Your membership is suspended. Contact an organization administrator.",
  account_exists: "An account with this email already exists. Sign in with your password first, then link SSO.",
  no_account: "No account exists. Ask an administrator to invite you.",
  account_conflict: "This sign-in conflicts with an existing account. Contact an administrator.",
};

type SsoProvider = { id: string; name: string; preset: string };

const schema = z.object({
  email: z.email("Enter a valid email address."),
  password: z.string().min(1, "Enter your password."),
});
type Values = z.infer<typeof schema>;

export function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const next = safeNext(params.get("next"));
  const login = useLogin();
  const [formError, setFormError] = useState<string>();
  const [choices, setChoices] = useState<OrganizationChoice[]>();
  const [ssoError, setSsoError] = useState<string | undefined>(() => {
    const code = params.get("sso_error");
    return code ? (SSO_ERRORS[code] ?? "Single sign-on failed. Try again or use your password.") : undefined;
  });
  const [ssoSlug, setSsoSlug] = useState("");
  const [ssoProviders, setSsoProviders] = useState<SsoProvider[]>();
  const [ssoPending, setSsoPending] = useState(false);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { email: "", password: "" } });
  const { errors, isSubmitting } = form.formState;

  function startSso(id: string) {
    const url = new URL(`/api/v1/hugimethara/auth/oidc/${id}/start`, window.location.origin);
    url.searchParams.set("redirect", next);
    window.location.assign(url.toString());
  }

  async function findProviders() {
    const organization = ssoSlug.trim();
    if (!organization) {
      setSsoError("Enter your organization slug to find its SSO provider.");
      return;
    }
    setSsoError(undefined);
    setSsoProviders(undefined);
    setSsoPending(true);
    try {
      const { data, response } = await api.GET("/v1/hugimethara/auth/oidc/providers", {
        params: { query: { organization } },
      });
      if (!response.ok || !data) throw new Error("Could not look up SSO providers.");
      if (data.data.length === 0) {
        setSsoError("No SSO provider is enabled for that organization.");
      } else if (data.data.length === 1) {
        startSso(data.data[0].id);
      } else {
        setSsoProviders(data.data);
      }
    } catch (err) {
      setSsoError(errorMessage(err));
    } finally {
      setSsoPending(false);
    }
  }

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
    <div className="flex flex-col gap-4">
      <form noValidate onSubmit={form.handleSubmit((v) => submit(v))} className="flex flex-col gap-4">
        <FormField id="email" label="Email" error={errors.email?.message}>
          <Input type="email" autoComplete="username" autoFocus {...fieldAria("email", errors.email?.message)} {...form.register("email")} />
        </FormField>
        <FormField id="password" label="Password" error={errors.password?.message}>
          <Input type="password" autoComplete="current-password" {...fieldAria("password", errors.password?.message)} {...form.register("password")} />
        </FormField>
        {ssoError && (
          <Alert variant="destructive">
            <AlertDescription>{ssoError}</AlertDescription>
          </Alert>
        )}
        {formError && (
          <Alert variant="destructive">
            <AlertDescription>{formError}</AlertDescription>
          </Alert>
        )}
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Signing in…" : "Sign in"}
        </Button>
      </form>

      <div className="flex items-center gap-3" aria-hidden>
        <span className="h-px flex-1 bg-border" />
        <span className="text-xs uppercase tracking-wide text-muted-foreground">or</span>
        <span className="h-px flex-1 bg-border" />
      </div>

      <div className="flex flex-col gap-2">
        <FormField id="sso-org" label="Organization slug">
          <div className="flex gap-2">
            <Input
              id="sso-org"
              value={ssoSlug}
              autoComplete="organization"
              placeholder="acme-id"
              onChange={(e) => setSsoSlug(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  findProviders();
                }
              }}
            />
            <Button type="button" variant="outline" disabled={ssoPending} onClick={findProviders}>
              {ssoPending ? "Looking up…" : "Continue with SSO"}
            </Button>
          </div>
        </FormField>
        {ssoProviders && ssoProviders.length > 1 && (
          <div className="flex flex-col gap-2">
            <p className="text-sm text-muted-foreground">Choose an identity provider:</p>
            {ssoProviders.map((p) => (
              <Button key={p.id} type="button" variant="outline" className="w-full justify-start" onClick={() => startSso(p.id)}>
                {p.name}
              </Button>
            ))}
          </div>
        )}
      </div>

      <p className="text-center text-sm text-muted-foreground">
        New to Hugimethara?{" "}
        <Link href="/signup" className="font-medium text-foreground underline underline-offset-4">
          Create an organization
        </Link>
      </p>
    </div>
  );
}
