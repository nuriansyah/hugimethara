# ADR-0013: Plans, paid features and quotas (entitlements)

**Status:** Accepted (2026-11-19)

## Context
Hugimethara is sold in tiers. Each organization ("merchant") must only get the features and capacity of its plan,
enforced by the server, while its data stays isolated from every other organization (ADR-0002).

## Decision
- Global catalog: `features` (flag or limit), `plans`, `plan_entitlements`. Per organization: `org_subscriptions`
  (plan + status) and `org_entitlement_overrides` (operator grants, optional expiry). Override > plan;
  missing = not included; NULL limit = unlimited; a canceled subscription falls back to the default plan.
- The `entitlement` package is a leaf (platform-only dependencies) so every domain can enforce it inside its
  own transaction: `ent.Require(feature)` and `ent.CheckLimit(feature, current, adding)`.
- Quota checks take a transaction-scoped advisory lock per (org, feature) before counting, so concurrent requests
  cannot overshoot a limit (integration-tested with 8 parallel creates).
- Violations return **HTTP 402** with `feature_not_in_plan` or `plan_limit_reached` plus `feature`, `plan`,
  `limit`, `current`, so the UI can explain the limit and link to plan comparison.
- The runtime role can only *read* plans and insert the default subscription at signup. Plan changes and overrides
  are operator actions via `hugi-admin` (audited as `system` in the org's audit log); a billing-provider
  integration will later drive the same tables under its own role.
- Downgrades never delete data; they block new creation above the new limits.

## Enforced today
| Feature | Free | Pro | Enterprise |
|---|---|---|---|
| `workspaces.max` (active) | 3 | 25 | unlimited |
| `environments.max` | 3 | 10 | unlimited |
| `departments` | – | ✓ | ✓ |
| `environments.egress_allowlist` | – | ✓ | ✓ |
| `audit_log.retention_days` (viewable history) | 30 | 365 | unlimited |

New paid features are added with the slice that ships them (one catalog row + plan rows in a migration, one
`Require`/`CheckLimit` call in the owning service, one integration test).
