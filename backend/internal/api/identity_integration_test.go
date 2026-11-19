//go:build integration

package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func invite(t *testing.T, c *client, body map[string]any) (string, map[string]any) {
	t.Helper()
	r := c.do("POST", base+"/invitations", body, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, "/invite/"+r.Body["token"].(string), r.Body["accept_path"])
	return r.Body["token"].(string), r.Body["invitation"].(map[string]any)
}

func workspaceID(t *testing.T, c *client, slug string) string {
	t.Helper()
	r := c.do("GET", base+"/workspaces/"+slug, nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	return r.Body["id"].(string)
}

// bearer is an API-key client (no cookies, no CSRF).
type bearer struct {
	c   *client
	key string
}

func (b bearer) do(method, path string, body any) resp {
	b.c.e.t.Helper()
	return b.c.doWithHeader(method, path, body, http.Header{"Authorization": {"Bearer " + b.key}})
}

func TestInvitedUserJoinsWithWorkspaceRole(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("inv-co", "owner@inv.test"), false).Status)
	require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": "ops", "name": "Ops"}, true).Status)
	require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": "fin", "name": "Finance"}, true).Status)
	opsID := workspaceID(t, owner, "ops")

	token, inv := invite(t, owner, map[string]any{"email": "New@Inv.Test",
		"workspaces": []map[string]any{{"workspace_id": opsID, "role_id": roleIDByKey(t, owner, "editor")}}})
	require.Equal(t, "new@inv.test", inv["email"], "normalized")
	require.Equal(t, "pending", inv["status"])
	require.Equal(t, "Ops", inv["workspaces"].([]any)[0].(map[string]any)["workspace_name"])
	require.Len(t, data(owner.do("GET", base+"/invitations", nil, false)), 1)

	// The public link shows what it is for.
	anon := e.client()
	pv := anon.do("GET", base+"/invitation-tokens/"+token, nil, false)
	require.Equal(t, 200, pv.Status, pv.Body)
	require.Equal(t, "Org inv-co", pv.Body["organization"].(map[string]any)["name"])
	require.Equal(t, false, pv.Body["account_exists"])
	require.Equal(t, 404, anon.do("GET", base+"/invitation-tokens/not-a-token", nil, false).Status)

	// New account: validated, then joined and signed in.
	r := anon.do("POST", base+"/invitation-tokens/"+token+"/accept", map[string]any{"display_name": "Nina", "password": "short"}, false)
	require.Equal(t, 422, r.Status, r.Body)
	r = anon.do("POST", base+"/invitation-tokens/"+token+"/accept", map[string]any{"display_name": "Nina", "password": "correct horse battery"}, false)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "inv-co", r.Body["organization"].(map[string]any)["slug"])
	me := anon.do("GET", base+"/me", nil, false)
	require.Equal(t, 200, me.Status)
	require.Equal(t, "Nina", me.Body["user"].(map[string]any)["display_name"])
	org, ws := perms(t, anon)
	require.Contains(t, org, "organization:read", "baseline member role")
	require.Equal(t, []any{"workspace:read"}, ws[opsID], "editor in the invited workspace")
	require.Equal(t, []string{"ops"}, slugs(anon.do("GET", base+"/workspaces", nil, false)))

	// Single use.
	require.Equal(t, 410, anon.do("POST", base+"/invitation-tokens/"+token+"/accept", map[string]any{"password": "correct horse battery"}, true).Status)
	require.Equal(t, 410, e.client().do("GET", base+"/invitation-tokens/"+token, nil, false).Status)
	all := data(owner.do("GET", base+"/invitations?status=all", nil, false))
	require.Equal(t, "accepted", all[0]["status"])
	require.Empty(t, data(owner.do("GET", base+"/invitations", nil, false)), "no open invitations left")

	// Inviting a member again is pointless.
	r = owner.do("POST", base+"/invitations", map[string]any{"email": "new@inv.test"}, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "already_member", r.Body["code"])

	// Someone with an account elsewhere proves it with their password.
	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations", signupBody("other-co", "olga@other.test"), false).Status)
	token, _ = invite(t, owner, map[string]any{"email": "olga@other.test", "org_role_id": roleIDByKey(t, owner, "org_admin")})
	pv = e.client().do("GET", base+"/invitation-tokens/"+token, nil, false)
	require.Equal(t, true, pv.Body["account_exists"])
	stranger := e.client()
	require.Equal(t, 401, stranger.do("POST", base+"/invitation-tokens/"+token+"/accept", map[string]any{"password": "wrong password here"}, false).Status)

	// A session for a different address is not enough.
	r = anon.do("POST", base+"/invitation-tokens/"+token+"/accept", nil, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "signed_in_as_other", r.Body["code"])

	// Signed in as the invited address (in their own org): no password needed.
	r = other.do("POST", base+"/invitation-tokens/"+token+"/accept", nil, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "inv-co", r.Body["organization"].(map[string]any)["slug"])
	require.Equal(t, 200, other.do("GET", base+"/audit-events", nil, false).Status, "org_admin from the invitation")
	login := e.client().do("POST", base+"/auth/login", map[string]any{"email": "olga@other.test", "password": "correct horse battery"}, false)
	require.Equal(t, 409, login.Status, "now in two organizations")

	actions := auditActions(t, owner, "?resource_type=user")
	require.Contains(t, actions, "user.invited:success")
	require.Contains(t, actions, "user.joined:success")
}

