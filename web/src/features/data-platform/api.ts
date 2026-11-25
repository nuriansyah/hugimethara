import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@hugimethara/api-client";
import type { components } from "@hugimethara/shared-types";

export type DataSource = components["schemas"]["DataSource"];
export type DataSourceKind = components["schemas"]["DataSourceKind"];
export type DataSourceSchema = components["schemas"]["DataSourceSchema"];
export type DataSourceCreate = components["schemas"]["DataSourceCreate"];
export type IngestionPipeline = components["schemas"]["IngestionPipeline"];
export type IngestionRun = components["schemas"]["IngestionRun"];
export type PipelineCreate = components["schemas"]["PipelineCreate"];
export type Dataset = components["schemas"]["Dataset"];
export type DatasetVersion = components["schemas"]["DatasetVersion"];
export type DatasetLineage = components["schemas"]["DatasetLineage"];
export type DatasetCreate = components["schemas"]["DatasetCreate"];
export type DatasetPreview = components["schemas"]["DatasetPreview"];
export type ColumnPolicy = components["schemas"]["ColumnPolicy"];
export type RowPolicy = components["schemas"]["RowPolicy"];
export type SemanticModel = components["schemas"]["SemanticModel"];
export type Metric = components["schemas"]["Metric"];
export type MetricValue = components["schemas"]["MetricValue"];
export type QueryRun = components["schemas"]["QueryRun"];
export type QueryResult = components["schemas"]["ExecuteQuery200JSONResponse"];
export type Dashboard = components["schemas"]["Dashboard"];
export type DashboardVersion = components["schemas"]["DashboardVersion"];
export type DashboardDefinition = components["schemas"]["DashboardDefinition"];
export type WidgetData = components["schemas"]["GetDashboardWidgetData200JSONResponse"];
export type ScheduledReport = components["schemas"]["ScheduledReport"];
export type ShareDashboardResult = components["schemas"]["ShareDashboardResult"];

const dataKeys = {
  sources: (workspaceId: string) => ["data-sources", workspaceId] as const,
  sourceSchemas: (id: string) => ["data-sources", id, "schemas"] as const,
  pipelines: (workspaceId: string) => ["pipelines", workspaceId] as const,
  runs: (id: string) => ["pipelines", id, "runs"] as const,
  datasets: (workspaceId: string) => ["datasets", workspaceId] as const,
  versions: (id: string) => ["datasets", id, "versions"] as const,
  lineage: (id: string) => ["datasets", id, "lineage"] as const,
  columnPolicies: (id: string) => ["datasets", id, "column-policies"] as const,
  rowPolicies: (id: string) => ["datasets", id, "row-policies"] as const,
  models: (workspaceId: string) => ["semantic-models", workspaceId] as const,
  metrics: (workspaceId: string) => ["metrics", workspaceId] as const,
  modelMetrics: (id: string) => ["semantic-models", id, "metrics"] as const,
  queries: (workspaceId: string) => ["queries", workspaceId] as const,
  dashboards: (workspaceId: string) => ["dashboards", workspaceId] as const,
  dashboardVersions: (id: string) => ["dashboards", id, "versions"] as const,
  reports: (workspaceId: string) => ["scheduled-reports", workspaceId] as const,
};

export function useDataSourceKinds() {
  return useQuery({
    queryKey: ["data-source-kinds"],
    staleTime: 10 * 60_000,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/data-source-kinds")).data,
  });
}

export function useDataSources(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.sources(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/data-sources", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useDataSourceSchemas(id: string, enabled = true) {
  return useQuery({
    queryKey: dataKeys.sourceSchemas(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/data-sources/{data_source_id}/schemas", {
        params: { path: { data_source_id: id } },
      })).data,
  });
}

export function useDataSourceMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: dataKeys.sources(workspaceId) });
  return {
    create: useMutation({
      mutationFn: async (body: DataSourceCreate) => unwrap(await api.POST("/v1/hugimethara/data-sources", { body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["DataSourceUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/data-sources/{data_source_id}", {
          params: { path: { data_source_id: id } }, body,
        })),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/data-sources/{data_source_id}", {
          params: { path: { data_source_id: id } },
        })),
      onSuccess: refresh,
    }),
    test: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/data-sources/{data_source_id}/test", {
          params: { path: { data_source_id: id } },
        })),
    }),
    discover: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/data-sources/{data_source_id}/discover", {
          params: { path: { data_source_id: id } },
        })),
      onSuccess: (_result, id) => qc.invalidateQueries({ queryKey: dataKeys.sourceSchemas(id) }),
    }),
  };
}

export function usePipelines(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.pipelines(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/pipelines", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function usePipelineRuns(id: string, enabled = true) {
  return useQuery({
    queryKey: dataKeys.runs(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/pipelines/{pipeline_id}/runs", {
        params: { path: { pipeline_id: id } },
      })).data,
  });
}

