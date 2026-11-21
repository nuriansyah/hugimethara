"use client";

import { Trash2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Textarea } from "@hugimethara/ui/components/textarea";
import { useWebhookEndpoints } from "@/features/webhooks/api";
import type { NodeType, WorkflowDefinition, WorkflowNode } from "../api";
import { TRIGGER_ID } from "./workflow-canvas";

type Field = NodeType["fields"][number];

/** A JSON-valued field: edited as text, committed only when it parses. */
function JsonField({ field, value, onCommit }: { field: Field; value: unknown; onCommit: (v: unknown) => void }) {
  const [text, setText] = useState(value === undefined ? "" : JSON.stringify(value, null, 2));
  const [error, setError] = useState("");
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={`f-${field.key}`}>
        {field.label}
        {field.required && <span className="text-destructive"> *</span>}
      </Label>
      <Textarea
        id={`f-${field.key}`}
        className="min-h-28 font-mono text-xs"
        value={text}
        aria-invalid={Boolean(error)}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => {
          if (text.trim() === "") {
            setError("");
            onCommit(undefined);
            return;
          }
          try {
            onCommit(JSON.parse(text));
            setError("");
          } catch {
            setError("Not valid JSON — the previous value is kept.");
          }
        }}
      />
      {error ? <p className="text-xs text-destructive">{error}</p> : field.description && (
        <p className="text-xs text-muted-foreground">{field.description}</p>
      )}
    </div>
  );
}

