//go:build integration

package services_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/cache"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/testsupport"
)

// Two API instances share Redis. A change committed through one must be
// visible to the other on its next evaluation, not after a cache TTL.
func TestVersionStampPropagatesAcrossInstances(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	ctx := context.Background()
	rdb, err := cache.Open(ctx, testsupport.RedisURL(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	org, user := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	pg.CreateOrg(t, org)
	_, err = pg.Owner.ExecContext(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'U')`, user, user.String()+"@x.test")
	require.NoError(t, err)
	_, err = pg.Owner.ExecContext(ctx, `INSERT INTO org_memberships (org_id, user_id) VALUES ($1, $2)`, org, user)
	require.NoError(t, err)
	require.NoError(t, db.InTenantTx(ctx, pg.App, org, func(tx *sqlx.Tx) error {
		return services.SeedOrganization(ctx, tx, org, user) // user becomes owner + member
	}))

	a := services.NewRBAC(pg.App, rdb, log)
	b := services.NewRBAC(pg.App, rdb, log)
	p := principal.Principal{ID: user, OrgID: org, Type: principal.User}
	in := dto.AuthorizationInput{Principal: p, Action: "bind", Resource: model.Resource{Type: "role", OrgID: org}}

	d, err := a.Evaluate(ctx, in)
	require.NoError(t, err)
	require.True(t, d.Allowed)

	// Instance B removes the owner binding (as the service does: bump + invalidate).
	require.NoError(t, db.InTenantTx(ctx, pg.App, org, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM role_bindings WHERE user_id = $1 AND role_id =
			(SELECT id FROM roles WHERE key = 'org_owner')`, user); err != nil {
			return err
		}
		return services.BumpVersion(ctx, tx, org)
	}))
	b.Invalidate(ctx, org)

	d, err = a.Evaluate(ctx, in)
	require.NoError(t, err)
	require.False(t, d.Allowed, "instance A must observe the new version immediately")
	require.Greater(t, d.PolicyVersion, int64(0))

	// Tenant guard holds regardless of grants.
	other := in
	other.Resource.OrgID = uuid.New()
	d, err = a.Evaluate(ctx, other)
	require.NoError(t, err)
	require.Equal(t, model.ReasonTenantMismatch, d.Reason)
}
