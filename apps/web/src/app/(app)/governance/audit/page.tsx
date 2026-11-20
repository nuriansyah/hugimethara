import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { AuditLog } from "@/features/audit/components/audit-log";

export const metadata: Metadata = { title: "Audit log" };

export default function AuditPage() {
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Audit log" description="Every change and sign-in in this organization, newest first." />
      <AuditLog />
    </div>
  );
}
