import { PageHeader } from "@/components/page-header";
import { SystemStatus } from "@/features/system/components/system-status";

export default function OverviewPage() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader title="Overview" description="Platform health for this Hugimethara deployment." />
      <SystemStatus />
    </div>
  );
}
