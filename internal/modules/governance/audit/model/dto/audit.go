package dto

import (
	"time"

	"github.com/google/uuid"
)

// Filter narrows a listing. Zero values mean "no filter".
type Filter struct {
	Action       string // exact action or prefix ("organization" matches "organization.*")
	ActorID      uuid.UUID
	ResourceType string
	Since, Until time.Time
}
