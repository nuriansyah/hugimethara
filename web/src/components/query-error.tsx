"use client";

import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/client";

export function QueryError({ error, onRetry, title }: { error: Error; onRetry: () => void; title: string }) {
  const requestId = error instanceof ApiError ? error.requestId : undefined;
  return (
    <Alert variant="destructive">
      <AlertCircle aria-hidden />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <p>{error instanceof ApiError ? error.message : "The API could not be reached. Check that hugi-api is running."}</p>
        {requestId && <p className="font-mono text-xs">Request ID: {requestId}</p>}
        <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
          Try again
        </Button>
      </AlertDescription>
    </Alert>
  );
}
