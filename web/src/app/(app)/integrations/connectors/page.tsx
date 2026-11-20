import type { Metadata } from "next";
import { ConnectorsManager } from "@/features/integrations/components/connectors-manager";

export const metadata: Metadata = { title: "Connectors" };

export default function Page() {
  return <ConnectorsManager />;
}
