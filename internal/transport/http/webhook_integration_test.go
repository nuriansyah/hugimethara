//go:build integration

package api_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	whmodel "github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	webhooksvc "github.com/nuriansyah/hugimethara/internal/modules/webhook/services"
	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
)

// outboxEvent is a platform event row as the relay would publish it.
type outboxEvent struct {
	ID      int64     `db:"id"`
	OrgID   uuid.UUID `db:"org_id"`
	Subject string    `db:"subject"`
	Payload []byte    `db:"payload"`
}

func (o outboxEvent) name() string  { n, _ := workflowsvc.EventFromSubject(o.Subject); return n }
func (o outboxEvent) msgID() string { return "outbox-" + strconv.FormatInt(o.ID, 10) }

// takeEvents removes the unsent platform events from the outbox.
func takeEvents(t *testing.T, e *env) []outboxEvent {
	t.Helper()
	ctx := context.Background()
	var rows []outboxEvent
	require.NoError(t, e.pg.Owner.SelectContext(ctx, &rows,
		`SELECT id, org_id, subject, payload FROM outbox WHERE sent_at IS NULL AND subject LIKE 'hugi.events.%' ORDER BY id`))
	for _, r := range rows {
		_, err := e.pg.Owner.ExecContext(ctx, `UPDATE outbox SET sent_at = now() WHERE id = $1`, r.ID)
		require.NoError(t, err)
	}
	return rows
}

// relayEvents hands every pending platform event to the workflow trigger
// runner and the webhook fan-out, as the two hugi-worker consumers do.
func relayEvents(t *testing.T, e *env, triggers *workflowsvc.TriggerRunner, d *webhooksvc.Dispatcher) []outboxEvent {
	t.Helper()
	evs := takeEvents(t, e)
	for _, ev := range evs {
		if triggers != nil {
			require.NoError(t, triggers.HandleEvent(context.Background(), workflowsvc.Event{OrgID: ev.OrgID,
				Name: ev.name(), ID: ev.msgID(), Data: ev.Payload}))
		}
		if d != nil {
			require.NoError(t, d.HandleEvent(context.Background(), webhooksvc.PlatformEvent{OrgID: ev.OrgID,
				Name: ev.name(), ID: ev.msgID(), Data: ev.Payload}))
		}
	}
	return evs
}

// received keeps only the webhook.received events.
func received(evs []outboxEvent) []outboxEvent {
	var out []outboxEvent
	for _, ev := range evs {
		if ev.Subject == "hugi.events.webhook.received" {
			out = append(out, ev)
		}
	}
	return out
}