func TestInvitationRevocationAndEscalation(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("rev-co", "owner@rev.test"), false).Status)
	require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": "ops", "name": "Ops"}, true).Status)
	opsID := workspaceID(t, owner, "ops")

	token, inv := invite(t, owner, map[string]any{"email": "x@rev.test"})
	// Re-inviting replaces the open invitation.
	token2, _ := invite(t, owner, map[string]any{"email": "x@rev.test", "expires_in_days": 2})
	require.Equal(t, 410, e.client().do("GET", base+"/invitation-tokens/"+token, nil, false).Status)
	require.Len(t, data(owner.do("GET", base+"/invitations", nil, false)), 1)
	open := data(owner.do("GET", base+"/invitations", nil, false))[0]
	require.Equal(t, 204, owner.do("DELETE", base+"/invitations/"+open["id"].(string), nil, true).Status)
	require.Equal(t, 409, owner.do("DELETE", base+"/invitations/"+open["id"].(string), nil, true).Status)
	require.Equal(t, 404, owner.do("DELETE", base+"/invitations/"+inv["id"].(string)[:24]+"000000000000", nil, true).Status)
	r := e.client().do("POST", base+"/invitation-tokens/"+token2+"/accept", map[string]any{"display_name": "X", "password": "correct horse battery"}, false)
	require.Equal(t, 410, r.Status)
	require.Equal(t, "invitation_revoked", r.Body["code"])

	// Expired links are refused.
	token3, _ := invite(t, owner, map[string]any{"email": "late@rev.test"})
	_, err := e.pg.Owner.ExecContext(context.Background(), `UPDATE invitations SET expires_at = now() - interval '1 minute' WHERE email = 'late@rev.test'`)
	require.NoError(t, err)
	r = e.client().do("POST", base+"/invitation-tokens/"+token3+"/accept", map[string]any{"display_name": "L", "password": "correct horse battery"}, false)
	require.Equal(t, "invitation_expired", r.Body["code"])

	// Validation.
	r = owner.do("POST", base+"/invitations", map[string]any{"email": "nope", "expires_in_days": 90}, true)
	require.Equal(t, 422, r.Status)
	require.Len(t, r.Body["errors"], 2)

	// An inviter who may only invite cannot hand out roles or workspace access.
	ivy, ivyID := addMember(t, e, "rev-co", "ivy@rev.test")
	role := owner.do("POST", base+"/roles", map[string]any{"key": "inviter", "name": "Inviter", "permissions": []string{"user:invite"}}, true)
	require.Equal(t, 201, role.Status, role.Body)
	require.Equal(t, 201, assign(owner, ivyID, role.Body["id"].(string), map[string]any{"type": "org"}).Status)
	invite(t, ivy, map[string]any{"email": "plain@rev.test"})
	require.Equal(t, 403, ivy.do("POST", base+"/invitations", map[string]any{"email": "a@rev.test", "org_role_id": roleIDByKey(t, owner, "org_admin")}, true).Status)
	require.Equal(t, 403, ivy.do("POST", base+"/invitations", map[string]any{"email": "b@rev.test",
		"workspaces": []map[string]any{{"workspace_id": opsID, "role_id": roleIDByKey(t, owner, "viewer")}}}, true).Status)

	// A workspace admin (who holds role:bind nowhere) still cannot grant org_owner rights through an invitation.
	wsa, wsaID := addMember(t, e, "rev-co", "wsa@rev.test")
	require.Equal(t, 201, assign(owner, wsaID, role.Body["id"].(string), map[string]any{"type": "org"}).Status)
	require.Equal(t, 204, owner.do("POST", base+"/workspaces/ops/members", map[string]any{"user_id": wsaID, "role_key": "workspace_admin"}, true).Status)
	invite(t, wsa, map[string]any{"email": "c@rev.test",
		"workspaces": []map[string]any{{"workspace_id": opsID, "role_id": roleIDByKey(t, owner, "viewer")}}})
	r = wsa.do("POST", base+"/invitations", map[string]any{"email": "d@rev.test",
		"workspaces": []map[string]any{{"workspace_id": opsID, "role_id": roleIDByKey(t, owner, "org_owner")}}}, true)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "privilege_escalation", r.Body["code"])
}

