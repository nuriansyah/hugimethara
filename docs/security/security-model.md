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

## Authentication (S1)

| Control | Implementation |
|---|---|
| Passwords | argon2id PHC strings; parameters read from the hash so they can be raised without breaking old hashes; 12–128 chars, not equal to email |
| Enumeration | Unknown email, wrong password, locked and disabled accounts all return the same 401 and spend the same hashing time. Signup necessarily reveals that an email or slug is taken (409) — mitigated by IP rate limiting; email verification in S4 |
| Brute force | Per-account lockout after 10 consecutive failures for 15 min (Postgres, authoritative). Redis rate limits: login 30/5 min per IP and 10/5 min per email; signup 10/h per IP. Redis outage fails open (logged) because lockout still bounds guessing |
| Sessions | 256-bit random token in `HttpOnly; SameSite=Lax` cookie (`__Host-` + `Secure` over HTTPS); only SHA-256 stored; idle + absolute expiry from org settings; new session on every login (no fixation); server-side revocation; disabled users, suspended memberships/orgs are rejected on every request |
| CSRF | Synchronizer token: random CSRF token per session, hash stored with the session, required in `X-CSRF-Token` for every state-changing request made with a session cookie (including logout). Plus an Origin check on all state-changing requests (also blocks login CSRF) and SameSite cookies |
| Org choice | The list of a user's organizations is returned only after the password is verified |
| Audit | Sign-in success/failure, logout, session revocation and org changes are recorded in-transaction with request id, IP and user agent |

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
