import { useQuery } from "@tanstack/react-query";
import { api, ApiError, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";

export type HealthReport = components["schemas"]["HealthReport"];

export function useSystemInfo() {
  return useQuery({
    queryKey: ["system", "info"],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/system/info")),
  });
}

/**
 * Readiness returns 503 with a report body when a dependency is down; that is
 * data to display, not a transport failure, so it is returned rather than thrown.
 */
export function useReadiness() {
  return useQuery({
    queryKey: ["system", "readiness"],
    refetchInterval: 15_000,
    queryFn: async (): Promise<HealthReport> => {
      const res = await api.GET("/readyz");
      if (res.data) return res.data;
      if (res.response.status === 503 && res.error && "status" in res.error) {
        return res.error as HealthReport;
      }
      unwrap(res); // throws ApiError for any other failure
      throw new ApiError(res.response.status, undefined, "Readiness returned no body.");
    },
    retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 1,
  });
}
