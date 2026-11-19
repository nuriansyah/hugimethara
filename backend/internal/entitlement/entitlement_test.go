package entitlement

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func set(fs ...Feature) Set {
	s := Set{PlanKey: "free", Features: map[string]Feature{}}
	for _, f := range fs {
		s.Features[f.Key] = f
	}
	return s
}

func TestFlagsAndLimits(t *testing.T) {
	s := set(
		Feature{Key: Departments, Name: "Departments", Kind: "flag", Enabled: false},
		Feature{Key: WorkspacesMax, Name: "Workspaces", Kind: "limit", Enabled: true, Limit: sql.NullInt64{Int64: 3, Valid: true}},
		Feature{Key: EnvironmentsMax, Name: "Environments", Kind: "limit", Enabled: true}, // unlimited
	)

	var ne *NotEntitled
	require.True(t, errors.As(s.Require(Departments), &ne))
	require.Equal(t, 402, ne.Problem().Status)
	require.Equal(t, "feature_not_in_plan", ne.Problem().Code)
	require.True(t, errors.As(s.Require("unknown.feature"), &ne), "unknown features are denied")

	require.NoError(t, s.CheckLimit(WorkspacesMax, 2, 1))
	var lr *LimitReached
	require.True(t, errors.As(s.CheckLimit(WorkspacesMax, 3, 1), &lr))
	p := lr.Problem()
	require.Equal(t, "plan_limit_reached", p.Code)
	require.Equal(t, int64(3), p.Extensions["limit"])
	require.Contains(t, p.Detail, "3 workspaces")

	require.NoError(t, s.CheckLimit(EnvironmentsMax, 1_000_000, 1), "NULL limit = unlimited")
	require.Error(t, s.CheckLimit("not.in.plan", 0, 1), "absent limit feature = 0")
}
