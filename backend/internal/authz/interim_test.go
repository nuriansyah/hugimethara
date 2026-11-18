package authz

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

func TestInterimAuthorizer(t *testing.T) {
	az := InterimAuthorizer{}
	orgA, orgB := uuid.New(), uuid.New()
	owner := principal.Principal{ID: uuid.New(), OrgID: orgA, IsOwner: true}
	member := principal.Principal{ID: uuid.New(), OrgID: orgA}

	cases := []struct {
		name   string
		p      principal.Principal
		action string
		res    Resource
		allow  bool
		reason string
	}{
		{"owner updates own org", owner, "update", Resource{Type: "organization", OrgID: orgA}, true, "interim_owner"},
		{"owner cannot cross tenant", owner, "read", Resource{Type: "organization", OrgID: orgB}, false, ReasonTenantMismatch},
		{"member reads org", member, "read", Resource{Type: "organization", OrgID: orgA}, true, "interim_member"},
		{"member cannot update org", member, "update", Resource{Type: "organization", OrgID: orgA}, false, "interim_owner_required"},
		{"member cannot read audit", member, "read", Resource{Type: "audit_event", OrgID: orgA}, false, "interim_owner_required"},
		{"empty principal denied", principal.Principal{}, "read", Resource{Type: "organization", OrgID: orgA}, false, ReasonNoPrincipal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := az.Evaluate(context.Background(), AuthorizationInput{Principal: c.p, Action: c.action, Resource: c.res})
			require.NoError(t, err)
			require.Equal(t, c.allow, d.Allowed)
			require.Equal(t, c.reason, d.Reason)
		})
	}
}

func TestRequireMapsDecisionsToProblems(t *testing.T) {
	orgA := uuid.New()
	status := func(err error) int {
		var p *httpx.Problem
		require.True(t, errors.As(err, &p))
		return p.Status
	}

	_, err := Require(context.Background(), InterimAuthorizer{}, "read", Resource{Type: "organization", OrgID: orgA})
	require.Equal(t, http.StatusUnauthorized, status(err))

	ctx := principal.With(context.Background(), principal.Principal{ID: uuid.New(), OrgID: orgA})
	_, err = Require(ctx, InterimAuthorizer{}, "update", Resource{Type: "organization", OrgID: orgA})
	require.Equal(t, http.StatusForbidden, status(err))

	_, err = Require(ctx, InterimAuthorizer{}, "read", Resource{Type: "organization", OrgID: uuid.New()})
	require.Equal(t, http.StatusNotFound, status(err))

	_, err = Require(ctx, InterimAuthorizer{}, "read", Resource{Type: "organization", OrgID: orgA})
	require.NoError(t, err)
}
