//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	connectorsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/services"
	secretssvc "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/services"
	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
)

// newConnectorEngine is newEngine plus the connector runtime, as hugi-worker
// builds it (same master key and KEK id as the API in setup).
func newConnectorEngine(t *testing.T, e *env, worker string) *workflowsvc.Engine {
	t.Helper()
	x, err := workflowsvc.NewExpressions()
	require.NoError(t, err)
	opener, err := secretssvc.NewOpener(secretbox.DeriveKey("integration-test"), "test-v1")
	require.NoError(t, err)
	nodes := workflowsvc.NewRegistry(x).WithConnectors(x, connectorsvc.NewRuntime(opener))
	return workflowsvc.NewEngine(e.pg.System, nodes, workflowsvc.EngineConfig{
		WorkerID: worker, Lease: 2 * time.Second, RedispatchAge: time.Hour,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// toolVersions returns the connector version id per external tool key.
func toolVersions(t *testing.T, owner *client) map[string]string {
	t.Helper()
	defs := owner.do("GET", base+"/connector-definitions", nil, false)
	require.Equal(t, 200, defs.Status, defs.Body)
	out := map[string]string{}
	for _, d := range data(defs) {
		key := d["key"].(string)
		if key != "n8n" && key != "make" && key != "zapier" {
			continue
		}
		require.Equal(t, "automation", d["category"])
		versions := owner.do("GET", base+"/connector-definitions/"+d["id"].(string)+"/versions", nil, false)
		require.Equal(t, 200, versions.Status, versions.Body)
		v := data(versions)[0]
		require.Contains(t, mustJSONString(t, v["operations"]), "trigger_workflow")
		out[key] = v["id"].(string)
	}
	require.Len(t, out, 3, "n8n, Make and Zapier are in the catalog")
	return out
}

func mustJSONString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func connectorDraft(ref, op string, retry map[string]any) map[string]any {
	n := node("handoff", "action.connector", map[string]any{"connector": ref, "operation": op,
		"input_expression": `{"email": input.email, "source": "hugimethara"}`})
	if retry != nil {
		n["retry"] = retry
	}
	return map[string]any{"schema_version": 1, "trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{n, node("done", "action.transform", map[string]any{"rules": []any{
			map[string]any{"target": "tool_status", "source": "$.status"}}})},
		"edges": []any{edge("trigger", "handoff", ""), edge("handoff", "done", "")}}
}

// tryDeploy creates and publishes a draft and attempts the deployment.
func (f workflowFixture) tryDeploy(t *testing.T, slug string, draft map[string]any) resp {
	t.Helper()
	created := f.owner.do("POST", base+"/workflows", map[string]any{
		"workspace_id": f.workspaceID, "name": slug, "slug": slug, "draft": draft}, true)
	require.Equal(t, 201, created.Status, created.Body)
	pub := f.owner.do("POST", base+"/workflows/"+created.Body["id"].(string)+"/publish", nil, true)
	require.Equal(t, 201, pub.Status, pub.Body)
	return f.owner.do("PUT", base+"/workflows/"+created.Body["id"].(string)+"/deployments/"+f.envID,
		map[string]any{"version_id": pub.Body["id"], "enabled": true}, true)
}

func fieldCode(r resp) string {
	return r.Body["errors"].([]any)[0].(map[string]any)["code"].(string)
}

