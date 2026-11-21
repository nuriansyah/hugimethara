import type { Metadata } from "next";
import { Suspense } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { LoginForm } from "@/features/auth/components/login-form";

export const metadata: Metadata = { title: "Sign in" };

export default function LoginPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Sign in</CardTitle>
        <CardDescription>Use your work email and password.</CardDescription>
      </CardHeader>
      <CardContent>
        {/* LoginForm reads ?next=, so it renders client-side under Suspense. */}
        <Suspense fallback={<Skeleton className="h-48 w-full" />}>
          <LoginForm />
        </Suspense>
      </CardContent>
    </Card>
  );
}
