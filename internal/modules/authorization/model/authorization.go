package model

import (
	"database/sql"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Resource types owned by this domain.
const (
	RoleResource          = "role"
	AuthorizationResource = "authorization"
	OwnerRoleKey          = "org_owner"
	MemberRoleKey         = "member"
)

func Escalation(missing []string) *problem.Problem {
	p := problem.NewProblem(http.StatusForbidden, "privilege_escalation", "Not allowed",
		"You can only grant permissions you hold yourself at that scope. Missing: "+strings.Join(missing, ", ")+".")
	p.Extensions = map[string]any{"missing_permissions": missing}
	return p
}

var RoleKeyRe = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

var ErrRoleNotFound = problem.NotFound("Role not found.")

func NullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func NullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func ResourceForScope(scope ScopeRef) Resource {
	r := Resource{Type: RoleResource}
	switch scope.Type {
	case ScopeWorkspace:
		ws := scope.WorkspaceID
		r.WorkspaceID = &ws
	case ScopeResource:
		r.ID = scope.ResourceType + ":" + scope.ResourceID
	}
	return r
}

func PrincipalDetails(pr PrincipalRef) map[string]any {
	return map[string]any{"type": pr.Type, "id": pr.ID}
}

func ScopeDetails(sc ScopeRef) map[string]any {
	m := map[string]any{"type": sc.Type}
	if sc.Type == ScopeWorkspace {
		m["workspace_id"] = sc.WorkspaceID
	}
	if sc.Type == ScopeResource {
		m["resource_type"], m["resource_id"] = sc.ResourceType, sc.ResourceID
	}
	return m
}

var ErrLastOwner = problem.NewProblem(http.StatusConflict, "last_owner", "Conflict",
	"An organization needs at least one owner. Make someone else an owner first.")

// RefOf converts a request principal into a binding principal.
func RefOf(p principal.Principal) PrincipalRef {
	if p.Type == principal.ServiceAccount {
		return PrincipalRef{Type: PrincipalServiceAccount, ID: p.ID}
	}
	return UserRef(p.ID)
}

// Effective lists concrete permissions held at organization scope and, per
// workspace, those held only through workspace-scope bindings.
type Effective struct {
	Organization []string
	Workspaces   map[uuid.UUID][]string
}

// expand turns permission patterns into concrete "type:action" strings.
func Expand(types map[string]ResourceType, patterns []string) []string {
	var out []string
	for _, rt := range types {
		for _, a := range rt.Actions {
			perm := rt.Type + ":" + a
			for _, p := range patterns {
				if Covers(p, perm) {
					out = append(out, perm)
					break
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

// Decision is one logged authorization decision.
type Decision struct {
	ID             uuid.UUID      `db:"id"`
	OccurredAt     time.Time      `db:"occurred_at"`
	PrincipalID    uuid.UUID      `db:"principal_id"`
	PrincipalType  string         `db:"principal_type"`
	PrincipalEmail sql.NullString `db:"principal_email"`
	Action         string         `db:"action"`
	ResourceType   string         `db:"resource_type"`
	ResourceID     sql.NullString `db:"resource_id"`
	WorkspaceID    uuid.NullUUID  `db:"workspace_id"`
	Allowed        bool           `db:"allowed"`
	Reason         string         `db:"reason"`
	MatchedRaw     []byte         `db:"matched_rules"`
	RequestID      sql.NullString `db:"request_id"`
	MatchedRules   []string
}

type DecisionCursor struct {
	T  time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}
