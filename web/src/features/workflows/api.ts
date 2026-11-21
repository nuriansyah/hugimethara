import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ensureOk, unwrap } from "@hugimethara/api-client";
import type { Schema } from "@hugimethara/shared-types";

export type Workflow = Schema<"Workflow">;
export type WorkflowDefinition = Schema<"WorkflowDefinition">;
export type WorkflowNode = Schema<"WorkflowNode">;
export type WorkflowEdge = Schema<"WorkflowEdge">;
export type WorkflowVersion = Schema<"WorkflowVersion">;
export type WorkflowDeployment = Schema<"WorkflowDeployment">;
export type WorkflowExecution = Schema<"WorkflowExecution">;
export type WorkflowStep = Schema<"WorkflowStep">;
export type WorkflowIssue = Schema<"WorkflowIssue">;
export type NodeType = Schema<"NodeType">;

const TERMINAL = new Set(["succeeded", "failed", "cancelled", "timed_out"]);

export function isTerminal(status: string | undefined) {
  return status !== undefined && TERMINAL.has(status);
}

export function useNodeTypes() {
  return useQuery({
    queryKey: ["workflow-node-types"],
    staleTime: Infinity,
    queryFn: async () => unwrap(await api.GET("/v1/hugimethara/node-types")).data,
  });
}

export function useWorkflows(workspaceId: string) {
  return useQuery({
    queryKey: ["workflows", workspaceId],
    enabled: Boolean(workspaceId),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workflows", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useWorkflowVersions(id: string) {
  return useQuery({
    queryKey: ["workflows", id, "versions"],
    enabled: Boolean(id),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workflows/{workflow_id}/versions", { params: { path: { workflow_id: id } } })).data,
  });
}

export function useWorkflowDeployments(id: string) {
  return useQuery({
    queryKey: ["workflows", id, "deployments"],
    enabled: Boolean(id),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workflows/{workflow_id}/deployments", { params: { path: { workflow_id: id } } }))
        .data,
  });
}

export function useWorkflowExecutions(workspaceId: string, workflowId?: string, status?: string) {
  return useQuery({
    queryKey: ["workflow-executions", workspaceId, workflowId ?? "", status ?? ""],
    enabled: Boolean(workspaceId),
    refetchInterval: (q) => (q.state.data?.some((e) => !isTerminal(e.status)) ? 2000 : false),
    queryFn: async () =>
      unwrap(
        await api.GET("/v1/hugimethara/workflow-executions", {
          params: {
            query: { workspace_id: workspaceId, workflow_id: workflowId || undefined, status: status || undefined },
          },
        }),
      ).data,
  });
}

/** One execution with every step; polls until the execution is terminal. */
export function useWorkflowExecution(id: string) {
  return useQuery({
    queryKey: ["workflow-execution", id],
    enabled: Boolean(id),
    refetchInterval: (q) => (q.state.data && isTerminal(q.state.data.status) ? false : 1500),
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workflow-executions/{execution_id}", { params: { path: { execution_id: id } } })),
  });
}

export function useWorkflowMutations(workspaceId: string) {
  const qc = useQueryClient();
  const refresh = (id?: string) => {
    qc.invalidateQueries({ queryKey: ["workflows", workspaceId] });
    if (id) qc.invalidateQueries({ queryKey: ["workflows", id] });
  };
  return {
    create: useMutation({
      mutationFn: async (body: Schema<"WorkflowCreate">) => unwrap(await api.POST("/v1/hugimethara/workflows", { body })),
      onSuccess: (w) => refresh(w.id),
    }),
    update: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: Schema<"WorkflowUpdate"> }) =>
        unwrap(await api.PATCH("/v1/hugimethara/workflows/{workflow_id}", { params: { path: { workflow_id: id } }, body })),
      onSuccess: (w) => refresh(w.id),
    }),
    remove: useMutation({
      mutationFn: async (id: string) =>
        ensureOk(await api.DELETE("/v1/hugimethara/workflows/{workflow_id}", { params: { path: { workflow_id: id } } })),
      onSuccess: () => refresh(),
    }),
    validate: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/workflows/{workflow_id}/validate", { params: { path: { workflow_id: id } } })),
    }),
    publish: useMutation({
      mutationFn: async (id: string) =>
        unwrap(await api.POST("/v1/hugimethara/workflows/{workflow_id}/publish", { params: { path: { workflow_id: id } } })),
      onSuccess: (v) => refresh(v.workflow_id),
    }),
    deploy: useMutation({
      mutationFn: async ({ id, environmentId, body }: { id: string; environmentId: string; body: Schema<"WorkflowDeploy"> }) =>
        unwrap(
          await api.PUT("/v1/hugimethara/workflows/{workflow_id}/deployments/{environment_id}", {
            params: { path: { workflow_id: id, environment_id: environmentId } },
            body,
          }),
        ),
      onSuccess: (d) => refresh(d.workflow_id),
    }),
    execute: useMutation({
      mutationFn: async ({ id, body }: { id: string; body: Schema<"WorkflowExecute"> }) =>
        unwrap(await api.POST("/v1/hugimethara/workflows/{workflow_id}/execute", { params: { path: { workflow_id: id } }, body })),
      onSuccess: () => qc.invalidateQueries({ queryKey: ["workflow-executions", workspaceId] }),
    }),
  };
}

