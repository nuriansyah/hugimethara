import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { DeliveriesPage } from "@/features/webhooks/components/deliveries-page";

export const metadata: Metadata = { title: "Webhook deliveries" };

export default function Page() {
  return (
    // DeliveriesPage reads ?subscription=, so it renders client-side under Suspense.
    <Suspense fallback={<Skeleton className="mx-auto h-72 w-full max-w-6xl" />}>
      <DeliveriesPage />
    </Suspense>
  );
}
