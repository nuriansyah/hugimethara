import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";

export type Credential = components["schemas"]["Credential"];
export type CredentialCreate = components["schemas"]["CredentialCreate"];
export type CredentialUpdate = components["schemas"]["CredentialUpdate"];

export function useCredentials(enabled = true) {
  return useQuery({
    queryKey: ["credentials"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/credentials")).data,
  });
}

export function useCredentialMutations() {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["credentials"] });
  return {
    create: useMutation({
      mutationFn: async (body: CredentialCreate) => unwrap(await api.POST("/v1/hugimethara/credentials", { body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: CredentialUpdate }) =>
        unwrap(await api.PATCH("/v1/hugimethara/credentials/{credential_id}", {
          params: { path: { credential_id: id } },
          body,
        })),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/credentials/{credential_id}", {
          params: { path: { credential_id: id } },
        })),
      onSuccess: refresh,
    }),
  };
}
