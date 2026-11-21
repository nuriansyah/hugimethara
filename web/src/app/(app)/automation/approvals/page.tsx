import type { Metadata } from "next";
import { ApprovalsInbox } from "@/features/workflows/components/approvals-inbox";

export const metadata: Metadata = { title: "Approvals" };

export default function Page() {
  return <ApprovalsInbox />;
}
