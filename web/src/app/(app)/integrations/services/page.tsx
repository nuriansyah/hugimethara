import type { Metadata } from "next";
import { ServiceRegistryManager } from "@/features/integrations/components/service-registry";

export const metadata: Metadata = { title: "Service registry" };

export default function Page() {
  return <ServiceRegistryManager />;
}
