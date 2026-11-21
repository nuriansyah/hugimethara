package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
)

type ServiceRow struct {
	ID            uuid.UUID `db:"id"`
	WorkspaceID   uuid.UUID `db:"workspace_id"`
	Name          string    `db:"name"`
	Owner         string    `db:"owner"`
	Description   string    `db:"description"`
	Protocol      string    `db:"protocol"`
	EndpointCount int64     `db:"endpoint_count"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type EnvironmentRow struct {
	EnvironmentID       uuid.UUID     `db:"environment_id"`
	BaseURL             string        `db:"base_url"`
	ConnectorInstanceID uuid.NullUUID `db:"connector_instance_id"`
}

type EndpointRow struct {
	ID              uuid.UUID       `db:"id"`
	ServiceID       uuid.UUID       `db:"service_id"`
	OperationID     string          `db:"operation_id"`
	Method          string          `db:"method"`
	Path            string          `db:"path"`
	Summary         string          `db:"summary"`
	Parameters      json.RawMessage `db:"parameters"`
	RequestSchema   []byte          `db:"request_schema"`
	ResponseSchemas json.RawMessage `db:"response_schemas"`
	Security        json.RawMessage `db:"security"`
	Deprecated      bool            `db:"deprecated"`
}

type DiffEndpointRow struct {
	OperationID     string          `db:"operation_id"`
	Method          string          `db:"method"`
	Path            string          `db:"path"`
	Summary         string          `db:"summary"`
	Parameters      json.RawMessage `db:"parameters"`
	RequestSchema   []byte          `db:"request_schema"`
	ResponseSchemas json.RawMessage `db:"response_schemas"`
	Security        json.RawMessage `db:"security"`
	Deprecated      bool            `db:"deprecated"`
}

type TryEndpointRow struct {
	ID              uuid.UUID       `db:"id"`
	ServiceID       uuid.UUID       `db:"service_id"`
	Method          string          `db:"method"`
	Path            string          `db:"path"`
	WorkspaceID     uuid.UUID       `db:"workspace_id"`
	BaseURL         string          `db:"base_url"`
	EgressAllowlist json.RawMessage `db:"egress_allowlist"`
}

func (r ServiceRow) Model() serviceregmodel.RegisteredService {
	return serviceregmodel.RegisteredService{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Owner: r.Owner,
		Description: r.Description, Protocol: r.Protocol, EndpointCount: r.EndpointCount,
		Environments: []serviceregmodel.Environment{}, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r EnvironmentRow) Model() serviceregmodel.Environment {
	out := serviceregmodel.Environment{EnvironmentID: r.EnvironmentID, BaseURL: r.BaseURL}
	if r.ConnectorInstanceID.Valid {
		id := r.ConnectorInstanceID.UUID
		out.ConnectorInstanceID = &id
	}
	return out
}

func (r EndpointRow) Model() (serviceregmodel.Endpoint, error) {
	out := serviceregmodel.Endpoint{ID: r.ID, ServiceID: r.ServiceID, OperationID: r.OperationID, Method: r.Method,
		Path: r.Path, Summary: r.Summary, Deprecated: r.Deprecated}
	if err := serviceregmodel.UnmarshalJSON(r.Parameters, &out.Parameters); err != nil {
		return serviceregmodel.Endpoint{}, err
	}
	if len(r.RequestSchema) > 0 && string(r.RequestSchema) != "null" {
		out.RequestSchema = map[string]any{}
		if err := serviceregmodel.UnmarshalJSON(r.RequestSchema, &out.RequestSchema); err != nil {
			return serviceregmodel.Endpoint{}, err
		}
	}
	if err := serviceregmodel.UnmarshalJSON(r.ResponseSchemas, &out.ResponseSchemas); err != nil {
		return serviceregmodel.Endpoint{}, err
	}
	if err := serviceregmodel.UnmarshalJSON(r.Security, &out.Security); err != nil {
		return serviceregmodel.Endpoint{}, err
	}
	return out, nil
}
