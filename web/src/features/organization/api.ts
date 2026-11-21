import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";
import { meKey } from "@/features/auth/api";

export type Organization = components["schemas"]["Organization"];
export type UpdateOrganizationRequest = components["schemas"]["UpdateOrganizationRequest"];

const orgKey = ["organization", "current"] as const;

export function useCurrentOrganization() {
  return useQuery({
    queryKey: orgKey,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/organizations/current")),
  });
}

export function useUpdateOrganization() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: UpdateOrganizationRequest) =>
      unwrap(await api.PATCH("/v1/hugimethara/organizations/current", { body })),
    onSuccess: (org) => {
      qc.setQueryData(orgKey, org);
      qc.invalidateQueries({ queryKey: meKey });
    },
  });
}
