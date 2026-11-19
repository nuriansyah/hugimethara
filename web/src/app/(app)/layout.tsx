import { Suspense, type ReactNode } from "react";
import { AppShell } from "@/components/layout/app-shell";
import { AuthGate, GateFallback } from "@/features/auth/components/auth-gate";

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    // The gate reads the current path and session in the browser, so it renders
    // client-side under this boundary (required for dynamic routes with Cache Components).
    <Suspense fallback={<GateFallback />}>
      <AuthGate>
        <AppShell>{children}</AppShell>
      </AuthGate>
    </Suspense>
  );
}
