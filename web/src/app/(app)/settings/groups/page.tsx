import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { GroupsManager } from "@/features/identity/components/groups-manager";

export const metadata: Metadata = { title: "Groups" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader title="Groups" description="Groups of members that roles can be assigned to." />
      <GroupsManager />
    </div>
  );
}