export function usePipelineMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = (id?: string) => {
    qc.invalidateQueries({ queryKey: dataKeys.pipelines(workspaceId) });
    if (id) qc.invalidateQueries({ queryKey: dataKeys.runs(id) });
  };
  return {
    create: useMutation({
      mutationFn: async (body: PipelineCreate) => unwrap(await api.POST("/v1/hugimethara/pipelines", { body })),
      onSuccess: () => refresh(),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["PipelineUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/pipelines/{pipeline_id}", {
          params: { path: { pipeline_id: id } }, body,
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/pipelines/{pipeline_id}", { params: { path: { pipeline_id: id } } })),
      onSuccess: () => refresh(),
    }),
    run: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/pipelines/{pipeline_id}/run", { params: { path: { pipeline_id: id } } })),
      onSuccess: (_result, id) => refresh(id),
    }),
  };
}

export function useDatasets(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.datasets(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/datasets", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useDatasetVersions(id: string, enabled = true) {
  return useQuery({
    queryKey: dataKeys.versions(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/datasets/{dataset_id}/versions", {
        params: { path: { dataset_id: id } },
      })).data,
  });
}

export function useDatasetLineage(id: string, enabled = true) {
  return useQuery({
    queryKey: dataKeys.lineage(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/datasets/{dataset_id}/lineage", {
        params: { path: { dataset_id: id } },
      })).data,
  });
}

export function useDatasetPolicies(id: string, enabled = true) {
  const columns = useQuery({
    queryKey: dataKeys.columnPolicies(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/datasets/{dataset_id}/column-policies", {
        params: { path: { dataset_id: id } },
      })).data,
  });
  const rows = useQuery({
    queryKey: dataKeys.rowPolicies(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/datasets/{dataset_id}/row-policies", {
        params: { path: { dataset_id: id } },
      })).data,
  });
  return { columns, rows };
}

