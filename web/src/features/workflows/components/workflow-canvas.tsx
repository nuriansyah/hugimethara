"use client";

import "@xyflow/react/dist/style.css";

import {
  Background,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Connection,
  type Edge,
  type Node,
  type NodeProps,
  type NodeTypes as FlowNodeTypes,
} from "@xyflow/react";
import { useMemo } from "react";
import { cn } from "@hugimethara/ui/lib/utils";
import type { NodeType, WorkflowDefinition } from "../api";

export const TRIGGER_ID = "trigger";

type CanvasNodeData = {
  label: string;
  type: string;
  ports: string[];
  isTrigger: boolean;
  status?: string;
};

/** Step status → node accent; shared with the execution history legend. */
export const STATUS_STYLES: Record<string, string> = {
  succeeded: "border-emerald-500 bg-emerald-50 dark:bg-emerald-950/40",
  failed: "border-destructive bg-destructive/10",
  running: "border-sky-500 bg-sky-50 dark:bg-sky-950/40 animate-pulse",
  queued: "border-sky-300",
  retry_scheduled: "border-amber-500 bg-amber-50 dark:bg-amber-950/40",
  waiting: "border-violet-500 bg-violet-50 dark:bg-violet-950/40",
  skipped: "border-dashed opacity-60",
  cancelled: "border-dashed opacity-60",
};

function WorkflowNodeView({ data, selected }: NodeProps<Node<CanvasNodeData>>) {
  return (
    <div
      className={cn(
        "min-w-40 rounded-md border-2 bg-card px-3 py-2 text-card-foreground shadow-sm",
        data.isTrigger && "border-primary",
        data.status && STATUS_STYLES[data.status],
        selected && "ring-2 ring-ring ring-offset-2 ring-offset-background",
      )}
    >
      {!data.isTrigger && <Handle type="target" position={Position.Left} className="!size-2.5" />}
      <p className="text-sm font-medium leading-tight">{data.label}</p>
      <p className="font-mono text-[11px] text-muted-foreground">{data.type}</p>
      {data.status && <p className="mt-1 text-[11px] font-medium capitalize">{data.status.replace("_", " ")}</p>}
      {data.ports.length === 0 ? (
        <Handle type="source" position={Position.Right} className="!size-2.5" />
      ) : (
        data.ports.map((port, i) => (
          <Handle
            key={port}
            id={port}
            type="source"
            position={Position.Right}
            style={{ top: `${((i + 1) / (data.ports.length + 1)) * 100}%` }}
            className="!size-2.5"
          >
            <span className="pointer-events-none absolute left-3 -translate-y-1/2 text-[10px] text-muted-foreground">{port}</span>
          </Handle>
        ))
      )}
    </div>
  );
}

const flowNodeTypes: FlowNodeTypes = { workflow: WorkflowNodeView };

/** A switch has one output per configured case plus default; others use the catalog. */
export function nodePorts(node: WorkflowDefinition["nodes"][number], catalogPorts: string[]): string[] {
  if (node.type !== "logic.switch") return catalogPorts;
  const cases = Array.isArray(node.config?.cases) ? (node.config.cases as unknown[]) : [];
  const ports = cases
    .map((c) => (c && typeof c === "object" ? (c as { port?: unknown }).port : undefined))
    .filter((p): p is string => typeof p === "string" && p !== "" && p !== "default");
  return [...new Set(ports), "default"];
}

/** Depth-first column layout for nodes that have no saved position. */
function autoLayout(def: WorkflowDefinition): Map<string, { x: number; y: number }> {
  const depth = new Map<string, number>([[TRIGGER_ID, 0]]);
  const queue = [TRIGGER_ID];
  while (queue.length) {
    const id = queue.shift()!;
    for (const e of def.edges.filter((edge) => edge.from === id)) {
      const d = (depth.get(id) ?? 0) + 1;
      if (d > (depth.get(e.to) ?? -1)) {
        depth.set(e.to, d);
        queue.push(e.to);
      }
    }
  }
  const rows = new Map<number, number>();
  const out = new Map<string, { x: number; y: number }>();
  for (const id of [TRIGGER_ID, ...def.nodes.map((n) => n.id)]) {
    const col = depth.get(id) ?? 1;
    const row = rows.get(col) ?? 0;
    rows.set(col, row + 1);
    out.set(id, { x: col * 260, y: row * 120 });
  }
  return out;
}

