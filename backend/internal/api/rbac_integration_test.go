//go:build integration

package api_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func roleIDByKey(t *testing.T, c *client, key string) string {
	t.Helper()
	r := c.do("GET", base+"/roles", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	for _, role := range data(r) {
		if role["key"] == key {
			return role["id"].(string)
		}
	}
	t.Fatalf("role %s not found", key)
	return ""
}

func orgBinding(t *testing.T, c *client, userID, roleKey string) string {
	t.Helper()
	r := c.do("GET", base+"/role-bindings?user_id="+userID+"&scope_type=org", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	for _, b := range data(r) {
		if b["role"].(map[string]any)["key"] == roleKey {
			return b["id"].(string)
		}
	}
	return ""
}

func assign(c *client, userID, roleID string, scope map[string]any) resp {
	return c.do("POST", base+"/role-bindings", map[string]any{"user_id": userID, "role_id": roleID, "scope": scope}, true)
}

func perms(t *testing.T, c *client) (org []string, ws map[string]any) {
	t.Helper()
	r := c.do("GET", base+"/me/permissions", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	for _, p := range r.Body["organization"].([]any) {
		org = append(org, p.(string))
	}
	return org, r.Body["workspaces"].(map[string]any)
}

func TestTemplatesAndEffectivePermissions(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("rbac-co", "owner@rbac.test"), false).Status)

	var keys []string
	for _, r := range data(owner.do("GET", base+"/roles", nil, false)) {
		keys = append(keys, r["key"].(string))
	}
	require.ElementsMatch(t, []string{"org_owner", "org_admin", "member", "workspace_admin", "editor", "viewer"}, keys)

	org, _ := perms(t, owner)
	require.Contains(t, org, "role:bind")
	require.Contains(t, org, "workspace:create")
	require.True(t, owner.do("GET", base+"/me", nil, false).Body["membership"].(map[string]any)["is_owner"].(bool))

	carol, carolID := addMember(t, e, "rbac-co", "carol@rbac.test")
	org, ws := perms(t, carol)
	require.ElementsMatch(t, []string{"organization:read", "department:read", "environment:read", "subscription:read"}, org)
	require.Empty(t, ws)
	require.False(t, carol.do("GET", base+"/me", nil, false).Body["membership"].(map[string]any)["is_owner"].(bool))

	// Workspace roles show up per workspace.
	w := owner.do("POST", base+"/workspaces", map[string]any{"slug": "ops", "name": "Ops"}, true)
	require.Equal(t, 201, w.Status)
	require.Equal(t, 204, owner.do("POST", base+"/workspaces/ops/members", map[string]any{"user_id": carolID, "role_key": "viewer"}, true).Status)
	_, ws = perms(t, carol)
	require.Equal(t, []any{"workspace:read"}, ws[w.Body["id"].(string)])
	members := data(owner.do("GET", base+"/workspaces/ops/members", nil, false))
	for _, m := range members {
		if m["id"] == carolID {
			require.Equal(t, []any{"viewer"}, m["roles"])
		} else {
			require.Equal(t, []any{"workspace_admin"}, m["roles"], "the creator administers the workspace")
		}
	}
}

func TestCustomRoleGrantsAndRevokesImmediately(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("audit-co", "owner@audit.test"), false).Status)
	carol, carolID := addMember(t, e, "audit-co", "carol@audit.test")
	require.Equal(t, 403, carol.do("GET", base+"/audit-events", nil, false).Status)

	r := owner.do("POST", base+"/roles", map[string]any{"key": "auditor", "name": "Auditor", "permissions": []string{"audit_event:read", "audit_event:read"}}, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, []any{"audit_event:read"}, r.Body["permissions"], "de-duplicated")
	roleID := r.Body["id"].(string)

	b := assign(owner, carolID, roleID, map[string]any{"type": "org"})
	require.Equal(t, 201, b.Status, b.Body)
	require.Equal(t, 409, assign(owner, carolID, roleID, map[string]any{"type": "org"}).Status, "already assigned")
	require.Equal(t, 200, carol.do("GET", base+"/audit-events", nil, false).Status)

	// Editing the role takes effect on the next request (version-stamped cache).
	r = owner.do("PUT", base+"/roles/"+roleID, map[string]any{"name": "Auditor", "permissions": []string{"user:read"}}, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, 403, carol.do("GET", base+"/audit-events", nil, false).Status)
	require.Equal(t, 200, carol.do("GET", base+"/users", nil, false).Status)

	require.Equal(t, 409, owner.do("DELETE", base+"/roles/"+roleID, nil, true).Status, "role in use")
	require.Equal(t, 204, owner.do("DELETE", base+"/role-bindings/"+b.Body["id"].(string), nil, true).Status)
	require.Equal(t, 403, carol.do("GET", base+"/users", nil, false).Status, "revocation is immediate")
	require.Equal(t, 204, owner.do("DELETE", base+"/roles/"+roleID, nil, true).Status)

	// Validation and built-in protections.
	r = owner.do("POST", base+"/roles", map[string]any{"key": "bad", "name": "Bad", "permissions": []string{"workspace:fly"}}, true)
	require.Equal(t, 422, r.Status)
	require.Equal(t, "unknown_permission", r.Body["errors"].([]any)[0].(map[string]any)["code"])
	ownerRole := roleIDByKey(t, owner, "org_owner")
	require.Equal(t, 409, owner.do("PUT", base+"/roles/"+ownerRole, map[string]any{"name": "Owner", "permissions": []string{"organization:read"}}, true).Status)
	require.Equal(t, 409, owner.do("DELETE", base+"/roles/"+roleIDByKey(t, owner, "viewer"), nil, true).Status, "built-in roles cannot be deleted")

	// Audit trail records role administration.
	actions := auditActions(t, owner, "?action=role")
	for _, want := range []string{"role.created:success", "role.assigned:success", "role.updated:success", "role.unassigned:success", "role.deleted:success"} {
		require.Contains(t, actions, want)
	}
}