export function useDatasetMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = (id?: string) => {
    qc.invalidateQueries({ queryKey: dataKeys.datasets(workspaceId) });
    if (id) {
      qc.invalidateQueries({ queryKey: dataKeys.versions(id) });
      qc.invalidateQueries({ queryKey: dataKeys.lineage(id) });
      qc.invalidateQueries({ queryKey: dataKeys.columnPolicies(id) });
      qc.invalidateQueries({ queryKey: dataKeys.rowPolicies(id) });
    }
  };
  return {
    create: useMutation({
      mutationFn: async (body: DatasetCreate) => unwrap(await api.POST("/v1/hugimethara/datasets", { body })),
      onSuccess: () => refresh(),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["DatasetUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/datasets/{dataset_id}", {
          params: { path: { dataset_id: id } }, body,
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/datasets/{dataset_id}", { params: { path: { dataset_id: id } } })),
      onSuccess: () => refresh(),
    }),
    publish: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/datasets/{dataset_id}/publish", {
          params: { path: { dataset_id: id } },
        })),
      onSuccess: (_result, id) => refresh(id),
    }),
    preview: useMutation({
      mutationFn: async ({ id, sql }: { id: string; sql: string }) =>
        unwrap(await api.POST("/v1/hugimethara/datasets/{dataset_id}/preview", {
          params: { path: { dataset_id: id } }, body: { sql },
        })),
    }),
    putColumnPolicy: useMutation({
      mutationFn: async ({ id, column, body }: { id: string; column: string; body: components["schemas"]["ColumnPolicyBody"] }) =>
        unwrap(await api.PUT("/v1/hugimethara/datasets/{dataset_id}/column-policies/{column}", {
          params: { path: { dataset_id: id, column } }, body,
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    removeColumnPolicy: useMutation({
      mutationFn: async ({ id, column }: { id: string; column: string }) =>
        ensureOk(await api.DELETE("/v1/hugimethara/datasets/{dataset_id}/column-policies/{column}", {
          params: { path: { dataset_id: id, column } },
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    putRowPolicy: useMutation({
      mutationFn: async ({ id, roleKey, body }: { id: string; roleKey: string; body: components["schemas"]["RowPolicyBody"] }) =>
        unwrap(await api.PUT("/v1/hugimethara/datasets/{dataset_id}/row-policies/{role_key}", {
          params: { path: { dataset_id: id, role_key: roleKey } }, body,
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    removeRowPolicy: useMutation({
      mutationFn: async ({ id, roleKey }: { id: string; roleKey: string }) =>
        ensureOk(await api.DELETE("/v1/hugimethara/datasets/{dataset_id}/row-policies/{role_key}", {
          params: { path: { dataset_id: id, role_key: roleKey } },
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
  };
}

export function useSemanticModels(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.models(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/semantic-models", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useModelMetrics(id: string, enabled = true) {
  return useQuery({
    queryKey: dataKeys.modelMetrics(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/semantic-models/{model_id}/metrics", {
        params: { path: { model_id: id } },
      })).data,
  });
}

export function useMetrics(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.metrics(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/metrics", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useAnalyticsMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = (modelId?: string) => {
    qc.invalidateQueries({ queryKey: dataKeys.models(workspaceId) });
    qc.invalidateQueries({ queryKey: dataKeys.metrics(workspaceId) });
    if (modelId) qc.invalidateQueries({ queryKey: dataKeys.modelMetrics(modelId) });
  };
  return {
    createModel: useMutation({
      mutationFn: async (body: components["schemas"]["SemanticModelCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/semantic-models", { body })),
      onSuccess: () => refresh(),
    }),
    removeModel: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/semantic-models/{model_id}", { params: { path: { model_id: id } } })),
      onSuccess: () => refresh(),
    }),
    createMetric: useMutation({
      mutationFn: async (body: components["schemas"]["MetricCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/metrics", { body })),
      onSuccess: (metric) => refresh(metric.semantic_model_id),
    }),
    removeMetric: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/metrics/{metric_id}", { params: { path: { metric_id: id } } })),
      onSuccess: () => refresh(),
    }),
    queryMetric: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["MetricQueryBody"] }) =>
        unwrap(await api.POST("/v1/hugimethara/metrics/{metric_id}/query", {
          params: { path: { metric_id: id } }, body,
        })),
    }),
  };
}

export function useQueryRuns(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.queries(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/queries", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useQueryMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: dataKeys.queries(workspaceId) });
  return {
    execute: useMutation({
      mutationFn: async (body: components["schemas"]["QueryBody"]) =>
        unwrap(await api.POST("/v1/hugimethara/queries", { body })),
      onSuccess: refresh,
    }),
    cancel: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.POST("/v1/hugimethara/queries/{query_id}/cancel", {
          params: { path: { query_id: id } },
        })),
      onSuccess: refresh,
    }),
  };
}

export function useDashboards(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.dashboards(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/dashboards", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useDashboardVersions(id: string, enabled = true) {
  return useQuery({
    queryKey: dataKeys.dashboardVersions(id),
    enabled: Boolean(id) && enabled,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/dashboards/{dashboard_id}/versions", {
        params: { path: { dashboard_id: id } },
      })).data,
  });
}

export function useDashboardMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = (id?: string) => {
    qc.invalidateQueries({ queryKey: dataKeys.dashboards(workspaceId) });
    if (id) qc.invalidateQueries({ queryKey: dataKeys.dashboardVersions(id) });
  };
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["DashboardCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/dashboards", { body })),
      onSuccess: () => refresh(),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["DashboardUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/dashboards/{dashboard_id}", {
          params: { path: { dashboard_id: id } }, body,
        })),
      onSuccess: (_result, { id }) => refresh(id),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/dashboards/{dashboard_id}", { params: { path: { dashboard_id: id } } })),
      onSuccess: () => refresh(),
    }),
    publish: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/dashboards/{dashboard_id}/publish", {
          params: { path: { dashboard_id: id } },
        })),
      onSuccess: (_result, id) => refresh(id),
    }),
    share: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["ShareDashboardBody"] }) =>
        unwrap(await api.POST("/v1/hugimethara/dashboards/{dashboard_id}/share", {
          params: { path: { dashboard_id: id } }, body,
        })),
    }),
    widgetData: useMutation({
      mutationFn: async ({ id, widgetId, filters }: { id: string; widgetId: string; filters?: Record<string, string> }) =>
        unwrap(await api.POST("/v1/hugimethara/dashboards/{dashboard_id}/widgets/{widget_id}/data", {
          params: { path: { dashboard_id: id, widget_id: widgetId } }, body: { filters: filters ?? {} },
        })),
    }),
  };
}

export function useScheduledReports(workspaceId: string) {
  return useQuery({
    queryKey: dataKeys.reports(workspaceId),
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/scheduled-reports", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useScheduledReportMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: dataKeys.reports(workspaceId) });
  return {
    create: useMutation({
      mutationFn: async (body: components["schemas"]["ScheduledReportCreate"]) =>
        unwrap(await api.POST("/v1/hugimethara/scheduled-reports", { body })),
      onSuccess: refresh,
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: components["schemas"]["ScheduledReportUpdate"] }) =>
        unwrap(await api.PATCH("/v1/hugimethara/scheduled-reports/{report_id}", {
          params: { path: { report_id: id } }, body,
        })),
      onSuccess: refresh,
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/scheduled-reports/{report_id}", {
          params: { path: { report_id: id } },
        })),
      onSuccess: refresh,
    }),
  };
}

export function useSharedDashboard(token: string) {
  return useQuery({
    queryKey: ["shared-dashboard", token],
    enabled: Boolean(token),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/shared-dashboards/{token}", {
        params: { path: { token } },
      })),
  });
}
