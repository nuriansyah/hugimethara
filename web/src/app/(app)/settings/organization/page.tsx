import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { OrganizationSettingsForm } from "@/features/organization/components/organization-settings-form";

export const metadata: Metadata = { title: "Organization settings" };

export default function OrganizationSettingsPage() {
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <PageHeader title="Organization settings" description="Name and session security for everyone in this organization." />
      <OrganizationSettingsForm />
    </div>
  );
}
