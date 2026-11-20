import createClient from "openapi-fetch";
import type { components, paths } from "./schema";

export type Problem = components["schemas"]["Problem"];

/** Typed client for the Hugimethara API, generated from api/openapi. */
export const api = createClient<paths>({ baseUrl: "/api", credentials: "same-origin" });

const CSRF_COOKIES = ["__Host-hugi_csrf", "hugi_csrf"];

function readCookie(name: string): string | undefined {
  if (typeof document === "undefined") return undefined;
  for (const part of document.cookie.split("; ")) {
    const eq = part.indexOf("=");
    if (eq > 0 && part.slice(0, eq) === name) return decodeURIComponent(part.slice(eq + 1));
  }
  return undefined;
}

// State-changing requests echo the CSRF cookie in a header; the API compares it
// with the value bound to the session (synchronizer token).
api.use({
  onRequest({ request }) {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method)) {
      const token = CSRF_COOKIES.map(readCookie).find(Boolean);
      if (token) request.headers.set("X-CSRF-Token", token);
    }
    return request;
  },
});

/** Error thrown for non-2xx API responses, carrying the RFC 9457 problem. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly fieldErrors: Problem["errors"];
  /** The full problem body, including extension members. */
  readonly body?: Record<string, unknown>;

  constructor(status: number, problem: Partial<Problem> | undefined, fallback: string) {
    super(problem?.detail ?? problem?.title ?? fallback);
    this.name = "ApiError";
    this.status = status;
    this.code = problem?.code ?? "unknown_error";
    this.requestId = problem?.request_id;
    this.fieldErrors = problem?.errors;
    this.body = problem as Record<string, unknown> | undefined;
  }
}

function isProblem(v: unknown): v is Partial<Problem> {
  return typeof v === "object" && v !== null && ("code" in v || "title" in v);
}

/**
 * Unwraps an openapi-fetch result: returns data, or throws ApiError so
 * TanStack Query surfaces the failure (failed calls are never silently ignored).
 */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.error !== undefined || !result.response.ok) {
    const requestId = result.response.headers.get("X-Request-Id") ?? undefined;
    const problem = isProblem(result.error) ? result.error : { request_id: requestId };
    throw new ApiError(
      result.response.status,
      { request_id: requestId, ...problem },
      `Request failed with status ${result.response.status}.`,
    );
  }
  return result.data as T;
}

/** For endpoints that answer 204: throws on failure, returns nothing. */
export function ensureOk(result: { error?: unknown; response: Response }): void {
  if (!result.response.ok) unwrap(result);
}
