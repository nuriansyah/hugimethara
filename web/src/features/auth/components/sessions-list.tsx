"use client";

import { toast } from "sonner";
import { QueryError } from "@/components/query-error";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { ApiError } from "@hugimethara/api-client";
import { useMySessions, useRevokeSession } from "../api";

function describeAgent(ua?: string) {
  if (!ua) return "Unknown device";
  const browser = /Edg\//.test(ua) ? "Edge" : /Chrome\//.test(ua) ? "Chrome" : /Firefox\//.test(ua) ? "Firefox" : /Safari\//.test(ua) ? "Safari" : "Browser";
  const os = /Windows/.test(ua) ? "Windows" : /Mac OS X/.test(ua) ? "macOS" : /Android/.test(ua) ? "Android" : /iPhone|iPad/.test(ua) ? "iOS" : /Linux/.test(ua) ? "Linux" : "";
  return os ? `${browser} on ${os}` : browser;
}

export function SessionsList() {
  const { data, error, isPending, refetch } = useMySessions();
  const revoke = useRevokeSession();

  async function onRevoke(id: string) {
    try {
      await revoke.mutateAsync(id);
      toast.success("Session signed out.");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Could not sign out that session.");
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Active sessions</CardTitle>
        <CardDescription>Devices signed in to this organization with your account.</CardDescription>
      </CardHeader>
      <CardContent>
        {isPending ? (
          <div className="flex flex-col gap-2" aria-busy="true" aria-label="Loading">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : error ? (
          <QueryError error={error} onRetry={() => refetch()} title="Could not load sessions" />
        ) : data.length === 0 ? (
          <p className="text-sm text-muted-foreground">No active sessions.</p>
        ) : (
          <ul className="divide-y">
            {data.map((s) => (
              <li key={s.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                <div className="flex flex-col">
                  <span className="flex items-center gap-2 text-sm font-medium">
                    {describeAgent(s.user_agent)}
                    {s.current && <Badge variant="secondary">This device</Badge>}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {s.ip ?? "IP unknown"} · last active {new Date(s.last_seen_at).toLocaleString()} · expires{" "}
                    {new Date(s.expires_at).toLocaleString()}
                  </span>
                </div>
                {!s.current && (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={revoke.isPending && revoke.variables === s.id}
                    onClick={() => onRevoke(s.id)}
                  >
                    Sign out
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
