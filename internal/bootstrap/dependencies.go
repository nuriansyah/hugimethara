package bootstrap

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	orgsvc "github.com/nuriansyah/hugimethara/internal/modules/organization/services"
)

func usageCounters() map[string]entsvc.UsageFunc {
	out := map[string]entsvc.UsageFunc{}
	for k, fn := range orgsvc.UsageCounters() {
		out[k] = fn
	}
	return out
}

func auditAdapter(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action, resourceType, resourceID string, details map[string]any) error {
	return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: org, Action: action, ResourceType: resourceType,
		ResourceID: resourceID, Details: details})
}
