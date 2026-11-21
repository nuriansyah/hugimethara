"use client";

import { useState } from "react";
import { QueryError } from "@/components/query-error";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { useAuditEvents, type AuditEvent, type AuditFilter } from "../api";

function toISO(local: string) {
  return local ? new Date(local).toISOString() : undefined;
}

export function AuditLog() {
  const [draft, setDraft] = useState({ action: "", since: "", until: "" });
  const [filter, setFilter] = useState<AuditFilter>({});
  const q = useAuditEvents(filter);
  const rows = q.data?.pages.flatMap((p) => p.data) ?? [];

  return (
    <div className="flex flex-col gap-4">
      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          setFilter({ action: draft.action.trim(), since: toISO(draft.since), until: toISO(draft.until) });
        }}
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="f-action">Action</Label>
          <Input id="f-action" placeholder="e.g. auth or organization.updated" value={draft.action}
            onChange={(e) => setDraft({ ...draft, action: e.target.value })} className="w-64" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="f-since">From</Label>
          <Input id="f-since" type="datetime-local" value={draft.since} onChange={(e) => setDraft({ ...draft, since: e.target.value })} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="f-until">To</Label>
          <Input id="f-until" type="datetime-local" value={draft.until} onChange={(e) => setDraft({ ...draft, until: e.target.value })} />
        </div>
        <Button type="submit">Apply</Button>
        <Button type="button" variant="ghost" onClick={() => { setDraft({ action: "", since: "", until: "" }); setFilter({}); }}>
          Clear
        </Button>
      </form>

      {q.isPending ? (
        <div className="flex flex-col gap-2" aria-busy="true" aria-label="Loading">
          {Array.from({ length: 6 }, (_, i) => <Skeleton key={i} className="h-10 w-full" />)}
        </div>
      ) : q.error ? (
        <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load the audit log" />
      ) : rows.length === 0 ? (
        <p className="rounded-md border border-dashed p-8 text-center text-sm text-muted-foreground">
          No events match these filters.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Resource</TableHead>
                <TableHead>Outcome</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((e) => <Row key={e.id} e={e} />)}
            </TableBody>
          </Table>
        </div>
      )}

      {q.hasNextPage && (
        <Button variant="outline" className="self-center" disabled={q.isFetchingNextPage} onClick={() => q.fetchNextPage()}>
          {q.isFetchingNextPage ? "Loading…" : "Load older events"}
        </Button>
      )}
      {q.isFetchNextPageError && (
        <p role="alert" className="text-center text-sm text-destructive">Could not load more events. Try again.</p>
      )}
    </div>
  );
}

function Row({ e }: { e: AuditEvent }) {
  const details = Object.keys(e.details).length ? JSON.stringify(e.details) : "";
  return (
    <TableRow>
      <TableCell className="whitespace-nowrap text-xs">{new Date(e.occurred_at).toLocaleString()}</TableCell>
      <TableCell className="text-sm">{e.actor_email ?? (e.actor_type === "anonymous" ? "Anonymous" : e.actor_type)}</TableCell>
      <TableCell className="font-mono text-xs">{e.action}</TableCell>
      <TableCell className="text-xs">
        {e.resource_type}
        {e.resource_id && <span className="block font-mono text-muted-foreground">{e.resource_id.slice(0, 8)}…</span>}
      </TableCell>
      <TableCell>
        <Badge variant={e.outcome === "success" ? "secondary" : "destructive"}>{e.outcome}</Badge>
      </TableCell>
      <TableCell className="max-w-xs truncate font-mono text-xs text-muted-foreground" title={details}>
        {details}
      </TableCell>
    </TableRow>
  );
}
