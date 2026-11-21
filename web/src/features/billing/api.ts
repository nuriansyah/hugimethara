import { useQuery } from "@tanstack/react-query";
import { api, ApiError, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";

export type Entitlements = components["schemas"]["Entitlements"];
export type EntitlementFeature = components["schemas"]["EntitlementFeature"];
export type Plan = components["schemas"]["Plan"];
export type FeatureDefinition = components["schemas"]["FeatureDefinition"];

export const entitlementsKey = ["entitlements"] as const;

export function useEntitlements() {
  return useQuery({
    queryKey: entitlementsKey,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/entitlements")),
    staleTime: 60_000,
  });
}

export function usePlans() {
  return useQuery({
    queryKey: ["plans"],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/plans")),
    staleTime: 5 * 60_000,
  });
}

/** Whether a flag feature is enabled; undefined while loading. */
export function useFeature(key: string): boolean | undefined {
  const { data } = useEntitlements();
  return data?.features.find((f) => f.key === key)?.enabled;
}

/** A plan-limit / paid-feature error from the API (HTTP 402), if err is one. */
export function planError(err: unknown): { code: string; feature?: string; message: string } | undefined {
  if (err instanceof ApiError && err.status === 402) {
    return { code: err.code, feature: err.body?.feature as string | undefined, message: err.message };
  }
  return undefined;
}
