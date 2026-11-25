// Package dto contains transport-agnostic use-case inputs for the data platform.
package dto

import "github.com/google/uuid"

type CreateDataSourceInput struct {
	WorkspaceID   uuid.UUID
	EnvironmentID uuid.UUID
	Name          string
	Slug          string
	Kind          string
	Config        map[string]any
	SecretID      *uuid.UUID
}

type UpdateDataSourceInput struct {
	Name     *string
	Slug     *string
	Config   *map[string]any
	SecretID *uuid.UUID
	Status   *string
}

type CreatePipelineInput struct {
	WorkspaceID     uuid.UUID
	DataSourceID    uuid.UUID
	Name            string
	Slug            string
	TargetTable     string
	Mode            string
	SourceObject    string
	WatermarkColumn string
	WatermarkType   string
	ScheduleCron    string
	Config          map[string]any
	Enabled         *bool
}

type UpdatePipelineInput struct {
	Name            *string
	TargetTable     *string
	Mode            *string
	SourceObject    *string
	WatermarkColumn *string
	WatermarkType   *string
	ScheduleCron    *string
	Config          *map[string]any
	Enabled         *bool
}

type CreateDatasetInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Slug        string
	Description string
	Owner       string
	SQL         string
}

type UpdateDatasetInput struct {
	Name        *string
	Slug        *string
	Description *string
	Owner       *string
	SQL         *string
}

type ColumnPolicyInput struct {
	ColumnName     string
	Classification string
	Mask           string
}

type RowPolicyInput struct {
	RoleKey     string
	RowFilter   string
	ColumnMasks map[string]string
}
