import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@hugimethara/api-client";
import type { Schema } from "@hugimethara/shared-types";

export type WebhookEndpoint = Schema<"WebhookEndpoint">;
export type WebhookEndpointCreate = Schema<"WebhookEndpointCreate">;
export type WebhookEndpointUpdate = Schema<"WebhookEndpointUpdate">;
export type WebhookEvent = Schema<"WebhookEvent">;
export type WebhookSubscription = Schema<"WebhookSubscription">;
export type WebhookSubscriptionCreate = Schema<"WebhookSubscriptionCreate">;
export type WebhookSubscriptionUpdate = Schema<"WebhookSubscriptionUpdate">;
export type WebhookDelivery = Schema<"WebhookDelivery">;
export type EventType = Schema<"EventType">;

export const VERIFIERS: { value: WebhookEndpoint["verifier"]; label: string; hint: string }[] = [
  { value: "standard_webhooks", label: "Standard Webhooks", hint: "webhook-id / webhook-timestamp / webhook-signature; we generate the secret." },
  { value: "hmac_sha256", label: "Generic HMAC-SHA256", hint: "Signature header of your choice over the raw body; we generate the secret." },
  { value: "stripe", label: "Stripe", hint: "Stripe-Signature; paste the endpoint's signing secret from Stripe." },
  { value: "github", label: "GitHub", hint: "X-Hub-Signature-256; paste the webhook secret you set in GitHub." },
  { value: "slack", label: "Slack", hint: "X-Slack-Signature; paste the app's signing secret." },
];

export function verifierLabel(v: string) {
  return VERIFIERS.find((x) => x.value === v)?.label ?? v;
}

/** The URL senders POST to: through the web app's /api proxy (same origin as the UI). */
export function ingressURL(e: Pick<WebhookEndpoint, "ingress_path">) {
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  return `${origin}/api${e.ingress_path}`;
}

const keys = {
  endpoints: (ws: string) => ["webhooks", ws] as const,
  events: (endpointId: string) => ["webhook-events", endpointId] as const,
  subscriptions: (ws: string) => ["webhook-subscriptions", ws] as const,
  deliveries: ["webhook-deliveries"] as const,
};

export function useEventTypes() {
  return useQuery({
    queryKey: ["event-types"],
    staleTime: Infinity,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/event-types")).data,
  });
}

export function useWebhookEndpoints(workspaceId: string) {
  return useQuery({
    queryKey: keys.endpoints(workspaceId),
    enabled: Boolean(workspaceId),
    refetchInterval: 15_000,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/webhooks", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useWebhookEvents(endpointId: string) {
  return useQuery({
    queryKey: keys.events(endpointId),
    enabled: Boolean(endpointId),
    refetchInterval: 10_000,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/webhook-events", { params: { query: { endpoint_id: endpointId, limit: 100 } } }))
        .data,
  });
}

export function useWebhookEvent(id: string) {
  return useQuery({
    queryKey: ["webhook-event", id],
    enabled: Boolean(id),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/webhook-events/{event_id}", { params: { path: { event_id: id } } })),
  });
}

export function useWebhookEndpointMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: keys.endpoints(workspaceId) });
  return {
    create: useMutation({
      mutationFn: async (body: WebhookEndpointCreate) => unwrap(await api.POST("/v1/hugimethara/webhooks", { body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: WebhookEndpointUpdate }) =>
        unwrap(await api.PATCH("/v1/hugimethara/webhooks/{webhook_id}", { params: { path: { webhook_id: id } }, body })),
      onSuccess: refresh,
    }),
    rotate: useMutation({
      mutationFn: async ({ id, secret }: { id: string; secret?: string }) =>
        unwrap(
          await api.POST("/v1/hugimethara/webhooks/{webhook_id}/rotate-secret", {
            params: { path: { webhook_id: id } },
            body: { secret: secret || undefined },
          }),
        ),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/webhooks/{webhook_id}", { params: { path: { webhook_id: id } } })),
      onSuccess: refresh,
    }),
  };
}

export function useWebhookSubscriptions(workspaceId: string) {
  return useQuery({
    queryKey: keys.subscriptions(workspaceId),
    enabled: Boolean(workspaceId),
    refetchInterval: 15_000,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/webhook-subscriptions", { params: { query: { workspace_id: workspaceId } } }))
        .data,
  });
}

export function useWebhookSubscriptionMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = () => {
    qc.invalidateQueries({ queryKey: keys.subscriptions(workspaceId) });
    qc.invalidateQueries({ queryKey: keys.deliveries });
  };
  const path = (id: string) => ({ params: { path: { subscription_id: id } } });
  return {
    create: useMutation({
      mutationFn: async (body: WebhookSubscriptionCreate) =>
        unwrap(await api.POST("/v1/hugimethara/webhook-subscriptions", { body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: WebhookSubscriptionUpdate }) =>
        unwrap(await api.PATCH("/v1/hugimethara/webhook-subscriptions/{subscription_id}", { ...path(id), body })),
      onSuccess: refresh,
    }),
    rotate: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/webhook-subscriptions/{subscription_id}/rotate-secret", path(id))),
      onSuccess: refresh,
    }),
    ping: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/webhook-subscriptions/{subscription_id}/ping", path(id))),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/webhook-subscriptions/{subscription_id}", path(id))),
      onSuccess: refresh,
    }),
  };
}

export function useWebhookDeliveries(filter: { workspaceId: string; subscriptionId?: string; status?: string }) {
  return useQuery({
    queryKey: [...keys.deliveries, filter.workspaceId, filter.subscriptionId ?? "", filter.status ?? ""],
    enabled: Boolean(filter.workspaceId),
    refetchInterval: (q) => (q.state.data?.some((d) => d.status === "pending") ? 3_000 : 15_000),
    queryFn: async () =>
      unwrap(
        await api.GET("/v1/hugimethara/webhook-deliveries", {
          params: {
            query: {
              workspace_id: filter.subscriptionId ? undefined : filter.workspaceId,
              subscription_id: filter.subscriptionId || undefined,
              status: filter.status || undefined,
              limit: 200,
            },
          },
        }),
      ).data,
  });
}

export function useWebhookDelivery(id: string) {
  return useQuery({
    queryKey: [...keys.deliveries, "one", id],
    enabled: Boolean(id),
    refetchInterval: (q) => (q.state.data?.status === "pending" ? 2_000 : false),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/webhook-deliveries/{delivery_id}", { params: { path: { delivery_id: id } } })),
  });
}

export function useReplayDelivery() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.POST("/v1/hugimethara/webhook-deliveries/{delivery_id}/replay", { params: { path: { delivery_id: id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.deliveries }),
  });
}
