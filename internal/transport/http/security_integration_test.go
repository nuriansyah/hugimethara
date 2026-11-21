//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	whmodel "github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	api "github.com/nuriansyah/hugimethara/internal/transport/http"
)

// S14 security suite. It is driven by the operation table, so a route added
// later is covered automatically — and fails the coverage check below until
// its path parameter is mapped to a real resource of the victim tenant.

var pathParamRe = regexp.MustCompile(`\{([a-z_]+)\}`)

// paramsWithoutVictimResource are path parameters for which the suite uses a
// random id instead of a resource of the victim tenant, with the reason.
var paramsWithoutVictimResource = map[string]string{
	"idp_id":        "public OIDC start route, not tenant data",
	"token":         "public invitation token routes",
	"definition_id": "the connector catalog is global (shared built-ins)",
	"provider_id":   "creating an IdP needs a live discovery endpoint; covered by oidc_integration_test",
	"task_id":       "needs a human task; approval tenancy covered by workflow_s11_integration_test",
	"delivery_id":   "needs the delivery worker; replay isolation covered by webhook_integration_test",
	"endpoint_id":   "needs an imported OpenAPI spec; explorer isolation covered by connectors_services_integration_test",
}

type victim struct {
	wsID    string            // the workspace holding the victim's resources (workspace_id query parameter)
	ids     map[string]string // path parameter → victim resource id
	markers []string          // strings that must never appear in another tenant's responses
	secrets []string          // plaintext secrets that must never appear in any response
}

func materialize(path string, ids map[string]string) string {
	return pathParamRe.ReplaceAllStringFunc(path, func(m string) string {
		if v := ids[m[1:len(m)-1]]; v != "" {
			return v
		}
		return uuid.NewString()
	})
}

