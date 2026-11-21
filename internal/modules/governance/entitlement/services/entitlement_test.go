package services

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
)

func set(fs ...model.Feature) model.Set {
	s := model.Set{PlanKey: "free", Features: map[string]model.Feature{}}
	for _, f := range fs {
		s.Features[f.Key] = f
	}
	return s
}

func TestFlagsAndLimits(t *testing.T) {
	s := set(
		model.Feature{Key: model.Departments, Name: "Departments", Kind: "flag", Enabled: false},
		model.Feature{Key: model.WorkspacesMax, Name: "Workspaces", Kind: "limit", Enabled: true, Limit: sql.NullInt64{Int64: 3, Valid: true}},
		model.Feature{Key: model.EnvironmentsMax, Name: "Environments", Kind: "limit", Enabled: true}, // unlimited
	)

	var ne *model.NotEntitled
	require.True(t, errors.As(s.Require(model.Departments), &ne))
	require.Equal(t, 402, ne.Problem().Status)
	require.Equal(t, "feature_not_in_plan", ne.Problem().Code)
	require.True(t, errors.As(s.Require("unknown.feature"), &ne), "unknown features are denied")

	require.NoError(t, s.CheckLimit(model.WorkspacesMax, 2, 1))
	var lr *model.LimitReached
	require.True(t, errors.As(s.CheckLimit(model.WorkspacesMax, 3, 1), &lr))
	p := lr.Problem()
	require.Equal(t, "plan_limit_reached", p.Code)
	require.Equal(t, int64(3), p.Extensions["limit"])
	require.Contains(t, p.Detail, "3 workspaces")

	require.NoError(t, s.CheckLimit(model.EnvironmentsMax, 1_000_000, 1), "NULL limit = unlimited")
	require.Error(t, s.CheckLimit("not.in.plan", 0, 1), "absent limit feature = 0")
}
