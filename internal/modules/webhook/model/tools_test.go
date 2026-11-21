package model

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalToolPresetsVerifyTheTokenHeader(t *testing.T) {
	for _, scheme := range []Scheme{SchemeN8N, SchemeMake, SchemeZapier} {
		t.Run(string(scheme), func(t *testing.T) {
			require.True(t, scheme.Valid())
			require.True(t, scheme.IsToken())
			require.True(t, scheme.GeneratesSecret())
			secret, err := GenerateSecret(scheme)
			require.NoError(t, err)
			require.Len(t, secret, 64)
			require.NoError(t, ValidateSecret(scheme, secret))

			body := []byte(`{"run_id":"r-42","status":"success","workflow":"lead-sync"}`)
			verify := func(h http.Header) (Verified, error) {
				return Verify(scheme, HMACConfig{}, []byte(secret), VerifyRequest{Header: h, Body: body, Now: now})
			}

			_, err = verify(http.Header{})
			require.Equal(t, ReasonMissingSignature, reason(t, err))
			_, err = verify(http.Header{"X-Hugi-Token": {secret[:63] + "x"}})
			require.Equal(t, ReasonInvalidSignature, reason(t, err))
			_, err = verify(http.Header{"X-Hugi-Token": {secret + "extra"}})
			require.Equal(t, ReasonInvalidSignature, reason(t, err))

			v, err := verify(http.Header{"X-Hugi-Token": {secret}})
			require.NoError(t, err)
			require.Equal(t, "r-42:success", v.EventID)
			require.Equal(t, string(scheme)+".success", v.EventType)

			v, err = verify(http.Header{"X-Hugi-Token": {secret}, "X-Hugi-Event-Id": {"evt-1"}})
			require.NoError(t, err)
			require.Equal(t, "evt-1", v.EventID)
		})
	}
}

func TestExternalToolStatusFallbacks(t *testing.T) {
	v := toolStatus(SchemeMake, http.Header{}, map[string]any{"event_id": "e1", "event": "scenario.finished", "status": "error"})
	require.Equal(t, Verified{EventID: "e1", EventType: "scenario.finished"}, v)
	v = toolStatus(SchemeZapier, http.Header{}, map[string]any{"run_id": float64(7), "status": "running"})
	require.Equal(t, Verified{EventID: "7:running", EventType: "zapier.running"}, v)
	// Without a run id there is nothing stable to deduplicate on.
	v = toolStatus(SchemeN8N, http.Header{}, map[string]any{"status": "success"})
	require.Equal(t, Verified{EventType: "n8n.success"}, v)
}