func TestNoPrivilegeEscalation(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("esc-co", "owner@esc.test"), false).Status)
	dave, daveID := addMember(t, e, "esc-co", "dave@esc.test")
	_, carolID := addMember(t, e, "esc-co", "carol@esc.test")

	// Dave may manage roles but only holds a few permissions himself.
	r := owner.do("POST", base+"/roles", map[string]any{"key": "role_manager", "name": "Role manager",
		"permissions": []string{"role:read", "role:create", "role:update", "role:bind", "workspace:read"}}, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, 201, assign(owner, daveID, r.Body["id"].(string), map[string]any{"type": "org"}).Status)

	// He cannot create or widen roles beyond what he holds…
	r = dave.do("POST", base+"/roles", map[string]any{"key": "godmode", "name": "God", "permissions": []string{"*:*"}}, true)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "privilege_escalation", r.Body["code"])
	require.Equal(t, []any{"*:*"}, r.Body["missing_permissions"])
	viewerID := roleIDByKey(t, dave, "viewer")
	r = dave.do("PUT", base+"/roles/"+viewerID, map[string]any{"name": "Viewer", "permissions": []string{"workspace:read", "audit_event:read"}}, true)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "privilege_escalation", r.Body["code"])

	// …nor hand out roles containing permissions he lacks, including to himself.
	r = assign(dave, daveID, roleIDByKey(t, dave, "org_owner"), map[string]any{"type": "org"})
	require.Equal(t, 403, r.Status)
	require.Equal(t, "privilege_escalation", r.Body["code"])
	require.Equal(t, 403, assign(dave, carolID, roleIDByKey(t, dave, "org_admin"), map[string]any{"type": "org"}).Status)

	// What he does hold, he can delegate.
	require.Equal(t, 201, assign(dave, carolID, viewerID, map[string]any{"type": "org"}).Status)
	r = dave.do("POST", base+"/roles", map[string]any{"key": "reader", "name": "Reader", "permissions": []string{"workspace:read"}}, true)
	require.Equal(t, 201, r.Status, r.Body)

	// Nothing escalated: Dave's own permissions are unchanged.
	org, _ := perms(t, dave)
	require.NotContains(t, org, "audit_event:read")
	require.Equal(t, 403, dave.do("GET", base+"/audit-events", nil, false).Status)
}

