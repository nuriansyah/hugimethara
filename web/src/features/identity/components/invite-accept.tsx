"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { QueryError } from "@/components/query-error";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { useMe } from "@/features/auth/api";
import { ApiError } from "@hugimethara/api-client";
import { errorMessage } from "@/lib/errors";
import { useAcceptInvitation, useInvitationPreview } from "../api";

export function InviteAccept({ token }: { token: string }) {
  const router = useRouter();
  const preview = useInvitationPreview(token);
  const me = useMe();
  const accept = useAcceptInvitation(token);
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  if (preview.isPending || (me.isPending && !me.error)) {
    return <Card><CardContent className="pt-6"><Skeleton className="h-40 w-full" /></CardContent></Card>;
  }
  if (preview.error) {
    const gone = preview.error instanceof ApiError && (preview.error.status === 404 || preview.error.status === 410);
    return (
      <Card>
        <CardHeader>
          <CardTitle>Invitation unavailable</CardTitle>
          <CardDescription>{gone ? preview.error.message : "We could not open this invitation."}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {!gone && <QueryError error={preview.error} onRetry={() => preview.refetch()} title="Could not load the invitation" />}
          <Button asChild variant="outline"><Link href="/login">Go to sign in</Link></Button>
        </CardContent>
      </Card>
    );
  }

  const inv = preview.data;
  const signedInAs = me.data?.user.email;
  const sameAccount = !!signedInAs && signedInAs.toLowerCase() === inv.email.toLowerCase();

  async function join(withPassword: boolean) {
    setError(undefined);
    setFieldErrors({});
    try {
      await accept.mutateAsync(withPassword ? { password, display_name: inv.account_exists ? undefined : name } : {});
      router.replace("/");
    } catch (err) {
      if (err instanceof ApiError && err.fieldErrors?.length) {
        setFieldErrors(Object.fromEntries(err.fieldErrors.map((f) => [f.field, f.message])));
      } else {
        setError(errorMessage(err));
      }
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Join {inv.organization.name}</CardTitle>
        <CardDescription>
          {inv.invited_by_name ? `${inv.invited_by_name} invited ` : "You were invited as "}
          <span className="font-medium text-foreground">{inv.email}</span>
. The invitation expires on {new Date(inv.expires_at).toLocaleDateString()}.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {sameAccount ? (
          <Button disabled={accept.isPending} onClick={() => join(false)}>
            {accept.isPending ? "Joining…" : `Join as ${signedInAs}`}
          </Button>
        ) : (
          <form noValidate className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); join(true); }}>
            {signedInAs && (
              <Alert>
                <AlertDescription>
                  You are signed in as {signedInAs}.{" "}
                  {inv.account_exists
                    ? `To accept for ${inv.email}, enter that account's password below.`
                    : `Create the account for ${inv.email} below; you will be signed in as it.`}
                </AlertDescription>
              </Alert>
            )}
            {!inv.account_exists && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ia-name">Your name</Label>
                <Input id="ia-name" autoComplete="name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)}
                  aria-invalid={fieldErrors.display_name ? true : undefined}
                  aria-describedby={fieldErrors.display_name ? "ia-name-error" : undefined} />
                {fieldErrors.display_name && <p id="ia-name-error" role="alert" className="text-sm text-destructive">{fieldErrors.display_name}</p>}
              </div>
            )}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ia-password">{inv.account_exists ? "Password" : "Choose a password"}</Label>
              <Input id="ia-password" type="password" value={password}
                autoComplete={inv.account_exists ? "current-password" : "new-password"}
                onChange={(e) => setPassword(e.target.value)}
                aria-invalid={fieldErrors.password ? true : undefined}
                aria-describedby={fieldErrors.password ? "ia-password-error" : inv.account_exists ? undefined : "ia-password-hint"} />
              {fieldErrors.password ? (
                <p id="ia-password-error" role="alert" className="text-sm text-destructive">{fieldErrors.password}</p>
              ) : inv.account_exists ? (
                <p className="text-xs text-muted-foreground">You already have an account with this email. Sign in with its password to join.</p>
              ) : (
                <p id="ia-password-hint" className="text-xs text-muted-foreground">At least 12 characters.</p>
              )}
            </div>
            <Button type="submit" disabled={!password || (!inv.account_exists && !name.trim()) || accept.isPending}>
              {accept.isPending ? "Joining…" : inv.account_exists ? "Sign in and join" : "Create account and join"}
            </Button>
          </form>
        )}
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      </CardContent>
    </Card>
  );
}
