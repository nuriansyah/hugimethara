//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	compositionsvc "github.com/nuriansyah/hugimethara/internal/modules/api_composition/services"
	overviewsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/overview/services"
	webhooksvc "github.com/nuriansyah/hugimethara/internal/modules/webhook/services"
	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
	compositionhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/api_composition"
	overviewhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/overview"
	webhookhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/webhook"
	workflowhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/workflow"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/health"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitysvc "github.com/nuriansyah/hugimethara/internal/modules/identity/services"
	connectorsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/services"
	mappingsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/services"
	secretssvc "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/services"
	serviceregsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/services"
	orgsvc "github.com/nuriansyah/hugimethara/internal/modules/organization/services"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
	"github.com/nuriansyah/hugimethara/internal/platform/testsupport"
	api "github.com/nuriansyah/hugimethara/internal/transport/http"
	audithandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/audit"
	authzhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/authorization"
	connectorhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/connector"
	enthandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/entitlement"
	identityhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/identity"
	mappinghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/mapping"
	orghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/organization"
	secretshandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/secrets"
	servicereghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/service_registry"
)

const base = "/v1/hugimethara"

// cheap hashing keeps the suite fast; production uses identity.DefaultArgon2.
var testArgon = identitymodel.Argon2Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

type env struct {
	az      *authzsvc.RBAC
	t       *testing.T
	pg      *testsupport.Postgres
	srv     *httptest.Server
	secrets *secretssvc.SecretsService
	sealer  *secretbox.Key
}

func setup(t *testing.T) *env {
	t.Helper()
	pg := testsupport.StartPostgres(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	az := authzsvc.NewRBAC(pg.App, nil, log)
	acl := authzsvc.NewService(pg.App, az, func(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action, rt, rid string, details map[string]any) error {
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: org, Action: action, ResourceType: rt, ResourceID: rid, Details: details})
	})
	orgs := orgsvc.NewService(pg.App, az, acl)
	sealer, err := secretbox.New(secretbox.DeriveKey("integration-test"))
	require.NoError(t, err)
	masterKey := secretbox.DeriveKey("integration-test")
	secrets, err := secretssvc.NewService(pg.App, az, masterKey, "test-v1")
	require.NoError(t, err)
	connectors, err := connectorsvc.NewService(pg.App, az)
	require.NoError(t, err)
	servicePolicy, err := egress.ParsePolicy(nil)
	require.NoError(t, err)
	services := serviceregsvc.NewService(pg.App, az, servicePolicy)
	mappings := mappingsvc.NewService(pg.App, az)
	ids := identitysvc.NewService(pg.App, orgs, az, acl, nil, log, identitysvc.Config{
		Hasher: identitysvc.NewHasher(testArgon), MaxFailedLogins: 3,
		PublicURL: "http://localhost:3000", Sealer: sealer,
	})
	expressions, err := workflowsvc.NewExpressions()
	require.NoError(t, err)
	router, err := api.NewRouter(api.Deps{
		Env: "test", Health: health.New(0), Registerer: prometheus.NewRegistry(),
		Identity:     identityhandler.NewHTTP(ids, identityhandler.Cookies{Secure: false}),
		Organization: orghandler.NewHTTP(orgs),
		Audit:        audithandler.NewHTTP(auditsvc.NewService(pg.App, az)),
		Entitlement:  enthandler.NewHTTP(entsvc.NewService(pg.App, az, usage())),
		Access:       authzhandler.NewHTTP(acl),
		Secrets:      secretshandler.NewHTTP(secrets),
		Connectors:   connectorhandler.NewHTTP(connectors),
		Services:     servicereghandler.NewHTTP(services),
		Mappings:     mappinghandler.NewHTTP(mappings),
		Workflows:    workflowhandler.NewHTTP(workflowsvc.NewService(pg.App, az, expressions)),
		Webhooks:     webhookhandler.NewHTTP(webhooksvc.NewService(pg.App, az, sealer, webhooksvc.Config{})),
		Compositions: compositionhandler.NewHTTP(compositionsvc.NewService(pg.App, az, services)),
		Overview:     overviewhandler.NewHTTP(overviewsvc.NewService(pg.App, az)),
	})
	require.NoError(t, err)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &env{t: t, pg: pg, srv: srv, secrets: secrets, az: az, sealer: sealer}
}

