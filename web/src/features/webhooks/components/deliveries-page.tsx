"use client";

import { RotateCcw, Send } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent } from "@hugimethara/ui/components/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@hugimethara/ui/components/dialog";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { useCan } from "@/features/access/api";
import { toastError } from "@/lib/errors";
import {
  useReplayDelivery,
  useWebhookDeliveries,
  useWebhookDelivery,
  useWebhookSubscriptions,
  type WebhookDelivery,
} from "../api";
import { useWorkspaceChoice, WorkspacePicker } from "./workspace-picker";

const ALL = "all";

function StatusBadge({ d }: { d: Pick<WebhookDelivery, "status" | "attempts"> }) {
  if (d.status === "succeeded") return <Badge>Succeeded</Badge>;
  if (d.status === "dead_lettered") return <Badge variant="destructive">Dead-lettered</Badge>;
  return <Badge variant="outline">{d.attempts === 0 ? "Queued" : "Retrying"}</Badge>;
}

function response(d: Pick<WebhookDelivery, "response_status" | "error" | "latency_ms">) {
  const parts = [];
  if (d.response_status) parts.push(`HTTP ${d.response_status}`);
  if (d.error && !d.error.startsWith("http_")) parts.push(d.error);
  if (d.latency_ms !== undefined) parts.push(`${d.latency_ms} ms`);
  return parts.join(" · ") || "—";
}

