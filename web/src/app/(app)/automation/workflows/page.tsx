import type { Metadata } from "next";
import { WorkflowStudio } from "@/features/workflows/components/workflow-studio";

export const metadata: Metadata = { title: "Workflows" };

export default function Page() {
  return <WorkflowStudio />;
}
