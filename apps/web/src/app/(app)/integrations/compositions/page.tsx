import type { Metadata } from "next";
import { CompositionStudio } from "@/features/integrations/components/composition-studio";

export const metadata: Metadata = { title: "API compositions" };

export default function Page() {
  return <CompositionStudio />;
}
