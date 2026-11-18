import { useInfiniteQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type AuditEvent = components["schemas"]["AuditEvent"];
export type AuditFilter = { action?: string; since?: string; until?: string };

export function useAuditEvents(filter: AuditFilter) {
  return useInfiniteQuery({
    queryKey: ["audit-events", filter],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) =>
      unwrap(
        await api.GET("/v1/hugimethara/audit-events", {
          params: {
            query: {
              action: filter.action || undefined,
              since: filter.since || undefined,
              until: filter.until || undefined,
              limit: 50,
              cursor: pageParam,
            },
          },
        }),
      ),
    getNextPageParam: (last) => last.next_cursor,
  });
}
