package authz

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCovers(t *testing.T) {
	cases := []struct {
		pattern, perm string
		want          bool
	}{
		{"workspace:read", "workspace:read", true},
		{"workspace:read", "workspace:update", false},
		{"workspace:*", "workspace:update", true},
		{"workspace:*", "environment:update", false},
		{"*:read", "environment:read", true},
		{"*:read", "environment:update", false},
		{"*:*", "role:bind", true},
		{"*:*", "*:*", true},
		{"workspace:*", "workspace:*", true},
		{"workspace:read", "workspace:*", false}, // cannot grant a wildcard you do not hold
		{"*:read", "*:*", false},
		{"workspace:*", "*:*", false},
		{"bad", "workspace:read", false},
		{"workspace:read", "bad", false},
		{":", "workspace:read", false},
	}
	for _, c := range cases {
		require.Equal(t, c.want, Covers(c.pattern, c.perm), "%s covers %s", c.pattern, c.perm)
	}
}

func TestScopeContainsAndApplies(t *testing.T) {
	w1, w2 := uuid.New(), uuid.New()
	org := ScopeRef{Type: ScopeOrg}
	ws1 := ScopeRef{Type: ScopeWorkspace, WorkspaceID: w1}
	res := ScopeRef{Type: ScopeResource, ResourceType: "dataset", ResourceID: "finance.payments"}

	require.True(t, org.Contains(ws1))
	require.True(t, org.Contains(res))
	require.True(t, ws1.Contains(ws1))
	require.False(t, ws1.Contains(ScopeRef{Type: ScopeWorkspace, WorkspaceID: w2}))
	require.False(t, ws1.Contains(org))
	require.False(t, res.Contains(org))
	require.True(t, res.Contains(res))
	require.False(t, res.Contains(ScopeRef{Type: ScopeResource, ResourceType: "dataset", ResourceID: "other"}))

	require.True(t, ws1.appliesTo(Resource{Type: "workspace", WorkspaceID: &w1}))
	require.False(t, ws1.appliesTo(Resource{Type: "workspace", WorkspaceID: &w2}))
	require.False(t, ws1.appliesTo(Resource{Type: "workspace"}), "collection-level actions need org scope")
	require.True(t, res.appliesTo(Resource{Type: "dataset", ID: "finance.payments"}))
	require.False(t, res.appliesTo(Resource{Type: "dataset", ID: "other"}))
	require.False(t, res.appliesTo(Resource{Type: "workflow", ID: "finance.payments"}))
}

func TestMatchingAndCanGrant(t *testing.T) {
	w1 := uuid.New()
	grants := []Grant{
		{Permission: "organization:read", Scope: ScopeRef{Type: ScopeOrg}},
		{Permission: "workspace:*", Scope: ScopeRef{Type: ScopeWorkspace, WorkspaceID: w1}},
		{Permission: "dataset:read", Scope: ScopeRef{Type: ScopeResource, ResourceType: "dataset", ResourceID: "finance.payments"}},
	}
	require.Len(t, matching(grants, "manage_members", Resource{Type: "workspace", WorkspaceID: &w1}), 1)
	require.Empty(t, matching(grants, "create", Resource{Type: "workspace"}), "no org-scope workspace grant")
	require.Len(t, matching(grants, "read", Resource{Type: "dataset", ID: "finance.payments"}), 1)
	require.Empty(t, matching(grants, "export", Resource{Type: "dataset", ID: "finance.payments"}))

	ok, _ := CanGrant(grants, []string{"workspace:read", "workspace:update"}, ScopeRef{Type: ScopeWorkspace, WorkspaceID: w1})
	require.True(t, ok)
	ok, missing := CanGrant(grants, []string{"workspace:read", "role:bind"}, ScopeRef{Type: ScopeWorkspace, WorkspaceID: w1})
	require.False(t, ok)
	require.Equal(t, []string{"role:bind"}, missing)
	ok, _ = CanGrant(grants, []string{"workspace:read"}, ScopeRef{Type: ScopeOrg})
	require.False(t, ok, "workspace-scoped holder cannot grant at org scope")
	ok, _ = CanGrant([]Grant{{Permission: "*:*", Scope: ScopeRef{Type: ScopeOrg}}}, []string{"*:*"}, ScopeRef{Type: ScopeOrg})
	require.True(t, ok, "owners can create owners")
}
