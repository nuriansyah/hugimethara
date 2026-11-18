# ADR-0006: Server-side sessions, OIDC federation, local auth for bootstrap

**Status:** Proposed

## Decision
- Browser auth uses **opaque session cookies** (`HttpOnly; Secure; SameSite=Lax`, 256-bit random id, stored hashed
  in Postgres, cached in Redis). Idle timeout 8h, absolute 7d (org-configurable lower). Revocable instantly.
  No JWTs in the browser.
- **OIDC Authorization Code + PKCE** via `coreos/go-oidc` + `x/oauth2`, per-org IdP config (issuer, client id,
  encrypted client secret, scopes, claim mappings, allowed email domains). Presets for Entra ID, Google Workspace,
  Keycloak; generic OIDC for others. Users are JIT-provisioned or must be pre-invited (per IdP setting).
  IdP groups claim → `group_mappings` → internal groups (basic in Phase 1, full sync in Phase 3).
- **Local password auth** (argon2id, breach-length policy, lockout) for bootstrap owners and orgs without an IdP;
  org admins can enforce "SSO only". TOTP MFA is P1.
- **Service accounts** authenticate with API keys (`hgm_` prefix + random; only hash stored; shown once; scoped
  expiry). Machine requests are CSRF-exempt because they use the `Authorization` header, not cookies.
- SAML/SCIM: `identity_providers.protocol` and a `Provisioner` interface reserved; implemented in Phase 3.

## Alternatives
Delegating all auth to Keycloak (would force an IdP on every customer, contradicts "onboard without replacing IdP");
stateless JWT sessions (no instant revocation).
