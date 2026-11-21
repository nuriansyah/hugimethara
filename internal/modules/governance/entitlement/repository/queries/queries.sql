-- governance/entitlement repository — Read side (side-effect-free statements).
-- Entitlements: plan catalog, subscriptions, overrides.

-- name: GetSubscription
SELECT s.plan_key, p.name AS plan_name, s.status, s.current_period_end
FROM org_subscriptions s JOIN plans p ON p.key = s.plan_key
WHERE s.org_id = $1;

-- name: EffectiveFeatures
-- Every catalog feature with the org's effective entitlement: an unexpired
-- override wins over the plan; features absent from both are disabled.
SELECT f.key, f.name, f.description, f.kind, f.unit,
       COALESCE(o.enabled, pe.enabled, false) AS enabled,
       CASE WHEN o.feature_key IS NOT NULL THEN o.limit_value ELSE pe.limit_value END AS limit_value,
       (o.feature_key IS NOT NULL) AS overridden
FROM features f
LEFT JOIN plan_entitlements pe ON pe.feature_key = f.key AND pe.plan_key = $2
LEFT JOIN org_entitlement_overrides o
       ON o.feature_key = f.key AND o.org_id = $1 AND (o.expires_at IS NULL OR o.expires_at > now())
ORDER BY f.key;

-- name: LockFeature
-- Serializes check-then-create for one org + feature inside a transaction.
SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0));

-- name: ListPlans
SELECT key, name, description, price_monthly_cents, currency FROM plans ORDER BY sort_order;

-- name: ListPlanEntitlements
SELECT plan_key, feature_key, enabled, limit_value FROM plan_entitlements;