// newVictim creates one resource of (nearly) every type in a fresh tenant.
func newVictim(t *testing.T, e *env) (*client, victim) {
	t.Helper()
	f := newWorkflowFixture(t, e, "sec-victim")
	o := f.owner
	// {workspace} routes take a slug; both tenants have an "ops" workspace, so
	// the victim also gets a uniquely named one for those routes.
	v := victim{wsID: f.workspaceID, ids: map[string]string{"workspace": "victim-space", "environment_id": f.envID}}
	require.Equal(t, 201, o.do("POST", base+"/workspaces", map[string]any{"slug": "victim-space", "name": "Victim space"}, true).Status)
	mustID := func(r resp, want int) string {
		t.Helper()
		require.Equal(t, want, r.Status, r.Body)
		return r.Body["id"].(string)
	}
	me := o.do("GET", base+"/me", nil, false)
	v.ids["user_id"] = me.Body["user"].(map[string]any)["id"].(string)
	v.ids["session_id"] = data(o.do("GET", base+"/me/sessions", nil, false))[0]["id"].(string)

	const credValue = "victim-credential-plaintext-0001"
	v.ids["credential_id"] = mustID(o.do("POST", base+"/credentials", map[string]any{
		"name": "victim cred", "kind": "bearer", "value": credValue}, true), 201)
	v.secrets = append(v.secrets, credValue)

	var httpVersion string
	for _, d := range data(o.do("GET", base+"/connector-definitions", nil, false)) {
		if d["key"] == "http_rest" {
			httpVersion = data(o.do("GET", base+"/connector-definitions/"+d["id"].(string)+"/versions", nil, false))[0]["id"].(string)
		}
	}
	v.ids["connector_id"] = mustID(o.do("POST", base+"/connectors", map[string]any{
		"workspace_id": f.workspaceID, "environment_id": f.envID, "ref_key": "victim-http",
		"connector_version_id": httpVersion, "config": map[string]any{"base_url": "https://victim.example"},
		"auth_method": "bearer", "secret_id": v.ids["credential_id"]}, true), 201)
	v.ids["service_id"] = mustID(o.do("POST", base+"/services", map[string]any{
		"workspace_id": f.workspaceID, "name": "Victim Orders API", "protocol": "rest"}, true), 201)
	v.ids["mapping_id"] = mustID(o.do("POST", base+"/mappings", map[string]any{"workspace_id": f.workspaceID,
		"name": "Victim mapping", "draft": map[string]any{"rules": []any{
			map[string]any{"target": "name", "source": "$.n"}}}}, true), 201)
	v.ids["test_id"] = mustID(o.do("POST", base+"/mappings/"+v.ids["mapping_id"]+"/tests", map[string]any{
		"name": "victim test", "input": map[string]any{"n": "x"}, "expected": map[string]any{"name": "x"}}, true), 201)
	v.ids["composition_id"] = mustID(o.do("POST", base+"/api-compositions", map[string]any{
		"workspace_id": f.workspaceID, "name": "Victim composition", "slug": "victim-composition"}, true), 201)

	wf := f.deploy(t, "victim-flow", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"},
		"nodes":   []any{node("shape", "action.transform", map[string]any{"rules": []any{map[string]any{"target": "x", "source": "$.x"}}})},
		"edges":   []any{edge("trigger", "shape", "")}})
	v.ids["workflow_id"] = wf
	v.ids["execution_id"] = f.execute(t, wf, map[string]any{"x": 1}, "").Body["id"].(string)

	wh := o.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"name": "Victim hook", "verifier": "standard_webhooks"}, true)
	v.ids["webhook_id"] = mustID(wh, 201)
	v.secrets = append(v.secrets, wh.Body["secret"].(string))
	sub := o.do("POST", base+"/webhook-subscriptions", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "name": "Victim bus", "url": "https://victim.example/events",
		"event_types": []string{"workflow.execution_finished"}}, true)
	v.ids["subscription_id"] = mustID(sub, 201)
	v.secrets = append(v.secrets, sub.Body["secret"].(string))

	v.ids["group_id"] = mustID(o.do("POST", base+"/groups", map[string]any{"name": "Victim team"}, true), 201)
	v.ids["role_id"] = mustID(o.do("POST", base+"/roles", map[string]any{"key": "victim_role", "name": "Victim role",
		"permissions": []string{"workflow:read"}}, true), 201)
	v.ids["binding_id"] = mustID(o.do("POST", base+"/role-bindings", map[string]any{"group_id": v.ids["group_id"],
		"role_id": v.ids["role_id"], "scope": map[string]any{"type": "org"}}, true), 201)
	v.ids["service_account_id"] = mustID(o.do("POST", base+"/service-accounts", map[string]any{"name": "Victim CI"}, true), 201)
	key := o.do("POST", base+"/service-accounts/"+v.ids["service_account_id"]+"/keys", map[string]any{"name": "primary"}, true)
	require.Equal(t, 201, key.Status, key.Body)
	v.ids["key_id"] = key.Body["key"].(map[string]any)["id"].(string)
	v.secrets = append(v.secrets, key.Body["secret"].(string))
	inv := o.do("POST", base+"/invitations", map[string]any{"email": "invitee@victim.test"}, true)
	require.Equal(t, 201, inv.Status, inv.Body)
	v.ids["invitation_id"] = inv.Body["invitation"].(map[string]any)["id"].(string)
	v.secrets = append(v.secrets, inv.Body["token"].(string))
	setPlan(t, e, "sec-victim", "enterprise") // departments are a paid feature
	v.ids["department_id"] = mustID(o.do("POST", base+"/departments", map[string]any{"slug": "victim-dept", "name": "Victim dept"}, true), 201)
	hookBody := []byte(`{"type":"victim.event","data":{"n":1}}`)
	signed, err := whmodel.SignedHeaders([]byte(wh.Body["secret"].(string)), "msg_victim", time.Now(), hookBody)
	require.NoError(t, err)
	hdr := http.Header{}
	for k, val := range signed {
		hdr.Set(k, val)
	}
	ev := postHook(t, e, wh.Body["public_id"].(string), hookBody, hdr)
	v.ids["event_id"] = mustID(ev, 202)

	for k, id := range v.ids {
		if k != "environment_id" && k != "user_id" { // the target also has users/environments; ids are unique anyway
			v.markers = append(v.markers, id)
		}
	}
	v.markers = append(v.markers, v.wsID, "Victim Orders API", "Victim composition", "victim-http", "invitee@victim.test")
	return o, v
}

func isMutation(method string) bool { return method != http.MethodGet && method != http.MethodHead }

func dumpBody(r resp) string { b, _ := json.Marshal(r.Body); return string(b) }