func newDispatcher(t *testing.T, e *env) *webhooksvc.Dispatcher {
	t.Helper()
	return webhooksvc.NewDispatcher(e.pg.System, e.sealer, webhooksvc.DispatcherConfig{Timeout: 5 * time.Second,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

// postHook sends a raw request to the ingress like an external sender.
func postHook(t *testing.T, e *env, publicID string, body []byte, header http.Header) resp {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), "POST", e.srv.URL+"/hooks/"+publicID, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	out := resp{Status: res.StatusCode, Header: res.Header}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

func stripeHeader(secret string, at time.Time, body []byte) http.Header {
	ts := strconv.FormatInt(at.Unix(), 10)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(ts + "." + string(body)))
	return http.Header{"Stripe-Signature": {"t=" + ts + ",v1=" + hex.EncodeToString(m.Sum(nil))}}
}

func stripeEvent(id, typ, payment string, amount int) []byte {
	b, _ := json.Marshal(map[string]any{"id": id, "type": typ, "data": map[string]any{
		"object": map[string]any{"id": payment, "amount": amount, "currency": "idr"}}})
	return b
}

func reconciliationDraft(endpointID, ledgerURL string) map[string]any {
	return map[string]any{
		"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.webhook", "config": map[string]any{
			"endpoint_id": endpointID, "condition": "input.event_type == 'payment_intent.succeeded'"}},
		"nodes": []any{
			node("reconcile", "action.http", map[string]any{"method": "POST", "url": ledgerURL + "/reconcile",
				"body_expression": `{"payment_id": input.payload.data.object.id, "amount": input.payload.data.object.amount, "stripe_event": input.provider_event_id}`}),
		},
		"edges": []any{edge("trigger", "reconcile", "")},
	}
}

// S12 acceptance: a signed payment webhook starts the reconciliation
// workflow; forged, stale and replayed requests are rejected or deduplicated.
func TestPaymentWebhookReconciliationAndForgeryRejected(t *testing.T) {
	e := setup(t)
	var calls atomic.Int64
	var mu sync.Mutex
	var ledger []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		ledger = append(ledger, body)
		mu.Unlock()
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reconciled":true}`))
	}))
	defer srv.Close()
	f := newWorkflowFixture(t, e, "wh-pay")
	eng := newEngine(t, e, "worker-wh")
	runner := newTriggerRunner(t, e)

	// A provider preset needs the provider's secret; it is never echoed back.
	r := f.owner.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"name": "Stripe payments", "verifier": "stripe"}, true)
	require.Equal(t, 422, r.Status, r.Body)
	const stripeSecret = "whsec_stripe_test_0123456789abcdef"
	r = f.owner.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"name": "Stripe payments", "verifier": "stripe", "secret": stripeSecret}, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Nil(t, r.Body["secret"], "a supplied secret is not echoed")
	endpointID, publicID := r.Body["id"].(string), r.Body["public_id"].(string)
	require.Equal(t, "/hooks/"+publicID, r.Body["ingress_path"])
	require.Equal(t, "…cdef", r.Body["secret_hint"])
	got := f.owner.do("GET", base+"/webhooks/"+endpointID, nil, false)
	require.Equal(t, 200, got.Status)
	require.NotContains(t, got.Body, "secret")

	// Deploying a webhook trigger checks the endpoint.
	bogus := reconciliationDraft(uuid.NewString(), srv.URL)
	created := f.owner.do("POST", base+"/workflows", map[string]any{"workspace_id": f.workspaceID, "name": "bogus",
		"slug": "bogus", "draft": bogus}, true)
	require.Equal(t, 201, created.Status, created.Body)
	pub := f.owner.do("POST", base+"/workflows/"+created.Body["id"].(string)+"/publish", nil, true)
	require.Equal(t, 201, pub.Status, pub.Body)
	dep := f.owner.do("PUT", base+"/workflows/"+created.Body["id"].(string)+"/deployments/"+f.envID,
		map[string]any{"version_id": pub.Body["id"], "enabled": true}, true)
	require.Equal(t, 422, dep.Status, dep.Body)
	require.Equal(t, "webhook_endpoint_not_found", dep.Body["errors"].([]any)[0].(map[string]any)["code"])
	workflowID := f.deploy(t, "reconcile-payments", reconciliationDraft(endpointID, srv.URL))
	takeEvents(t, e) // workflow.published events are not under test here

	// A genuine payment event is accepted and starts one execution.
	now := time.Now()
	body := stripeEvent("evt_1", "payment_intent.succeeded", "pi_1", 150000)
	acc := postHook(t, e, publicID, body, stripeHeader(stripeSecret, now, body))
	require.Equal(t, 202, acc.Status, acc.Body)
	require.Equal(t, "accepted", acc.Body["status"])
	eventID := acc.Body["id"].(string)
	evs := received(relayEvents(t, e, runner, nil))
	require.Len(t, evs, 1)
	// The same stream message delivered twice starts nothing new.
	require.NoError(t, runner.HandleEvent(context.Background(), workflowsvc.Event{OrgID: evs[0].OrgID,
		Name: evs[0].name(), ID: evs[0].msgID(), Data: evs[0].Payload}))
	pump(t, e, eng)
	execs := f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+workflowID, nil, false)
	require.Equal(t, 200, execs.Status, execs.Body)
	require.Len(t, data(execs), 1)
	exec := data(execs)[0]
	require.Equal(t, "succeeded", exec["status"], exec)
	require.Equal(t, "trigger.webhook", exec["trigger_type"])
	require.EqualValues(t, 1, calls.Load())
	require.Equal(t, map[string]any{"payment_id": "pi_1", "amount": 150000.0, "stripe_event": "evt_1"}, ledger[0])

	// Replay of the captured request: deduplicated on the Stripe event id.
	dup := postHook(t, e, publicID, body, stripeHeader(stripeSecret, now, body))
	require.Equal(t, 200, dup.Status, dup.Body)
	require.Equal(t, "duplicate", dup.Body["status"])
	require.Empty(t, received(relayEvents(t, e, runner, nil)))

	// Forgeries: tampered body, stale timestamp, missing or wrong signature.
	forged := stripeEvent("evt_2", "payment_intent.succeeded", "pi_attacker", 999999999)
	cases := []struct {
		name   string
		body   []byte
		header http.Header
		code   string
	}{
		{"tampered", forged, stripeHeader(stripeSecret, now, body), "signature_invalid"},
		{"wrong secret", forged, stripeHeader("whsec_attacker_guess_0000000000", now, forged), "signature_invalid"},
		{"stale", forged, stripeHeader(stripeSecret, now.Add(-10*time.Minute), forged), "timestamp_out_of_tolerance"},
		{"unsigned", forged, nil, "signature_missing"},
	}
	for _, c := range cases {
		rej := postHook(t, e, publicID, c.body, c.header)
		require.Equal(t, 401, rej.Status, c.name)
		require.Equal(t, c.code, rej.Body["code"], c.name)
	}
	require.Empty(t, received(relayEvents(t, e, runner, nil)), "rejected requests emit nothing")
	require.Equal(t, 404, postHook(t, e, "0123456789abcdef0123456789abcdef", body, stripeHeader(stripeSecret, now, body)).Status)
	require.Equal(t, 404, postHook(t, e, "not-a-public-id", body, nil).Status)

	// Only the genuine event was stored; rejections were counted.
	list := f.owner.do("GET", base+"/webhook-events?endpoint_id="+endpointID, nil, false)
	require.Equal(t, 200, list.Status, list.Body)
	require.Len(t, data(list), 1)
	require.Equal(t, "evt_1", data(list)[0]["provider_event_id"])
	require.Equal(t, "payment_intent.succeeded", data(list)[0]["event_type"])
	require.Nil(t, data(list)[0]["payload"], "list omits bodies")
	one := f.owner.do("GET", base+"/webhook-events/"+eventID, nil, false)
	require.Equal(t, 200, one.Status, one.Body)
	require.Equal(t, "[redacted]", one.Body["headers"].(map[string]any)["Stripe-Signature"])
	require.Equal(t, "pi_1", one.Body["payload"].(map[string]any)["data"].(map[string]any)["object"].(map[string]any)["id"])
	got = f.owner.do("GET", base+"/webhooks/"+endpointID, nil, false)
	require.EqualValues(t, 1, got.Body["received_count"])
	require.EqualValues(t, 4, got.Body["rejected_count"])
	require.Equal(t, "signature_missing", got.Body["last_rejected_reason"])

	// Other event types are stored but filtered out by the trigger condition.
	other := stripeEvent("evt_3", "payment_intent.created", "pi_3", 10)
	require.Equal(t, 202, postHook(t, e, publicID, other, stripeHeader(stripeSecret, now, other)).Status)
	relayEvents(t, e, runner, nil)
	pump(t, e, eng)
	execs = f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+workflowID, nil, false)
	require.Len(t, data(execs), 1)

	// Disabling the endpoint makes it indistinguishable from an unknown one.
	upd := f.owner.do("PATCH", base+"/webhooks/"+endpointID, map[string]any{"enabled": false}, true)
	require.Equal(t, 200, upd.Status, upd.Body)
	fresh := stripeEvent("evt_4", "payment_intent.succeeded", "pi_4", 5)
	require.Equal(t, 404, postHook(t, e, publicID, fresh, stripeHeader(stripeSecret, now, fresh)).Status)

	// Rotating to a new provider secret: the old one stops verifying.
	require.Equal(t, 200, f.owner.do("PATCH", base+"/webhooks/"+endpointID, map[string]any{"enabled": true}, true).Status)
	rot := f.owner.do("POST", base+"/webhooks/"+endpointID+"/rotate-secret",
		map[string]any{"secret": "whsec_rotated_secret_abcdef0123"}, true)
	require.Equal(t, 200, rot.Status, rot.Body)
	require.Equal(t, 401, postHook(t, e, publicID, fresh, stripeHeader(stripeSecret, now, fresh)).Status)
	require.Equal(t, 202, postHook(t, e, publicID, fresh, stripeHeader("whsec_rotated_secret_abcdef0123", now, fresh)).Status)

	// Tenant isolation and authorization.
	outsider := e.client()
	require.Equal(t, 201, outsider.do("POST", base+"/organizations", signupBody("wh-other", "owner@wh-other.test"), false).Status)
	require.Equal(t, 404, outsider.do("GET", base+"/webhooks/"+endpointID, nil, false).Status)
	require.Equal(t, 404, outsider.do("GET", base+"/webhook-events/"+eventID, nil, false).Status)
	require.Equal(t, 404, outsider.do("GET", base+"/webhook-events?endpoint_id="+endpointID, nil, false).Status)
	member, _ := addMember(t, e, "wh-pay", "member@wh-pay.test")
	denied := member.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "name": "Sneaky"}, true)
	require.Equal(t, 403, denied.Status, denied.Body)

	// Audit trail.
	actions := auditActions(t, f.owner, "?resource_type=webhook")
	require.Contains(t, actions, "webhook.created:success")
	require.Contains(t, actions, "webhook.secret_rotated:success")
}