// ---- S11: approvals inbox, schedules, cancel / re-run ----

export type ApprovalTask = Schema<"ApprovalTask">;
export type ApprovalDecision = Schema<"ApprovalDecision">;
export type WorkflowSchedule = Schema<"WorkflowSchedule">;

export function useApprovalTasks(filter: { status?: string; executionId?: string; enabled?: boolean } = {}) {
  return useQuery({
    queryKey: ["approval-tasks", filter.status ?? "", filter.executionId ?? ""],
    enabled: filter.enabled ?? true,
    refetchInterval: 10_000,
    queryFn: async () =>
      unwrap(
        await api.GET("/v1/hugimethara/approval-tasks", {
          params: {
            query: { assigned_to: "me", status: filter.status || undefined, execution_id: filter.executionId || undefined },
          },
        }),
      ).data,
  });
}

/** Every task of one execution (needs workflow:read in its workspace). */
export function useExecutionTasks(workspaceId: string, executionId: string, enabled = true) {
  return useQuery({
    queryKey: ["approval-tasks", "execution", executionId],
    enabled: enabled && Boolean(workspaceId && executionId),
    refetchInterval: 5_000,
    queryFn: async () =>
      unwrap(
        await api.GET("/v1/hugimethara/approval-tasks", {
          params: { query: { assigned_to: "any", workspace_id: workspaceId, execution_id: executionId } },
        }),
      ).data,
  });
}

export function useDecideTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body: ApprovalDecision }) =>
      unwrap(await api.POST("/v1/hugimethara/approval-tasks/{task_id}/decide", { params: { path: { task_id: id } }, body })),
    onSuccess: (t) => {
      qc.invalidateQueries({ queryKey: ["approval-tasks"] });
      qc.invalidateQueries({ queryKey: ["workflow-execution", t.execution_id] });
    },
  });
}

export function useWorkflowSchedules(workspaceId: string) {
  return useQuery({
    queryKey: ["workflow-schedules", workspaceId],
    enabled: Boolean(workspaceId),
    refetchInterval: 30_000,
    queryFn: async () =>
      unwrap(await api.GET("/v1/hugimethara/workflow-schedules", { params: { query: { workspace_id: workspaceId } } })).data,
  });
}

export function useExecutionControls() {
  const qc = useQueryClient();
  const refresh = (e: WorkflowExecution) => {
    qc.setQueryData(["workflow-execution", e.id], e);
    qc.invalidateQueries({ queryKey: ["workflow-executions", e.workspace_id] });
    qc.invalidateQueries({ queryKey: ["approval-tasks"] });
  };
  return {
    cancel: useMutation({
      mutationFn: async (id: string) =>
        unwrap(
          await api.POST("/v1/hugimethara/workflow-executions/{execution_id}/cancel", { params: { path: { execution_id: id } } }),
        ),
      onSuccess: refresh,
    }),
    rerun: useMutation({
      mutationFn: async (id: string) =>
        unwrap(
          await api.POST("/v1/hugimethara/workflow-executions/{execution_id}/retry", { params: { path: { execution_id: id } } }),
        ),
      onSuccess: refresh,
    }),
  };
}