func TestAPIKeyAuthentication(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("key-co", "owner@key.test"), false).Status)

	sa := owner.do("POST", base+"/service-accounts", map[string]any{"name": "CI deployer", "description": "GitHub Actions"}, true)
	require.Equal(t, 201, sa.Status, sa.Body)
	saID := sa.Body["id"].(string)
	require.Equal(t, 409, owner.do("POST", base+"/service-accounts", map[string]any{"name": "ci DEPLOYER"}, true).Status)

	k := owner.do("POST", base+"/service-accounts/"+saID+"/keys", map[string]any{"name": "primary"}, true)
	require.Equal(t, 201, k.Status, k.Body)
	secret := k.Body["secret"].(string)
	require.Regexp(t, `^hugi_[a-z0-9]{12}_[0-9a-f]{64}$`, secret)
	keys := data(owner.do("GET", base+"/service-accounts/"+saID+"/keys", nil, false))
	require.Len(t, keys, 1)
	require.NotContains(t, keys[0], "secret")

	api := bearer{c: e.client(), key: secret}
	// No roles yet: authenticated, but sees nothing and may do nothing.
	require.Equal(t, 200, api.do("GET", base+"/workspaces", nil).Status)
	require.Equal(t, 403, api.do("GET", base+"/audit-events", nil).Status)
	r := api.do("GET", base+"/me", nil)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "not_a_user", r.Body["code"])

	// Roles are assigned like for people.
	b := owner.do("POST", base+"/role-bindings", map[string]any{"service_account_id": saID, "role_id": roleIDByKey(t, owner, "org_admin"), "scope": map[string]any{"type": "org"}}, true)
	require.Equal(t, 201, b.Status, b.Body)
	require.Equal(t, "service_account", b.Body["principal"].(map[string]any)["type"])
	require.Equal(t, "CI deployer", b.Body["principal"].(map[string]any)["name"])
	require.Equal(t, 200, api.do("GET", base+"/audit-events", nil).Status)
	w := api.do("POST", base+"/workspaces", map[string]any{"slug": "deploys", "name": "Deploys"}) // no CSRF token needed
	require.Equal(t, 201, w.Status, w.Body)
	sb := data(owner.do("GET", base+"/role-bindings?service_account_id="+saID, nil, false))
	require.Len(t, sb, 2, "org_admin + workspace_admin of what it created")
	perms := api.do("GET", base+"/me/permissions", nil)
	require.Contains(t, perms.Body["organization"], "workspace:create")

	// The audit trail names the service account.
	ev := data(owner.do("GET", base+"/audit-events?action=workspace.created", nil, false))[0]
	require.Equal(t, "service_account", ev["actor_type"])
	require.Equal(t, "CI deployer", ev["actor_email"])

	// Bad keys are all the same 401.
	for _, bad := range []string{"nope", "hugi_aaaaaaaaaaaa_" + secret[len(secret)-64:], secret[:len(secret)-1] + "0"} {
		r := bearer{c: e.client(), key: bad}.do("GET", base+"/workspaces", nil)
		require.Equal(t, 401, r.Status, bad)
		require.Equal(t, "invalid_api_key", r.Body["code"])
	}
	r = e.client().doWithHeader("GET", base+"/workspaces", nil, http.Header{"Authorization": {"Basic Zm9vOmJhcg=="}})
	require.Equal(t, 401, r.Status)

	keys = data(owner.do("GET", base+"/service-accounts/"+saID+"/keys", nil, false))
	require.NotNil(t, keys[0]["last_used_at"])

	// Disabling stops every key at once; re-enabling restores them.
	require.Equal(t, 200, owner.do("PATCH", base+"/service-accounts/"+saID, map[string]any{"name": "CI deployer", "disabled": true}, true).Status)
	require.Equal(t, 401, api.do("GET", base+"/workspaces", nil).Status)
	r = owner.do("POST", base+"/service-accounts/"+saID+"/keys", map[string]any{"name": "second"}, true)
	require.Equal(t, 409, r.Status, "no keys for disabled accounts")
	require.Equal(t, 200, owner.do("PATCH", base+"/service-accounts/"+saID, map[string]any{"name": "CI deployer", "disabled": false}, true).Status)
	require.Equal(t, 200, api.do("GET", base+"/workspaces", nil).Status)

	// Revocation and expiry are immediate.
	k2 := owner.do("POST", base+"/service-accounts/"+saID+"/keys", map[string]any{"name": "rotating"}, true)
	api2 := bearer{c: e.client(), key: k2.Body["secret"].(string)}
	require.Equal(t, 200, api2.do("GET", base+"/workspaces", nil).Status)
	require.Equal(t, 204, owner.do("DELETE", base+"/service-accounts/"+saID+"/keys/"+keys[0]["id"].(string), nil, true).Status)
	require.Equal(t, 401, api.do("GET", base+"/workspaces", nil).Status)
	_, err := e.pg.Owner.ExecContext(context.Background(), `UPDATE api_keys SET expires_at = now() - interval '1 second' WHERE name = 'rotating'`)
	require.NoError(t, err)
	require.Equal(t, 401, api2.do("GET", base+"/workspaces", nil).Status)

	// A key from one organization never reaches another.
	k3 := owner.do("POST", base+"/service-accounts/"+saID+"/keys", map[string]any{"name": "third"}, true)
	api3 := bearer{c: e.client(), key: k3.Body["secret"].(string)}
	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations", signupBody("key-other", "owner@other-key.test"), false).Status)
	require.Equal(t, 201, other.do("POST", base+"/workspaces", map[string]any{"slug": "secret", "name": "Secret"}, true).Status)
	require.Equal(t, 404, api3.do("GET", base+"/workspaces/secret", nil).Status)
	require.Equal(t, []string{"deploys"}, slugs(api3.do("GET", base+"/workspaces", nil)))

	// Deleting the account removes its keys and assignments.
	require.Equal(t, 204, owner.do("DELETE", base+"/service-accounts/"+saID, nil, true).Status)
	require.Equal(t, 401, api3.do("GET", base+"/workspaces", nil).Status)
	require.Empty(t, data(owner.do("GET", base+"/role-bindings?principal_type=service_account", nil, false)))
	require.Contains(t, auditActions(t, owner, "?action=service_account"), "service_account.key_revoked:success")
}

