import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { InvitePage } from "./invite-page";

export const metadata: Metadata = { title: "Join organization" };

export default function Page() {
  return (
    // The token is read client-side and only ever sent to the API.
    <Suspense fallback={<Skeleton className="h-64 w-full" />}>
      <InvitePage />
    </Suspense>
  );
}
