import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { RolesManager } from "@/features/access/components/roles-manager";

export const metadata: Metadata = { title: "Roles" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Roles" description="What each role allows. Assign roles on the Access page or inside a workspace." />
      <RolesManager />
    </div>
  );
}