func usage() map[string]entsvc.UsageFunc {
	out := map[string]entsvc.UsageFunc{}
	for k, fn := range orgsvc.UsageCounters() {
		out[k] = fn
	}
	return out
}

// client is a browser-like client with its own cookie jar.
type client struct {
	e   *env
	jar *cookiejar.Jar
	hc  *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, jar: jar, hc: &http.Client{Jar: jar}}
}

func (c *client) csrf() string {
	u, _ := url.Parse(c.e.srv.URL)
	for _, ck := range c.jar.Cookies(u) {
		if ck.Name == "hugi_csrf" {
			return ck.Value
		}
	}
	return ""
}

type resp struct {
	Status int
	Body   map[string]any
	Header http.Header
}

func (c *client) do(method, path string, body any, withCSRF bool) resp {
	c.e.t.Helper()
	var h http.Header
	if withCSRF {
		h = http.Header{identityhandler.CSRFHeader: {c.csrf()}}
	}
	return c.doWithHeader(method, path, body, h)
}

func (c *client) doWithHeader(method, path string, body any, header http.Header) resp {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, c.e.srv.URL+path, rd)
	require.NoError(c.e.t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := c.hc.Do(req)
	require.NoError(c.e.t, err)
	defer func() { _ = res.Body.Close() }()
	out := resp{Status: res.StatusCode, Header: res.Header}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

func signupBody(slug, email string) map[string]any {
	return map[string]any{
		"organization_name": "Org " + slug, "organization_slug": slug,
		"display_name": "Owner " + slug, "email": email, "password": "correct horse battery",
	}
}

func auditActions(t *testing.T, c *client, query string) []string {
	t.Helper()
	r := c.do("GET", base+"/audit-events"+query, nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	var out []string
	for _, e := range r.Body["data"].([]any) {
		ev := e.(map[string]any)
		out = append(out, ev["action"].(string)+":"+ev["outcome"].(string))
	}
	return out
}

func TestSignupLoginLogoutFlow(t *testing.T) {
	e := setup(t)
	alice := e.client()

	// Sign up: creates org + owner, signs in, sets cookies.
	r := alice.do("POST", base+"/organizations", signupBody("acme", "alice@acme.test"), false)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, "acme", r.Body["organization"].(map[string]any)["slug"])
	setCookies := strings.Join(r.Header.Values("Set-Cookie"), "\n")
	require.Contains(t, setCookies, "hugi_session=")
	require.Contains(t, setCookies, "HttpOnly")
	require.Contains(t, setCookies, "SameSite=Lax")
	require.NotEmpty(t, alice.csrf())

	me := alice.do("GET", base+"/me", nil, false)
	require.Equal(t, 200, me.Status, me.Body)
	require.Equal(t, true, me.Body["membership"].(map[string]any)["is_owner"])
	require.Equal(t, "alice@acme.test", me.Body["user"].(map[string]any)["email"])

	// State change without the CSRF token is rejected; with it, it succeeds.
	patch := map[string]any{"name": "Acme Indonesia", "settings": map[string]any{
		"session_idle_timeout_minutes": 60, "session_absolute_timeout_hours": 24}}
	r = alice.do("PATCH", base+"/organizations/current", patch, false)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "csrf_failed", r.Body["code"])
	r = alice.do("PATCH", base+"/organizations/current", patch, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "Acme Indonesia", r.Body["name"])

	// Invalid settings come back as field errors.
	r = alice.do("PATCH", base+"/organizations/current", map[string]any{"settings": map[string]any{
		"session_idle_timeout_minutes": 2000, "session_absolute_timeout_hours": 24}}, true)
	require.Equal(t, 422, r.Status)
	require.Equal(t, "settings.session_idle_timeout_minutes", r.Body["errors"].([]any)[0].(map[string]any)["field"])

	// Everything above was audited in-transaction.
	require.Equal(t, []string{"organization.updated:success", "auth.login:success", "organization.created:success"},
		auditActions(t, alice, ""))
	require.Equal(t, []string{"auth.login:success"}, auditActions(t, alice, "?action=auth"))

	// Logout revokes the session server-side.
	r = alice.do("POST", base+"/auth/logout", nil, true)
	require.Equal(t, 204, r.Status)
	require.Equal(t, 401, alice.do("GET", base+"/me", nil, false).Status)

	// Sign back in.
	r = alice.do("POST", base+"/auth/login", map[string]any{"email": "ALICE@acme.test", "password": "correct horse battery"}, false)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, 200, alice.do("GET", base+"/me", nil, false).Status)
}