func TestSecuritySuite(t *testing.T) {
	e := setup(t)
	victimOwner, v := newVictim(t, e)
	attacker := newWorkflowFixture(t, e, "sec-attacker").owner // owner = full admin of its own tenant
	anonymous := e.client()
	ops := api.Operations()
	require.Greater(t, len(ops), 160)

	t.Run("every path parameter is mapped", func(t *testing.T) {
		for _, op := range ops {
			for _, m := range pathParamRe.FindAllStringSubmatch(op.Path, -1) {
				_, real := v.ids[m[1]]
				_, waived := paramsWithoutVictimResource[m[1]]
				require.True(t, real || waived, "%s %s: map {%s} to a victim resource or waive it with a reason",
					op.Method, op.Path, m[1])
			}
		}
	})

	t.Run("unauthenticated requests are refused", func(t *testing.T) {
		for _, op := range ops {
			if op.Public {
				continue
			}
			r := anonymous.do(op.Method, materialize(op.Path, v.ids), map[string]any{}, false)
			if op.Method == http.MethodGet {
				require.Equal(t, 401, r.Status, "%s %s", op.Method, op.Path)
			} else {
				require.Contains(t, []int{401, 403}, r.Status, "%s %s", op.Method, op.Path)
			}
		}
	})

	t.Run("mutations without the CSRF token are refused", func(t *testing.T) {
		for _, op := range ops {
			if op.Public || !isMutation(op.Method) {
				continue
			}
			r := victimOwner.do(op.Method, materialize(op.Path, v.ids), map[string]any{}, false)
			require.Equal(t, 403, r.Status, "%s %s: %s", op.Method, op.Path, dumpBody(r))
			require.Equal(t, "csrf_failed", r.Body["code"], "%s %s", op.Method, op.Path)
		}
	})

	// Before the cross-tenant pass: afterwards the attacker's own decision and
	// audit logs legitimately contain the victim ids the attacker sent.
	t.Run("collection routes never list another tenant's data", func(t *testing.T) {
		for _, op := range ops {
			if op.Public || op.Method != http.MethodGet || strings.Contains(op.Path, "{") {
				continue
			}
			for _, q := range []string{"", "?workspace_id=" + v.wsID, "?workspace_id=" + v.wsID + "&assigned_to=any"} {
				r := attacker.do("GET", op.Path+q, nil, false)
				require.Less(t, r.Status, 500, "GET %s%s: %s", op.Path, q, dumpBody(r))
				body := dumpBody(r)
				for _, mk := range v.markers {
					if mk == v.wsID && q != "" {
						continue // the attacker supplied it
					}
					require.NotContains(t, body, mk, "GET %s%s leaks victim data", op.Path, q)
				}
			}
		}
	})

	t.Run("cross-tenant object references are refused", func(t *testing.T) {
		for _, op := range ops {
			if op.Public || !strings.Contains(op.Path, "{") {
				continue
			}
			uses := false
			for _, m := range pathParamRe.FindAllStringSubmatch(op.Path, -1) {
				uses = uses || v.ids[m[1]] != ""
			}
			if !uses {
				continue // only random ids: covered by the unauthenticated/CSRF passes
			}
			path := materialize(op.Path, v.ids)
			r := attacker.do(op.Method, path, map[string]any{}, true)
			require.Contains(t, []int{400, 403, 404, 409, 422}, r.Status, "%s %s answered %d: %s",
				op.Method, op.Path, r.Status, dumpBody(r))
			body := dumpBody(r)
			for _, mk := range v.markers {
				if strings.Contains(path, mk) {
					continue // an id the attacker sent may be echoed in a not-found detail
				}
				require.NotContains(t, body, mk, "%s %s leaks victim data", op.Method, op.Path)
			}
		}
	})

	t.Run("no response ever contains a secret", func(t *testing.T) {
		for _, op := range ops {
			if op.Method != http.MethodGet || op.Public {
				continue
			}
			for _, q := range []string{"", "?workspace_id=" + v.wsID} {
				r := victimOwner.do("GET", materialize(op.Path, v.ids)+q, nil, false)
				require.Less(t, r.Status, 500, "GET %s%s: %s", op.Path, q, dumpBody(r))
				body := dumpBody(r)
				for _, s := range v.secrets {
					require.NotContains(t, body, s, "GET %s exposes a secret", op.Path)
				}
				for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Cache-Control"} {
					require.NotEmpty(t, r.Header.Get(h), "GET %s misses %s", op.Path, h)
				}
			}
		}
	})

	t.Run("the victim tenant is intact", func(t *testing.T) {
		for _, p := range []string{"/workflows/" + v.ids["workflow_id"], "/credentials/" + v.ids["credential_id"],
			"/connectors/" + v.ids["connector_id"], "/webhooks/" + v.ids["webhook_id"], "/groups/" + v.ids["group_id"],
			"/roles/" + v.ids["role_id"], "/service-accounts/" + v.ids["service_account_id"],
			"/api-compositions/" + v.ids["composition_id"], "/mappings/" + v.ids["mapping_id"],
			"/webhook-subscriptions/" + v.ids["subscription_id"], "/workspaces/" + v.ids["workspace"]} {
			r := victimOwner.do("GET", base+p, nil, false)
			require.Equal(t, 200, r.Status, "%s: %s", p, dumpBody(r))
		}
		require.Equal(t, 200, victimOwner.do("GET", base+"/me", nil, false).Status, "the victim's session survived")
	})
}