func TestGroupsGrantRolesToMembers(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("grp-co", "owner@grp.test"), false).Status)
	require.Equal(t, 201, owner.do("POST", base+"/workspaces", map[string]any{"slug": "ops", "name": "Ops"}, true).Status)
	opsID := workspaceID(t, owner, "ops")
	carol, carolID := addMember(t, e, "grp-co", "carol@grp.test")

	g := owner.do("POST", base+"/groups", map[string]any{"name": "Ops team"}, true)
	require.Equal(t, 201, g.Status, g.Body)
	gID := g.Body["id"].(string)
	require.Equal(t, 409, owner.do("POST", base+"/groups", map[string]any{"name": "OPS TEAM"}, true).Status)
	b := owner.do("POST", base+"/role-bindings", map[string]any{"group_id": gID, "role_id": roleIDByKey(t, owner, "workspace_admin"),
		"scope": map[string]any{"type": "workspace", "workspace_id": opsID}}, true)
	require.Equal(t, 201, b.Status, b.Body)

	require.Equal(t, 404, carol.do("GET", base+"/workspaces/ops", nil, false).Status)
	require.Equal(t, 204, owner.do("POST", base+"/groups/"+gID+"/members", map[string]any{"user_id": carolID}, true).Status)
	require.Equal(t, 204, owner.do("POST", base+"/groups/"+gID+"/members", map[string]any{"user_id": carolID}, true).Status, "idempotent")
	require.Equal(t, 200, carol.do("PATCH", base+"/workspaces/ops", map[string]any{"name": "Operations"}, true).Status, "via the group")
	_, ws := perms(t, carol)
	require.Contains(t, ws[opsID], "workspace:manage_members")
	sim := owner.do("POST", base+"/authorization/evaluate", map[string]any{"user_id": carolID, "resource_type": "workspace",
		"action": "update", "workspace_id": opsID}, true)
	require.Equal(t, true, sim.Body["allowed"])
	require.Contains(t, sim.Body["matched_rules"].([]any)[0], "(via group)")
	u := owner.do("GET", base+"/users/"+carolID, nil, false)
	require.Equal(t, "Ops team", u.Body["groups"].([]any)[0].(map[string]any)["name"])
	require.Len(t, data(owner.do("GET", base+"/groups/"+gID+"/members", nil, false)), 1)

	// Leaving the group revokes the access on the next request.
	require.Equal(t, 204, owner.do("DELETE", base+"/groups/"+gID+"/members/"+carolID, nil, true).Status)
	require.Equal(t, 404, carol.do("GET", base+"/workspaces/ops", nil, false).Status)
	require.Equal(t, 404, owner.do("DELETE", base+"/groups/"+gID+"/members/"+carolID, nil, true).Status)

	// No escalation through groups: managing members of a group that holds
	// more than you do is refused.
	dave, daveID := addMember(t, e, "grp-co", "dave@grp.test")
	role := owner.do("POST", base+"/roles", map[string]any{"key": "group_admin", "name": "Group admin", "permissions": []string{"group:*", "user:read"}}, true)
	require.Equal(t, 201, role.Status)
	require.Equal(t, 201, assign(owner, daveID, role.Body["id"].(string), map[string]any{"type": "org"}).Status)
	r := dave.do("POST", base+"/groups/"+gID+"/members", map[string]any{"user_id": daveID}, true)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "privilege_escalation", r.Body["code"])
	plain := dave.do("POST", base+"/groups", map[string]any{"name": "Readers"}, true)
	require.Equal(t, 201, plain.Status)
	require.Equal(t, 204, dave.do("POST", base+"/groups/"+plain.Body["id"].(string)+"/members", map[string]any{"user_id": carolID}, true).Status)
	require.Equal(t, 422, dave.do("POST", base+"/groups/"+plain.Body["id"].(string)+"/members", map[string]any{"user_id": opsID}, true).Status, "not a member")

	// Deleting a group removes its assignments.
	require.Equal(t, 204, owner.do("DELETE", base+"/groups/"+gID, nil, true).Status)
	require.Empty(t, data(owner.do("GET", base+"/role-bindings?group_id="+gID, nil, false)))
	require.Equal(t, 403, carol.do("GET", base+"/groups", nil, false).Status)
}

