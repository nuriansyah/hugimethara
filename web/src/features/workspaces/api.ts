import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { entitlementsKey } from "@/features/billing/api";

export type Workspace = components["schemas"]["Workspace"];
export type WorkspaceCreate = components["schemas"]["WorkspaceCreate"];
export type WorkspaceUpdate = components["schemas"]["WorkspaceUpdate"];
export type Member = components["schemas"]["Member"];
export type Department = components["schemas"]["Department"];
export type Environment = components["schemas"]["Environment"];

const keys = {
  workspaces: (archived: boolean) => ["workspaces", { archived }] as const,
  workspace: (slug: string) => ["workspace", slug] as const,
  members: (slug: string) => ["workspace", slug, "members"] as const,
  users: ["users"] as const,
  departments: ["departments"] as const,
  environments: ["environments"] as const,
};

// Workspaces ----------------------------------------------------------------

export function useWorkspaces(includeArchived: boolean) {
  return useQuery({
    queryKey: keys.workspaces(includeArchived),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workspaces", { params: { query: { include_archived: includeArchived } } })).data,
  });
}

export function useWorkspace(slug: string) {
  return useQuery({
    queryKey: keys.workspace(slug),
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/workspaces/{workspace}", { params: { path: { workspace: slug } } })),
  });
}

function useInvalidateWorkspaces() {
  const qc = useQueryClient();
  return (slug?: string) => {
    qc.invalidateQueries({ queryKey: ["workspaces"] });
    qc.invalidateQueries({ queryKey: entitlementsKey });
    if (slug) qc.invalidateQueries({ queryKey: ["workspace", slug] });
  };
}

export function useCreateWorkspace() {
  const invalidate = useInvalidateWorkspaces();
  return useMutation({
    mutationFn: async (body: WorkspaceCreate) => unwrap(await api.POST("/v1/hugimethara/workspaces", { body })),
    onSuccess: () => invalidate(),
  });
}

export function useUpdateWorkspace(slug: string) {
  const invalidate = useInvalidateWorkspaces();
  return useMutation({
    mutationFn: async (body: WorkspaceUpdate) =>
      unwrap(await api.PATCH("/v1/hugimethara/workspaces/{workspace}", { params: { path: { workspace: slug } }, body })),
    onSuccess: () => invalidate(slug),
  });
}

export function useSetArchived(slug: string) {
  const invalidate = useInvalidateWorkspaces();
  return useMutation({
    mutationFn: async (archived: boolean) =>
      unwrap(
        archived
          ? await api.POST("/v1/hugimethara/workspaces/{workspace}/archive", { params: { path: { workspace: slug } } })
          : await api.POST("/v1/hugimethara/workspaces/{workspace}/restore", { params: { path: { workspace: slug } } }),
      ),
    onSuccess: () => invalidate(slug),
  });
}

export function useWorkspaceMembers(slug: string) {
  return useQuery({
    queryKey: keys.members(slug),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workspaces/{workspace}/members", { params: { path: { workspace: slug } } })).data,
  });
}

export function useChangeMember(slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, add, role }: { userId: string; add: boolean; role?: string }) =>
      ensureOk(
        add
          ? await api.POST("/v1/hugimethara/workspaces/{workspace}/members", {
              params: { path: { workspace: slug } },
              body: { user_id: userId, role_key: role },
            })
          : await api.DELETE("/v1/hugimethara/workspaces/{workspace}/members/{user_id}", {
              params: { path: { workspace: slug, user_id: userId } },
            }),
      ),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: keys.members(slug) });
      qc.invalidateQueries({ queryKey: ["me", "permissions"] });
      qc.invalidateQueries({ queryKey: ["workspaces"] });
    },
  });
}

export function useOrgUsers(enabled: boolean) {
  return useQuery({
    queryKey: keys.users,
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/users")).data,
  });
}

// Departments ---------------------------------------------------------------

export function useDepartments() {
  return useQuery({
    queryKey: keys.departments,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/departments")).data,
  });
}

export function useDepartmentMutations() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: keys.departments });
    qc.invalidateQueries({ queryKey: ["workspaces"] });
  };
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["DepartmentCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/departments", { body })),
      onSuccess: done,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["DepartmentUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/departments/{department_id}", { params: { path: { department_id: id } }, body })),
      onSuccess: done,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/departments/{department_id}", { params: { path: { department_id: id } } })),
      onSuccess: done,
    }),
  };
}

// Environments --------------------------------------------------------------

export function useEnvironments() {
  return useQuery({
    queryKey: keys.environments,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/environments")).data,
  });
}

export function useEnvironmentMutations() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: keys.environments });
    qc.invalidateQueries({ queryKey: entitlementsKey });
  };
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["EnvironmentCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/environments", { body })),
      onSuccess: done,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["EnvironmentUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/environments/{environment_id}", { params: { path: { environment_id: id } }, body })),
      onSuccess: done,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/environments/{environment_id}", { params: { path: { environment_id: id } } })),
      onSuccess: done,
    }),
  };
}