// S13 acceptance: n8n / Make / Zapier are optional integrations. A workflow
// hands off to n8n with a vault credential and receives n8n's status back on
// a token-preset webhook; while Zapier is unreachable only the step that
// calls it fails (retried, then a clear reason code) and everything else —
// deploys, native workflows, webhooks, connection tests — keeps working.
func TestExternalWorkflowToolsAreOptional(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	var mu sync.Mutex
	var n8nCalls []*http.Request
	var n8nBodies []map[string]any
	n8n := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		n8nCalls = append(n8nCalls, r.Clone(context.Background()))
		n8nBodies = append(n8nBodies, body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"executionId":"n8n-run-1","accepted":true}`))
	}))
	defer n8n.Close()
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer native.Close()
	// Zapier is "down": a loopback port nobody listens on (connection refused).
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	downAddr := l.Addr().String()
	require.NoError(t, l.Close())

	f := newWorkflowFixture(t, e, "ext-tools")
	eng := newConnectorEngine(t, e, "worker-ext")
	runner := newTriggerRunner(t, e)
	versions := toolVersions(t, f.owner)

	// Catalog: action.connector is available now.
	types := f.owner.do("GET", base+"/node-types", nil, false)
	require.Equal(t, 200, types.Status, types.Body)
	for _, nt := range data(types) {
		if nt["type"] == "action.connector" {
			require.Equal(t, true, nt["available"], nt)
		}
	}

	const n8nToken = "Bearer n8n-header-credential-value"
	cred := f.owner.do("POST", base+"/credentials", map[string]any{"name": "n8n header", "kind": "bearer",
		"value": n8nToken}, true)
	require.Equal(t, 201, cred.Status, cred.Body)
	r := f.owner.do("POST", base+"/connectors", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "ref_key": "lead-sync", "connector_version_id": versions["n8n"],
		"config": map[string]any{"webhook_url": n8n.URL + "/webhook/leads"}, "auth_method": "header",
		"secret_id": cred.Body["id"]}, true)
	require.Equal(t, 201, r.Status, r.Body)
	n8nConnector := r.Body["id"].(string)
	r = f.owner.do("POST", base+"/connectors", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "ref_key": "crm-zap", "connector_version_id": versions["zapier"],
		"config": map[string]any{"webhook_url": "http://" + downAddr + "/hooks/catch/1/abc/"}}, true)
	require.Equal(t, 201, r.Status, r.Body)
	zapConnector := r.Body["id"].(string)
	// Zapier offers no header auth (its catch-hook URL is the secret).
	r = f.owner.do("POST", base+"/connectors", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "ref_key": "zap-bad", "connector_version_id": versions["zapier"],
		"config": map[string]any{"webhook_url": "https://hooks.zapier.com/x"}, "auth_method": "header",
		"secret_id": cred.Body["id"]}, true)
	require.Equal(t, 422, r.Status, r.Body)

	// Deploy checks the reference without contacting the tool.
	dep := f.tryDeploy(t, "missing", connectorDraft("no-such-tool", "trigger_workflow", nil))
	require.Equal(t, 422, dep.Status, dep.Body)
	require.Equal(t, "connector_not_found", fieldCode(dep))
	dep = f.tryDeploy(t, "wrong-op", connectorDraft("lead-sync", "delete_everything", nil))
	require.Equal(t, 422, dep.Status, dep.Body)
	require.Equal(t, "connector_operation_unsupported", fieldCode(dep))
	handoff := f.deploy(t, "lead-handoff", connectorDraft("lead-sync", "trigger_workflow", nil))
	zap := f.deploy(t, "crm-zap", connectorDraft("crm-zap", "trigger_workflow",
		map[string]any{"max_attempts": 2, "backoff": "fixed", "initial_ms": 1}))
	nativeWF := f.deploy(t, "native", conditionalParallelDraft(native.URL))

	// Trigger: n8n receives the input, the vault credential and correlation headers.
	execID := f.execute(t, handoff, map[string]any{"email": "ada@example.com"}, "").Body["id"].(string)
	pump(t, e, eng)
	got := f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	step := stepsByNode(got)["handoff"]
	out := step["output"].(map[string]any)
	require.EqualValues(t, 200, out["status"])
	require.Equal(t, "n8n-run-1", out["body"].(map[string]any)["executionId"])
	require.Equal(t, map[string]any{"tool_status": 200.0}, stepsByNode(got)["done"]["output"])
	require.NotContains(t, mustJSONString(t, got.Body), "n8n-header-credential-value")
	require.Len(t, n8nCalls, 1)
	require.Equal(t, n8nToken, n8nCalls[0].Header.Get("Authorization"))
	require.Equal(t, execID, n8nCalls[0].Header.Get("X-Hugi-Execution-Id"))
	require.NotEmpty(t, n8nCalls[0].Header.Get("Idempotency-Key"))
	require.Equal(t, "/webhook/leads", n8nCalls[0].URL.Path)
	require.Equal(t, map[string]any{"email": "ada@example.com", "source": "hugimethara"}, n8nBodies[0])

	// Zapier unreachable: retried, then failed with connector_unavailable.
	zapExec := f.execute(t, zap, map[string]any{"email": "bob@example.com"}, "").Body["id"].(string)
	// Meanwhile a native workflow in the same organization runs normally.
	nativeExec := f.execute(t, nativeWF, map[string]any{"amount": 250}, "").Body["id"].(string)
	pump(t, e, eng)
	got = f.owner.do("GET", base+"/workflow-executions/"+zapExec, nil, false)
	require.Equal(t, "running", got.Body["status"], got.Body)
	require.Equal(t, "retry_scheduled", stepsByNode(got)["handoff"]["status"])
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, eng.ReapOnce(ctx))
	pump(t, e, eng)
	got = f.owner.do("GET", base+"/workflow-executions/"+zapExec, nil, false)
	require.Equal(t, "failed", got.Body["status"], got.Body)
	step = stepsByNode(got)["handoff"]
	require.EqualValues(t, 2, step["attempt"])
	stepErr := step["error"].(map[string]any)
	require.Equal(t, "connector_unavailable", stepErr["code"])
	require.Equal(t, true, stepErr["retryable"])
	require.NotContains(t, mustJSONString(t, got.Body), downAddr, "the webhook URL is not leaked")
	require.NotContains(t, stepsByNode(got), "done", "nothing runs after the failed hand-off")
	got = f.owner.do("GET", base+"/workflow-executions/"+nativeExec, nil, false)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)

	// Connection test reports the outage; the tool being down is not an API error.
	test := f.owner.do("POST", base+"/connectors/"+zapConnector+"/test", nil, true)
	require.Equal(t, 200, test.Status, test.Body)
	require.Equal(t, false, test.Body["ok"])
	require.Equal(t, "unreachable", test.Body["error_code"])
	require.Equal(t, 200, f.owner.do("GET", base+"/connectors/"+zapConnector+"/health", nil, false).Status)

	// Inbound status from n8n on a token preset endpoint starts a workflow.
	r = f.owner.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"name": "n8n status", "verifier": "n8n"}, true)
	require.Equal(t, 201, r.Status, r.Body)
	endpointID, publicID := r.Body["id"].(string), r.Body["public_id"].(string)
	token := r.Body["secret"].(string)
	require.Len(t, token, 64, "the token is generated and returned once")
	require.NotContains(t, f.owner.do("GET", base+"/webhooks/"+endpointID, nil, false).Body, "secret")
	statusWF := f.deploy(t, "n8n-status", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.webhook", "config": map[string]any{
			"endpoint_id": endpointID, "condition": "input.event_type == 'n8n.success'"}},
		"nodes": []any{node("record", "action.transform", map[string]any{"rules": []any{
			map[string]any{"target": "run", "source": "$.payload.run_id"},
			map[string]any{"target": "execution", "source": "$.payload.execution_id"}}})},
		"edges": []any{edge("trigger", "record", "")}})
	takeEvents(t, e)

	status, _ := json.Marshal(map[string]any{"run_id": "n8n-run-1", "status": "success", "execution_id": execID})
	for name, c := range map[string]struct {
		header http.Header
		code   string
	}{
		"missing": {nil, "signature_missing"},
		"wrong":   {http.Header{"X-Hugi-Token": {token[:60] + "beef"}}, "signature_invalid"},
	} {
		rej := postHook(t, e, publicID, status, c.header)
		require.Equal(t, 401, rej.Status, name)
		require.Equal(t, c.code, rej.Body["code"], name)
	}
	acc := postHook(t, e, publicID, status, http.Header{"X-Hugi-Token": {token}})
	require.Equal(t, 202, acc.Status, acc.Body)
	evs := received(relayEvents(t, e, runner, nil))
	require.Len(t, evs, 1)
	pump(t, e, eng)
	execs := f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+statusWF, nil, false)
	require.Len(t, data(execs), 1)
	require.Equal(t, "succeeded", data(execs)[0]["status"], data(execs)[0])
	got = f.owner.do("GET", base+"/workflow-executions/"+data(execs)[0]["id"].(string), nil, false)
	require.Equal(t, map[string]any{"run": "n8n-run-1", "execution": execID}, stepsByNode(got)["record"]["output"])
	// The same run status again is a duplicate; the token is never stored.
	dup := postHook(t, e, publicID, status, http.Header{"X-Hugi-Token": {token}})
	require.Equal(t, 200, dup.Status, dup.Body)
	require.Equal(t, "duplicate", dup.Body["status"])
	var stored string
	require.NoError(t, e.pg.Owner.GetContext(ctx, &stored, `SELECT headers::text FROM webhook_events WHERE endpoint_id = $1`, endpointID))
	require.NotContains(t, stored, token)
	require.Contains(t, stored, "[redacted]")

	// A disabled connector fails its step without a call.
	r = f.owner.do("PATCH", base+"/connectors/"+n8nConnector, map[string]any{"status": "disabled"}, true)
	require.Equal(t, 200, r.Status, r.Body)
	execID = f.execute(t, handoff, map[string]any{"email": "eve@example.com"}, "").Body["id"].(string)
	pump(t, e, eng)
	got = f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, "failed", got.Body["status"], got.Body)
	require.Equal(t, "connector_disabled", stepsByNode(got)["handoff"]["error"].(map[string]any)["code"])
	require.Len(t, n8nCalls, 1)

	// Another organization cannot reference this workspace's connector.
	other := newWorkflowFixture(t, e, "ext-other")
	dep = other.tryDeploy(t, "steal", connectorDraft("lead-sync", "trigger_workflow", nil))
	require.Equal(t, 422, dep.Status, dep.Body)
	require.Equal(t, "connector_not_found", fieldCode(dep))
}
