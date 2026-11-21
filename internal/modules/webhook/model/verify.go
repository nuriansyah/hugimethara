package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Scheme is an inbound signature verification strategy.
type Scheme string

const (
	// SchemeStandardWebhooks is the Standard Webhooks spec (webhook-id,
	// webhook-timestamp, webhook-signature; secret "whsec_<base64>"). Default.
	SchemeStandardWebhooks Scheme = "standard_webhooks"
	// SchemeHMAC is a configurable HMAC-SHA256 over the raw body (optionally
	// "<timestamp>.<body>") in a header of the sender's choosing.
	SchemeHMAC Scheme = "hmac_sha256"
	// SchemeStripe is Stripe's Stripe-Signature (t=…,v1=…) scheme.
	SchemeStripe Scheme = "stripe"
	// SchemeGitHub is GitHub's X-Hub-Signature-256 scheme (no timestamp:
	// replay protection relies on X-GitHub-Delivery dedupe).
	SchemeGitHub Scheme = "github"
	// SchemeSlack is Slack's v0 request signing (X-Slack-Signature).
	SchemeSlack Scheme = "slack"
	// SchemeN8N, SchemeMake and SchemeZapier are inbound status presets for
	// external workflow tools (S13). None of them signs outgoing requests
	// natively, but each can send a static header, so they authenticate with
	// a platform-generated token in TokenHeader over TLS. Status callbacks
	// follow one payload convention: {"run_id", "status", "event_id"?, …}.
	SchemeN8N    Scheme = "n8n"
	SchemeMake   Scheme = "make"
	SchemeZapier Scheme = "zapier"
)

// TokenHeader carries the shared token of the external-tool presets.
const TokenHeader = "X-Hugi-Token" //nolint:gosec // a header name, not a credential

// EventIDHeader optionally carries the sender's event id for the
// external-tool presets (dedupe); the body's event_id, or run_id + status,
// is used otherwise.
const EventIDHeader = "X-Hugi-Event-Id"

// Schemes lists every supported scheme.
var Schemes = []Scheme{SchemeStandardWebhooks, SchemeHMAC, SchemeStripe, SchemeGitHub, SchemeSlack,
	SchemeN8N, SchemeMake, SchemeZapier}

// Valid reports whether s is a supported scheme.
func (s Scheme) Valid() bool {
	for _, k := range Schemes {
		if k == s {
			return true
		}
	}
	return false
}

// GeneratesSecret reports whether the platform generates the signing secret
// (the sender is configured with it) rather than receiving it from the
// provider (Stripe, GitHub and Slack issue their own).
func (s Scheme) GeneratesSecret() bool {
	return s == SchemeStandardWebhooks || s == SchemeHMAC || s.IsToken()
}

// IsToken reports whether s is an external-tool preset authenticated by a
// shared token header rather than a signature.
func (s Scheme) IsToken() bool { return s == SchemeN8N || s == SchemeMake || s == SchemeZapier }

// HMACConfig configures SchemeHMAC. Every header name is optional except the
// signature header, which defaults to X-Signature.
type HMACConfig struct {
	SignatureHeader string `json:"signature_header,omitempty"`
	// Encoding of the digest: hex (default) or base64.
	Encoding string `json:"encoding,omitempty"`
	// Prefix stripped from the header value before comparing, e.g. "sha256=".
	Prefix string `json:"prefix,omitempty"`
	// TimestampHeader, when set, makes the signed content "<timestamp>.<body>"
	// and enforces the tolerance window on that Unix timestamp.
	TimestampHeader string `json:"timestamp_header,omitempty"`
	// IDHeader, when set, carries the sender's event id used for dedupe.
	IDHeader string `json:"id_header,omitempty"`
}

// Normalize fills defaults and canonicalizes header names.
func (c HMACConfig) Normalize() HMACConfig {
	if strings.TrimSpace(c.SignatureHeader) == "" {
		c.SignatureHeader = "X-Signature"
	}
	c.SignatureHeader = http.CanonicalHeaderKey(strings.TrimSpace(c.SignatureHeader))
	if c.TimestampHeader != "" {
		c.TimestampHeader = http.CanonicalHeaderKey(strings.TrimSpace(c.TimestampHeader))
	}
	if c.IDHeader != "" {
		c.IDHeader = http.CanonicalHeaderKey(strings.TrimSpace(c.IDHeader))
	}
	if c.Encoding == "" {
		c.Encoding = "hex"
	}
	return c
}

