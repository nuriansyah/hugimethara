"use client";

import { Check, Inbox, Send, X } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { toastError } from "@/lib/errors";
import { useApprovalTasks, useDecideTask, type ApprovalTask } from "../api";
import { StatusBadge } from "./execution-detail";

const STATUSES = ["pending", "approved", "rejected", "submitted", "expired", "cancelled"] as const;

function when(iso?: string) {
  return iso ? new Date(iso).toLocaleString() : "—";
}

type FormValue = string | number | boolean | undefined;

/** The form of a manual-input task, typed by field. */
function TaskForm({
  task,
  values,
  onChange,
}: {
  task: ApprovalTask;
  values: Record<string, FormValue>;
  onChange: (key: string, v: FormValue) => void;
}) {
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {task.fields.map((f) => {
        const id = `task-${task.id}-${f.key}`;
        const label = (
          <Label htmlFor={id}>
            {f.label}
            {f.required && <span className="text-destructive"> *</span>}
          </Label>
        );
        if (f.type === "boolean") {
          return (
            <div key={f.key} className="flex flex-col gap-1.5">
              {label}
              <Select
                value={values[f.key] === undefined ? "" : String(values[f.key])}
                onValueChange={(v) => onChange(f.key, v === "true")}
              >
                <SelectTrigger id={id} className="w-full">
                  <SelectValue placeholder="Choose…" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="true">Yes</SelectItem>
                  <SelectItem value="false">No</SelectItem>
                </SelectContent>
              </Select>
            </div>
          );
        }
        return (
          <div key={f.key} className={f.type === "text" ? "flex flex-col gap-1.5 md:col-span-2" : "flex flex-col gap-1.5"}>
            {label}
            {f.type === "text" ? (
              <Textarea id={id} value={String(values[f.key] ?? "")} onChange={(e) => onChange(f.key, e.target.value || undefined)} />
            ) : (
              <Input
                id={id}
                type={f.type === "number" ? "number" : "text"}
                value={values[f.key] === undefined ? "" : String(values[f.key])}
                onChange={(e) =>
                  onChange(
                    f.key,
                    e.target.value === "" ? undefined : f.type === "number" ? Number(e.target.value) : e.target.value,
                  )
                }
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function TaskCard({ task }: { task: ApprovalTask }) {
  const decide = useDecideTask();
  const [comment, setComment] = useState("");
  const [values, setValues] = useState<Record<string, FormValue>>({});

  async function submit(decision: "approve" | "reject" | "submit") {
    try {
      const data = Object.fromEntries(Object.entries(values).filter(([, v]) => v !== undefined));
      await decide.mutateAsync({
        id: task.id,
        body: { decision, comment: comment || undefined, ...(decision === "submit" ? { data } : {}) },
      });
      toast.success(decision === "approve" ? "Approved" : decision === "reject" ? "Rejected" : "Submitted");
    } catch (err) {
      toastError(err);
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div>
            <CardTitle className="text-base">{task.title}</CardTitle>
            <CardDescription>
              {task.workflow_name} · node <span className="font-mono">{task.node_id}</span> · requested {when(task.created_at)}
              {task.due_at && task.status === "pending" && <> · expires {when(task.due_at)}</>}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="outline">{task.kind === "approval" ? "Approval" : "Input"}</Badge>
            <StatusBadge status={task.status} />
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {task.instructions && <p className="whitespace-pre-wrap text-sm">{task.instructions}</p>}
        {task.context !== undefined && task.context !== null && (
          <details open={task.status === "pending"}>
            <summary className="cursor-pointer text-xs text-muted-foreground">Request details</summary>
            <pre className="mt-2 max-h-56 overflow-auto rounded bg-muted p-2 font-mono text-[11px] leading-snug">
              {JSON.stringify(task.context, null, 2)}
            </pre>
          </details>
        )}
        {task.can_decide ? (
          <>
            {task.kind === "input" && (
              <TaskForm task={task} values={values} onChange={(k, v) => setValues((cur) => ({ ...cur, [k]: v }))} />
            )}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor={`comment-${task.id}`}>Comment</Label>
              <Textarea
                id={`comment-${task.id}`}
                value={comment}
                maxLength={2000}
                placeholder="Optional — recorded with the decision and in the audit log."
                onChange={(e) => setComment(e.target.value)}
              />
            </div>
            <div className="flex flex-wrap gap-2">
              {task.kind === "approval" ? (
                <>
                  <Button disabled={decide.isPending} onClick={() => submit("approve")}>
                    <Check aria-hidden /> Approve
                  </Button>
                  <Button variant="outline" disabled={decide.isPending} onClick={() => submit("reject")}>
                    <X aria-hidden /> Reject
                  </Button>
                </>
              ) : (
                <Button disabled={decide.isPending} onClick={() => submit("submit")}>
                  <Send aria-hidden /> Submit
                </Button>
              )}
            </div>
            {task.kind === "approval" && !task.allow_self_approval && (
              <p className="text-xs text-muted-foreground">
                If you started this execution, another assignee has to approve it.
              </p>
            )}
          </>
        ) : (
          task.status !== "pending" && (
            <p className="text-sm text-muted-foreground">
              {task.status === "expired" ? "Expired without a decision" : `Resolved ${when(task.decided_at)}`}
              {task.comment && <> — “{task.comment}”</>}
            </p>
          )
        )}
      </CardContent>
    </Card>
  );
}

/** Tasks assigned to the caller directly or through a group, in every workspace. */
export function ApprovalsInbox() {
  const [status, setStatus] = useState<string>("pending");
  const tasks = useApprovalTasks({ status });
  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <PageHeader
        title="Approvals"
        description="Approval and manual-input tasks assigned to you or to a group you belong to. Deciding resumes the waiting workflow on the matching branch."
      />
      <div className="flex items-end gap-3">
        <div className="flex min-w-48 flex-col gap-1.5">
          <Label htmlFor="task-status">Status</Label>
          <Select value={status} onValueChange={setStatus}>
            <SelectTrigger id="task-status" className="w-full capitalize">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {STATUSES.map((s) => (
                <SelectItem key={s} value={s} className="capitalize">
                  {s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      {tasks.isPending && <Skeleton className="h-40 w-full" />}
      {tasks.isError && <QueryError error={tasks.error} onRetry={() => tasks.refetch()} title="Tasks unavailable" />}
      {tasks.data?.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <Inbox className="size-8 text-muted-foreground" aria-hidden />
            <p className="font-medium">{status === "pending" ? "Nothing waiting for you" : `No ${status} tasks`}</p>
            <p className="text-sm text-muted-foreground">Tasks appear here when a workflow reaches an approval or input step.</p>
          </CardContent>
        </Card>
      )}
      {tasks.data?.map((t) => <TaskCard key={t.id} task={t} />)}
    </div>
  );
}
