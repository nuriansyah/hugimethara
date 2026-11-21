import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";

export type Member = components["schemas"]["Member"];
export type Invitation = components["schemas"]["Invitation"];
export type InvitationCreate = components["schemas"]["InvitationCreate"];
export type InvitationCreated = components["schemas"]["InvitationCreated"];
export type Group = components["schemas"]["Group"];
export type GroupMember = components["schemas"]["GroupMember"];
export type ServiceAccount = components["schemas"]["ServiceAccount"];
export type ApiKey = components["schemas"]["ApiKey"];
export type ApiKeyCreated = components["schemas"]["ApiKeyCreated"];
export type IdentityProvider = components["schemas"]["IdentityProvider"];
export type IdentityProviderCreate = components["schemas"]["IdentityProviderCreate"];
export type IdentityProviderUpdate = components["schemas"]["IdentityProviderUpdate"];
export type IdentityProviderTest = components["schemas"]["IdentityProviderTest"];
export type GroupMapping = components["schemas"]["GroupMapping"];
export type GroupMappingCreate = components["schemas"]["GroupMappingCreate"];

/** Membership, groups and service accounts all feed authorization: refresh it too. */
function useInvalidate() {
  const qc = useQueryClient();
  return (...keys: string[]) => {
    for (const k of keys) qc.invalidateQueries({ queryKey: [k] });
    qc.invalidateQueries({ queryKey: ["role-bindings"] });
    qc.invalidateQueries({ queryKey: ["me", "permissions"] });
  };
}

// Members ---------------------------------------------------------------------

export function useMembers() {
  return useQuery({
    queryKey: ["users"],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/users")).data,
  });
}

export function useMemberMutations() {
  const invalidate = useInvalidate();
  return {
    setStatus: useMutation({
      mutationFn: async ({ id, status }: { id: string; status: "active" | "suspended" }) =>
        unwrap(await api.PATCH("/v1/hugimethara/users/{user_id}", { params: { path: { user_id: id } }, body: { status } })),
      onSuccess: () => invalidate("users"),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/users/{user_id}", { params: { path: { user_id: id } } })),
      onSuccess: () => invalidate("users", "groups"),
    }),
  };
}

// Invitations -------------------------------------------------------------------

export function useInvitations(enabled: boolean) {
  return useQuery({
    queryKey: ["invitations"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/invitations", { params: { query: { status: "pending" } } })).data,
  });
}

export function useInvitationMutations() {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["invitations"] });
  return {
    create: useMutation({
      mutationFn: async (body: InvitationCreate) => unwrap(await api.POST("/v1/hugimethara/invitations", { body })),
      onSuccess: refresh,
    }),
    revoke: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/invitations/{invitation_id}", { params: { path: { invitation_id: id } } })),
      onSuccess: refresh,
    }),
  };
}

/** Public: what an invitation link is for. */
export function useInvitationPreview(token: string) {
  return useQuery({
    queryKey: ["invitation-preview", token],
    retry: false,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/invitation-tokens/{token}", { params: { path: { token } } })),
  });
}

export function useAcceptInvitation(token: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: components["schemas"]["InvitationAccept"]) =>
      unwrap(await api.POST("/v1/hugimethara/invitation-tokens/{token}/accept", { params: { path: { token } }, body })),
    // Now signed in to the invited organization: drop everything cached for the
    // previous session, except this (now used) invitation, which would refetch as 410.
    onSuccess: () => qc.removeQueries({ predicate: (q) => q.queryKey[0] !== "invitation-preview" }),
  });
}

// Groups ------------------------------------------------------------------------

export function useGroups(enabled = true) {
  return useQuery({
    queryKey: ["groups"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/groups")).data,
  });
}

export function useGroupMembers(id: string) {
  return useQuery({
    queryKey: ["groups", id, "members"],
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/groups/{group_id}/members", { params: { path: { group_id: id } } })).data,
  });
}

export function useGroupMutations() {
  const invalidate = useInvalidate();
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["GroupInput"]) => unwrap(await api.POST("/v1/hugimethara/groups", { body })),
      onSuccess: () => invalidate("groups"),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["GroupInput"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/groups/{group_id}", { params: { path: { group_id: id } }, body })),
      onSuccess: () => invalidate("groups"),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/groups/{group_id}", { params: { path: { group_id: id } } })),
      onSuccess: () => invalidate("groups", "users"),
    }),
    addMember: useMutation({
      mutationFn: async ({ id, userId }: { id: string; userId: string }) =>
        ensureOk(await api.POST("/v1/hugimethara/groups/{group_id}/members", { params: { path: { group_id: id } }, body: { user_id: userId } })),
      onSuccess: () => invalidate("groups", "users"),
    }),
    removeMember: useMutation({
      mutationFn: async ({ id, userId }: { id: string; userId: string }) =>
        ensureOk(await api.DELETE("/v1/hugimethara/groups/{group_id}/members/{user_id}", { params: { path: { group_id: id, user_id: userId } } })),
      onSuccess: () => invalidate("groups", "users"),
    }),
  };
}

