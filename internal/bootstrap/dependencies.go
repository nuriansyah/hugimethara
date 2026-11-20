package bootstrap

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement"
	"github.com/nuriansyah/hugimethara/internal/modules/organization"
)

func usageCounters() map[string]entitlement.UsageFunc {
	out := map[string]entitlement.UsageFunc{}
	for k, fn := range organization.UsageCounters() {
		out[k] = fn
	}
	return out
}

func auditAdapter(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action, resourceType, resourceID string, details map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{OrgID: org, Action: action, ResourceType: resourceType,
		ResourceID: resourceID, Details: details})
}
