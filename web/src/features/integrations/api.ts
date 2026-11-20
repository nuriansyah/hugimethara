import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type ConnectorDefinition = components["schemas"]["ConnectorDefinition"];
export type ConnectorVersion = components["schemas"]["ConnectorVersion"];
export type ConnectorInstance = components["schemas"]["ConnectorInstance"];
export type ConnectorTestResult = components["schemas"]["ConnectorTestResult"];
export type Service = components["schemas"]["Service"];
export type ServiceEndpoint = components["schemas"]["ServiceEndpoint"];
export type OpenAPIImportRequest = components["schemas"]["OpenAPIImportRequest"];
export type OpenAPIImportResult = components["schemas"]["OpenAPIImportResult"];
export type ServiceTryRequest = components["schemas"]["ServiceTryRequest"];
export type ServiceTryResult = components["schemas"]["ServiceTryResult"];

export function useConnectorDefinitions() {
  return useQuery({
    queryKey: ["connector-definitions"],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/connector-definitions")).data,
  });
}

export function useConnectorVersions(id: string) {
  return useQuery({
    queryKey: ["connector-definitions", id, "versions"],
    enabled: Boolean(id),
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/connector-definitions/{definition_id}/versions", {
      params: { path: { definition_id: id } },
    })).data,
  });
}

export function useConnectors() {
  return useQuery({
    queryKey: ["connectors"],
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/connectors")).data,
  });
}

export function useConnectorHealth(id: string, enabled: boolean) {
  return useQuery({
    queryKey: ["connectors", id, "health"],
    enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/connectors/{connector_id}/health", {
      params: { path: { connector_id: id } },
    })).data,
  });
}

export function useConnectorMutations() {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["connectors"] });
  const path = (id: string) => ({ params: { path: { connector_id: id } } });
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["ConnectorInstanceCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/connectors", { body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["ConnectorInstanceUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/connectors/{connector_id}", { ...path(id), body })),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/connectors/{connector_id}", path(id))),
      onSuccess: refresh,
    }),
    test: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/connectors/{connector_id}/test", path(id))),
      onSuccess: (_result, id) => qc.invalidateQueries({ queryKey: ["connectors", id, "health"] }),
    }),
  };
}

export function useServices(workspaceId: string) {
  return useQuery({
    queryKey: ["services", workspaceId],
    enabled: Boolean(workspaceId),
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/services", {
      params: { query: { workspace_id: workspaceId } },
    })).data,
  });
}

export function useServiceEndpoints(id: string, enabled = true) {
  return useQuery({
    queryKey: ["services", id, "endpoints"],
    enabled: Boolean(id) && enabled,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/services/{service_id}/endpoints", {
      params: { path: { service_id: id } },
    })).data,
  });
}

export function useServiceMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = (id?: string) => {
    qc.invalidateQueries({ queryKey: ["services", workspaceId] });
    if (id) qc.invalidateQueries({ queryKey: ["services", id] });
  };
  const path = (id: string) => ({ params: { path: { service_id: id } } });
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["ServiceCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/services", { body })),
      onSuccess: () => refresh(),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["ServiceUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/services/{service_id}", { ...path(id), body })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/services/{service_id}", path(id))),
      onSuccess: () => refresh(),
    }),
    putEnvironment: useMutation({
      mutationFn: async ({ id, environmentId, body }: {
        id: string;
        environmentId: string;
        body: components["schemas"]["ServiceEnvironmentInput"];
      }) => unwrap(await api.PUT("/v1/hugimethara/services/{service_id}/environments/{environment_id}", {
        params: { path: { service_id: id, environment_id: environmentId } },
        body,
      })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    deleteEnvironment: useMutation({
      mutationFn: async ({ id, environmentId }: { id: string; environmentId: string }) =>
        ensureOk(await api.DELETE("/v1/hugimethara/services/{service_id}/environments/{environment_id}", {
          params: { path: { service_id: id, environment_id: environmentId } },
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    importOpenAPI: useMutation({
      mutationFn: async ({ id, body, dryRun }: { id: string; body: OpenAPIImportRequest; dryRun: boolean }) =>
        unwrap(await api.POST("/v1/hugimethara/services/{service_id}/import-openapi", {
          ...path(id),
          params: { path: { service_id: id }, query: { dry_run: dryRun } },
          body,
        })),
      onSuccess: (_result, { id, dryRun }) => {
        if (!dryRun) refresh(id);
      },
    }),
    tryEndpoint: useMutation({
      mutationFn: async ({ endpointId, body }: { endpointId: string; body: ServiceTryRequest }) =>
        unwrap(await api.POST("/v1/hugimethara/service-endpoints/{endpoint_id}/try", {
          params: { path: { endpoint_id: endpointId } },
          body,
        })),
    }),
  };
}
