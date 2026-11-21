package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

func hexMAC(key []byte, content string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(content))
	return hex.EncodeToString(m.Sum(nil))
}

func reason(t *testing.T, err error) string {
	t.Helper()
	var r *Rejection
	require.True(t, errors.As(err, &r), "expected a rejection, got %v", err)
	return r.Reason
}

func TestStandardWebhooksRoundTripAndTampering(t *testing.T) {
	secret, err := GenerateSecret(SchemeStandardWebhooks)
	require.NoError(t, err)
	body := []byte(`{"type":"invoice.paid","data":{"id":"inv_1"}}`)
	hdr, err := SignedHeaders([]byte(secret), "msg_1", now, body)
	require.NoError(t, err)
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	v, err := Verify(SchemeStandardWebhooks, HMACConfig{}, []byte(secret), VerifyRequest{Header: h, Body: body, Now: now})
	require.NoError(t, err)
	require.Equal(t, Verified{EventID: "msg_1", EventType: "invoice.paid"}, v)

	// Several signatures (key rotation): any valid v1 one passes.
	h.Set("Webhook-Signature", "v1,AAAA "+hdr["Webhook-Signature"])
	_, err = Verify(SchemeStandardWebhooks, HMACConfig{}, []byte(secret), VerifyRequest{Header: h, Body: body, Now: now})
	require.NoError(t, err)

	cases := map[string]func() (http.Header, []byte, time.Time){
		ReasonInvalidSignature: func() (http.Header, []byte, time.Time) { return h.Clone(), []byte(`{"type":"forged"}`), now },
		ReasonTimestamp:        func() (http.Header, []byte, time.Time) { return h.Clone(), body, now.Add(6 * time.Minute) },
		ReasonMissingSignature: func() (http.Header, []byte, time.Time) {
			c := h.Clone()
			c.Del("Webhook-Signature")
			return c, body, now
		},
	}
	for want, mk := range cases {
		hh, b, at := mk()
		_, err := Verify(SchemeStandardWebhooks, HMACConfig{}, []byte(secret), VerifyRequest{Header: hh, Body: b, Now: at})
		require.Equal(t, want, reason(t, err))
	}
	other, _ := GenerateSecret(SchemeStandardWebhooks)
	_, err = Verify(SchemeStandardWebhooks, HMACConfig{}, []byte(other), VerifyRequest{Header: h, Body: body, Now: now})
	require.Equal(t, ReasonInvalidSignature, reason(t, err))
}

func TestStripeVerifier(t *testing.T) {
	secret := []byte("whsec_test_stripe_secret_value")
	body := []byte(`{"id":"evt_1","type":"payment_intent.succeeded"}`)
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := hexMAC(secret, ts+"."+string(body))
	h := http.Header{"Stripe-Signature": {"t=" + ts + ",v1=deadbeef,v1=" + sig}}
	v, err := Verify(SchemeStripe, HMACConfig{}, secret, VerifyRequest{Header: h, Body: body, Now: now})
	require.NoError(t, err)
	require.Equal(t, Verified{EventID: "evt_1", EventType: "payment_intent.succeeded"}, v)

	_, err = Verify(SchemeStripe, HMACConfig{}, secret, VerifyRequest{Header: h, Body: body, Now: now.Add(-10 * time.Minute)})
	require.Equal(t, ReasonTimestamp, reason(t, err))
	_, err = Verify(SchemeStripe, HMACConfig{}, secret, VerifyRequest{Header: http.Header{"Stripe-Signature": {"v1=" + sig}},
		Body: body, Now: now})
	require.Equal(t, ReasonMalformedSignature, reason(t, err))
}