/** Picks an inbound webhook endpoint of the workflow's workspace (trigger.webhook). */
function EndpointField({ field, value, onChange, workspaceId }: { field: Field; value: unknown; onChange: (v: unknown) => void; workspaceId: string }) {
  const endpoints = useWebhookEndpoints(workspaceId);
  const id = `f-${field.key}`;
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>
        {field.label}
        {field.required && <span className="text-destructive"> *</span>}
      </Label>
      <Select value={typeof value === "string" ? value : ""} onValueChange={(v) => onChange(v)}>
        <SelectTrigger id={id} className="w-full">
          <SelectValue placeholder={endpoints.data?.length === 0 ? "No endpoints in this workspace" : "Choose…"} />
        </SelectTrigger>
        <SelectContent>
          {(endpoints.data ?? []).map((e) => (
            <SelectItem key={e.id} value={e.id}>
              {e.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        {field.description} Create endpoints on the{" "}
        <Link href="/automation/webhooks" className="underline underline-offset-2">
          Webhooks
        </Link>{" "}
        page.
      </p>
    </div>
  );
}

function ConfigField({
  field,
  value,
  onChange,
  workspaceId,
}: {
  field: Field;
  value: unknown;
  onChange: (v: unknown) => void;
  workspaceId?: string;
}) {
  const id = `f-${field.key}`;
  if (field.kind === "json") return <JsonField key={field.key} field={field} value={value} onCommit={onChange} />;
  if (field.kind === "webhook_endpoint" && workspaceId) {
    return <EndpointField field={field} value={value} onChange={onChange} workspaceId={workspaceId} />;
  }
  if (field.kind === "boolean") {
    return (
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={id}>{field.label}</Label>
        <Select value={value === true ? "true" : "false"} onValueChange={(v) => onChange(v === "true" ? true : undefined)}>
          <SelectTrigger id={id} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="false">No</SelectItem>
            <SelectItem value="true">Yes</SelectItem>
          </SelectContent>
        </Select>
        {field.description && <p className="text-xs text-muted-foreground">{field.description}</p>}
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>
        {field.label}
        {field.required && <span className="text-destructive"> *</span>}
      </Label>
      {field.kind === "select" ? (
        <Select value={typeof value === "string" ? value : ""} onValueChange={(v) => onChange(v)}>
          <SelectTrigger id={id} className="w-full">
            <SelectValue placeholder="Choose…" />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((o) => (
              <SelectItem key={o} value={o}>
                {o}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : field.kind === "expression" || field.kind === "text" ? (
        <Textarea
          id={id}
          className={field.kind === "expression" ? "font-mono text-xs" : undefined}
          value={typeof value === "string" ? value : ""}
          placeholder={field.kind === "expression" ? "input.amount > 100.0" : undefined}
          onChange={(e) => onChange(e.target.value || undefined)}
        />
      ) : (
        <Input
          id={id}
          type={field.kind === "number" ? "number" : "text"}
          value={typeof value === "string" || typeof value === "number" ? value : ""}
          onChange={(e) => onChange(field.kind === "number" ? Number(e.target.value) : e.target.value || undefined)}
        />
      )}
      {field.description && <p className="text-xs text-muted-foreground">{field.description}</p>}
    </div>
  );
}

export function NodeInspector({
  workspaceId,
  definition,
  catalog,
  selectedId,
  onChange,
  onSelect,
  readOnly,
}: {
  workspaceId?: string;
  definition: WorkflowDefinition;
  catalog: NodeType[];
  selectedId?: string;
  onChange: (def: WorkflowDefinition) => void;
  onSelect: (id: string | undefined) => void;
  readOnly: boolean;
}) {
  if (!selectedId) {
    return (
      <p className="text-sm text-muted-foreground">
        Select a node to configure it. Drag from a node&apos;s right handle to another node to connect them; press Delete to
        remove the selection. Expressions use CEL over <code className="font-mono">input</code>,{" "}
        <code className="font-mono">trigger</code> and <code className="font-mono">nodes.&lt;id&gt;.output</code> — JSON
        numbers are doubles, so write <code className="font-mono">100.0</code>.
      </p>
    );
  }
  if (selectedId === TRIGGER_ID) {
    const triggers = catalog.filter((t) => t.kind === "trigger");
    return (
      <fieldset disabled={readOnly} className="flex flex-col gap-3">
        <h3 className="text-sm font-semibold">Trigger</h3>
        <Select
          value={definition.trigger.type}
          disabled={readOnly}
          onValueChange={(type) => onChange({ ...definition, trigger: { type } })}
        >
          <SelectTrigger className="w-full" aria-label="Trigger type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {triggers.map((t) => (
              <SelectItem key={t.type} value={t.type} disabled={!t.available}>
                {t.label}
                {!t.available && ` — ${t.unavailable_reason}`}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {(triggers.find((t) => t.type === definition.trigger.type)?.fields ?? []).map((field) => (
          <ConfigField
            key={`${definition.trigger.type}-${field.key}`}
            workspaceId={workspaceId}
            field={field}
            value={definition.trigger.config?.[field.key]}
            onChange={(v) => {
              const config = { ...(definition.trigger.config ?? {}) };
              if (v === undefined) delete config[field.key];
              else config[field.key] = v;
              onChange({ ...definition, trigger: { ...definition.trigger, config } });
            }}
          />
        ))}
        <p className="text-xs text-muted-foreground">
          The execution input is passed to every node connected to the trigger. Any deployed workflow can also be run
          manually.
        </p>
      </fieldset>
    );
  }
  const node = definition.nodes.find((n) => n.id === selectedId);
  if (!node) return null;
  const type = catalog.find((t) => t.type === node.type);
  const patch = (next: Partial<WorkflowNode>) =>
    onChange({ ...definition, nodes: definition.nodes.map((n) => (n.id === node.id ? { ...n, ...next } : n)) });
  const setConfig = (key: string, v: unknown) => {
    const config = { ...(node.config ?? {}) };
    if (v === undefined) delete config[key];
    else config[key] = v;
    patch({ config });
  };

  return (
    <fieldset disabled={readOnly} className="flex flex-col gap-3" key={node.id}>
      <div className="flex items-start justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold">{type?.label ?? node.type}</h3>
          <p className="font-mono text-xs text-muted-foreground">{node.id}</p>
        </div>
        <Badge variant="outline">{type?.kind ?? "node"}</Badge>
      </div>
      {type && <p className="text-xs text-muted-foreground">{type.description}</p>}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="node-name">Display name</Label>
        <Input id="node-name" value={node.name ?? ""} onChange={(e) => patch({ name: e.target.value || undefined })} />
      </div>
      {(type?.fields ?? []).map((field) => (
        <ConfigField key={field.key} field={field} value={node.config?.[field.key]} onChange={(v) => setConfig(field.key, v)} />
      ))}
      {(type?.kind === "action" || node.type === "logic.loop") && (
        <div className="grid grid-cols-2 gap-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="node-attempts">Max attempts</Label>
            <Input
              id="node-attempts"
              type="number"
              min={1}
              max={10}
              value={node.retry?.max_attempts ?? 1}
              onChange={(e) => {
                const n = Number(e.target.value);
                patch({ retry: n > 1 ? { ...node.retry, max_attempts: n } : undefined });
              }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="node-timeout">Timeout (ms)</Label>
            <Input
              id="node-timeout"
              type="number"
              min={0}
              max={300000}
              value={node.timeout_ms ?? ""}
              placeholder="30000"
              onChange={(e) => patch({ timeout_ms: e.target.value ? Number(e.target.value) : undefined })}
            />
          </div>
        </div>
      )}
      {!readOnly && (
        <Button
          variant="outline"
          size="sm"
          className="self-start"
          onClick={() => {
            onChange({
              ...definition,
              nodes: definition.nodes.filter((n) => n.id !== node.id),
              edges: definition.edges.filter((e) => e.from !== node.id && e.to !== node.id),
            });
            onSelect(undefined);
          }}
        >
          <Trash2 aria-hidden /> Remove node
        </Button>
      )}
    </fieldset>
  );
}
