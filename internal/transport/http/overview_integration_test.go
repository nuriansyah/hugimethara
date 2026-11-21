//go:build integration

package api_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	whmodel "github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
)

// S14: the Overview KPIs reflect executions, failures, connector health and
// webhook traffic of the workspace, and are only shown to readers.
func TestWorkspaceOverviewKPIs(t *testing.T) {
	e := setup(t)
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ok.Close()
	f := newWorkflowFixture(t, e, "kpi-co")
	eng := newEngine(t, e, "worker-kpi")
	overview := func(window string) resp {
		q := base + "/overview?workspace_id=" + f.workspaceID
		if window != "" {
			q += "&window=" + window
		}
		return f.owner.do("GET", q, nil, false)
	}

	// Empty workspace: zeros, a full zero-filled series, no success rate yet.
	r := overview("")
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "24h", r.Body["window"])
	wf := r.Body["workflows"].(map[string]any)
	require.EqualValues(t, 0, wf["total"])
	require.Nil(t, wf["success_rate"])
	require.GreaterOrEqual(t, len(wf["series"].([]any)), 24)

	good := f.deploy(t, "kpi-good", conditionalParallelDraft(ok.URL))
	bad := f.deploy(t, "kpi-bad", map[string]any{"schema_version": 1, "trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{node("call", "action.http", map[string]any{"method": "GET", "url": ok.URL + "/bad"})},
		"edges": []any{edge("trigger", "call", "")}})
	f.execute(t, good, map[string]any{"amount": 250}, "")
	f.execute(t, good, map[string]any{"amount": 5}, "")
	f.execute(t, bad, map[string]any{}, "")
	pump(t, e, eng)

	// A connector whose last test failed.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	down := l.Addr().String()
	require.NoError(t, l.Close())
	var httpVersion string
	for _, d := range data(f.owner.do("GET", base+"/connector-definitions", nil, false)) {
		if d["key"] == "http_rest" {
			httpVersion = data(f.owner.do("GET", base+"/connector-definitions/"+d["id"].(string)+"/versions", nil, false))[0]["id"].(string)
		}
	}
	c := f.owner.do("POST", base+"/connectors", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"ref_key": "billing-api", "connector_version_id": httpVersion, "config": map[string]any{"base_url": "http://" + down}}, true)
	require.Equal(t, 201, c.Status, c.Body)
	require.Equal(t, 200, f.owner.do("POST", base+"/connectors/"+c.Body["id"].(string)+"/test", nil, true).Status)
	require.Equal(t, 201, f.owner.do("POST", base+"/connectors", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "ref_key": "never-tested", "connector_version_id": httpVersion,
		"config": map[string]any{"base_url": ok.URL}}, true).Status)

	// One verified webhook, one rejected.
	wh := f.owner.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"name": "KPI hook", "verifier": "standard_webhooks"}, true)
	require.Equal(t, 201, wh.Status, wh.Body)
	body := []byte(`{"type":"kpi.test"}`)
	signed, err := whmodel.SignedHeaders([]byte(wh.Body["secret"].(string)), "msg_kpi", time.Now(), body)
	require.NoError(t, err)
	hdr := http.Header{}
	for k, v := range signed {
		hdr.Set(k, v)
	}
	require.Equal(t, 202, postHook(t, e, wh.Body["public_id"].(string), body, hdr).Status)
	require.Equal(t, 401, postHook(t, e, wh.Body["public_id"].(string), body, nil).Status)

	r = overview("7d")
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "7d", r.Body["window"])
	wf = r.Body["workflows"].(map[string]any)
	require.EqualValues(t, 3, wf["total"])
	require.EqualValues(t, 2, wf["succeeded"])
	require.EqualValues(t, 1, wf["failed"])
	require.EqualValues(t, 0, wf["running"])
	require.EqualValues(t, 2, wf["active_workflows"])
	require.InDelta(t, 2.0/3.0, wf["success_rate"], 0.001)
	require.NotNil(t, wf["p95_ms"])
	series := wf["series"].([]any)
	require.GreaterOrEqual(t, len(series), 7)
	var sumOK, sumFail float64
	for _, p := range series {
		sumOK += p.(map[string]any)["succeeded"].(float64)
		sumFail += p.(map[string]any)["failed"].(float64)
	}
	require.Equal(t, []float64{2, 1}, []float64{sumOK, sumFail})
	top := wf["top_failing"].([]any)
	require.Len(t, top, 1)
	require.Equal(t, "kpi-bad", top[0].(map[string]any)["name"])
	require.Equal(t, "node_failed", top[0].(map[string]any)["last_error_code"])

	cn := r.Body["connectors"].(map[string]any)
	require.EqualValues(t, 2, cn["total"])
	require.EqualValues(t, 0, cn["healthy"])
	require.EqualValues(t, 1, cn["unhealthy"])
	require.EqualValues(t, 1, cn["unchecked"])
	failing := cn["failing"].([]any)
	require.Len(t, failing, 1)
	require.Equal(t, "billing-api", failing[0].(map[string]any)["ref_key"])
	require.Equal(t, "unreachable", failing[0].(map[string]any)["error_code"])

	hk := r.Body["webhooks"].(map[string]any)
	require.EqualValues(t, 1, hk["endpoints"])
	require.EqualValues(t, 1, hk["received"])
	require.EqualValues(t, 1, hk["rejected_total"])

	// Validation and access.
	bad422 := overview("1y")
	require.Equal(t, 422, bad422.Status, bad422.Body)
	require.Equal(t, 400, f.owner.do("GET", base+"/overview", nil, false).Status)
	member, _ := addMember(t, e, "kpi-co", "viewer@kpi-co.test")
	denied := member.do("GET", base+"/overview?workspace_id="+f.workspaceID, nil, false)
	require.Equal(t, 403, denied.Status, denied.Body)
}
