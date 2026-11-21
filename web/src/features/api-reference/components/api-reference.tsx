"use client";

import { Download, Lock, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@hugimethara/ui/components/badge";
import { Button } from "@hugimethara/ui/components/button";
import { Card, CardContent } from "@hugimethara/ui/components/card";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { OPENAPI_JSON, OPENAPI_YAML, useOpenAPIDocument, type OpenAPIDocument, type Operation, type SchemaObject } from "../api";

const METHODS = ["get", "post", "put", "patch", "delete"] as const;
const METHOD_STYLE: Record<string, string> = {
  get: "bg-sky-600 text-white",
  post: "bg-emerald-600 text-white",
  put: "bg-amber-600 text-white",
  patch: "bg-violet-600 text-white",
  delete: "bg-red-600 text-white",
};

type Entry = { method: string; path: string; op: Operation; tag: string };

function refName(ref: string) {
  return ref.split("/").pop() ?? ref;
}

function resolve(doc: OpenAPIDocument, s?: SchemaObject): SchemaObject | undefined {
  if (s?.$ref) return doc.components?.schemas?.[refName(s.$ref)];
  return s;
}

function typeLabel(doc: OpenAPIDocument, s?: SchemaObject): string {
  if (!s) return "any";
  if (s.$ref) return refName(s.$ref);
  if (s.type === "array") return `${typeLabel(doc, s.items)}[]`;
  if (s.enum) return s.enum.map((e) => JSON.stringify(e)).join(" | ");
  if (s.allOf?.length === 1) return typeLabel(doc, s.allOf[0]);
  return [s.type ?? "object", s.format].filter(Boolean).join(" · ");
}

/** A schema as a nested property list; refs expand up to a small depth to keep pages readable. */
function SchemaView({ doc, schema, depth = 0, seen = [] }: { doc: OpenAPIDocument; schema?: SchemaObject; depth?: number; seen?: string[] }) {
  const ref = schema?.$ref ? refName(schema.$ref) : undefined;
  const s = resolve(doc, schema?.type === "array" ? schema.items : schema);
  if (!s?.properties || depth > 2 || (ref && seen.includes(ref))) return null;
  const required = new Set(s.required ?? []);
  return (
    <ul className={`flex flex-col gap-1 text-xs ${depth > 0 ? "ml-4 border-l pl-3" : ""}`}>
      {Object.entries(s.properties).map(([name, p]) => (
        <li key={name}>
          <span className="font-mono font-medium">{name}</span>
          {required.has(name) && <span className="text-destructive"> *</span>}{" "}
          <span className="font-mono text-muted-foreground">{typeLabel(doc, p)}</span>
          {p.description && <span className="text-muted-foreground"> — {p.description}</span>}
          <SchemaView doc={doc} schema={p} depth={depth + 1} seen={ref ? [...seen, ref] : seen} />
        </li>
      ))}
    </ul>
  );
}

function OperationView({ doc, e }: { doc: OpenAPIDocument; e: Entry }) {
  const { op } = e;
  const isPublic = !op.security || op.security.length === 0;
  const body = op.requestBody?.content?.["application/json"]?.schema;
  return (
    <details className="group rounded-md border" id={op.operationId}>
      <summary className="flex cursor-pointer list-none flex-wrap items-center gap-2 p-3 hover:bg-muted/50">
        <span className={`w-16 rounded px-2 py-0.5 text-center font-mono text-xs font-semibold uppercase ${METHOD_STYLE[e.method] ?? ""}`}>
          {e.method}
        </span>
        <code className="break-all text-sm">{e.path}</code>
        <span className="text-sm text-muted-foreground">{op.summary}</span>
        {!isPublic && <Lock className="ml-auto size-3.5 text-muted-foreground" aria-label="Requires a session or API key" />}
      </summary>
      <div className="flex flex-col gap-4 border-t p-4 text-sm">
        {op.description && <p className="text-muted-foreground">{op.description}</p>}
        <p className="text-xs text-muted-foreground">
          Operation <code>{op.operationId}</code> ·{" "}
          {isPublic ? "public" : "session cookie (+ X-CSRF-Token on writes) or Authorization: Bearer <API key>"}
        </p>
        {op.parameters && op.parameters.length > 0 && (
          <div className="flex flex-col gap-1">
            <h4 className="text-xs font-semibold uppercase text-muted-foreground">Parameters</h4>
            <ul className="flex flex-col gap-1 text-xs">
              {op.parameters.map((p) => (
                <li key={`${p.in}-${p.name}`}>
                  <span className="font-mono font-medium">{p.name}</span>
                  {p.required && <span className="text-destructive"> *</span>}{" "}
                  <Badge variant="outline" className="px-1 py-0 text-[10px]">{p.in}</Badge>{" "}
                  <span className="font-mono text-muted-foreground">{typeLabel(doc, p.schema)}</span>
                  {p.description && <span className="text-muted-foreground"> — {p.description}</span>}
                </li>
              ))}
            </ul>
          </div>
        )}
        {body && (
          <div className="flex flex-col gap-1">
            <h4 className="text-xs font-semibold uppercase text-muted-foreground">
              Request body <span className="font-mono normal-case">{typeLabel(doc, body)}</span>
            </h4>
            <SchemaView doc={doc} schema={body} />
          </div>
        )}
        <div className="flex flex-col gap-2">
          <h4 className="text-xs font-semibold uppercase text-muted-foreground">Responses</h4>
          {Object.entries(op.responses ?? {}).map(([status, r]) => {
            const schema = r.content?.["application/json"]?.schema;
            return (
              <div key={status} className="flex flex-col gap-1">
                <p className="text-xs">
                  <Badge variant={status.startsWith("2") ? "default" : "secondary"} className="font-mono">{status}</Badge>{" "}
                  {r.description} {schema && <span className="font-mono text-muted-foreground">{typeLabel(doc, schema)}</span>}
                </p>
                {status.startsWith("2") && schema && <SchemaView doc={doc} schema={schema} />}
              </div>
            );
          })}
        </div>
      </div>
    </details>
  );
}

/** Browsable reference generated from the contract hugi-api serves (S14). */
export function ApiReference() {
  const docQuery = useOpenAPIDocument();
  const [query, setQuery] = useState("");
  const [tag, setTag] = useState("all");
  const doc = docQuery.data;
  const entries = useMemo<Entry[]>(() => {
    if (!doc) return [];
    const out: Entry[] = [];
    for (const [path, item] of Object.entries(doc.paths)) {
      for (const method of METHODS) {
        const op = item[method];
        if (op) out.push({ method, path, op, tag: op.tags?.[0] ?? "other" });
      }
    }
    return out.sort((a, b) => a.tag.localeCompare(b.tag) || a.path.localeCompare(b.path));
  }, [doc]);
  const tags = useMemo(() => [...new Set(entries.map((e) => e.tag))], [entries]);
  const q = query.trim().toLowerCase();
  const visible = entries.filter(
    (e) =>
      (tag === "all" || e.tag === tag) &&
      (!q || [e.path, e.method, e.op.summary, e.op.operationId].some((s) => s?.toLowerCase().includes(q))),
  );
  const grouped = tags.map((t) => [t, visible.filter((e) => e.tag === t)] as const).filter(([, list]) => list.length > 0);

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader
        title="API reference"
        description="Every operation of the Hugimethara REST API, generated from the OpenAPI contract the server publishes. Errors are RFC 9457 problem details."
      />
      <div className="flex flex-wrap items-end gap-4">
        <div className="flex min-w-64 flex-1 flex-col gap-1.5">
          <Label htmlFor="api-search">Search</Label>
          <div className="relative">
            <Search className="pointer-events-none absolute top-2.5 left-2.5 size-4 text-muted-foreground" aria-hidden />
            <Input id="api-search" className="pl-8" placeholder="Path, method, summary or operation id"
              value={query} onChange={(e) => setQuery(e.target.value)} />
          </div>
        </div>
        <div className="flex min-w-48 flex-col gap-1.5">
          <Label htmlFor="api-tag">Area</Label>
          <Select value={tag} onValueChange={setTag}>
            <SelectTrigger id="api-tag" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All areas</SelectItem>
              {tags.map((t) => (
                <SelectItem key={t} value={t}>{t}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <a href={OPENAPI_JSON} download="hugimethara-openapi.json"><Download aria-hidden /> JSON</a>
          </Button>
          <Button variant="outline" asChild>
            <a href={OPENAPI_YAML} download="hugimethara-openapi.yaml"><Download aria-hidden /> YAML</a>
          </Button>
        </div>
      </div>
      {docQuery.isPending && <Skeleton className="h-96 w-full" />}
      {docQuery.isError && <QueryError error={docQuery.error} onRetry={() => docQuery.refetch()} title="Contract unavailable" />}
      {doc && (
        <p className="text-sm text-muted-foreground">
          {doc.info.title} {doc.info.version} · OpenAPI {doc.openapi} · {visible.length} of {entries.length} operations
        </p>
      )}
      {doc && grouped.length === 0 && (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">No operation matches “{query}”.</CardContent>
        </Card>
      )}
      {doc &&
        grouped.map(([t, list]) => (
          <section key={t} className="flex flex-col gap-2" aria-labelledby={`tag-${t}`}>
            <h2 id={`tag-${t}`} className="text-lg font-semibold capitalize">{t.replaceAll("_", " ")}</h2>
            {list.map((e) => (
              <OperationView key={`${e.method} ${e.path}`} doc={doc} e={e} />
            ))}
          </section>
        ))}
    </div>
  );
}
