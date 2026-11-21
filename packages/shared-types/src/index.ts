/**
 * Contract types shared by every web workspace. schema.d.ts is generated from
 * api/openapi/openapi.yaml (`pnpm generate:api`) and must never be edited by hand.
 */
export type { components, operations, paths } from "./schema";

import type { components } from "./schema";

/** A named schema of the API contract, e.g. `Schema<"Workspace">`. */
export type Schema<N extends keyof components["schemas"]> = components["schemas"][N];

/** RFC 9457 problem details returned by every failing API call. */
export type Problem = Schema<"Problem">;