export function WorkflowCanvas({
  definition,
  catalog,
  onChange,
  selectedId,
  onSelect,
  readOnly = false,
  stepStatus,
  height = 520,
}: {
  definition: WorkflowDefinition;
  catalog: NodeType[];
  onChange?: (def: WorkflowDefinition) => void;
  selectedId?: string;
  onSelect?: (id: string | undefined) => void;
  readOnly?: boolean;
  stepStatus?: Record<string, string>;
  height?: number;
}) {
  const portsOf = useMemo(() => new Map(catalog.map((t) => [t.type, t.ports])), [catalog]);
  const labelOf = useMemo(() => new Map(catalog.map((t) => [t.type, t.label])), [catalog]);

  const nodes: Node<CanvasNodeData>[] = useMemo(() => {
    const layout = autoLayout(definition);
    const trigger: Node<CanvasNodeData> = {
      id: TRIGGER_ID,
      type: "workflow",
      position: layout.get(TRIGGER_ID)!,
      draggable: false,
      deletable: false,
      selected: selectedId === TRIGGER_ID,
      data: {
        label: labelOf.get(definition.trigger.type) ?? "Trigger",
        type: definition.trigger.type,
        ports: [],
        isTrigger: true,
      },
    };
    return [
      trigger,
      ...definition.nodes.map((n) => ({
        id: n.id,
        type: "workflow",
        position: n.position ?? layout.get(n.id) ?? { x: 0, y: 0 },
        selected: selectedId === n.id,
        data: {
          label: n.name || n.id,
          type: n.type,
          ports: nodePorts(n, portsOf.get(n.type) ?? []),
          isTrigger: false,
          status: stepStatus?.[n.id],
        },
      })),
    ];
  }, [definition, labelOf, portsOf, selectedId, stepStatus]);

  const edges: Edge[] = useMemo(
    () =>
      definition.edges.map((e) => ({
        id: `${e.from}->${e.to}:${e.port ?? ""}`,
        source: e.from,
        target: e.to,
        sourceHandle: e.port ?? null,
        label: e.port,
        markerEnd: { type: MarkerType.ArrowClosed },
        animated: stepStatus?.[e.to] === "running",
      })),
    [definition.edges, stepStatus],
  );

  function update(next: Partial<WorkflowDefinition>) {
    onChange?.({ ...definition, ...next });
  }

  function onConnect(c: Connection) {
    if (!c.source || !c.target || c.source === c.target) return;
    const port = c.sourceHandle ?? undefined;
    if (definition.edges.some((e) => e.from === c.source && e.to === c.target && (e.port ?? undefined) === port)) return;
    update({ edges: [...definition.edges, { from: c.source, to: c.target, ...(port ? { port } : {}) }] });
  }

  return (
    <div className="rounded-md border bg-muted/20" style={{ height }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={flowNodeTypes}
        fitView
        fitViewOptions={{ padding: 0.2, maxZoom: 1.1 }}
        nodesDraggable={!readOnly}
        nodesConnectable={!readOnly}
        elementsSelectable
        deleteKeyCode={readOnly ? null : ["Backspace", "Delete"]}
        onNodeClick={(_, n) => onSelect?.(n.id)}
        onPaneClick={() => onSelect?.(undefined)}
        onConnect={readOnly ? undefined : onConnect}
        onNodeDragStop={(_, n) => {
          if (readOnly || n.id === TRIGGER_ID) return;
          const position = { x: Math.round(n.position.x), y: Math.round(n.position.y) };
          update({ nodes: definition.nodes.map((x) => (x.id === n.id ? { ...x, position } : x)) });
        }}
        onNodesDelete={(deleted) => {
          const ids = new Set(deleted.map((d) => d.id).filter((id) => id !== TRIGGER_ID));
          update({
            nodes: definition.nodes.filter((n) => !ids.has(n.id)),
            edges: definition.edges.filter((e) => !ids.has(e.from) && !ids.has(e.to)),
          });
          onSelect?.(undefined);
        }}
        onEdgesDelete={(deleted) => {
          const ids = new Set(deleted.map((d) => d.id));
          update({ edges: definition.edges.filter((e) => !ids.has(`${e.from}->${e.to}:${e.port ?? ""}`)) });
        }}
        proOptions={{ hideAttribution: true }}
      >
        <Background />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
