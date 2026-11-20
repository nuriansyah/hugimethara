import type { Metadata } from "next";
import { MappingStudio } from "@/features/integrations/components/mapping-studio";

export const metadata: Metadata = { title: "Mapping studio" };

export default function Page() {
  return <MappingStudio />;
}
