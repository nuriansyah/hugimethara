//go:build integration

package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

func TestCredentialValuesAreWriteOnlyAndTenantScoped(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations",
		signupBody("vault-co", "owner@vault.test"), false).Status)

	const plaintext = "top-secret-credential-value"
	created := owner.do("POST", base+"/credentials", map[string]any{
		"name": "slack-token", "kind": "bearer", "value": plaintext,
	}, true)
	require.Equal(t, 201, created.Status, created.Body)
	require.Equal(t, "slack-token", created.Body["name"])
	require.Equal(t, true, created.Body["configured"])
	require.Equal(t, "alue", created.Body["hint"])
	require.NotContains(t, created.Body, "value")
	id := created.Body["id"].(string)
	me := owner.do("GET", base+"/me", nil, false).Body
	ownerPrincipal := principal.Principal{
		ID:   uuid.MustParse(me["user"].(map[string]any)["id"].(string)),
		Type: principal.User, OrgID: uuid.MustParse(me["organization"].(map[string]any)["id"].(string)),
	}
	resolved, err := e.secrets.Resolve(principal.With(context.Background(), ownerPrincipal), uuid.MustParse(id))
	require.NoError(t, err)
	require.Equal(t, plaintext, string(resolved))
	for i := range resolved {
		resolved[i] = 0
	}

	listed := owner.do("GET", base+"/credentials", nil, false)
	require.Equal(t, 200, listed.Status, listed.Body)
	require.NotContains(t, listed.Body["data"].([]any)[0].(map[string]any), "value")
	got := owner.do("GET", base+"/credentials/"+id, nil, false)
	require.Equal(t, 200, got.Status, got.Body)
	require.NotContains(t, got.Body, "value")

	orgID := ownerPrincipal.OrgID
	var ciphertext, wrapped []byte
	require.NoError(t, e.pg.Owner.Get(&ciphertext, "SELECT ciphertext FROM secrets WHERE id = $1", id))
	require.NoError(t, e.pg.Owner.Get(&wrapped, "SELECT wrapped_key FROM data_keys WHERE org_id = $1", orgID))
	require.NotContains(t, string(ciphertext), plaintext)
	require.NotContains(t, string(wrapped), plaintext)

	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations",
		signupBody("other-vault", "owner@other-vault.test"), false).Status)
	require.Equal(t, 404, other.do("GET", base+"/credentials/"+id, nil, false).Status)

	rotated := "rotated-credential-value"
	updated := owner.do("PATCH", base+"/credentials/"+id, map[string]any{"value": rotated}, true)
	require.Equal(t, 200, updated.Status, updated.Body)
	require.Equal(t, int64(2), int64(updated.Body["version"].(float64)))
	require.NotContains(t, updated.Body, "value")
	require.Equal(t, 204, owner.do("DELETE", base+"/credentials/"+id, nil, true).Status)
	require.Equal(t, 404, owner.do("GET", base+"/credentials/"+id, nil, false).Status)

	require.Equal(t, 0, func() int {
		var count int
		err := e.pg.Owner.Get(&count, "SELECT count(*) FROM secrets WHERE id = $1", id)
		require.NoError(t, err)
		return count
	}())
}

func TestCredentialRequiresPermissionAndValidValue(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations",
		signupBody("vault-rbac", "owner@vault-rbac.test"), false).Status)
	member, _ := addMember(t, e, "vault-rbac", "member@vault-rbac.test")
	require.Equal(t, 403, member.do("GET", base+"/credentials", nil, false).Status)
	memberMe := member.do("GET", base+"/me", nil, false).Body
	_, resolveErr := e.secrets.Resolve(principal.With(context.Background(), principal.Principal{
		ID:   uuid.MustParse(memberMe["user"].(map[string]any)["id"].(string)),
		Type: principal.User, OrgID: uuid.MustParse(memberMe["organization"].(map[string]any)["id"].(string)),
	}), uuid.New())
	require.Error(t, resolveErr)
	require.Equal(t, 422, owner.do("POST", base+"/credentials",
		map[string]any{"name": "blank", "kind": "token", "value": " "}, true).Status)
}
