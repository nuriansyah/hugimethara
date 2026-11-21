"use client";

import { Inbox, KeyRound, ListChecks, Plus, Send, Trash2, Webhook } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { ApiError } from "@hugimethara/api-client";
import { Alert, AlertDescription } from "@hugimethara/ui/components/alert";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Checkbox } from "@hugimethara/ui/components/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@hugimethara/ui/components/dialog";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Switch } from "@hugimethara/ui/components/switch";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { CopyField } from "@/components/copy-field";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { useCan } from "@/features/access/api";
import { useEnvironments } from "@/features/workspaces/api";
import { errorMessage, toastError } from "@/lib/errors";
import { keepOpen } from "@/lib/forms";
import {
  ingressURL,
  useEventTypes,
  useWebhookEndpointMutations,
  useWebhookEndpoints,
  useWebhookEvent,
  useWebhookEvents,
  useWebhookSubscriptionMutations,
  useWebhookSubscriptions,
  verifierLabel,
  VERIFIERS,
  type WebhookEndpoint,
  type WebhookSubscription,
} from "../api";
import { useWorkspaceChoice, WorkspacePicker } from "./workspace-picker";

function when(iso?: string) {
  return iso ? new Date(iso).toLocaleString() : "never";
}

/** Inbound endpoints and outbound subscriptions of one workspace. */
export function WebhooksPage() {
  const [chosen, setChosen] = useState("");
  const { workspaces, resolved } = useWorkspaceChoice(chosen);
  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader
        title="Webhooks"
        description="Receive signed webhooks from other systems (they can trigger workflows) and send platform events to your own endpoints, signed with Standard Webhooks and retried for about 21 hours."
      />
      <WorkspacePicker id="wh-workspace" value={resolved} onChange={setChosen} workspaces={workspaces.data ?? []} />
      {workspaces.isPending && <Skeleton className="h-40 w-full" />}
      {resolved && (
        <>
          <EndpointsCard workspaceId={resolved} />
          <SubscriptionsCard workspaceId={resolved} />
        </>
      )}
    </div>
  );
}

// ---- inbound ----

function EndpointsCard({ workspaceId }: { workspaceId: string }) {
  const can = useCan();
  const q = useWebhookEndpoints(workspaceId);
  const [creating, setCreating] = useState(false);
  const [eventsFor, setEventsFor] = useState<WebhookEndpoint>();
  const [revealed, setRevealed] = useState<{ endpoint: WebhookEndpoint; title: string }>();
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Inbound endpoints</CardTitle>
          <CardDescription>
            Each endpoint has its own URL and signature check. Requests with a missing or wrong signature, or a timestamp
            more than the tolerance away, are refused and counted, never stored. A repeated provider event id is
            acknowledged once. Bind an endpoint to a workflow with the Webhook trigger.
          </CardDescription>
        </div>
        {can("webhook:create", workspaceId) && (
          <Button onClick={() => setCreating(true)}>
            <Plus aria-hidden /> New endpoint
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load endpoints" />
        ) : q.data.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <Inbox className="size-8 text-muted-foreground" aria-hidden />
            <p className="text-sm text-muted-foreground">No inbound endpoints in this workspace yet.</p>
          </div>
        ) : (
          <ul className="divide-y">
            {q.data.map((e) => (
              <EndpointRow
                key={e.id}
                endpoint={e}
                workspaceId={workspaceId}
                onEvents={() => setEventsFor(e)}
                onRotated={(endpoint) => setRevealed({ endpoint, title: "New signing secret" })}
              />
            ))}
          </ul>
        )}
      </CardContent>
      {creating && (
        <CreateEndpointDialog
          workspaceId={workspaceId}
          onClose={() => setCreating(false)}
          onCreated={(endpoint) => {
            setCreating(false);
            setRevealed({ endpoint, title: `Endpoint “${endpoint.name}” created` });
          }}
        />
      )}
      {eventsFor && <EventsDialog endpoint={eventsFor} onClose={() => setEventsFor(undefined)} />}
      {revealed && <EndpointCreatedDialog {...revealed} onClose={() => setRevealed(undefined)} />}
    </Card>
  );
}