func TestSignupValidationAndConflicts(t *testing.T) {
	e := setup(t)
	c := e.client()

	r := c.do("POST", base+"/organizations", map[string]any{
		"organization_name": "A", "organization_slug": "Bad Slug", "display_name": "",
		"email": "not-an-email", "password": "short"}, false)
	require.Equal(t, 422, r.Status)
	fields := map[string]bool{}
	for _, fe := range r.Body["errors"].([]any) {
		fields[fe.(map[string]any)["field"].(string)] = true
	}
	require.Equal(t, map[string]bool{"organization_name": true, "organization_slug": true, "display_name": true, "email": true, "password": true}, fields)

	require.Equal(t, 201, e.client().do("POST", base+"/organizations", signupBody("taken", "first@x.test"), false).Status)
	r = e.client().do("POST", base+"/organizations", signupBody("taken", "second@x.test"), false)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "slug_taken", r.Body["code"])
	r = e.client().do("POST", base+"/organizations", signupBody("other", "FIRST@x.test"), false)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "email_taken", r.Body["code"])

	// Failed signups left nothing behind: no orphan orgs, users, audit or outbox rows.
	var orgs, users, audits, outbox int
	ctx := context.Background()
	require.NoError(t, e.pg.Owner.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM organizations), (SELECT count(*) FROM users),
		(SELECT count(*) FROM audit_events), (SELECT count(*) FROM outbox)`).Scan(&orgs, &users, &audits, &outbox))
	require.Equal(t, []int{1, 1, 2, 1}, []int{orgs, users, audits, outbox}, "orgs, users, audit rows, outbox rows")
}

func TestLoginFailuresLockoutAndAudit(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("lockco", "bob@lock.test"), false).Status)

	bad := map[string]any{"email": "bob@lock.test", "password": "wrong password!!"}
	r := e.client().do("POST", base+"/auth/login", bad, false)
	require.Equal(t, 401, r.Status)
	require.Equal(t, "invalid_credentials", r.Body["code"])

	// Unknown accounts get the identical response.
	r2 := e.client().do("POST", base+"/auth/login", map[string]any{"email": "nobody@lock.test", "password": "whatever whatever"}, false)
	require.Equal(t, r.Status, r2.Status)
	require.Equal(t, r.Body["detail"], r2.Body["detail"])

	// Two more failures reach MaxFailedLogins=3; now even the right password fails.
	e.client().do("POST", base+"/auth/login", bad, false)
	e.client().do("POST", base+"/auth/login", bad, false)
	good := map[string]any{"email": "bob@lock.test", "password": "correct horse battery"}
	require.Equal(t, 401, e.client().do("POST", base+"/auth/login", good, false).Status)

	failures := 0
	for _, a := range auditActions(t, owner, "?action=auth.login") {
		if a == "auth.login:failure" {
			failures++
		}
	}
	require.Equal(t, 4, failures, "3 wrong passwords + 1 attempt while locked")

	// Lockout expires.
	_, err := e.pg.Owner.ExecContext(context.Background(), `UPDATE user_credentials SET locked_until = now() - interval '1 second'`)
	require.NoError(t, err)
	require.Equal(t, 200, e.client().do("POST", base+"/auth/login", good, false).Status)
}

func TestMultiOrgLoginRequiresChoice(t *testing.T) {
	e := setup(t)
	require.Equal(t, 201, e.client().do("POST", base+"/organizations", signupBody("org-one", "carol@multi.test"), false).Status)
	require.Equal(t, 201, e.client().do("POST", base+"/organizations", signupBody("org-two", "dave@multi.test"), false).Status)
	// Fixture: carol is also a (non-owner) member of org-two (invitations arrive in S4).
	_, err := e.pg.Owner.ExecContext(context.Background(), `INSERT INTO org_memberships (org_id, user_id)
		SELECT o.id, u.id FROM organizations o, users u WHERE o.slug = 'org-two' AND u.email = 'carol@multi.test'`)
	require.NoError(t, err)
	bindMemberRole(t, e, "org-two", "carol@multi.test")

	creds := map[string]any{"email": "carol@multi.test", "password": "correct horse battery"}
	r := e.client().do("POST", base+"/auth/login", creds, false)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "organization_required", r.Body["code"])
	require.Len(t, r.Body["organizations"], 2)

	// A wrong password never reveals the organization list.
	r = e.client().do("POST", base+"/auth/login", map[string]any{"email": "carol@multi.test", "password": "nope nope nope"}, false)
	require.Equal(t, 401, r.Status)
	require.Nil(t, r.Body["organizations"])

	carol := e.client()
	creds["organization_slug"] = "org-two"
	r = carol.do("POST", base+"/auth/login", creds, false)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "org-two", r.Body["organization"].(map[string]any)["slug"])

	// As a non-owner member: may read the org, may not change it or read the audit trail.
	require.Equal(t, 200, carol.do("GET", base+"/organizations/current", nil, false).Status)
	r = carol.do("PATCH", base+"/organizations/current", map[string]any{"name": "Hijacked"}, true)
	require.Equal(t, 403, r.Status)
	require.Equal(t, "forbidden", r.Body["code"])
	require.Equal(t, 403, carol.do("GET", base+"/audit-events", nil, false).Status)
}

func TestTenantIsolationThroughTheAPI(t *testing.T) {
	e := setup(t)
	a, b := e.client(), e.client()
	require.Equal(t, 201, a.do("POST", base+"/organizations", signupBody("tenant-a", "a@a.test"), false).Status)
	require.Equal(t, 201, b.do("POST", base+"/organizations", signupBody("tenant-b", "b@b.test"), false).Status)

	// Each owner sees only their own organization and audit trail.
	require.Equal(t, "tenant-a", a.do("GET", base+"/organizations/current", nil, false).Body["slug"])
	require.Equal(t, "tenant-b", b.do("GET", base+"/organizations/current", nil, false).Body["slug"])
	r := a.do("GET", base+"/audit-events?limit=200", nil, false)
	for _, ev := range r.Body["data"].([]any) {
		require.NotContains(t, ev.(map[string]any)["details"], "tenant-b")
	}
	// B's sessions cannot be revoked by A even by id.
	bSessions := b.do("GET", base+"/me/sessions", nil, false)
	bSessionID := bSessions.Body["data"].([]any)[0].(map[string]any)["id"].(string)
	require.Equal(t, 404, a.do("DELETE", base+"/me/sessions/"+bSessionID, nil, true).Status)
	require.Equal(t, 200, b.do("GET", base+"/me", nil, false).Status)
}

func TestSessionManagementAndExpiry(t *testing.T) {
	e := setup(t)
	laptop := e.client()
	require.Equal(t, 201, laptop.do("POST", base+"/organizations", signupBody("sessco", "erin@sess.test"), false).Status)
	phone := e.client()
	require.Equal(t, 200, phone.do("POST", base+"/auth/login",
		map[string]any{"email": "erin@sess.test", "password": "correct horse battery"}, false).Status)

	list := laptop.do("GET", base+"/me/sessions", nil, false)
	require.Len(t, list.Body["data"], 2)
	var phoneID string
	for _, s := range list.Body["data"].([]any) {
		if !s.(map[string]any)["current"].(bool) {
			phoneID = s.(map[string]any)["id"].(string)
		}
	}
	require.NotEmpty(t, phoneID)
	require.Equal(t, 204, laptop.do("DELETE", base+"/me/sessions/"+phoneID, nil, true).Status)
	require.Equal(t, 401, phone.do("GET", base+"/me", nil, false).Status, "revoked session must stop working")
	require.Equal(t, 404, laptop.do("DELETE", base+"/me/sessions/"+uuid.NewString(), nil, true).Status)

	// Idle expiry is enforced server-side.
	_, err := e.pg.Owner.ExecContext(context.Background(), `UPDATE sessions SET idle_expires_at = now() - interval '1 second'`)
	require.NoError(t, err)
	require.Equal(t, 401, laptop.do("GET", base+"/me", nil, false).Status)
}

// bindMemberRole gives a fixture user the baseline "member" role, as
// invitations will in S4.
func bindMemberRole(t *testing.T, e *env, orgSlug, email string) {
	t.Helper()
	_, err := e.pg.Owner.ExecContext(context.Background(), `INSERT INTO role_bindings (org_id, role_id, user_id, scope_type)
		SELECT o.id, r.id, u.id, 'org' FROM organizations o
		JOIN roles r ON r.org_id = o.id AND r.key = 'member'
		JOIN users u ON u.email = $2
		WHERE o.slug = $1`, orgSlug, email)
	require.NoError(t, err)
}
