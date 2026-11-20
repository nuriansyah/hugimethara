import { Suspense } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { WorkspacePage } from "./workspace-page";

export default function Page() {
  return (
    <div className="mx-auto max-w-4xl">
      {/* The slug is read client-side; data comes from the API under the session. */}
      <Suspense fallback={<Skeleton className="h-72 w-full" />}>
        <WorkspacePage />
      </Suspense>
    </div>
  );
}