func TestSuspendAndRemoveMembers(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("sus-co", "owner@sus.test"), false).Status)
	ownerID := owner.do("GET", base+"/me", nil, false).Body["user"].(map[string]any)["id"].(string)
	carol, carolID := addMember(t, e, "sus-co", "carol@sus.test")
	adam, adamID := addMember(t, e, "sus-co", "adam@sus.test")
	require.Equal(t, 201, assign(owner, adamID, roleIDByKey(t, owner, "org_admin"), map[string]any{"type": "org"}).Status)

	r := owner.do("PATCH", base+"/users/"+ownerID, map[string]any{"status": "suspended"}, true)
	require.Equal(t, "cannot_suspend_self", r.Body["code"])
	r = adam.do("PATCH", base+"/users/"+ownerID, map[string]any{"status": "suspended"}, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "last_owner", r.Body["code"])
	require.Equal(t, 409, adam.do("DELETE", base+"/users/"+ownerID, nil, true).Status)

	r = adam.do("PATCH", base+"/users/"+carolID, map[string]any{"status": "suspended"}, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "suspended", r.Body["membership_status"])
	require.Equal(t, 401, carol.do("GET", base+"/me", nil, false).Status, "sessions end")
	login := e.client().do("POST", base+"/auth/login", map[string]any{"email": "carol@sus.test", "password": "correct horse battery"}, false)
	require.Equal(t, 403, login.Status)
	require.Equal(t, 200, adam.do("PATCH", base+"/users/"+carolID, map[string]any{"status": "active"}, true).Status)
	carol = e.client()
	login = carol.do("POST", base+"/auth/login", map[string]any{"email": "carol@sus.test", "password": "correct horse battery"}, false)
	require.Equal(t, 200, login.Status)

	// Members without user:update cannot suspend anyone.
	require.Equal(t, 403, carol.do("PATCH", base+"/users/"+adamID, map[string]any{"status": "suspended"}, true).Status)

	require.Equal(t, 204, adam.do("DELETE", base+"/users/"+carolID, nil, true).Status)
	require.Equal(t, 404, adam.do("GET", base+"/users/"+carolID, nil, false).Status)
	require.Empty(t, data(owner.do("GET", base+"/role-bindings?user_id="+carolID, nil, false)))
	var emails []string
	for _, m := range data(owner.do("GET", base+"/users", nil, false)) {
		emails = append(emails, m["email"].(string))
	}
	require.ElementsMatch(t, []string{"owner@sus.test", "adam@sus.test"}, emails)
	actions := auditActions(t, owner, "?resource_type=user")
	for _, want := range []string{"user.suspended:success", "user.reactivated:success", "user.removed:success"} {
		require.Contains(t, actions, want)
	}

	// The removed person can be invited back.
	token, _ := invite(t, owner, map[string]any{"email": "carol@sus.test"})
	require.Equal(t, 200, e.client().do("POST", base+"/invitation-tokens/"+token+"/accept", map[string]any{"password": "correct horse battery"}, false).Status)
}