// Validate checks the configuration.
func (c HMACConfig) Validate() error {
	n := c.Normalize()
	if n.Encoding != "hex" && n.Encoding != "base64" {
		return invalidField("verifier_config.encoding", "invalid_encoding", "encoding must be hex or base64.")
	}
	for field, h := range map[string]string{"signature_header": n.SignatureHeader,
		"timestamp_header": n.TimestampHeader, "id_header": n.IDHeader} {
		if h != "" && !headerNameRe.MatchString(h) {
			return invalidField("verifier_config."+field, "invalid_header", "Not a valid HTTP header name.")
		}
	}
	if len(n.Prefix) > 32 {
		return invalidField("verifier_config.prefix", "too_long", "prefix must be at most 32 characters.")
	}
	return nil
}

// Rejection reasons. They are logged, counted and returned to the sender as a
// problem code; the payload of a rejected request is never stored.
const (
	ReasonMissingSignature   = "signature_missing"
	ReasonMalformedSignature = "signature_malformed"
	ReasonInvalidSignature   = "signature_invalid"
	ReasonTimestamp          = "timestamp_out_of_tolerance"
)

// Rejection is a failed verification.
type Rejection struct{ Reason string }

func (r *Rejection) Error() string { return "webhook rejected: " + r.Reason }

func reject(reason string) error { return &Rejection{Reason: reason} }

// VerifyRequest is an inbound request to verify.
type VerifyRequest struct {
	Header    http.Header
	Body      []byte
	Now       time.Time
	Tolerance time.Duration
}

// Verified describes an authentic request.
type Verified struct {
	// EventID is the sender's id for this event, used for dedupe ("" if none).
	EventID string
	// EventType is the sender's event type when it can be determined.
	EventType string
}

// Verify authenticates req with secret under scheme. Comparisons are
// constant-time; timestamped schemes reject requests outside the tolerance.
func Verify(scheme Scheme, cfg HMACConfig, secret []byte, req VerifyRequest) (Verified, error) {
	var body map[string]any
	_ = json.Unmarshal(req.Body, &body)
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return s }
	switch scheme {
	case SchemeStandardWebhooks:
		id, ts, sigs := req.Header.Get("Webhook-Id"), req.Header.Get("Webhook-Timestamp"), req.Header.Get("Webhook-Signature")
		if id == "" || ts == "" || sigs == "" {
			return Verified{}, reject(ReasonMissingSignature)
		}
		if err := checkTimestamp(ts, req.Now, req.Tolerance); err != nil {
			return Verified{}, err
		}
		key, err := StandardKey(secret)
		if err != nil {
			return Verified{}, reject(ReasonInvalidSignature)
		}
		want := mac(key, []byte(id+"."+ts+"."), req.Body)
		for _, part := range strings.Fields(sigs) {
			v, sig, ok := strings.Cut(part, ",")
			if !ok || v != "v1" {
				continue
			}
			got, err := base64.StdEncoding.DecodeString(sig)
			if err == nil && hmac.Equal(got, want) {
				return Verified{EventID: id, EventType: str(body, "type")}, nil
			}
		}
		return Verified{}, reject(ReasonInvalidSignature)

	case SchemeStripe:
		h := req.Header.Get("Stripe-Signature")
		if h == "" {
			return Verified{}, reject(ReasonMissingSignature)
		}
		var ts string
		var sigs []string
		for _, part := range strings.Split(h, ",") {
			k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
			switch k {
			case "t":
				ts = v
			case "v1":
				sigs = append(sigs, v)
			}
		}
		if ts == "" || len(sigs) == 0 {
			return Verified{}, reject(ReasonMalformedSignature)
		}
		if err := checkTimestamp(ts, req.Now, req.Tolerance); err != nil {
			return Verified{}, err
		}
		want := mac(secret, []byte(ts+"."), req.Body)
		if !anyHexEqual(sigs, want) {
			return Verified{}, reject(ReasonInvalidSignature)
		}
		return Verified{EventID: str(body, "id"), EventType: str(body, "type")}, nil

	case SchemeGitHub:
		h := req.Header.Get("X-Hub-Signature-256")
		if h == "" {
			return Verified{}, reject(ReasonMissingSignature)
		}
		sig, ok := strings.CutPrefix(h, "sha256=")
		if !ok {
			return Verified{}, reject(ReasonMalformedSignature)
		}
		if !anyHexEqual([]string{sig}, mac(secret, nil, req.Body)) {
			return Verified{}, reject(ReasonInvalidSignature)
		}
		typ := req.Header.Get("X-GitHub-Event")
		if a := str(body, "action"); typ != "" && a != "" {
			typ += "." + a
		}
		return Verified{EventID: req.Header.Get("X-GitHub-Delivery"), EventType: typ}, nil

	case SchemeSlack:
		sig, ts := req.Header.Get("X-Slack-Signature"), req.Header.Get("X-Slack-Request-Timestamp")
		if sig == "" || ts == "" {
			return Verified{}, reject(ReasonMissingSignature)
		}
		hexSig, ok := strings.CutPrefix(sig, "v0=")
		if !ok {
			return Verified{}, reject(ReasonMalformedSignature)
		}
		if err := checkTimestamp(ts, req.Now, req.Tolerance); err != nil {
			return Verified{}, err
		}
		if !anyHexEqual([]string{hexSig}, mac(secret, []byte("v0:"+ts+":"), req.Body)) {
			return Verified{}, reject(ReasonInvalidSignature)
		}
		typ := str(body, "type")
		if ev, ok := body["event"].(map[string]any); ok && str(ev, "type") != "" {
			typ = str(ev, "type")
		}
		return Verified{EventID: str(body, "event_id"), EventType: typ}, nil

	case SchemeN8N, SchemeMake, SchemeZapier:
		token := strings.TrimSpace(req.Header.Get(TokenHeader))
		if token == "" {
			return Verified{}, reject(ReasonMissingSignature)
		}
		// Compare digests so the comparison time does not depend on the length.
		got, want := sha256.Sum256([]byte(token)), sha256.Sum256(secret)
		if !hmac.Equal(got[:], want[:]) {
			return Verified{}, reject(ReasonInvalidSignature)
		}
		return toolStatus(scheme, req.Header, body), nil

	case SchemeHMAC:
		c := cfg.Normalize()
		h := strings.TrimSpace(req.Header.Get(c.SignatureHeader))
		if h == "" {
			return Verified{}, reject(ReasonMissingSignature)
		}
		if c.Prefix != "" {
			var ok bool
			if h, ok = strings.CutPrefix(h, c.Prefix); !ok {
				return Verified{}, reject(ReasonMalformedSignature)
			}
		}
		var prefix []byte
		if c.TimestampHeader != "" {
			ts := req.Header.Get(c.TimestampHeader)
			if ts == "" {
				return Verified{}, reject(ReasonMissingSignature)
			}
			if err := checkTimestamp(ts, req.Now, req.Tolerance); err != nil {
				return Verified{}, err
			}
			prefix = []byte(ts + ".")
		}
		want := mac(secret, prefix, req.Body)
		var got []byte
		var err error
		if c.Encoding == "base64" {
			got, err = base64.StdEncoding.DecodeString(h)
		} else {
			got, err = hex.DecodeString(strings.ToLower(h))
		}
		if err != nil {
			return Verified{}, reject(ReasonMalformedSignature)
		}
		if !hmac.Equal(got, want) {
			return Verified{}, reject(ReasonInvalidSignature)
		}
		v := Verified{EventType: str(body, "type")}
		if v.EventType == "" {
			v.EventType = str(body, "event")
		}
		if c.IDHeader != "" {
			v.EventID = req.Header.Get(c.IDHeader)
		}
		return v, nil
	}
	return Verified{}, reject(ReasonInvalidSignature)
}

