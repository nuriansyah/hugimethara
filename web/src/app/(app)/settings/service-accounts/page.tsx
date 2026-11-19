import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { ServiceAccountsManager } from "@/features/identity/components/service-accounts-manager";

export const metadata: Metadata = { title: "Service accounts" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader title="Service accounts" description="Non-human identities for scripts and integrations, and their API keys." />
      <ServiceAccountsManager />
    </div>
  );
}