/** Outbound delivery log with per-attempt detail and replay. */
export function DeliveriesPage() {
  const params = useSearchParams();
  const [chosen, setChosen] = useState("");
  const { workspaces, resolved } = useWorkspaceChoice(chosen);
  const subs = useWebhookSubscriptions(resolved);
  const [subscription, setSubscription] = useState(params.get("subscription") ?? ALL);
  const [status, setStatus] = useState(ALL);
  const [open, setOpen] = useState("");
  const subId = subscription !== ALL && subs.data?.some((s) => s.id === subscription) ? subscription : undefined;
  const q = useWebhookDeliveries({ workspaceId: resolved, subscriptionId: subId, status: status === ALL ? undefined : status });
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader
        title="Webhook deliveries"
        description="Every outbound delivery with its attempts. A failed attempt is retried on the schedule 30s, 2m, 10m, 30m, 2h, 6h, 12h; after the eighth it is dead-lettered and can be replayed."
      />
      <div className="flex flex-wrap items-end gap-4">
        <WorkspacePicker
          id="dl-workspace"
          value={resolved}
          onChange={(v) => {
            setChosen(v);
            setSubscription(ALL);
          }}
          workspaces={workspaces.data ?? []}
        />
        <div className="flex min-w-48 flex-col gap-1.5">
          <Label htmlFor="dl-sub">Subscription</Label>
          <Select value={subId ?? ALL} onValueChange={setSubscription}>
            <SelectTrigger id="dl-sub" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>All subscriptions</SelectItem>
              {(subs.data ?? []).map((s) => (
                <SelectItem key={s.id} value={s.id}>
                  {s.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex min-w-40 flex-col gap-1.5">
          <Label htmlFor="dl-status">Status</Label>
          <Select value={status} onValueChange={setStatus}>
            <SelectTrigger id="dl-status" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Any status</SelectItem>
              <SelectItem value="pending">Pending</SelectItem>
              <SelectItem value="succeeded">Succeeded</SelectItem>
              <SelectItem value="dead_lettered">Dead-lettered</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      {(workspaces.isPending || (resolved && q.isPending)) && <Skeleton className="h-40 w-full" />}
      {q.isError && <QueryError error={q.error} onRetry={() => q.refetch()} title="Deliveries unavailable" />}
      {q.data?.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <Send className="size-8 text-muted-foreground" aria-hidden />
            <p className="font-medium">No deliveries</p>
            <p className="text-sm text-muted-foreground">
              Create a subscription on the{" "}
              <Link href="/automation/webhooks" className="underline underline-offset-2">
                Webhooks
              </Link>{" "}
              page and send it a test.
            </p>
          </CardContent>
        </Card>
      )}
      {q.data && q.data.length > 0 && (
        <Card>
          <CardContent className="pt-6">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Event</TableHead>
                  <TableHead>Subscription</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Attempts</TableHead>
                  <TableHead>Last response</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data.map((d) => (
                  <TableRow key={d.id} className="cursor-pointer" onClick={() => setOpen(d.id)}>
                    <TableCell>
                      <button type="button" className="text-left font-mono text-xs underline-offset-2 hover:underline" onClick={() => setOpen(d.id)}>
                        {d.event_type}
                      </button>
                      {d.replay_of && (
                        <Badge variant="secondary" className="ml-2">
                          Replay
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>{d.subscription_name}</TableCell>
                    <TableCell>
                      <StatusBadge d={d} />
                    </TableCell>
                    <TableCell className="text-sm">
                      {d.attempts}/{d.max_attempts}
                      {d.status === "pending" && d.next_attempt_at && d.attempts > 0 && (
                        <span className="block text-xs text-muted-foreground">
                          next {new Date(d.next_attempt_at).toLocaleTimeString()}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="text-sm">{response(d)}</TableCell>
                    <TableCell className="text-sm">{new Date(d.created_at).toLocaleString()}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
      {open && <DeliveryDialog id={open} onClose={() => setOpen("")} onOpen={setOpen} />}
    </div>
  );
}

function DeliveryDialog({ id, onClose, onOpen }: { id: string; onClose: () => void; onOpen: (id: string) => void }) {
  const can = useCan();
  const q = useWebhookDelivery(id);
  const replay = useReplayDelivery();
  const d = q.data;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            Delivery of <code className="font-mono text-base">{d?.event_type ?? "…"}</code>
          </DialogTitle>
          <DialogDescription>
            {d ? (
              <>
                To {d.subscription_name} · event {d.event_id}
                {d.replay_of && (
                  <>
                    {" · replay of "}
                    <button type="button" className="underline underline-offset-2" onClick={() => onOpen(d.replay_of!)}>
                      the original
                    </button>
                  </>
                )}
              </>
            ) : (
              "Loading…"
            )}
          </DialogDescription>
        </DialogHeader>
        {q.isPending ? (
          <Skeleton className="h-48" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load the delivery" />
        ) : (
          d && (
            <div className="flex max-h-[60vh] flex-col gap-4 overflow-y-auto text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge d={d} />
                <span className="text-muted-foreground">
                  {d.attempts}/{d.max_attempts} attempts
                  {d.status === "pending" && d.next_attempt_at && ` · next at ${new Date(d.next_attempt_at).toLocaleString()}`}
                  {d.finished_at && ` · finished ${new Date(d.finished_at).toLocaleString()}`}
                </span>
              </div>
              <div>
                <p className="mb-1 font-medium">Attempts</p>
                {(d.attempt_log ?? []).length === 0 ? (
                  <p className="text-muted-foreground">Not attempted yet.</p>
                ) : (
                  <ol className="flex flex-col gap-2">
                    {(d.attempt_log ?? []).map((a) => (
                      <li key={a.attempt} className="rounded-md border p-2">
                        <p className="text-xs">
                          <span className="font-medium">#{a.attempt}</span> · {new Date(a.attempted_at).toLocaleString()} ·{" "}
                          {a.response_status ? `HTTP ${a.response_status}` : "no response"} · {a.latency_ms} ms
                          {a.error && (
                            <>
                              {" · "}
                              <code className="font-mono">{a.error}</code>
                            </>
                          )}
                        </p>
                        {a.response_excerpt && (
                          <pre className="mt-1 overflow-x-auto rounded bg-muted p-2 font-mono text-xs">{a.response_excerpt}</pre>
                        )}
                      </li>
                    ))}
                  </ol>
                )}
              </div>
              <div>
                <p className="mb-1 font-medium">Payload (data)</p>
                <pre className="overflow-x-auto rounded bg-muted p-2 font-mono text-xs">{JSON.stringify(d.payload, null, 2)}</pre>
              </div>
            </div>
          )
        )}
        <DialogFooter>
          {d && d.status !== "pending" && can("webhook:replay", d.workspace_id) && (
            <Button
              variant="outline"
              disabled={replay.isPending}
              onClick={async () => {
                try {
                  const next = await replay.mutateAsync(d.id);
                  toast.success("Replay queued.");
                  onOpen(next.id);
                } catch (err) {
                  toastError(err);
                }
              }}
            >
              <RotateCcw aria-hidden /> {replay.isPending ? "Queuing…" : "Replay"}
            </Button>
          )}
          <Button variant="ghost" onClick={onClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
