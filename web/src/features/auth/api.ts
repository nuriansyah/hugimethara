import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, ensureOk, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";

export type Me = components["schemas"]["Me"];
export type SessionInfo = components["schemas"]["SessionInfo"];
export type LoginRequest = components["schemas"]["LoginRequest"];
export type SignupRequest = components["schemas"]["SignupRequest"];
export type OrganizationChoice = components["schemas"]["OrganizationChoice"];

export const meKey = ["me"] as const;

export function useMe() {
  return useQuery({
    queryKey: meKey,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/me")),
    retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
    staleTime: 60_000,
  });
}

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: LoginRequest) => unwrap(await api.POST("/v1/hugimethara/auth/login", { body })),
    onSuccess: () => qc.clear(),
  });
}

export function useSignup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: SignupRequest) => unwrap(await api.POST("/v1/hugimethara/organizations", { body })),
    onSuccess: () => qc.clear(),
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => ensureOk(await api.POST("/v1/hugimethara/auth/logout")),
    onSuccess: () => qc.clear(),
  });
}

export function useMySessions() {
  return useQuery({
    queryKey: ["me", "sessions"],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/me/sessions")).data,
  });
}

export function useRevokeSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (sessionId: string) =>
      ensureOk(await api.DELETE("/v1/hugimethara/me/sessions/{session_id}", { params: { path: { session_id: sessionId } } })),
    onSettled: () => qc.invalidateQueries({ queryKey: ["me", "sessions"] }),
  });
}

/** Organizations offered when login answers 409 organization_required. */
export function organizationChoices(err: unknown): OrganizationChoice[] | undefined {
  if (err instanceof ApiError && err.code === "organization_required") {
    const orgs = err.body?.organizations;
    if (Array.isArray(orgs)) return orgs as OrganizationChoice[];
  }
  return undefined;
}
