"use client";

import { CheckCircle2, XCircle } from "lucide-react";
import { Badge } from "@hugimethara/ui/components/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { useReadiness, useSystemInfo } from "../api";
import { QueryError } from "@/components/query-error";

export function SystemStatus() {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <BuildInfoCard />
      <DependenciesCard />
    </div>
  );
}

function BuildInfoCard() {
  const { data, error, isPending, refetch } = useSystemInfo();
  return (
    <Card>
      <CardHeader>
        <CardTitle>Deployment</CardTitle>
        <CardDescription>Build and environment reported by hugi-api.</CardDescription>
      </CardHeader>
      <CardContent>
        {isPending ? (
          <Rows />
        ) : error ? (
          <QueryError error={error} onRetry={() => refetch()} title="Could not load deployment info" />
        ) : (
          <dl className="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">
            <dt className="text-muted-foreground">Environment</dt>
            <dd>
              <Badge variant={data.environment === "production" ? "default" : "secondary"}>{data.environment}</Badge>
            </dd>
            <dt className="text-muted-foreground">Version</dt>
            <dd className="font-mono">{data.version}</dd>
            <dt className="text-muted-foreground">Commit</dt>
            <dd className="font-mono">{data.commit}</dd>
            <dt className="text-muted-foreground">API</dt>
            <dd className="font-mono">{data.api_version}</dd>
            <dt className="text-muted-foreground">Go</dt>
            <dd className="font-mono">{data.go_version}</dd>
          </dl>
        )}
      </CardContent>
    </Card>
  );
}

function DependenciesCard() {
  const { data, error, isPending, refetch } = useReadiness();
  const checks = Object.entries(data?.checks ?? {}).sort(([a], [b]) => a.localeCompare(b));
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          Dependencies
          {data && (
            <Badge variant={data.status === "ok" ? "secondary" : "destructive"}>
              {data.status === "ok" ? "Ready" : "Degraded"}
            </Badge>
          )}
        </CardTitle>
        <CardDescription>Readiness checks, refreshed every 15 seconds.</CardDescription>
      </CardHeader>
      <CardContent>
        {isPending ? (
          <Rows />
        ) : error ? (
          <QueryError error={error} onRetry={() => refetch()} title="Could not load readiness" />
        ) : checks.length === 0 ? (
          <p className="text-sm text-muted-foreground">No readiness checks are registered.</p>
        ) : (
          <ul className="flex flex-col gap-2 text-sm">
            {checks.map(([name, c]) => (
              <li key={name} className="flex items-start gap-2">
                {c.status === "ok" ? (
                  <CheckCircle2 className="mt-0.5 size-4 text-emerald-600" aria-label="ok" />
                ) : (
                  <XCircle className="mt-0.5 size-4 text-destructive" aria-label="failing" />
                )}
                <span className="font-medium">{name}</span>
                {c.error && <span className="text-muted-foreground">— {c.error}</span>}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

function Rows() {
  return (
    <div className="flex flex-col gap-2" aria-busy="true" aria-label="Loading">
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-4 w-1/2" />
      <Skeleton className="h-4 w-3/5" />
    </div>
  );
}
