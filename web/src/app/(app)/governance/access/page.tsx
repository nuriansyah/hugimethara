import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { AccessManager } from "@/features/access/components/access-manager";

export const metadata: Metadata = { title: "Access" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Access" description="Role assignments, access checks and recent denials." />
      <AccessManager />
    </div>
  );
}
