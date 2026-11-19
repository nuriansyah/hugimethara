"use client";

import { usePathname, useRouter } from "next/navigation";
import { createContext, useContext, useEffect, type ReactNode } from "react";
import { QueryError } from "@/components/query-error";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError } from "@/lib/api/client";
import { useMe, type Me } from "../api";

const MeContext = createContext<Me | null>(null);

/** Placeholder shown while the session is being checked. */
export function GateFallback() {
  return (
    <div className="flex min-h-screen items-center justify-center p-8" aria-busy="true" aria-label="Loading">
      <div className="flex w-64 flex-col gap-3">
        <Skeleton className="h-6 w-1/2" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    </div>
  );
}

/** The signed-in identity. Only usable below <AuthGate>. */
export function useCurrentUser(): Me {
  const me = useContext(MeContext);
  if (!me) throw new Error("useCurrentUser must be used inside <AuthGate>");
  return me;
}

/**
 * Renders children only for a signed-in user and sends everyone else to the
 * login page. This is navigation UX; every API call is authorized server-side.
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { data, error, isPending, refetch } = useMe();
  const router = useRouter();
  const pathname = usePathname();
  const unauthenticated = error instanceof ApiError && error.status === 401;

  useEffect(() => {
    if (unauthenticated) router.replace(`/login?next=${encodeURIComponent(pathname)}`);
  }, [unauthenticated, pathname, router]);

  if (isPending || unauthenticated) return <GateFallback />;
  if (error) {
    return (
      <div className="mx-auto max-w-md p-8">
        <QueryError error={error} onRetry={() => refetch()} title="Could not load your account" />
      </div>
    );
  }
  return <MeContext.Provider value={data}>{children}</MeContext.Provider>;
}