func TestLastOwnerCannotBeRemoved(t *testing.T) {
	e := setup(t)
	owner := e.client()
	r := owner.do("POST", base+"/organizations", signupBody("own-co", "owner@own.test"), false)
	require.Equal(t, 201, r.Status)
	ownerID := r.Body["user"].(map[string]any)["id"].(string)
	carol, carolID := addMember(t, e, "own-co", "carol@own.test")

	ownBinding := orgBinding(t, owner, ownerID, "org_owner")
	r = owner.do("DELETE", base+"/role-bindings/"+ownBinding, nil, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "last_owner", r.Body["code"])

	// Hand over ownership, then step down.
	require.Equal(t, 201, assign(owner, carolID, roleIDByKey(t, owner, "org_owner"), map[string]any{"type": "org"}).Status)
	require.Equal(t, 204, owner.do("DELETE", base+"/role-bindings/"+ownBinding, nil, true).Status)
	require.Equal(t, 403, owner.do("GET", base+"/role-bindings", nil, false).Status, "former owner lost role:read")
	require.True(t, carol.do("GET", base+"/me", nil, false).Body["membership"].(map[string]any)["is_owner"].(bool))
}

func TestWorkspaceAdminIsScopedToTheirWorkspace(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("scope-co", "owner@scope.test"), false).Status)
	for _, s := range []string{"alpha", "beta"} {
		require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": s, "name": s}, true).Status)
	}
	erin, erinID := addMember(t, e, "scope-co", "erin@scope.test")
	_, frankID := addMember(t, e, "scope-co", "frank@scope.test")
	require.Equal(t, 204, owner.do("POST", base+"/workspaces/alpha/members", map[string]any{"user_id": erinID, "role_key": "workspace_admin"}, true).Status)

	// Erin administers alpha…
	require.Equal(t, 204, erin.do("POST", base+"/workspaces/alpha/members", map[string]any{"user_id": frankID}, true).Status)
	require.Equal(t, 200, erin.do("PATCH", base+"/workspaces/alpha", map[string]any{"name": "Alpha team"}, true).Status)
	require.Equal(t, []string{"alpha"}, slugs(erin.do("GET", base+"/workspaces", nil, false)))
	// …cannot grant roles beyond her own there…
	r := erin.do("POST", base+"/workspaces/alpha/members", map[string]any{"user_id": frankID, "role_key": "org_owner"}, true)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "privilege_escalation", r.Body["code"])
	// …and has no reach into beta or org-level actions.
	require.Equal(t, 404, erin.do("POST", base+"/workspaces/beta/members", map[string]any{"user_id": frankID}, true).Status)
	require.Equal(t, 404, erin.do("PATCH", base+"/workspaces/beta", map[string]any{"name": "x"}, true).Status)
	require.Equal(t, 403, erin.do("POST", base+"/workspaces", map[string]any{"slug": "gamma", "name": "g"}, true).Status)
	require.Equal(t, 403, erin.do("POST", base+"/environments", map[string]any{"slug": "qa", "name": "QA"}, true).Status)

	// Removing frank revokes access at once.
	require.Equal(t, 204, erin.do("DELETE", base+"/workspaces/alpha/members/"+frankID, nil, true).Status)
	require.Equal(t, 404, erin.do("DELETE", base+"/workspaces/alpha/members/"+frankID, nil, true).Status)
}

