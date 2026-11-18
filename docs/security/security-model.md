# Security Model

## Threat model summary (STRIDE, Phase 1 surface)

| Asset / entry point | Primary threats | Controls |
|---|---|---|
| Tenant data in Postgres | Cross-tenant read/write (IDOR, missing filter) | Principal-derived org, composite FKs, RLS, 404-on-foreign-id, security test suite |
| Sessions | Theft, fixation, CSRF | HttpOnly/Secure/SameSite cookies, rotation on login, hashed storage, CSRF double-submit, idle+absolute expiry |
| Credentials / secrets | Exposure via API, logs, errors, step outputs | Envelope encryption ([ADR-0011](../adr/0011-secrets-envelope-encryption.md)), write-only API, `Secret` type redaction, redacting log handler, step I/O scrubbing of resolved secret values |
| Authorization | Privilege escalation (grant self roles) | `role:bind` required; cannot grant permissions you don't hold at that scope ("no escalation" rule); `org_owner` immutable; decisions audited |
| Outbound HTTP (connectors, composition, explorer, OpenAPI URL import, webhooks) | SSRF to metadata / internal nets | `platform/egress` dial-time IP checks after DNS, per-environment allowlist, redirect re-validation, size/time caps |
| Inbound webhooks | Forgery, replay, flooding | HMAC verification, timestamp tolerance, provider event-id dedupe, per-endpoint rate limit, body cap |
| Expressions (CEL) | Code execution, DoS | CEL: no I/O, cost limit, type-checked at publish |
| Workflow / composition | Unbounded loops, fan-out, cost DoS | Hard limits (steps, iterations, branches, duration, concurrency), per-org quotas |
| DB query node | SQL injection | Parameterized queries only; statement allowlist per connector instance (read-only default), statement timeout |
| Custom connectors | Arbitrary code in core | Out-of-process only ([ADR-0005](../adr/0005-connector-isolation.md)) |
| OIDC | Token substitution, open redirect | PKCE, state+nonce, issuer/audience checks, exact redirect URI, email-domain allowlist |
| Supply chain | Vulnerable deps / images | `govulncheck`, `npm audit`/osv-scanner, Trivy image scan, pinned base images, distroless non-root runtime |

## SSRF
Default-deny ranges: `0.0.0.0/8, 10/8, 100.64/10, 127/8, 169.254/16, 172.16/12, 192.168/16, ::1, fc00::/7, fe80::/10`,
multicast, and cloud metadata hosts. Admins may allow specific CIDRs/hosts per environment (required for internal
services). Check happens in `Dialer.Control` on the resolved IP, so DNS rebinding cannot bypass it.

## Password & key handling
argon2id (m=64MiB, t=3, p=2 — tuned by benchmark), lockout after 10 failures/15 min, constant-time compares,
API keys and session tokens stored as SHA-256 of 256-bit random values.

## Logging
Never log: secret values, auth headers, cookies, full request/response bodies of connectors (only redacted excerpts on
error, opt-in per instance in non-production environments). PII fields in audit `changes` are masked.

## Headers & transport
TLS terminated at ingress; HSTS; CSP on web app; `X-Content-Type-Options`, `Referrer-Policy`, frame-ancestors none.
CORS: explicit allowlist of web origins, credentials only for those.
