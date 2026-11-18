# Frontend Architecture

## Stack
Next.js 16 App Router (Cache Components enabled) + TypeScript (strict), Tailwind v4, shadcn/ui on **Radix**
(`components.json` base `radix`; shadcn's newer default is Base UI — we keep Radix per the PRD), TanStack Query (server state),
TanStack Table, React Hook Form + Zod, Zustand only for local editor state (workflow canvas, composition editor),
React Flow (workflow designer), Monaco (JSON/CEL/SQL), ECharts (Phase 2 dashboards).

## Rendering & data model
- Authenticated app is **client-rendered under a server layout**: the layout performs a server-side session check
  (forwarding the cookie to `GET /me`) and redirects to login; pages fetch via TanStack Query against the Go API.
  No business logic in Next.js route handlers — Next.js is not a BFF.
- The browser only talks to its own origin: `src/proxy.ts` forwards `/api/*` to `HUGI_API_URL` at request time
  (next.config rewrites are frozen at build time, so they cannot be configured per environment). In production an
  ingress may route `/api` directly to hugi-api instead; the client code is the same.
- Same-site deployment (path-routed `/api`, or `app.example.com` + `api.example.com`) so the session cookie is
  `HttpOnly; Secure; SameSite=Lax`. Mutations send `X-CSRF-Token` (double-submit).
- API client generated from `api/openapi` with `openapi-typescript` + `openapi-fetch`; Zod schemas for forms live
  next to features. Contract drift fails CI.

## Structure
```
src/app/(auth)/login, /invite/[token], /oidc/callback
src/app/(app)/[org]/overview
src/app/(app)/[org]/[workspace]/integrations/{catalog,connections,services,credentials}
src/app/(app)/[org]/[workspace]/api-studio/{explorer,compositions,mappings}
src/app/(app)/[org]/[workspace]/automation/{workflows,executions,schedules,webhooks}
src/app/(app)/[org]/governance/{users,groups,roles,identity-providers,audit}
src/app/(app)/[org]/settings/{organization,environments,notifications,usage}
src/features/<feature>/{api.ts, hooks.ts, schemas.ts, components/}
```
Information architecture follows PRD §11; Phase 2/3 sections appear only when their phase ships (no dead links).

## UX rules
- Every query has loading, empty, and error states; errors render the API's `detail` + `request_id`.
- Every mutation shows pending state and surfaces failures (toast + inline field errors from `errors[]`).
- **Simple / Advanced mode** is a per-user preference toggling editors (form ↔ JSON/CEL) over the same model.
- Permission gating via `usePermission('workflow:execute')` from `/me/permissions` — UX only.
- Accessibility: Radix primitives, keyboard-navigable canvas actions, WCAG AA contrast, dark mode.
