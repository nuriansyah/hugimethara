import type { Metadata } from "next";
import { WebhooksPage } from "@/features/webhooks/components/webhooks-page";

export const metadata: Metadata = { title: "Webhooks" };

export default function Page() {
  return <WebhooksPage />;
}
