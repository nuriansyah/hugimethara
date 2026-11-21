package audit

import (
	"context"

	auditdto "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/pagination"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the audit slice of dto.StrictServerInterface.
type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary The organization audit trail, newest first
// @ID listAuditEvents
// @Tags audit
// @Param action query string false "Exact action or prefix (e.g. `auth` matches `auth.*`)."
// @Param actor_id query string false "actor_id"
// @Param resource_type query string false "resource_type"
// @Param since query string false "since"
// @Param until query string false "until"
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} dto.AuditEventPage "One page of events."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/audit-events [get]
func (h *HTTP) ListAuditEvents(ctx context.Context, req dto.ListAuditEventsRequestObject) (dto.ListAuditEventsResponseObject, error) {
	prm := req.Params
	page := pagination.Page{Limit: pagination.DefaultPageLimit}
	if prm.Limit != nil {
		if *prm.Limit < 1 || *prm.Limit > pagination.MaxPageLimit {
			return nil, apperrors.Validation(apperrors.FieldError{Field: "limit", Code: "out_of_range", Message: "limit must be between 1 and 200."})
		}
		page.Limit = *prm.Limit
	}
	if prm.Cursor != nil {
		page.Cursor = *prm.Cursor
	}
	var f auditdto.Filter
	if prm.Action != nil {
		f.Action = *prm.Action
	}
	if prm.ActorId != nil {
		f.ActorID = *prm.ActorId
	}
	if prm.ResourceType != nil {
		f.ResourceType = *prm.ResourceType
	}
	if prm.Since != nil {
		f.Since = *prm.Since
	}
	if prm.Until != nil {
		f.Until = *prm.Until
	}
	if !f.Since.IsZero() && !f.Until.IsZero() && !f.Since.Before(f.Until) {
		return nil, apperrors.Validation(apperrors.FieldError{Field: "until", Code: "before_since", Message: "until must be after since."})
	}

	entries, next, err := h.svc.List(ctx, f, page)
	if err != nil {
		return nil, err
	}
	out := dto.ListAuditEvents200JSONResponse{Data: make([]dto.AuditEvent, len(entries))}
	if next != "" {
		out.NextCursor = &next
	}
	for i, e := range entries {
		ev := dto.AuditEvent{
			Id: e.ID, OccurredAt: e.OccurredAt, ActorType: dto.AuditEventActorType(e.ActorType),
			Action: e.Action, ResourceType: e.ResourceType, Outcome: dto.AuditEventOutcome(e.Outcome),
			Details: e.Details, ActorId: e.ActorID, WorkspaceId: e.WorkspaceID,
			ActorEmail: opt(e.ActorEmail), ResourceId: opt(e.ResourceID), RequestId: opt(e.RequestID), Ip: opt(e.IP),
		}
		if ev.Details == nil {
			ev.Details = map[string]any{}
		}
		out.Data[i] = ev
	}
	return out, nil
}

func opt(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