// toolStatus derives the event id and type of an external-tool status
// callback: the id from X-Hugi-Event-Id, the body's event_id, or
// "<run_id>:<status>"; the type from the body's event, or "<tool>.<status>".
func toolStatus(scheme Scheme, h http.Header, body map[string]any) Verified {
	scalar := func(k string) string {
		switch v := body[k].(type) {
		case string:
			return strings.TrimSpace(v)
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return ""
	}
	v := Verified{EventID: strings.TrimSpace(h.Get(EventIDHeader)), EventType: scalar("event")}
	status, run := scalar("status"), scalar("run_id")
	if v.EventID == "" {
		v.EventID = scalar("event_id")
	}
	if v.EventID == "" && run != "" && status != "" {
		v.EventID = run + ":" + status
	}
	if v.EventType == "" && status != "" {
		v.EventType = string(scheme) + "." + status
	}
	if len(v.EventID) > 200 {
		v.EventID = v.EventID[:200]
	}
	if len(v.EventType) > 200 {
		v.EventType = v.EventType[:200]
	}
	return v
}

func mac(key, prefix, body []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(prefix)
	m.Write(body)
	return m.Sum(nil)
}

func anyHexEqual(candidates []string, want []byte) bool {
	ok := false
	for _, c := range candidates {
		got, err := hex.DecodeString(strings.ToLower(strings.TrimSpace(c)))
		if err == nil && hmac.Equal(got, want) {
			ok = true // keep looping: the comparison count does not depend on the match position
		}
	}
	return ok
}

func checkTimestamp(ts string, now time.Time, tolerance time.Duration) error {
	sec, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64)
	if err != nil {
		return reject(ReasonMalformedSignature)
	}
	if tolerance <= 0 {
		tolerance = DefaultTolerance
	}
	d := now.Sub(time.Unix(sec, 0))
	if d > tolerance || d < -tolerance {
		return reject(ReasonTimestamp)
	}
	return nil
}
