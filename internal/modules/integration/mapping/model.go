package mapping

import (
	"time"

	"github.com/google/uuid"
)

type Mapping struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	Name         string
	Draft        map[string]any
	InputSchema  map[string]any
	OutputSchema map[string]any
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MappingVersion struct {
	ID          uuid.UUID
	MappingID   uuid.UUID
	Version     int
	Definition  map[string]any
	Checksum    string
	PublishedBy uuid.UUID
	PublishedAt time.Time
}

type MappingTest struct {
	ID        uuid.UUID
	MappingID uuid.UUID
	Name      string
	Input     any
	Expected  any
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TestFailure struct {
	Name     string
	Actual   any
	Expected any
}

type TestResult struct {
	Output            any
	StoredTestsPassed int
	StoredTestsFailed []TestFailure
}
