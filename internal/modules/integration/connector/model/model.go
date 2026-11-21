package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const Resource = "connector"

type Definition struct {
	ID       uuid.UUID  `db:"id"`
	OrgID    *uuid.UUID `db:"org_id"`
	Key      string     `db:"key"`
	Name     string     `db:"name"`
	Category string     `db:"category"`
	Tier     string     `db:"tier"`
	Icon     string     `db:"icon"`
}

type Version struct {
	ID           uuid.UUID       `db:"id"`
	DefinitionID uuid.UUID       `db:"definition_id"`
	Version      string          `db:"version"`
	ConfigSchema json.RawMessage `db:"config_schema"`
	AuthMethods  []byte          `db:"auth_methods"`
	Operations   []byte          `db:"operations"`
	DeprecatedAt *time.Time      `db:"deprecated_at"`
}

type VersionDetail struct {
	Version
	DefinitionKey string `db:"definition_key"`
}

type Instance struct {
	ID                   uuid.UUID       `db:"id"`
	WorkspaceID          uuid.UUID       `db:"workspace_id"`
	EnvironmentID        uuid.UUID       `db:"environment_id"`
	RefKey               string          `db:"ref_key"`
	ConnectorVersionID   uuid.UUID       `db:"connector_version_id"`
	Config               json.RawMessage `db:"config"`
	AuthMethod           string          `db:"auth_method"`
	SecretID             *uuid.UUID      `db:"secret_id"`
	RateLimit            int             `db:"rate_limit"`
	TimeoutMs            int             `db:"timeout_ms"`
	Status               string          `db:"status"`
	CreatedAt            time.Time       `db:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"`
	Version              string          `db:"version"`
	DefinitionID         uuid.UUID       `db:"definition_id"`
	DefinitionKey        string          `db:"definition_key"`
	CredentialConfigured bool            `db:"credential_configured"`
}

type InstanceDetail struct {
	Instance
	ConfigSchema json.RawMessage `db:"config_schema"`
	AuthMethods  []byte          `db:"auth_methods"`
}

type Health struct {
	CheckedAt time.Time `db:"checked_at"`
	OK        bool      `db:"ok"`
	LatencyMs int       `db:"latency_ms"`
	ErrorCode string    `db:"error_code"`
	Message   string    `db:"message"`
}

type TestResult struct {
	CheckedAt time.Time
	OK        bool
	LatencyMs int
	ErrorCode string
	Message   string
}
