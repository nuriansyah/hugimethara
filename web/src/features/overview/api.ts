import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@hugimethara/api-client";
import type { Schema } from "@hugimethara/shared-types";

export type Overview = Schema<"Overview">;
export type OverviewWindow = "24h" | "7d" | "30d";

export function useOverview(workspaceId: string, window: OverviewWindow) {
  return useQuery({
    queryKey: ["overview", workspaceId, window],
    enabled: !!workspaceId,
    refetchInterval: 60_000,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/overview", { params: { query: { workspace_id: workspaceId, window } } })),
  });
}
