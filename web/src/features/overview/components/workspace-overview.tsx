"use client";

import { AlertTriangle, CheckCircle2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "@hugimethara/ui/components/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { QueryError } from "@/components/query-error";
import { useWorkspaces } from "@/features/workspaces/api";
import { useOverview, type Overview, type OverviewWindow } from "../api";

const WINDOWS: { value: OverviewWindow; label: string }[] = [
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
];

function duration(ms?: number) {
  if (ms === undefined || ms === null) return "—";
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`;
}

function when(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function Stat({ label, value, hint, tone }: { label: string; value: string | number; hint?: string; tone?: "bad" | "good" }) {
  return (
    <div className="flex flex-col gap-1 rounded-md border p-3">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className={`text-2xl font-semibold tabular-nums ${tone === "bad" ? "text-destructive" : ""}`}>{value}</span>
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </div>
  );
}

/** Executions per bucket as stacked bars (succeeded / failed), no chart library. */
function ExecutionBars({ series, bucket }: { series: NonNullable<Overview["workflows"]>["series"]; bucket: "hour" | "day" }) {
  const max = Math.max(1, ...series.map((p) => p.succeeded + p.failed));
  const label = (iso: string) =>
    new Date(iso).toLocaleString(undefined, bucket === "hour" ? { hour: "2-digit", minute: "2-digit" } : { month: "short", day: "numeric" });
  return (
    <figure className="flex flex-col gap-2">
      <div className="flex h-32 items-end gap-px" role="img"
        aria-label={`Executions per ${bucket}: ${series.reduce((a, p) => a + p.succeeded, 0)} succeeded, ${series.reduce((a, p) => a + p.failed, 0)} failed`}>
        {series.map((p) => (
          <div key={p.bucket} className="flex h-full min-w-0 flex-1 flex-col justify-end" title={`${label(p.bucket)} — ${p.succeeded} succeeded, ${p.failed} failed`}>
            <div className="bg-destructive/80" style={{ height: `${(p.failed / max) * 100}%` }} />
            <div className="bg-primary/70" style={{ height: `${(p.succeeded / max) * 100}%` }} />
          </div>
        ))}
      </div>
      <figcaption className="flex justify-between text-xs text-muted-foreground">
        <span>{series[0] && label(series[0].bucket)}</span>
        <span className="flex gap-3">
          <span className="flex items-center gap-1"><span className="inline-block size-2 bg-primary/70" aria-hidden /> Succeeded</span>
          <span className="flex items-center gap-1"><span className="inline-block size-2 bg-destructive/80" aria-hidden /> Failed</span>
        </span>
        <span>{series.at(-1) && label(series.at(-1)!.bucket)}</span>
      </figcaption>
    </figure>
  );
}

function WorkflowsCard({ w, window }: { w: NonNullable<Overview["workflows"]>; window: OverviewWindow }) {
  const rate = w.success_rate === undefined || w.success_rate === null ? "—" : `${Math.round(w.success_rate * 1000) / 10}%`;
  return (
    <Card className="lg:col-span-2">
      <CardHeader>
        <CardTitle>Workflow executions</CardTitle>
        <CardDescription>Started in the window; running counts what is in flight now.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <Stat label="Executions" value={w.total} />
          <Stat label="Success rate" value={rate} hint={`${w.succeeded} succeeded`} />
          <Stat label="Failed" value={w.failed} tone={w.failed > 0 ? "bad" : undefined} hint="incl. timed out" />
          <Stat label="Running now" value={w.running} />
          <Stat label="Duration p50 / p95" value={duration(w.p50_ms)} hint={`p95 ${duration(w.p95_ms)}`} />
          <Stat label="Pending approvals" value={w.pending_approvals} hint={`${w.active_workflows} active workflows`} />
        </div>
        <ExecutionBars series={w.series} bucket={window === "24h" ? "hour" : "day"} />
        {w.top_failing.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Most failing workflows</TableHead>
                <TableHead className="text-right">Failures</TableHead>
                <TableHead>Last error</TableHead>
                <TableHead>Last failure</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {w.top_failing.map((f) => (
                <TableRow key={f.workflow_id}>
                  <TableCell>
                    <Link href="/automation/workflows" className="underline underline-offset-2">{f.name}</Link>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{f.failures}</TableCell>
                  <TableCell><code className="text-xs">{f.last_error_code || "—"}</code></TableCell>
                  <TableCell className="text-muted-foreground">{when(f.last_failed_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function ConnectorsCard({ c }: { c: NonNullable<Overview["connectors"]> }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Connector health</CardTitle>
        <CardDescription>From the latest connection test of each instance.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid grid-cols-3 gap-3">
          <Stat label="Healthy" value={c.healthy} hint={`of ${c.total} (${c.enabled} enabled)`} />
          <Stat label="Unhealthy" value={c.unhealthy} tone={c.unhealthy > 0 ? "bad" : undefined} />
          <Stat label="Never tested" value={c.unchecked} />
        </div>
        {c.failing.length === 0 ? (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <CheckCircle2 className="size-4" aria-hidden /> No failing connection tests.
          </p>
        ) : (
          <ul className="flex flex-col gap-2 text-sm">
            {c.failing.map((f) => (
              <li key={f.connector_id} className="flex items-center justify-between gap-2">
                <span className="flex items-center gap-2">
                  <AlertTriangle className="size-4 text-destructive" aria-hidden />
                  <span className="font-mono">{f.ref_key}</span>
                  <Badge variant="outline">{f.definition_key}</Badge>
                </span>
                <span className="text-xs text-muted-foreground"><code>{f.error_code}</code> · {when(f.checked_at)}</span>
              </li>
            ))}
          </ul>
        )}
        <Link href="/integrations/connectors" className="text-sm underline underline-offset-2">Open connectors</Link>
      </CardContent>
    </Card>
  );
}

function WebhooksCard({ h }: { h: NonNullable<Overview["webhooks"]> }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Webhooks</CardTitle>
        <CardDescription>Verified requests and outbound deliveries in the window.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-3">
          <Stat label="Received" value={h.received} hint={`${h.endpoints} endpoints`} />
          <Stat label="Rejected" value={h.rejected_total} hint="all time" tone={h.rejected_total > 0 ? "bad" : undefined} />
          <Stat label="Delivered" value={h.deliveries_succeeded} hint={`${h.subscriptions} subscriptions · ${h.deliveries_pending} pending`} />
          <Stat label="Dead-lettered" value={h.deliveries_dead_lettered} tone={h.deliveries_dead_lettered > 0 ? "bad" : undefined} />
        </div>
        <Link href="/automation/webhook-deliveries" className="text-sm underline underline-offset-2">Open delivery log</Link>
      </CardContent>
    </Card>
  );
}

/** Operational KPIs of one workspace (S14). Sections the caller cannot read are absent. */
export function WorkspaceOverview() {
  const workspaces = useWorkspaces(false);
  const [workspaceId, setWorkspaceId] = useState("");
  const [window, setWindow] = useState<OverviewWindow>("24h");
  const resolved = workspaceId || workspaces.data?.[0]?.id || "";
  const overview = useOverview(resolved, window);
  return (
    <section className="flex flex-col gap-4" aria-label="Workspace activity">
      <div className="flex flex-wrap items-end gap-4">
        <div className="flex min-w-48 flex-col gap-1.5">
          <Label htmlFor="ov-workspace">Workspace</Label>
          <Select value={resolved} onValueChange={setWorkspaceId}>
            <SelectTrigger id="ov-workspace" className="w-full">
              <SelectValue placeholder={workspaces.data?.length === 0 ? "No workspaces yet" : "Choose a workspace"} />
            </SelectTrigger>
            <SelectContent>
              {(workspaces.data ?? []).map((w) => (
                <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex min-w-40 flex-col gap-1.5">
          <Label htmlFor="ov-window">Period</Label>
          <Select value={window} onValueChange={(v) => setWindow(v as OverviewWindow)}>
            <SelectTrigger id="ov-window" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {WINDOWS.map((w) => (
                <SelectItem key={w.value} value={w.value}>{w.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      {workspaces.data?.length === 0 && (
        <p className="text-sm text-muted-foreground">
          Create a <Link href="/workspaces" className="underline underline-offset-2">workspace</Link> to see its activity here.
        </p>
      )}
      {(workspaces.isPending || (resolved && overview.isPending)) && <Skeleton className="h-64 w-full" />}
      {overview.isError && <QueryError error={overview.error} onRetry={() => overview.refetch()} title="Activity unavailable" />}
      {overview.data && (
        <div className="grid gap-4 lg:grid-cols-2">
          {overview.data.workflows && <WorkflowsCard w={overview.data.workflows} window={window} />}
          {overview.data.connectors && <ConnectorsCard c={overview.data.connectors} />}
          {overview.data.webhooks && <WebhooksCard h={overview.data.webhooks} />}
        </div>
      )}
    </section>
  );
}
