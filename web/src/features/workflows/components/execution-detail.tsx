"use client";

import { Badge } from "@hugimethara/ui/components/badge";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { QueryError } from "@/components/query-error";
import { useNodeTypes, useWorkflowExecution, type WorkflowDefinition, type WorkflowStep } from "../api";
import { WorkflowCanvas } from "./workflow-canvas";

export function StatusBadge({ status }: { status: string }) {
  const variant =
    status === "succeeded" ? "default" : status === "failed" || status === "timed_out" ? "destructive" : "outline";
  return (
    <Badge variant={variant} className="capitalize">
      {status.replace("_", " ")}
    </Badge>
  );
}

function duration(step: { started_at?: string; finished_at?: string }) {
  if (!step.started_at || !step.finished_at) return "—";
  const ms = new Date(step.finished_at).getTime() - new Date(step.started_at).getTime();
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`;
}

function Json({ label, value }: { label: string; value: unknown }) {
  if (value === undefined || value === null) return null;
  return (
    <div className="min-w-0">
      <p className="mb-1 text-xs font-medium text-muted-foreground">{label}</p>
      <pre className="max-h-48 overflow-auto rounded bg-muted p-2 font-mono text-[11px] leading-snug">
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  );
}

function StepRow({ step }: { step: WorkflowStep }) {
  return (
    <>
      <TableRow>
        <TableCell className="font-mono text-xs">{step.node_id}</TableCell>
        <TableCell className="font-mono text-xs text-muted-foreground">{step.node_type}</TableCell>
        <TableCell>
          <StatusBadge status={step.status} />
        </TableCell>
        <TableCell className="text-xs">{step.attempt}</TableCell>
        <TableCell className="font-mono text-xs">{step.port ?? "—"}</TableCell>
        <TableCell className="text-xs">{duration(step)}</TableCell>
      </TableRow>
      {(step.input !== undefined || step.output !== undefined || step.error) && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={6} className="pt-0">
            <details>
              <summary className="cursor-pointer text-xs text-muted-foreground">Input, output and error</summary>
              <div className="mt-2 grid gap-2 md:grid-cols-3">
                <Json label="Input" value={step.input} />
                <Json label="Output" value={step.output} />
                <Json label="Error" value={step.error} />
              </div>
            </details>
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

/** One execution: graph coloured by step status, the error, and every step's I/O. */
export function ExecutionDetail({ executionId, definition }: { executionId: string; definition?: WorkflowDefinition }) {
  const execution = useWorkflowExecution(executionId);
  const catalog = useNodeTypes();
  if (execution.isPending) return <Skeleton className="h-64 w-full" />;
  if (execution.isError) return <QueryError error={execution.error} onRetry={() => execution.refetch()} title="Execution unavailable" />;
  const e = execution.data;
  const statuses = Object.fromEntries((e.steps ?? []).map((s) => [s.node_id, s.status]));
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3 text-sm">
        <StatusBadge status={e.status} />
        <span className="text-muted-foreground">v{e.version}</span>
        <span className="text-muted-foreground">{e.step_count} steps</span>
        <span className="font-mono text-xs text-muted-foreground">{e.id}</span>
      </div>
      {e.error && (
        <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm">
          <p className="font-medium">
            {e.error.code}
            {e.error.node_id && <span className="font-normal text-muted-foreground"> at node {e.error.node_id}</span>}
          </p>
          <p className="text-muted-foreground">{e.error.message}</p>
        </div>
      )}
      {definition && catalog.data && (
        <WorkflowCanvas definition={definition} catalog={catalog.data} readOnly stepStatus={statuses} height={320} />
      )}
      <div className="grid gap-2 md:grid-cols-2">
        <Json label="Execution input" value={e.input} />
        <Json label="Execution output" value={e.output} />
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Node</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Attempt</TableHead>
            <TableHead>Port</TableHead>
            <TableHead>Duration</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(e.steps ?? []).map((s) => (
            <StepRow key={s.id} step={s} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
