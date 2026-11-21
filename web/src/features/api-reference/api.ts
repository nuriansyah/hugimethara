import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@hugimethara/api-client";

/** The subset of OpenAPI 3.0 the reference page renders. */
export type SchemaObject = {
  $ref?: string;
  type?: string;
  format?: string;
  description?: string;
  enum?: unknown[];
  items?: SchemaObject;
  properties?: Record<string, SchemaObject>;
  additionalProperties?: boolean | SchemaObject;
  required?: string[];
  allOf?: SchemaObject[];
  oneOf?: SchemaObject[];
};
export type Parameter = { name: string; in: string; required?: boolean; description?: string; schema?: SchemaObject };
export type MediaContent = Record<string, { schema?: SchemaObject }>;
export type Operation = {
  operationId?: string;
  summary?: string;
  description?: string;
  tags?: string[];
  parameters?: Parameter[];
  requestBody?: { description?: string; required?: boolean; content?: MediaContent };
  responses?: Record<string, { description?: string; content?: MediaContent }>;
  security?: Record<string, string[]>[];
};
export type OpenAPIDocument = {
  openapi: string;
  info: { title: string; version: string; description?: string };
  paths: Record<string, Record<string, Operation>>;
  components?: { schemas?: Record<string, SchemaObject> };
};

export const OPENAPI_JSON = "/api/v1/hugimethara/openapi.json";
export const OPENAPI_YAML = "/api/v1/hugimethara/openapi.yaml";

export function useOpenAPIDocument() {
  return useQuery({
    queryKey: ["openapi"],
    staleTime: 5 * 60_000,
    queryFn: async (): Promise<OpenAPIDocument> => {
      const res = await fetch(OPENAPI_JSON, { credentials: "same-origin" });
      if (!res.ok) throw new ApiError(res.status, undefined, "The API contract could not be loaded.");
      return res.json();
    },
  });
}