function EndpointRow({
  endpoint: e,
  workspaceId,
  onEvents,
  onRotated,
}: {
  endpoint: WebhookEndpoint;
  workspaceId: string;
  onEvents: () => void;
  onRotated: (e: WebhookEndpoint) => void;
}) {
  const can = useCan();
  const envs = useEnvironments();
  const { update, remove } = useWebhookEndpointMutations(workspaceId);
  const [rotating, setRotating] = useState(false);
  const envName = envs.data?.find((x) => x.id === e.environment_id)?.name ?? "…";
  return (
    <li className="flex flex-col gap-3 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium">
            {e.name} <Badge variant="outline" className="ml-1">{verifierLabel(e.verifier)}</Badge>{" "}
            <Badge variant="secondary">{envName}</Badge>
            {!e.enabled && (
              <Badge variant="destructive" className="ml-1">
                Disabled
              </Badge>
            )}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            {e.received_count} received · last {when(e.last_received_at)} · {e.rejected_count} rejected
            {e.rejected_count > 0 && e.last_rejected_reason && (
              <>
                {" "}
                (last: <code className="font-mono">{e.last_rejected_reason}</code>, {when(e.last_rejected_at)})
              </>
            )}
            {` · secret ${e.secret_hint || "set"} · ±${e.tolerance_seconds}s · ${e.rate_limit_per_minute}/min`}
          </p>
          {e.description && <p className="mt-1 text-xs text-muted-foreground">{e.description}</p>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {can("webhook:update", workspaceId) && (
            <label className="flex items-center gap-2 text-sm">
              <Switch
                checked={e.enabled}
                disabled={update.isPending}
                aria-label={`${e.name} enabled`}
                onCheckedChange={async (on) => {
                  try {
                    await update.mutateAsync({ id: e.id, body: { enabled: on } });
                    toast.success(on ? `${e.name} accepts requests again.` : `${e.name} now answers 404 to every request.`);
                  } catch (err) {
                    toastError(err);
                  }
                }}
              />
              Enabled
            </label>
          )}
          <Button variant="outline" size="sm" onClick={onEvents}>
            <ListChecks aria-hidden /> Events
          </Button>
          {can("webhook:update", workspaceId) && (
            <Button variant="outline" size="sm" onClick={() => setRotating(true)}>
              <KeyRound aria-hidden /> Rotate secret
            </Button>
          )}
          {can("webhook:delete", workspaceId) && (
            <ConfirmDialog
              trigger={
                <Button variant="ghost" size="icon" aria-label={`Delete ${e.name}`}>
                  <Trash2 />
                </Button>
              }
              title={`Delete “${e.name}”?`}
              description="Senders get 404 from now on, the received events are deleted, and workflows bound to this endpoint stop being triggered. This cannot be undone."
              confirmLabel="Delete endpoint"
              onConfirm={async () => {
                try {
                  await remove.mutateAsync(e.id);
                  toast.success("Endpoint deleted.");
                } catch (err) {
                  toastError(err);
                  throw err;
                }
              }}
            />
          )}
        </div>
      </div>
      <CopyField id={`url-${e.id}`} label="Endpoint URL" value={ingressURL(e)} />
      {rotating && (
        <RotateEndpointDialog
          endpoint={e}
          workspaceId={workspaceId}
          onClose={() => setRotating(false)}
          onRotated={(next) => {
            setRotating(false);
            if (next.secret) onRotated(next);
            else toast.success("Secret replaced. The old one no longer verifies.");
          }}
        />
      )}
    </li>
  );
}

function fieldError(err: unknown, field: string) {
  return err instanceof ApiError ? err.fieldErrors?.find((f) => f.field === field)?.message : undefined;
}

function CreateEndpointDialog({
  workspaceId,
  onClose,
  onCreated,
}: {
  workspaceId: string;
  onClose: () => void;
  onCreated: (e: WebhookEndpoint) => void;
}) {
  const envs = useEnvironments();
  const { create } = useWebhookEndpointMutations(workspaceId);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [environmentId, setEnvironmentId] = useState("");
  const [verifier, setVerifier] = useState<WebhookEndpoint["verifier"]>("standard_webhooks");
  const [secret, setSecret] = useState("");
  const [sigHeader, setSigHeader] = useState("X-Signature");
  const [encoding, setEncoding] = useState<"hex" | "base64">("hex");
  const [prefix, setPrefix] = useState("");
  const [tsHeader, setTsHeader] = useState("");
  const [idHeader, setIdHeader] = useState("");
  const [err, setErr] = useState<unknown>();
  const env = environmentId || envs.data?.[0]?.id || "";
  const providerSecret = verifier === "stripe" || verifier === "github" || verifier === "slack";

  async function save() {
    setErr(undefined);
    try {
      const e = await create.mutateAsync({
        workspace_id: workspaceId,
        environment_id: env,
        name,
        description: description || undefined,
        verifier,
        secret: secret || undefined,
        verifier_config:
          verifier === "hmac_sha256"
            ? {
                signature_header: sigHeader || undefined,
                encoding,
                prefix: prefix || undefined,
                timestamp_header: tsHeader || undefined,
                id_header: idHeader || undefined,
              }
            : undefined,
      });
      onCreated(e);
    } catch (e) {
      setErr(e);
    }
  }

  const formError = err && !(err instanceof ApiError && err.fieldErrors?.length) ? errorMessage(err) : undefined;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen} className="sm:max-w-xl">
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            save();
          }}
        >
          <DialogHeader>
            <DialogTitle>New inbound endpoint</DialogTitle>
            <DialogDescription>
              Workflows deployed to the same environment can be triggered by it.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ep-name">Name</Label>
            <Input id="ep-name" value={name} maxLength={100} placeholder="e.g. Stripe payments" onChange={(e) => setName(e.target.value)}
              aria-invalid={fieldError(err, "name") ? true : undefined} />
            {fieldError(err, "name") && <p role="alert" className="text-sm text-destructive">{fieldError(err, "name")}</p>}
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ep-env">Environment</Label>
              <Select value={env} onValueChange={setEnvironmentId}>
                <SelectTrigger id="ep-env" className="w-full">
                  <SelectValue placeholder="Choose…" />
                </SelectTrigger>
                <SelectContent>
                  {(envs.data ?? []).map((x) => (
                    <SelectItem key={x.id} value={x.id}>
                      {x.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ep-verifier">Signature scheme</Label>
              <Select value={verifier} onValueChange={(v) => setVerifier(v as WebhookEndpoint["verifier"])}>
                <SelectTrigger id="ep-verifier" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VERIFIERS.map((v) => (
                    <SelectItem key={v.value} value={v.value}>
                      {v.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <p className="-mt-2 text-xs text-muted-foreground">{VERIFIERS.find((v) => v.value === verifier)?.hint}</p>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ep-secret">
              Signing secret {providerSecret ? <span className="text-destructive">*</span> : "(optional)"}
            </Label>
            <Input id="ep-secret" type="password" autoComplete="off" value={secret} onChange={(e) => setSecret(e.target.value)}
              placeholder={providerSecret ? "Paste the provider's secret" : "Leave empty to generate one"}
              aria-invalid={fieldError(err, "secret") ? true : undefined} />
            {fieldError(err, "secret") && <p role="alert" className="text-sm text-destructive">{fieldError(err, "secret")}</p>}
          </div>
          {verifier === "hmac_sha256" && (
            <fieldset className="grid gap-3 rounded-md border p-3 sm:grid-cols-2">
              <legend className="px-1 text-xs font-medium">HMAC options</legend>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ep-sig">Signature header</Label>
                <Input id="ep-sig" value={sigHeader} onChange={(e) => setSigHeader(e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ep-enc">Encoding</Label>
                <Select value={encoding} onValueChange={(v) => setEncoding(v as "hex" | "base64")}>
                  <SelectTrigger id="ep-enc" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="hex">hex</SelectItem>
                    <SelectItem value="base64">base64</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ep-prefix">Value prefix</Label>
                <Input id="ep-prefix" value={prefix} placeholder="e.g. sha256=" onChange={(e) => setPrefix(e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ep-ts">Timestamp header</Label>
                <Input id="ep-ts" value={tsHeader} placeholder="Optional; signs “timestamp.body”" onChange={(e) => setTsHeader(e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5 sm:col-span-2">
                <Label htmlFor="ep-id">Event id header</Label>
                <Input id="ep-id" value={idHeader} placeholder="Optional; used to drop duplicates" onChange={(e) => setIdHeader(e.target.value)} />
              </div>
            </fieldset>
          )}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ep-desc">Description</Label>
            <Textarea id="ep-desc" rows={2} maxLength={500} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          {err instanceof ApiError && err.fieldErrors?.some((f) => f.field !== "name" && f.field !== "secret") && (
            <Alert variant="destructive">
              <AlertDescription>
                {err.fieldErrors.filter((f) => f.field !== "name" && f.field !== "secret").map((f) => f.message).join(" ")}
              </AlertDescription>
            </Alert>
          )}
          {formError && (
            <Alert variant="destructive">
              <AlertDescription>{formError}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim() || !env || (providerSecret && !secret) || create.isPending}>
              {create.isPending ? "Creating…" : "Create endpoint"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function EndpointCreatedDialog({ endpoint, title, onClose }: { endpoint: WebhookEndpoint; title: string; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>Configure the sender with this URL{endpoint.secret ? " and secret" : ""}.</DialogDescription>
        </DialogHeader>
        <CopyField id="created-url" label="Endpoint URL" value={ingressURL(endpoint)} />
        {endpoint.secret && (
          <Alert>
            <AlertDescription className="flex flex-col gap-3">
              <span>Copy the signing secret now — it will not be shown again.</span>
              <CopyField id="created-secret" label="Signing secret" value={endpoint.secret} />
            </AlertDescription>
          </Alert>
        )}
        <DialogFooter>
          <Button onClick={onClose}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RotateEndpointDialog({
  endpoint,
  workspaceId,
  onClose,
  onRotated,
}: {
  endpoint: WebhookEndpoint;
  workspaceId: string;
  onClose: () => void;
  onRotated: (e: WebhookEndpoint) => void;
}) {
  const { rotate } = useWebhookEndpointMutations(workspaceId);
  const [secret, setSecret] = useState("");
  const [error, setError] = useState<string>();
  const provider = endpoint.verifier === "stripe" || endpoint.verifier === "github" || endpoint.verifier === "slack";
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen}>
        <form
          className="flex flex-col gap-4"
          onSubmit={async (ev) => {
            ev.preventDefault();
            setError(undefined);
            try {
              onRotated(await rotate.mutateAsync({ id: endpoint.id, secret }));
            } catch (err) {
              setError(errorMessage(err));
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>Rotate the secret of “{endpoint.name}”?</DialogTitle>
            <DialogDescription>
              The current secret stops verifying immediately, so update the sender right after.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="rot-secret">New secret {provider ? <span className="text-destructive">*</span> : "(optional)"}</Label>
            <Input id="rot-secret" type="password" autoComplete="off" value={secret} onChange={(e) => setSecret(e.target.value)}
              placeholder={provider ? "Paste the provider's new secret" : "Leave empty to generate one"} />
          </div>
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant="destructive" disabled={(provider && !secret) || rotate.isPending}>
              {rotate.isPending ? "Rotating…" : "Rotate secret"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function EventsDialog({ endpoint, onClose }: { endpoint: WebhookEndpoint; onClose: () => void }) {
  const q = useWebhookEvents(endpoint.id);
  const [selected, setSelected] = useState("");
  const detail = useWebhookEvent(selected);
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>Events received by {endpoint.name}</DialogTitle>
          <DialogDescription>Verified requests only, newest first. Signature headers are stored redacted.</DialogDescription>
        </DialogHeader>
        <div className="grid min-h-64 gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
          <div className="max-h-[60vh] overflow-y-auto rounded-md border">
            {q.isPending ? (
              <Skeleton className="m-3 h-20" />
            ) : q.error ? (
              <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load events" />
            ) : q.data.length === 0 ? (
              <p className="p-4 text-sm text-muted-foreground">Nothing received yet.</p>
            ) : (
              <ul className="divide-y">
                {q.data.map((ev) => (
                  <li key={ev.id}>
                    <button
                      type="button"
                      onClick={() => setSelected(ev.id)}
                      className={`flex w-full flex-col items-start gap-0.5 px-3 py-2 text-left text-sm hover:bg-muted ${selected === ev.id ? "bg-muted" : ""}`}
                    >
                      <span className="font-medium">{ev.event_type || "(no type)"}</span>
                      <span className="text-xs text-muted-foreground">
                        {new Date(ev.received_at).toLocaleString()} · {ev.provider_event_id ?? "no event id"} · {ev.payload_size} B
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="max-h-[60vh] overflow-y-auto rounded-md border p-3">
            {!selected ? (
              <p className="text-sm text-muted-foreground">Select an event to see its headers and body.</p>
            ) : detail.isPending ? (
              <Skeleton className="h-40" />
            ) : detail.error ? (
              <QueryError error={detail.error} onRetry={() => detail.refetch()} title="Could not load the event" />
            ) : (
              <div className="flex flex-col gap-3 text-xs">
                <p className="text-muted-foreground">
                  From {detail.data.source_ip || "unknown"} · {detail.data.content_type || "no content type"}
                </p>
                <div>
                  <p className="mb-1 font-medium">Headers</p>
                  <pre className="overflow-x-auto rounded bg-muted p-2 font-mono">
                    {Object.entries(detail.data.headers).map(([k, v]) => `${k}: ${v}`).join("\n")}
                  </pre>
                </div>
                <div>
                  <p className="mb-1 font-medium">Body</p>
                  <pre className="overflow-x-auto rounded bg-muted p-2 font-mono">
                    {detail.data.payload !== undefined
                      ? JSON.stringify(detail.data.payload, null, 2)
                      : (detail.data.raw_body ?? "(binary body not shown)")}
                  </pre>
                </div>
              </div>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---- outbound ----

function SubscriptionsCard({ workspaceId }: { workspaceId: string }) {
  const can = useCan();
  const q = useWebhookSubscriptions(workspaceId);
  const [creating, setCreating] = useState(false);
  const [secret, setSecret] = useState<{ name: string; secret: string }>();
  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle>Outbound subscriptions</CardTitle>
          <CardDescription>
            Platform events of this workspace are POSTed to your URL with <code className="font-mono text-xs">webhook-id</code>,{" "}
            <code className="font-mono text-xs">webhook-timestamp</code> and <code className="font-mono text-xs">webhook-signature</code>{" "}
            headers. Failed deliveries are retried after 30s, 2m, 10m, 30m, 2h, 6h and 12h, then dead-lettered; a
            subscription failing for 5 days is disabled. Destinations go through the environment&apos;s egress policy.
          </CardDescription>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link href="/automation/webhook-deliveries">
              <ListChecks aria-hidden /> Delivery log
            </Link>
          </Button>
          {can("webhook:create", workspaceId) && (
            <Button onClick={() => setCreating(true)}>
              <Plus aria-hidden /> New subscription
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {q.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : q.error ? (
          <QueryError error={q.error} onRetry={() => q.refetch()} title="Could not load subscriptions" />
        ) : q.data.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <Webhook className="size-8 text-muted-foreground" aria-hidden />
            <p className="text-sm text-muted-foreground">No outbound subscriptions in this workspace yet.</p>
          </div>
        ) : (
          <ul className="divide-y">
            {q.data.map((s) => (
              <SubscriptionRow key={s.id} sub={s} workspaceId={workspaceId} onSecret={(x) => setSecret({ name: s.name, secret: x })} />
            ))}
          </ul>
        )}
      </CardContent>
      {creating && (
        <CreateSubscriptionDialog
          workspaceId={workspaceId}
          onClose={() => setCreating(false)}
          onCreated={(s) => {
            setCreating(false);
            if (s.secret) setSecret({ name: s.name, secret: s.secret });
          }}
        />
      )}
      {secret && (
        <Dialog open onOpenChange={(o) => !o && setSecret(undefined)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Signing secret for “{secret.name}”</DialogTitle>
              <DialogDescription>
                Verify each delivery with any Standard Webhooks library and this secret. It will not be shown again.
              </DialogDescription>
            </DialogHeader>
            <CopyField id="sub-secret" label="Signing secret" value={secret.secret} />
            <DialogFooter>
              <Button onClick={() => setSecret(undefined)}>Done</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </Card>
  );
}

function SubscriptionRow({ sub: s, workspaceId, onSecret }: { sub: WebhookSubscription; workspaceId: string; onSecret: (s: string) => void }) {
  const can = useCan();
  const envs = useEnvironments();
  const { update, ping, rotate, remove } = useWebhookSubscriptionMutations(workspaceId);
  const envName = envs.data?.find((x) => x.id === s.environment_id)?.name ?? "…";
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 py-4">
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">
          {s.name} <Badge variant="secondary">{envName}</Badge>
          {!s.enabled && (
            <Badge variant="destructive" className="ml-1">
              {s.disabled_reason === "failing_for_5_days" ? "Disabled: failing for 5 days" : "Disabled"}
            </Badge>
          )}
          {s.enabled && s.failing_since && (
            <Badge variant="outline" className="ml-1">
              Failing since {new Date(s.failing_since).toLocaleString()}
            </Badge>
          )}
        </p>
        <p className="mt-1 break-all font-mono text-xs text-muted-foreground">{s.url}</p>
        <div className="mt-1 flex flex-wrap gap-1">
          {s.event_types.map((t) => (
            <Badge key={t} variant="outline" className="font-mono text-[11px]">
              {t}
            </Badge>
          ))}
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          Last success {when(s.last_success_at)} · secret {s.secret_hint}
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {can("webhook:update", workspaceId) && (
          <label className="flex items-center gap-2 text-sm">
            <Switch
              checked={s.enabled}
              disabled={update.isPending}
              aria-label={`${s.name} enabled`}
              onCheckedChange={async (on) => {
                try {
                  await update.mutateAsync({ id: s.id, body: { enabled: on } });
                  toast.success(on ? `${s.name} enabled.` : `${s.name} disabled; new events are not delivered.`);
                } catch (err) {
                  toastError(err);
                }
              }}
            />
            Enabled
          </label>
        )}
        {can("webhook:update", workspaceId) && (
          <Button
            variant="outline"
            size="sm"
            disabled={!s.enabled || ping.isPending}
            onClick={async () => {
              try {
                await ping.mutateAsync(s.id);
                toast.success("Test delivery queued. Follow it in the delivery log.");
              } catch (err) {
                toastError(err);
              }
            }}
          >
            <Send aria-hidden /> Send test
          </Button>
        )}
        <Button variant="outline" size="sm" asChild>
          <Link href={`/automation/webhook-deliveries?subscription=${s.id}`}>Deliveries</Link>
        </Button>
        {can("webhook:update", workspaceId) && (
          <ConfirmDialog
            trigger={
              <Button variant="outline" size="sm">
                <KeyRound aria-hidden /> Rotate secret
              </Button>
            }
            title={`Rotate the secret of “${s.name}”?`}
            description="Deliveries from now on, including pending retries, are signed with the new secret. Update the receiver right after."
            confirmLabel="Rotate secret"
            onConfirm={async () => {
              try {
                const next = await rotate.mutateAsync(s.id);
                if (next.secret) onSecret(next.secret);
              } catch (err) {
                toastError(err);
                throw err;
              }
            }}
          />
        )}
        {can("webhook:delete", workspaceId) && (
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Delete ${s.name}`}>
                <Trash2 />
              </Button>
            }
            title={`Delete “${s.name}”?`}
            description="Pending deliveries are dropped and the delivery log is deleted. This cannot be undone."
            confirmLabel="Delete subscription"
            onConfirm={async () => {
              try {
                await remove.mutateAsync(s.id);
                toast.success("Subscription deleted.");
              } catch (err) {
                toastError(err);
                throw err;
              }
            }}
          />
        )}
      </div>
    </li>
  );
}

function CreateSubscriptionDialog({
  workspaceId,
  onClose,
  onCreated,
}: {
  workspaceId: string;
  onClose: () => void;
  onCreated: (s: WebhookSubscription) => void;
}) {
  const envs = useEnvironments();
  const types = useEventTypes();
  const { create } = useWebhookSubscriptionMutations(workspaceId);
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [environmentId, setEnvironmentId] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [err, setErr] = useState<unknown>();
  const env = environmentId || envs.data?.[0]?.id || "";
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent onInteractOutside={keepOpen} className="sm:max-w-xl">
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            setErr(undefined);
            try {
              onCreated(await create.mutateAsync({ workspace_id: workspaceId, environment_id: env, name, url, event_types: selected }));
            } catch (e2) {
              setErr(e2);
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>New outbound subscription</DialogTitle>
            <DialogDescription>Only events of this workspace are delivered.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sub-name">Name</Label>
            <Input id="sub-name" value={name} maxLength={100} placeholder="e.g. Data warehouse" onChange={(e) => setName(e.target.value)} />
            {fieldError(err, "name") && <p role="alert" className="text-sm text-destructive">{fieldError(err, "name")}</p>}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sub-url">URL</Label>
            <Input id="sub-url" type="url" value={url} placeholder="https://example.com/hugimethara/events" onChange={(e) => setUrl(e.target.value)}
              aria-invalid={fieldError(err, "url") ? true : undefined} />
            {fieldError(err, "url") && <p role="alert" className="text-sm text-destructive">{fieldError(err, "url")}</p>}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sub-env">Egress environment</Label>
            <Select value={env} onValueChange={setEnvironmentId}>
              <SelectTrigger id="sub-env" className="w-full">
                <SelectValue placeholder="Choose…" />
              </SelectTrigger>
              <SelectContent>
                {(envs.data ?? []).map((x) => (
                  <SelectItem key={x.id} value={x.id}>
                    {x.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">Its egress allowlist decides whether private destinations are reachable.</p>
          </div>
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1 text-sm font-medium">Events</legend>
            {types.isPending && <Skeleton className="h-20" />}
            {(types.data ?? []).map((t) => (
              <div key={t.type} className="flex items-start gap-2 text-sm">
                <Checkbox
                  id={`evt-${t.type}`}
                  aria-describedby={`evt-${t.type}-desc`}
                  checked={selected.includes(t.type)}
                  onCheckedChange={(on) => setSelected((cur) => (on ? [...cur, t.type] : cur.filter((x) => x !== t.type)))}
                  className="mt-0.5"
                />
                <span>
                  <Label htmlFor={`evt-${t.type}`} className="font-mono text-xs">
                    {t.type}
                  </Label>
                  <span id={`evt-${t.type}-desc`} className="block text-xs text-muted-foreground">
                    {t.description}
                  </span>
                </span>
              </div>
            ))}
            {fieldError(err, "event_types") && <p role="alert" className="text-sm text-destructive">{fieldError(err, "event_types")}</p>}
          </fieldset>
          {err !== undefined && !(err instanceof ApiError && err.fieldErrors?.length) && (
            <Alert variant="destructive">
              <AlertDescription>{errorMessage(err)}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim() || !url.trim() || !env || selected.length === 0 || create.isPending}>
              {create.isPending ? "Creating…" : "Create subscription"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
