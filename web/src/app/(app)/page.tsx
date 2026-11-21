import { PageHeader } from "@/components/page-header";
import { WorkspaceOverview } from "@/features/overview/components/workspace-overview";
import { SystemStatus } from "@/features/system/components/system-status";

export default function OverviewPage() {
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Overview" description="Workspace activity and platform health for this Hugimethara deployment." />
      <WorkspaceOverview />
      <SystemStatus />
    </div>
  );
}