func TestStandardWebhooksEndpointGeneratesSecret(t *testing.T) {
	e := setup(t)
	f := newWorkflowFixture(t, e, "wh-std")
	r := f.owner.do("POST", base+"/webhooks", map[string]any{"workspace_id": f.workspaceID, "environment_id": f.envID,
		"name": "Partner orders"}, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, "standard_webhooks", r.Body["verifier"])
	secret := r.Body["secret"].(string)
	require.Regexp(t, `^whsec_`, secret)
	publicID := r.Body["public_id"].(string)

	body := []byte(`{"type":"order.created","data":{"id":"o-1"}}`)
	hdr, err := whmodel.SignedHeaders([]byte(secret), "msg_partner_1", time.Now(), body)
	require.NoError(t, err)
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	require.Equal(t, 202, postHook(t, e, publicID, body, h).Status)
	require.Equal(t, "duplicate", postHook(t, e, publicID, body, h).Body["status"])
	evs := received(takeEvents(t, e))
	require.Len(t, evs, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(evs[0].Payload, &payload))
	require.Equal(t, "msg_partner_1", payload["provider_event_id"])
	require.Equal(t, "order.created", payload["event_type"])
	require.Equal(t, map[string]any{"id": "o-1"}, payload["payload"].(map[string]any)["data"])
}

