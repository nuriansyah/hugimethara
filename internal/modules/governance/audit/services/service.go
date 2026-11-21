package services

import (
	"context"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model/dto"
	"github.com/nuriansyah/hugimethara/internal/platform/pagination"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on AuditService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	// List returns one page of the caller's organization audit trail, newest first.
	List(ctx context.Context, f dto.Filter, page pagination.Page) ([]model.Entry, string, error)
}

var _ Service = (*AuditService)(nil)
