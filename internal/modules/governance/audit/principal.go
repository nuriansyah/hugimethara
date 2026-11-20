package audit

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

func principalFrom(ctx context.Context) (uuid.UUID, bool) {
	p, ok := principal.From(ctx)
	return p.OrgID, ok
}