// receiver is an outbound webhook destination that verifies Standard
// Webhooks signatures and answers with a scripted status.
type receiver struct {
	mu       sync.Mutex
	secret   string
	statuses []int // consumed in order; 200 once exhausted
	got      []webhooksvc.Body
	ids      []string
	invalid  int
	srv      *httptest.Server
}

func newReceiver(t *testing.T) *receiver {
	rc := &receiver{}
	rc.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rc.mu.Lock()
		defer rc.mu.Unlock()
		if _, err := whmodel.Verify(whmodel.SchemeStandardWebhooks, whmodel.HMACConfig{}, []byte(rc.secret),
			whmodel.VerifyRequest{Header: r.Header, Body: raw, Now: time.Now()}); err != nil {
			rc.invalid++
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var b webhooksvc.Body
		_ = json.Unmarshal(raw, &b)
		rc.got = append(rc.got, b)
		rc.ids = append(rc.ids, r.Header.Get("Webhook-Id"))
		status := 200
		if len(rc.statuses) > 0 {
			status, rc.statuses = rc.statuses[0], rc.statuses[1:]
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"status":%d}`, status)
	}))
	t.Cleanup(rc.srv.Close)
	return rc
}

func (rc *receiver) script(secret string, statuses ...int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.secret, rc.statuses = secret, statuses
}

func makeDeliveriesDue(t *testing.T, e *env) {
	t.Helper()
	_, err := e.pg.Owner.ExecContext(context.Background(),
		`UPDATE webhook_deliveries SET next_attempt_at = now() - interval '1 second' WHERE status = 'pending'`)
	require.NoError(t, err)
}

// S12: outbound subscriptions — signed delivery, retry schedule, dead
// letter, replay, test ping, workspace scoping, egress guard, auto-disable.
func TestOutboundDeliveryRetryDeadLetterAndReplay(t *testing.T) {
	e := setup(t)
	f := newWorkflowFixture(t, e, "wh-out")
	eng := newEngine(t, e, "worker-out")
	d := newDispatcher(t, e)
	rc := newReceiver(t)

	r := f.owner.do("POST", base+"/webhook-subscriptions", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "name": "Ops bus", "url": rc.srv.URL + "/events",
		"event_types": []string{"workflow.execution_finished", "organization.updated"}}, true)
	require.Equal(t, 422, r.Status, "organization events are not subscribable")
	r = f.owner.do("POST", base+"/webhook-subscriptions", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "name": "Ops bus", "url": rc.srv.URL + "/events",
		"event_types": []string{"workflow.execution_finished"}}, true)
	require.Equal(t, 201, r.Status, r.Body)
	subID, secret := r.Body["id"].(string), r.Body["secret"].(string)
	require.Regexp(t, `^whsec_`, secret)
	rc.script(secret, 500) // first attempt fails, the retry succeeds
	types := f.owner.do("GET", base+"/event-types", nil, false)
	require.Equal(t, 200, types.Status)
	require.Len(t, data(types), len(whmodel.Catalog()))

	// A workflow run finishes → one delivery to the subscription.
	wfID := f.deploy(t, "quick", map[string]any{"schema_version": 1, "trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{node("shape", "action.transform", shape("ok"))}, "edges": []any{edge("trigger", "shape", "")}})
	f.execute(t, wfID, map[string]any{"n": 1}, "")
	pump(t, e, eng)
	evs := relayEvents(t, e, nil, d)
	relayAgain := func() { // redelivered stream messages enqueue nothing new
		for _, ev := range evs {
			require.NoError(t, d.HandleEvent(context.Background(), webhooksvc.PlatformEvent{OrgID: ev.OrgID,
				Name: ev.name(), ID: ev.msgID(), Data: ev.Payload}))
		}
	}
	relayAgain()
	list := f.owner.do("GET", base+"/webhook-deliveries?subscription_id="+subID, nil, false)
	require.Equal(t, 200, list.Status, list.Body)
	require.Len(t, data(list), 1)
	deliveryID := data(list)[0]["id"].(string)
	require.Equal(t, "workflow.execution_finished", data(list)[0]["event_type"])

	n, err := d.DeliverDue(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	got := f.owner.do("GET", base+"/webhook-deliveries/"+deliveryID, nil, false)
	require.Equal(t, "pending", got.Body["status"], got.Body)
	require.EqualValues(t, 1, got.Body["attempts"])
	require.EqualValues(t, 500, got.Body["response_status"])
	require.Equal(t, "http_internal_server_error", got.Body["error"])
	next, err := time.Parse(time.RFC3339Nano, got.Body["next_attempt_at"].(string))
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(30*time.Second), next, 10*time.Second, "first retry after 30s")
	n, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "not due yet")

	makeDeliveriesDue(t, e)
	_, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	got = f.owner.do("GET", base+"/webhook-deliveries/"+deliveryID, nil, false)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	require.Len(t, got.Body["attempt_log"], 2)
	require.Nil(t, got.Body["next_attempt_at"])
	require.Zero(t, rc.invalid, "every delivery carried a valid signature")
	require.Len(t, rc.got, 2)
	require.Equal(t, rc.ids[0], rc.ids[1], "retries keep the webhook-id")
	require.Equal(t, "workflow.execution_finished", rc.got[1].Type)
	var finished map[string]any
	require.NoError(t, json.Unmarshal(rc.got[1].Data, &finished))
	require.Equal(t, "succeeded", finished["status"])
	require.Equal(t, f.workspaceID, finished["workspace_id"])
	sub := f.owner.do("GET", base+"/webhook-subscriptions/"+subID, nil, false)
	require.NotEmpty(t, sub.Body["last_success_at"])
	require.Nil(t, sub.Body["secret"])

	// A destination that never recovers is dead-lettered after 8 attempts.
	rc.script(secret, 500, 500, 500, 500, 500, 500, 500, 500, 500)
	f.execute(t, wfID, map[string]any{"n": 2}, "")
	pump(t, e, eng)
	relayEvents(t, e, nil, d)
	for range whmodel.MaxAttempts {
		makeDeliveriesDue(t, e)
		_, err := d.DeliverDue(context.Background())
		require.NoError(t, err)
	}
	dead := f.owner.do("GET", base+"/webhook-deliveries?subscription_id="+subID+"&status=dead_lettered", nil, false)
	require.Len(t, data(dead), 1)
	deadID := data(dead)[0]["id"].(string)
	require.EqualValues(t, 8, data(dead)[0]["attempts"])
	makeDeliveriesDue(t, e)
	n, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "a dead-lettered delivery is never retried on its own")

	// Replay creates a new delivery linked to the original.
	rc.script(secret)
	rep := f.owner.do("POST", base+"/webhook-deliveries/"+deadID+"/replay", nil, true)
	require.Equal(t, 202, rep.Status, rep.Body)
	require.Equal(t, deadID, rep.Body["replay_of"])
	require.Equal(t, "pending", rep.Body["status"])
	again := f.owner.do("POST", base+"/webhook-deliveries/"+rep.Body["id"].(string)+"/replay", nil, true)
	require.Equal(t, 409, again.Status)
	require.Equal(t, "delivery_in_progress", again.Body["code"])
	_, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	replayed := f.owner.do("GET", base+"/webhook-deliveries/"+rep.Body["id"].(string), nil, false)
	require.Equal(t, "succeeded", replayed.Body["status"], replayed.Body)

	// Test ping, signed with a rotated secret.
	rot := f.owner.do("POST", base+"/webhook-subscriptions/"+subID+"/rotate-secret", nil, true)
	require.Equal(t, 200, rot.Status, rot.Body)
	rc.script(rot.Body["secret"].(string))
	ping := f.owner.do("POST", base+"/webhook-subscriptions/"+subID+"/ping", nil, true)
	require.Equal(t, 202, ping.Status, ping.Body)
	require.Equal(t, "webhook.ping", ping.Body["event_type"])
	_, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	require.Equal(t, "succeeded", f.owner.do("GET", base+"/webhook-deliveries/"+ping.Body["id"].(string), nil, false).Body["status"])
	require.Zero(t, rc.invalid)

	// Another workspace's events never reach this subscription.
	ws2 := f.owner.do("POST", base+"/workspaces", map[string]any{"slug": "finance", "name": "Finance"}, true)
	require.Equal(t, 201, ws2.Status, ws2.Body)
	other := workflowFixture{owner: f.owner, workspaceID: ws2.Body["id"].(string), envID: f.envID}
	otherWF := other.deploy(t, "other", map[string]any{"schema_version": 1, "trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{node("shape", "action.transform", shape("ok"))}, "edges": []any{edge("trigger", "shape", "")}})
	other.execute(t, otherWF, map[string]any{}, "")
	pump(t, e, eng)
	before := len(data(f.owner.do("GET", base+"/webhook-deliveries?subscription_id="+subID, nil, false)))
	relayEvents(t, e, nil, d)
	after := len(data(f.owner.do("GET", base+"/webhook-deliveries?subscription_id="+subID, nil, false)))
	require.Equal(t, before, after)

	// The egress guard refuses private destinations outside the allowlist: dead-lettered at once.
	blocked := f.owner.do("POST", base+"/webhook-subscriptions", map[string]any{"workspace_id": f.workspaceID,
		"environment_id": f.envID, "name": "Internal", "url": "http://10.255.255.1/hook",
		"event_types": []string{"workflow.execution_finished"}}, true)
	require.Equal(t, 201, blocked.Status, blocked.Body)
	bping := f.owner.do("POST", base+"/webhook-subscriptions/"+blocked.Body["id"].(string)+"/ping", nil, true)
	require.Equal(t, 202, bping.Status)
	_, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	bd := f.owner.do("GET", base+"/webhook-deliveries/"+bping.Body["id"].(string), nil, false)
	require.Equal(t, "dead_lettered", bd.Body["status"], bd.Body)
	require.Equal(t, "destination_blocked", bd.Body["error"])
	require.EqualValues(t, 1, bd.Body["attempts"])

	// Failing continuously for 5 days disables the subscription; re-enabling clears the streak.
	_, err = e.pg.Owner.ExecContext(context.Background(),
		`UPDATE webhook_subscriptions SET failing_since = now() - interval '6 days' WHERE id = $1`, subID)
	require.NoError(t, err)
	rc.script(rot.Body["secret"].(string), 503)
	require.Equal(t, 202, f.owner.do("POST", base+"/webhook-subscriptions/"+subID+"/ping", nil, true).Status)
	_, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	sub = f.owner.do("GET", base+"/webhook-subscriptions/"+subID, nil, false)
	require.Equal(t, false, sub.Body["enabled"], sub.Body)
	require.Equal(t, "failing_for_5_days", sub.Body["disabled_reason"])
	require.Equal(t, 409, f.owner.do("POST", base+"/webhook-subscriptions/"+subID+"/ping", nil, true).Status)
	makeDeliveriesDue(t, e)
	_, err = d.DeliverDue(context.Background())
	require.NoError(t, err)
	pending := f.owner.do("GET", base+"/webhook-deliveries?subscription_id="+subID+"&status=pending", nil, false)
	require.Empty(t, data(pending), "deliveries to a disabled subscription are dead-lettered, not retried")
	en := f.owner.do("PATCH", base+"/webhook-subscriptions/"+subID, map[string]any{"enabled": true}, true)
	require.Equal(t, 200, en.Status, en.Body)
	require.Equal(t, "", en.Body["disabled_reason"])
	require.Nil(t, en.Body["failing_since"])

	// Tenant isolation.
	outsider := e.client()
	require.Equal(t, 201, outsider.do("POST", base+"/organizations", signupBody("wh-out2", "owner@wh-out2.test"), false).Status)
	require.Equal(t, 404, outsider.do("GET", base+"/webhook-subscriptions/"+subID, nil, false).Status)
	require.Equal(t, 404, outsider.do("GET", base+"/webhook-deliveries/"+deliveryID, nil, false).Status)
	require.Equal(t, 404, outsider.do("POST", base+"/webhook-deliveries/"+deadID+"/replay", nil, true).Status)
}
