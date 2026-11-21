package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	mappingmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model"
)

type MappingRow struct {
	ID           uuid.UUID       `db:"id"`
	WorkspaceID  uuid.UUID       `db:"workspace_id"`
	Name         string          `db:"name"`
	Draft        json.RawMessage `db:"draft"`
	InputSchema  []byte          `db:"input_schema"`
	OutputSchema []byte          `db:"output_schema"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

type VersionRow struct {
	ID          uuid.UUID       `db:"id"`
	MappingID   uuid.UUID       `db:"mapping_id"`
	Version     int             `db:"version"`
	Definition  json.RawMessage `db:"definition"`
	Checksum    string          `db:"checksum"`
	PublishedBy uuid.UUID       `db:"published_by"`
	PublishedAt time.Time       `db:"published_at"`
}

type TestRow struct {
	ID        uuid.UUID       `db:"id"`
	MappingID uuid.UUID       `db:"mapping_id"`
	Name      string          `db:"name"`
	Input     json.RawMessage `db:"input"`
	Expected  json.RawMessage `db:"expected"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (r MappingRow) Model() mappingmodel.Mapping {
	draft := mappingmodel.RawObject(r.Draft)
	if draft == nil {
		draft = map[string]any{"rules": []any{}}
	}
	return mappingmodel.Mapping{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Draft: draft,
		InputSchema: mappingmodel.RawObject(r.InputSchema), OutputSchema: mappingmodel.RawObject(r.OutputSchema),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r VersionRow) Model() mappingmodel.MappingVersion {
	return mappingmodel.MappingVersion{ID: r.ID, MappingID: r.MappingID, Version: r.Version,
		Definition: mappingmodel.RawObject(r.Definition), Checksum: r.Checksum,
		PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}
}

func (r TestRow) Model() mappingmodel.MappingTest {
	return mappingmodel.MappingTest{ID: r.ID, MappingID: r.MappingID, Name: r.Name,
		Input: mappingmodel.RawAny(r.Input), Expected: mappingmodel.RawAny(r.Expected),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
