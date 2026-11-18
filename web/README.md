# Hugimethara web

Next.js (App Router) frontend. See [docs/architecture/frontend-architecture.md](../docs/architecture/frontend-architecture.md).

```bash
pnpm install
HUGI_API_URL=http://localhost:8080 pnpm dev   # /api/* is proxied to the Go API (src/proxy.ts)
pnpm generate:api                              # regenerate src/lib/api/schema.d.ts from api/openapi
pnpm lint && pnpm typecheck && pnpm build
```

Feature code lives in `src/features/<feature>/`; shadcn/ui (Radix) primitives in `src/components/ui/`.
Add a navigation entry in `src/config/navigation.ts` only in the change that ships its page.
