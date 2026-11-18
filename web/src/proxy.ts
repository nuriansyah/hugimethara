import { NextResponse, type NextRequest } from "next/server";

// Forwards /api/* to the Go API at request time (HUGI_API_URL is read at
// runtime, unlike next.config rewrites which are frozen at build time).
// Keeps the browser same-origin with the API: no CORS, cookies stay first-party.
// No business logic lives here — Next.js is not a BFF (ADR-0009).
export function proxy(request: NextRequest) {
  const apiBase = process.env.HUGI_API_URL ?? "http://localhost:8080";
  const target = new URL(
    request.nextUrl.pathname.replace(/^\/api/, "") + request.nextUrl.search,
    apiBase,
  );
  return NextResponse.rewrite(target);
}

export const config = {
  matcher: "/api/:path*",
};
