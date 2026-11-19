import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { DepartmentsManager } from "@/features/settings/components/departments-manager";

export const metadata: Metadata = { title: "Departments" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <PageHeader title="Departments" description="Group workspaces by department." />
      <DepartmentsManager />
    </div>
  );
}
