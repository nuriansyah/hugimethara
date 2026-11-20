import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { IdentityProvidersManager } from "@/features/identity/components/identity-providers-manager";

export const metadata: Metadata = { title: "Single sign-on" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader
        title="Single sign-on"
        description="Connect an OIDC identity provider so members can sign in with their work account."
      />
      <IdentityProvidersManager />
    </div>
  );
}