func TestSimulationAndDecisionLog(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("sim-co", "owner@sim.test"), false).Status)
	w := owner.do("POST", base+"/workspaces", map[string]any{"slug": "fin", "name": "Finance"}, true)
	wsID := w.Body["id"].(string)
	carol, carolID := addMember(t, e, "sim-co", "carol@sim.test")
	require.Equal(t, 204, owner.do("POST", base+"/workspaces/fin/members", map[string]any{"user_id": carolID, "role_key": "editor"}, true).Status)

	r := owner.do("POST", base+"/authorization/evaluate", map[string]any{"user_id": carolID, "resource_type": "workspace", "action": "read", "workspace_id": wsID}, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, true, r.Body["allowed"])
	require.Contains(t, r.Body["matched_rules"].([]any)[0], "editor@workspace")
	r = owner.do("POST", base+"/authorization/evaluate", map[string]any{"user_id": carolID, "resource_type": "workspace", "action": "manage_members", "workspace_id": wsID}, true)
	require.Equal(t, false, r.Body["allowed"])
	require.Equal(t, "no_matching_binding", r.Body["reason"])
	require.Equal(t, 422, owner.do("POST", base+"/authorization/evaluate", map[string]any{"user_id": carolID, "resource_type": "workspace", "action": "teleport"}, true).Status)
	require.Equal(t, 403, carol.do("POST", base+"/authorization/evaluate", map[string]any{"user_id": carolID, "resource_type": "workspace", "action": "read"}, true).Status)

	// Real denials are logged; simulations are not.
	require.Equal(t, 403, carol.do("GET", base+"/users", nil, false).Status)
	d := owner.do("GET", base+"/authorization/decisions?allowed=false", nil, false)
	require.Equal(t, 200, d.Status, d.Body)
	var denied []string
	for _, x := range data(d) {
		denied = append(denied, x["resource_type"].(string)+":"+x["action"].(string)+"@"+x["principal_email"].(string))
	}
	require.Contains(t, denied, "user:read@carol@sim.test")
	require.Contains(t, denied, "authorization:simulate@carol@sim.test")
	require.False(t, slices.Contains(denied, "workspace:manage_members@carol@sim.test"), "simulation must not be logged")
	// Sensitive allows are logged too (role:bind / workspace:manage_members by the owner).
	all := data(owner.do("GET", base+"/authorization/decisions?allowed=true", nil, false))
	var allowed []string
	for _, x := range all {
		allowed = append(allowed, x["resource_type"].(string)+":"+x["action"].(string))
	}
	require.Contains(t, allowed, "workspace:manage_members")
}

func TestRolesAreTenantScoped(t *testing.T) {
	e := setup(t)
	a, b := e.client(), e.client()
	require.Equal(t, 201, a.do("POST", base+"/organizations", signupBody("ten-a", "owner@ten-a.test"), false).Status)
	rb := b.do("POST", base+"/organizations", signupBody("ten-b", "owner@ten-b.test"), false)
	require.Equal(t, 201, rb.Status)
	bOwnerID := rb.Body["user"].(map[string]any)["id"].(string)

	aOwnerRole := roleIDByKey(t, a, "org_owner")
	require.Equal(t, 404, b.do("GET", base+"/roles/"+aOwnerRole, nil, false).Status)
	require.Equal(t, 404, b.do("PUT", base+"/roles/"+aOwnerRole, map[string]any{"name": "x", "permissions": []string{}}, true).Status)
	r := assign(b, bOwnerID, aOwnerRole, map[string]any{"type": "org"})
	require.Equal(t, 422, r.Status, "another org's role id is unknown here")
	r = assign(a, bOwnerID, aOwnerRole, map[string]any{"type": "org"})
	require.Equal(t, 422, r.Status, "cannot grant access to a user from another organization")
	for _, binding := range data(a.do("GET", base+"/role-bindings", nil, false)) {
		require.Equal(t, "owner@ten-a.test", binding["user"].(map[string]any)["email"])
	}
}
