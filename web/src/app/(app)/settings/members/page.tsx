import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { MembersManager } from "@/features/identity/components/members-manager";

export const metadata: Metadata = { title: "Members" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader title="Members" description="People in the organization, their status and pending invitations." />
      <MembersManager />
    </div>
  );
}
