import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { EnvironmentsManager } from "@/features/settings/components/environments-manager";

export const metadata: Metadata = { title: "Environments" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <PageHeader title="Environments" description="Deployment stages and their network access." />
      <EnvironmentsManager />
    </div>
  );
}
