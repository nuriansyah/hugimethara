import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { SessionsList } from "@/features/auth/components/sessions-list";

export const metadata: Metadata = { title: "My sessions" };

export default function SessionsPage() {
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <PageHeader title="My sessions" description="Sign out devices you no longer use." />
      <SessionsList />
    </div>
  );
}
