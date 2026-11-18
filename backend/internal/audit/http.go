package audit

import (
	"context"


	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// HTTP implements the audit slice of apigen.StrictServerInterface.
type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func (h *HTTP) ListAuditEvents(ctx context.Context, req apigen.ListAuditEventsRequestObject) (apigen.ListAuditEventsResponseObject, error) {
	prm := req.Params
	page := httpx.Page{Limit: httpx.DefaultPageLimit}
	if prm.Limit != nil {
		if *prm.Limit < 1 || *prm.Limit > httpx.MaxPageLimit {
			return nil, httpx.Validation(httpx.FieldError{Field: "limit", Code: "out_of_range", Message: "limit must be between 1 and 200."})
		}
		page.Limit = *prm.Limit
	}
	if prm.Cursor != nil {
		page.Cursor = *prm.Cursor
	}
	var f Filter
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
		return nil, httpx.Validation(httpx.FieldError{Field: "until", Code: "before_since", Message: "until must be after since."})
	}

	entries, next, err := h.svc.List(ctx, f, page)
	if err != nil {
		return nil, err
	}
	out := apigen.ListAuditEvents200JSONResponse{Data: make([]apigen.AuditEvent, len(entries))}
	if next != "" {
		out.NextCursor = &next
	}
	for i, e := range entries {
		ev := apigen.AuditEvent{
			Id: e.ID, OccurredAt: e.OccurredAt, ActorType: apigen.AuditEventActorType(e.ActorType),
			Action: e.Action, ResourceType: e.ResourceType, Outcome: apigen.AuditEventOutcome(e.Outcome),
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

