import type { Metadata } from "next";
import { CredentialsManager } from "@/features/secrets/components/credentials-manager";

export const metadata: Metadata = { title: "Credentials" };

export default function Page() {
  return <CredentialsManager />;
}
