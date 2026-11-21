"use client";

import { CalendarClock } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "@hugimethara/ui/components/badge";
import { Card, CardContent } from "@hugimethara/ui/components/card";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { useWorkspaces } from "@/features/workspaces/api";
import { useWorkflowSchedules, type WorkflowSchedule } from "../api";

function relative(iso: string) {
  const ms = new Date(iso).getTime() - Date.now();
  const abs = Math.abs(ms);
  const unit = abs < 3_600_000 ? [60_000, "minute"] : abs < 86_400_000 ? [3_600_000, "hour"] : [86_400_000, "day"];
  return new Intl.RelativeTimeFormat(undefined, { numeric: "auto" }).format(
    Math.round(ms / (unit[0] as number)),
    unit[1] as Intl.RelativeTimeFormatUnit,
  );
}

function state(s: WorkflowSchedule) {
  if (!s.enabled) return <Badge variant="outline">Deployment disabled</Badge>;
  if (s.workflow_status !== "active") return <Badge variant="outline">Workflow {s.workflow_status}</Badge>;
  if (!s.next_fire_at) return <Badge variant="destructive">Stopped</Badge>;
  return <Badge>Active</Badge>;
}

/** Deployed schedule triggers of a workspace with their next activation. */
export function SchedulesPage() {
  const workspaces = useWorkspaces(false);
  const [workspaceId, setWorkspaceId] = useState("");
  const resolved = workspaceId || workspaces.data?.[0]?.id || "";
  const schedules = useWorkflowSchedules(resolved);
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader
        title="Schedules"
        description="Workflows deployed with a schedule trigger. A missed activation (worker down) runs once when the worker is back; it is never replayed per missed slot. Pause a schedule by disabling its deployment or pausing the workflow."
      />
      <div className="flex min-w-48 max-w-xs flex-col gap-1.5">
        <Label htmlFor="sched-workspace">Workspace</Label>
        <Select value={resolved} onValueChange={setWorkspaceId}>
          <SelectTrigger id="sched-workspace" className="w-full">
            <SelectValue placeholder="Choose a workspace" />
          </SelectTrigger>
          <SelectContent>
            {(workspaces.data ?? []).map((w) => (
              <SelectItem key={w.id} value={w.id}>
                {w.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {(workspaces.isPending || (resolved && schedules.isPending)) && <Skeleton className="h-40 w-full" />}
      {schedules.isError && <QueryError error={schedules.error} onRetry={() => schedules.refetch()} title="Schedules unavailable" />}
      {schedules.data?.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <CalendarClock className="size-8 text-muted-foreground" aria-hidden />
            <p className="font-medium">No scheduled workflows</p>
            <p className="text-sm text-muted-foreground">
              Choose the Schedule trigger in the{" "}
              <Link href="/automation/workflows" className="underline underline-offset-2">
                workflow designer
              </Link>
              , then publish and deploy.
            </p>
          </CardContent>
        </Card>
      )}
      {schedules.data && schedules.data.length > 0 && (
        <Card>
          <CardContent className="pt-6">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Workflow</TableHead>
                  <TableHead>Environment</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Next run</TableHead>
                  <TableHead>State</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {schedules.data.map((s) => (
                  <TableRow key={`${s.workflow_id}-${s.environment_id}`}>
                    <TableCell>
                      <span className="font-medium">{s.workflow_name}</span>{" "}
                      <span className="text-xs text-muted-foreground">v{s.version}</span>
                    </TableCell>
                    <TableCell>{s.environment_name}</TableCell>
                    <TableCell>
                      <code className="font-mono text-xs">{s.cron}</code>{" "}
                      <span className="text-xs text-muted-foreground">{s.timezone}</span>
                    </TableCell>
                    <TableCell className="text-sm">
                      {s.next_fire_at && s.enabled && s.workflow_status === "active" ? (
                        <span title={new Date(s.next_fire_at).toLocaleString()}>{relative(s.next_fire_at)}</span>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>{state(s)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
