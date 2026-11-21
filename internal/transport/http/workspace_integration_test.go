//go:build integration

package api_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	identitysvc "github.com/nuriansyah/hugimethara/internal/modules/identity/services"
)

// setPlan changes an org's plan directly, as `hugi-migrate admin` does.
func setPlan(t *testing.T, e *env, slug, plan string) {
	t.Helper()
	_, err := e.pg.Owner.ExecContext(context.Background(), `UPDATE org_subscriptions SET plan_key = $2
		WHERE org_id = (SELECT id FROM organizations WHERE slug = $1)`, slug, plan)
	require.NoError(t, err)
}

// addMember creates a non-owner member of orgSlug (fixture: invitations arrive
// in S4) and returns a signed-in client for them.
func addMember(t *testing.T, e *env, orgSlug, email string) (*client, string) {
	t.Helper()
	ctx := context.Background()
	hash, err := identitysvc.NewHasher(testArgon).Hash("correct horse battery")
	require.NoError(t, err)
	id := uuid.Must(uuid.NewV7())
	_, err = e.pg.Owner.ExecContext(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3)`, id, email, email)
	require.NoError(t, err)
	_, err = e.pg.Owner.ExecContext(ctx, `INSERT INTO org_memberships (org_id, user_id) SELECT id, $2 FROM organizations WHERE slug = $1`, orgSlug, id)
	require.NoError(t, err)
	_, err = e.pg.Owner.ExecContext(ctx, `INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)`, id, hash)
	require.NoError(t, err)
	bindMemberRole(t, e, orgSlug, email)
	c := e.client()
	r := c.do("POST", base+"/auth/login", map[string]any{"email": email, "password": "correct horse battery", "organization_slug": orgSlug}, false)
	require.Equal(t, 200, r.Status, r.Body)
	return c, id.String()
}

func data(r resp) []map[string]any {
	var out []map[string]any
	for _, x := range r.Body["data"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

func slugs(r resp) []string {
	var out []string
	for _, w := range data(r) {
		out = append(out, w["slug"].(string))
	}
	return out
}

func entitlementOf(t *testing.T, c *client, key string) map[string]any {
	t.Helper()
	r := c.do("GET", base+"/entitlements", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	for _, f := range r.Body["features"].([]any) {
		if f.(map[string]any)["key"] == key {
			return f.(map[string]any)
		}
	}
	t.Fatalf("feature %s missing", key)
	return nil
}

func TestPlansGateFeaturesAndQuotas(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("quota-co", "owner@quota.test"), false).Status)

	// New orgs start on Free with the three default environments.
	ent := owner.do("GET", base+"/entitlements", nil, false)
	require.Equal(t, "free", ent.Body["plan"].(map[string]any)["key"])
	require.ElementsMatch(t, []string{"development", "staging", "production"}, slugs(owner.do("GET", base+"/environments", nil, false)))

	// Free allows 3 active workspaces.
	for i := 1; i <= 3; i++ {
		r := owner.do("POST", base+"/workspaces", map[string]any{"slug": fmt.Sprintf("ws-%d", i), "name": fmt.Sprintf("WS %d", i)}, true)
		require.Equal(t, 201, r.Status, r.Body)
		require.EqualValues(t, 1, r.Body["member_count"], "creator becomes a member")
	}
	r := owner.do("POST", base+"/workspaces", map[string]any{"slug": "ws-4", "name": "WS 4"}, true)
	require.Equal(t, 402, r.Status)
	require.Equal(t, "plan_limit_reached", r.Body["code"])
	require.EqualValues(t, 3, r.Body["limit"])
	require.Equal(t, "workspaces.max", r.Body["feature"])
	ws := entitlementOf(t, owner, "workspaces.max")
	require.EqualValues(t, 3, ws["used"])
	require.EqualValues(t, 3, ws["limit"])

	// Archiving frees a slot; restoring needs one again.
	require.Equal(t, 200, owner.do("POST", base+"/workspaces/ws-1/archive", nil, true).Status)
	require.Equal(t, 409, owner.do("POST", base+"/workspaces/ws-1/archive", nil, true).Status)
	require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": "ws-4", "name": "WS 4"}, true).Status)
	r = owner.do("POST", base+"/workspaces/ws-1/restore", nil, true)
	require.Equal(t, 402, r.Status)
	require.NotContains(t, slugs(owner.do("GET", base+"/workspaces", nil, false)), "ws-1")
	require.Contains(t, slugs(owner.do("GET", base+"/workspaces?include_archived=true", nil, false)), "ws-1")

	// Paid features are refused on Free.
	r = owner.do("POST", base+"/departments", map[string]any{"slug": "finance", "name": "Finance"}, true)
	require.Equal(t, 402, r.Status)
	require.Equal(t, "feature_not_in_plan", r.Body["code"])
	r = owner.do("POST", base+"/environments", map[string]any{"slug": "qa", "name": "QA"}, true)
	require.Equal(t, 402, r.Status, "Free allows 3 environments")
	envs := data(owner.do("GET", base+"/environments", nil, false))
	envID := envs[0]["id"].(string)
	r = owner.do("PATCH", base+"/environments/"+envID, map[string]any{"name": "Dev", "is_production": false,
		"egress_allowlist": []string{"10.20.0.0/16"}}, true)
	require.Equal(t, 402, r.Status)

	// Audit history is limited by plan: a 40-day-old event is hidden on Free.
	_, err := e.pg.Owner.ExecContext(context.Background(), `INSERT INTO audit_events (id, org_id, occurred_at, actor_type, action, resource_type)
		SELECT uuidv7(), id, now() - interval '40 days', 'system', 'test.old_event', 'test' FROM organizations WHERE slug = 'quota-co'`)
	require.NoError(t, err)
	require.Empty(t, auditActions(t, owner, "?action=test"))

	// Upgrade to Pro (operator action) unlocks everything above.
	setPlan(t, e, "quota-co", "pro")
	require.Equal(t, "pro", owner.do("GET", base+"/entitlements", nil, false).Body["plan"].(map[string]any)["key"])
	require.Equal(t, []string{"test.old_event:success"}, auditActions(t, owner, "?action=test"))
	r = owner.do("POST", base+"/departments", map[string]any{"slug": "finance", "name": "Finance"}, true)
	require.Equal(t, 201, r.Status, r.Body)
	deptID := r.Body["id"].(string)
	require.Equal(t, 200, owner.do("POST", base+"/workspaces/ws-1/restore", nil, true).Status)
	r = owner.do("POST", base+"/workspaces", map[string]any{"slug": "fin-ops", "name": "Finance Ops", "department_id": deptID}, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, "Finance", r.Body["department_name"])
	require.Equal(t, 201, owner.do("POST", base+"/environments", map[string]any{"slug": "qa", "name": "QA"}, true).Status)

	// Egress allowlist: validated and canonicalised.
	r = owner.do("PATCH", base+"/environments/"+envID, map[string]any{"name": "Dev", "is_production": false,
		"egress_allowlist": []string{"10.20.3.4/16", "192.168.1.5", "10.20.0.0/16"}}, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, []any{"10.20.0.0/16", "192.168.1.5/32"}, r.Body["egress_allowlist"])
	for _, bad := range []string{"not-an-ip", "0.0.0.0/0"} {
		r = owner.do("PATCH", base+"/environments/"+envID, map[string]any{"name": "Dev", "is_production": false, "egress_allowlist": []string{bad}}, true)
		require.Equal(t, 422, r.Status, bad)
	}

	// Deleting a department unassigns its workspaces.
	require.Equal(t, 204, owner.do("DELETE", base+"/departments/"+deptID, nil, true).Status)
	require.Nil(t, owner.do("GET", base+"/workspaces/fin-ops", nil, false).Body["department_id"])

	// Downgrade keeps existing data but blocks new creation over the quota.
	setPlan(t, e, "quota-co", "free")
	require.Equal(t, 402, owner.do("POST", base+"/workspaces", map[string]any{"slug": "ws-9", "name": "WS 9"}, true).Status)
	require.Len(t, data(owner.do("GET", base+"/workspaces", nil, false)), 5)

	// Per-org override (operator grant) beats the plan.
	_, err = e.pg.Owner.ExecContext(context.Background(), `INSERT INTO org_entitlement_overrides (org_id, feature_key, enabled, limit_value, reason)
		SELECT id, 'workspaces.max', true, 6, 'pilot deal' FROM organizations WHERE slug = 'quota-co'`)
	require.NoError(t, err)
	require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": "ws-9", "name": "WS 9"}, true).Status)
	require.Equal(t, true, entitlementOf(t, owner, "workspaces.max")["overridden"])
}

func TestConcurrentCreatesCannotExceedQuota(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("race-co", "owner@race.test"), false).Status)

	var wg sync.WaitGroup
	statuses := make([]int, 8)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = owner.do("POST", base+"/workspaces", map[string]any{"slug": fmt.Sprintf("p-%d", i), "name": "P"}, true).Status
		}()
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		require.Contains(t, []int{201, 402}, s)
		if s == 201 {
			created++
		}
	}
	require.Equal(t, 3, created, "exactly the Free quota, even under concurrency: %v", statuses)
}

func TestWorkspaceVisibilityForMembers(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("team-co", "owner@team.test"), false).Status)
	for _, s := range []string{"marketing", "finance"} {
		require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": s, "name": s}, true).Status)
	}
	carol, carolID := addMember(t, e, "team-co", "carol@team.test")

	// A member sees only workspaces they belong to.
	require.Empty(t, data(carol.do("GET", base+"/workspaces", nil, false)))
	require.Equal(t, 404, carol.do("GET", base+"/workspaces/finance", nil, false).Status)

	require.Equal(t, 204, owner.do("POST", base+"/workspaces/marketing/members", map[string]any{"user_id": carolID}, true).Status)
	require.Equal(t, 204, owner.do("POST", base+"/workspaces/marketing/members", map[string]any{"user_id": carolID}, true).Status, "idempotent")
	require.Equal(t, []string{"marketing"}, slugs(carol.do("GET", base+"/workspaces", nil, false)))
	require.Equal(t, 200, carol.do("GET", base+"/workspaces/marketing", nil, false).Status)
	require.Len(t, data(carol.do("GET", base+"/workspaces/marketing/members", nil, false)), 2)
	require.Equal(t, 404, carol.do("GET", base+"/workspaces/finance/members", nil, false).Status)

	// Members cannot administer.
	require.Equal(t, 403, carol.do("POST", base+"/workspaces/marketing/members", map[string]any{"user_id": carolID}, true).Status)
	require.Equal(t, 404, carol.do("POST", base+"/workspaces/finance/members", map[string]any{"user_id": carolID}, true).Status,
		"no existence oracle for workspaces the member cannot read")
	require.Equal(t, 403, carol.do("POST", base+"/workspaces", map[string]any{"slug": "x", "name": "x"}, true).Status)
	require.Equal(t, 403, carol.do("PATCH", base+"/workspaces/marketing", map[string]any{"name": "Mine"}, true).Status)
	require.Equal(t, 403, carol.do("GET", base+"/users", nil, false).Status)
	require.Equal(t, 200, carol.do("GET", base+"/environments", nil, false).Status)
	require.Equal(t, 200, carol.do("GET", base+"/entitlements", nil, false).Status)

	// Users from another organization cannot be added.
	other := e.client()
	r := other.do("POST", base+"/organizations", signupBody("other-co", "x@other.test"), false)
	require.Equal(t, 201, r.Status)
	strangerID := r.Body["user"].(map[string]any)["id"].(string)
	r = owner.do("POST", base+"/workspaces/marketing/members", map[string]any{"user_id": strangerID}, true)
	require.Equal(t, 422, r.Status)
	require.Equal(t, "user_id", r.Body["errors"].([]any)[0].(map[string]any)["field"])

	// Removal takes effect immediately; archived workspaces are frozen.
	require.Equal(t, 204, owner.do("DELETE", base+"/workspaces/marketing/members/"+carolID, nil, true).Status)
	require.Empty(t, data(carol.do("GET", base+"/workspaces", nil, false)))
	require.Equal(t, 200, owner.do("POST", base+"/workspaces/finance/archive", nil, true).Status)
	require.Equal(t, 409, owner.do("POST", base+"/workspaces/finance/members", map[string]any{"user_id": carolID}, true).Status)
}

// Merchant A must never see or touch merchant B's data, even when both use
// the same workspace names, and even when guessing ids.
func TestMerchantIsolation(t *testing.T) {
	e := setup(t)
	a, b := e.client(), e.client()
	require.Equal(t, 201, a.do("POST", base+"/organizations", signupBody("merchant-a", "owner@a.test"), false).Status)
	require.Equal(t, 201, b.do("POST", base+"/organizations", signupBody("merchant-b", "owner@b.test"), false).Status)
	setPlan(t, e, "merchant-b", "pro")

	ra := a.do("POST", base+"/workspaces", map[string]any{"slug": "sales", "name": "A Sales", "description": "secret A"}, true)
	rb := b.do("POST", base+"/workspaces", map[string]any{"slug": "sales", "name": "B Sales"}, true)
	require.Equal(t, 201, ra.Status)
	require.Equal(t, 201, rb.Status, "slugs are unique per organization, not globally")
	require.NotEqual(t, ra.Body["id"], rb.Body["id"])
	require.Equal(t, "B Sales", b.do("GET", base+"/workspaces/sales", nil, false).Body["name"])
	require.Equal(t, "A Sales", a.do("GET", base+"/workspaces/sales", nil, false).Body["name"])

	deptB := b.do("POST", base+"/departments", map[string]any{"slug": "ops", "name": "Ops"}, true)
	require.Equal(t, 201, deptB.Status)
	envB := data(b.do("GET", base+"/environments", nil, false))[0]["id"].(string)

	// A guessing B's ids gets 404s, never B's data, and changes nothing.
	require.Equal(t, 404, a.do("PATCH", base+"/departments/"+deptB.Body["id"].(string), map[string]any{"name": "pwned"}, true).Status)
	require.Equal(t, 404, a.do("DELETE", base+"/departments/"+deptB.Body["id"].(string), nil, true).Status)
	require.Equal(t, 404, a.do("PATCH", base+"/environments/"+envB, map[string]any{"name": "pwned", "is_production": true}, true).Status)
	require.Equal(t, 404, a.do("DELETE", base+"/environments/"+envB, nil, true).Status)
	r := a.do("POST", base+"/workspaces", map[string]any{"slug": "steal", "name": "x", "department_id": deptB.Body["id"]}, true)
	require.NotEqual(t, 201, r.Status, "cannot attach another merchant's department")

	// Lists contain only own data.
	for _, d := range data(a.do("GET", base+"/users", nil, false)) {
		require.Equal(t, "owner@a.test", d["email"])
	}
	require.Empty(t, data(a.do("GET", base+"/departments", nil, false)))
	require.Equal(t, "Ops", data(b.do("GET", base+"/departments", nil, false))[0]["name"])
	envsA := data(a.do("GET", base+"/environments", nil, false))
	for _, env := range envsA {
		require.NotEqual(t, envB, env["id"])
	}
	// Plans are per merchant too.
	require.Equal(t, "free", a.do("GET", base+"/entitlements", nil, false).Body["plan"].(map[string]any)["key"])
	require.Equal(t, "pro", b.do("GET", base+"/entitlements", nil, false).Body["plan"].(map[string]any)["key"])
	// And audit trails.
	for _, act := range auditActions(t, a, "?limit=200") {
		require.NotContains(t, act, "department.created")
	}
}
