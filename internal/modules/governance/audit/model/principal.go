package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

func PrincipalFrom(ctx context.Context) (uuid.UUID, bool) {
	p, ok := principal.From(ctx)
	return p.OrgID, ok
}