func TestGitHubAndSlackVerifiers(t *testing.T) {
	secret := []byte("github-shared-secret")
	body := []byte(`{"action":"opened","number":1}`)
	h := http.Header{"X-Hub-Signature-256": {"sha256=" + hexMAC(secret, string(body))},
		"X-Github-Event": {"pull_request"}, "X-Github-Delivery": {"d-1"}}
	v, err := Verify(SchemeGitHub, HMACConfig{}, secret, VerifyRequest{Header: h, Body: body, Now: now})
	require.NoError(t, err)
	require.Equal(t, Verified{EventID: "d-1", EventType: "pull_request.opened"}, v)
	h.Set("X-Hub-Signature-256", "sha1=abc")
	_, err = Verify(SchemeGitHub, HMACConfig{}, secret, VerifyRequest{Header: h, Body: body, Now: now})
	require.Equal(t, ReasonMalformedSignature, reason(t, err))

	slack := []byte("slack-signing-secret")
	sbody := []byte(`{"type":"event_callback","event_id":"Ev1","event":{"type":"message"}}`)
	ts := strconv.FormatInt(now.Unix(), 10)
	sh := http.Header{"X-Slack-Request-Timestamp": {ts}, "X-Slack-Signature": {"v0=" + hexMAC(slack, "v0:"+ts+":"+string(sbody))}}
	v, err = Verify(SchemeSlack, HMACConfig{}, slack, VerifyRequest{Header: sh, Body: sbody, Now: now})
	require.NoError(t, err)
	require.Equal(t, Verified{EventID: "Ev1", EventType: "message"}, v)
	_, err = Verify(SchemeSlack, HMACConfig{}, slack, VerifyRequest{Header: sh, Body: sbody, Now: now.Add(time.Hour)})
	require.Equal(t, ReasonTimestamp, reason(t, err))
}

func TestGenericHMACVerifier(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	body := []byte(`{"event":"order.created"}`)
	ts := strconv.FormatInt(now.Unix(), 10)
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(ts + "." + string(body)))
	cfg := HMACConfig{SignatureHeader: "x-acme-signature", Encoding: "base64", Prefix: "sha256=",
		TimestampHeader: "X-Acme-Timestamp", IDHeader: "X-Acme-Id"}
	require.NoError(t, cfg.Validate())
	h := http.Header{"X-Acme-Signature": {"sha256=" + base64.StdEncoding.EncodeToString(m.Sum(nil))},
		"X-Acme-Timestamp": {ts}, "X-Acme-Id": {"a-1"}}
	v, err := Verify(SchemeHMAC, cfg, secret, VerifyRequest{Header: h, Body: body, Now: now})
	require.NoError(t, err)
	require.Equal(t, Verified{EventID: "a-1", EventType: "order.created"}, v)

	// Defaults: X-Signature, hex, no timestamp.
	plain := http.Header{"X-Signature": {hexMAC(secret, string(body))}}
	_, err = Verify(SchemeHMAC, HMACConfig{}, secret, VerifyRequest{Header: plain, Body: body, Now: now})
	require.NoError(t, err)
	_, err = Verify(SchemeHMAC, HMACConfig{}, secret, VerifyRequest{Header: plain, Body: []byte(`{}`), Now: now})
	require.Equal(t, ReasonInvalidSignature, reason(t, err))
	require.Error(t, HMACConfig{Encoding: "rot13"}.Validate())
	require.Error(t, HMACConfig{SignatureHeader: "bad header"}.Validate())
}

func TestSecretsAndRetrySchedule(t *testing.T) {
	s, err := GenerateSecret(SchemeStandardWebhooks)
	require.NoError(t, err)
	require.NoError(t, ValidateSecret(SchemeStandardWebhooks, s))
	require.Error(t, ValidateSecret(SchemeStandardWebhooks, "not-a-standard-secret"))
	require.Error(t, ValidateSecret(SchemeStripe, "short"))
	h, err := GenerateSecret(SchemeHMAC)
	require.NoError(t, err)
	require.Len(t, h, 64)
	require.Equal(t, "…"+s[len(s)-4:], SecretHint(s))

	require.Equal(t, 8, MaxAttempts)
	total := time.Duration(0)
	for a := 1; ; a++ {
		d, ok := NextRetry(a)
		if !ok {
			require.Equal(t, 8, a, "the eighth failed attempt dead-letters")
			break
		}
		total += d
	}
	require.InDelta(t, 21*time.Hour, total, float64(time.Hour))

	pid, err := NewPublicID()
	require.NoError(t, err)
	require.Regexp(t, `^[a-f0-9]{32}$`, pid)
	require.NoError(t, ValidateURL("https://example.com/hooks"))
	require.Error(t, ValidateURL("ftp://example.com"))
	require.Error(t, ValidateURL("https://user:pw@example.com"))
	require.True(t, KnownEventType("workflow.execution_finished"))
	require.False(t, KnownEventType("organization.updated"))
}
