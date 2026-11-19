import type { Metadata } from "next";
import { PageHeader } from "@/components/page-header";
import { PlanUsage } from "@/features/billing/components/plan-usage";

export const metadata: Metadata = { title: "Plan & usage" };

export default function Page() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <PageHeader title="Plan & usage" description="Your organization's plan, paid features and quotas." />
      <PlanUsage />
    </div>
  );
}
