import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { WorkspaceList } from "@/features/workspaces/components/workspace-list";

export const metadata: Metadata = { title: "Workspaces" };

export default function WorkspacesPage() {
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Workspaces" description="Each team works in its own workspace." />
      <WorkspaceList />
    </div>
  );
}
