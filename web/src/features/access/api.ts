import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { api, ensureOk, unwrap } from "@hugimethara/api-client";
import type { components, paths } from "@hugimethara/shared-types";

export type Role = components["schemas"]["Role"];
export type RoleBinding = components["schemas"]["RoleBinding"];
export type ResourceType = components["schemas"]["ResourceType"];
export type AccessDecision = components["schemas"]["AccessDecision"];
export type LoggedDecision = components["schemas"]["LoggedDecision"];

export const permissionsKey = ["me", "permissions"] as const;

export function useMyPermissions() {
  return useQuery({
    queryKey: permissionsKey,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/me/permissions")),
    staleTime: 30_000,
  });
}

/**
 * can("workspace:update", workspaceId) — true when the caller holds the
 * permission organization-wide or in that workspace. UX only: the API
 * authorizes every request. Returns false while loading.
 */
export function useCan() {
  const { data } = useMyPermissions();
  return useCallback(
    (permission: string, workspaceId?: string) =>
      !!data &&
      (data.organization.includes(permission) ||
        (!!workspaceId && (data.workspaces[workspaceId] ?? []).includes(permission))),
    [data],
  );
}

function useInvalidateAccess() {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: ["roles"] });
    qc.invalidateQueries({ queryKey: ["role-bindings"] });
    qc.invalidateQueries({ queryKey: permissionsKey });
    qc.invalidateQueries({ queryKey: ["users"] });
    qc.invalidateQueries({ queryKey: ["me"] });
  };
}

export function useResourceTypes() {
  return useQuery({
    queryKey: ["resource-types"],
    staleTime: 10 * 60_000,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/resource-types")).data,
  });
}

export function useRoles(enabled = true) {
  return useQuery({
    queryKey: ["roles"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/roles")).data,
  });
}

export function useRoleMutations() {
  const invalidate = useInvalidateAccess();
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["RoleCreate"]) => unwrap(await api.POST("/v1/hugimethara/roles", { body })),
      onSuccess: invalidate,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["RoleUpdate"] }) =>
        unwrap(await api.PUT("/v1/hugimethara/roles/{role_id}", { params: { path: { role_id: id } }, body })),
      onSuccess: invalidate,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/roles/{role_id}", { params: { path: { role_id: id } } })),
      onSuccess: invalidate,
    }),
  };
}

export function useRoleBindings(query: NonNullable<paths["/v1/hugimethara/role-bindings"]["get"]["parameters"]["query"]> = {}) {
  return useQuery({
    queryKey: ["role-bindings", query],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/role-bindings", { params: { query } })).data,
  });
}

export function useBindingMutations() {
  const invalidate = useInvalidateAccess();
  return {
    assign: useMutation({
      mutationFn: async (body: components["schemas"]["RoleBindingCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/role-bindings", { body })),
      onSuccess: invalidate,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/role-bindings/{binding_id}", { params: { path: { binding_id: id } } })),
      onSuccess: invalidate,
    }),
  };
}

export function useEvaluate() {
  return useMutation({
    mutationFn: async (body: components["schemas"]["EvaluateRequest"]) =>
      unwrap(await api.POST("/v1/hugimethara/authorization/evaluate", { body })),
  });
}

export function useDecisions(allowed?: boolean) {
  return useQuery({
    queryKey: ["decisions", allowed],
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/authorization/decisions", { params: { query: { allowed, limit: 50 } } })).data,
  });
}
