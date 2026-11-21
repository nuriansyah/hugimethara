-- governance/entitlement repository — Write side (state-changing statements).
-- Entitlements: plan catalog, subscriptions, overrides.

-- name: ProvisionDefaultSubscription
INSERT INTO org_subscriptions (org_id, plan_key)
SELECT $1, key FROM plans WHERE is_default;