// Service accounts -------------------------------------------------------------------

export function useServiceAccounts(enabled = true) {
  return useQuery({
    queryKey: ["service-accounts"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/service-accounts")).data,
  });
}

export function useApiKeys(id: string) {
  return useQuery({
    queryKey: ["service-accounts", id, "keys"],
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/service-accounts/{service_account_id}/keys", { params: { path: { service_account_id: id } } })).data,
  });
}

export function useServiceAccountMutations() {
  const invalidate = useInvalidate();
  const path = (id: string) => ({ params: { path: { service_account_id: id } } });
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["ServiceAccountCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/service-accounts", { body })),
      onSuccess: () => invalidate("service-accounts"),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["ServiceAccountUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/service-accounts/{service_account_id}", { ...path(id), body })),
      onSuccess: () => invalidate("service-accounts"),
    }),
    remove: useMutation({
      mutationFn: async (id: string) => ensureOk(await api.DELETE("/v1/hugimethara/service-accounts/{service_account_id}", path(id))),
      onSuccess: () => invalidate("service-accounts"),
    }),
    createKey: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["ApiKeyCreate"] }) =>
        unwrap(await api.POST("/v1/hugimethara/service-accounts/{service_account_id}/keys", { ...path(id), body })),
      onSuccess: () => invalidate("service-accounts"),
    }),
    revokeKey: useMutation({
      mutationFn: async ({ id, keyId }: { id: string; keyId: string }) =>
        ensureOk(await api.DELETE("/v1/hugimethara/service-accounts/{service_account_id}/keys/{key_id}", {
          params: { path: { service_account_id: id, key_id: keyId } },
        })),
      onSuccess: () => invalidate("service-accounts"),
    }),
  };
}

// Identity providers (SSO) ------------------------------------------------------

export function useIdentityProviders(enabled = true) {
  return useQuery({
    queryKey: ["identity-providers"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/identity/providers")).data,
  });
}

export function useGroupMappings(providerId: string, enabled = true) {
  return useQuery({
    queryKey: ["identity-providers", providerId, "group-mappings"],
    enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/identity/providers/{provider_id}/group-mappings", {
        params: { path: { provider_id: providerId } },
      })).data,
  });
}

export function useIdentityProviderMutations() {
  const qc = useQueryClient();
  const invalidate = useInvalidate();
  const refreshMappings = () => qc.invalidateQueries({ queryKey: ["identity-providers"] });
  const path = (id: string) => ({ params: { path: { provider_id: id } } });
  return {
    create: useMutation({
      mutationFn: async (body: IdentityProviderCreate) =>
        unwrap(await api.POST("/v1/hugimethara/identity/providers", { body })),
      onSuccess: () => invalidate("identity-providers"),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: IdentityProviderUpdate }) =>
        unwrap(await api.PATCH("/v1/hugimethara/identity/providers/{provider_id}", { ...path(id), body })),
      onSuccess: () => invalidate("identity-providers"),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/identity/providers/{provider_id}", path(id))),
      onSuccess: () => invalidate("identity-providers", "groups"),
    }),
    test: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/identity/providers/{provider_id}/test", path(id))),
    }),
    createMapping: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: GroupMappingCreate }) =>
        unwrap(await api.POST("/v1/hugimethara/identity/providers/{provider_id}/group-mappings", { ...path(id), body })),
      onSuccess: () => {
        refreshMappings();
        invalidate("identity-providers", "groups");
      },
    }),
    removeMapping: useMutation({
      mutationFn: async ({ id, mappingId }: { id: string; mappingId: string }) =>
        ensureOk(await api.DELETE("/v1/hugimethara/identity/providers/{provider_id}/group-mappings/{mapping_id}", {
          params: { path: { provider_id: id, mapping_id: mappingId } },
        })),
      onSuccess: () => {
        refreshMappings();
        invalidate("identity-providers", "groups");
      },
    }),
  };
}
