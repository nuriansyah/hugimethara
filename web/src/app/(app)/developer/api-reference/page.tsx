import type { Metadata } from "next";
import { ApiReference } from "@/features/api-reference/components/api-reference";

export const metadata: Metadata = { title: "API reference" };

export default function Page() {
  return <ApiReference />;
}
