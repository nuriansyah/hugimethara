package model

import (
	"strings"

	"github.com/google/uuid"
)

// Scope kinds a role can be bound at.
const (
	ScopeOrg       = "org"
	ScopeWorkspace = "workspace"
	ScopeResource  = "resource"
)

// ScopeRef identifies where a binding applies.
type ScopeRef struct {
	Type         string    // org | workspace | resource
	WorkspaceID  uuid.UUID // workspace scope
	ResourceType string    // resource scope
	ResourceID   string    // resource scope
}

// Principal kinds a role can be bound to.
const (
	PrincipalUser           = "user"
	PrincipalGroup          = "group"
	PrincipalServiceAccount = "service_account"
)

// PrincipalRef identifies who a binding is for.
type PrincipalRef struct {
	Type string // user | group | service_account
	ID   uuid.UUID
}

// UserRef is a PrincipalRef for a user.
func UserRef(id uuid.UUID) PrincipalRef { return PrincipalRef{Type: PrincipalUser, ID: id} }

// Grant is one permission pattern held at a scope through a role binding.
type Grant struct {
	Permission string // "type:action"; either part may be "*"
	Scope      ScopeRef
	RoleKey    string
	BindingID  uuid.UUID
	Via        string // principal type of the binding (user, group, service_account)
}

func SplitPerm(p string) (string, string, bool) {
	t, a, ok := strings.Cut(p, ":")
	return t, a, ok && t != "" && a != ""
}

// Covers reports whether pattern grants perm. perm may itself contain "*"
// (when checking whether a holder may grant a wildcard permission), in which
// case only a "*" in the pattern covers it.
func Covers(pattern, perm string) bool {
	pt, pa, ok1 := SplitPerm(pattern)
	qt, qa, ok2 := SplitPerm(perm)
	if !ok1 || !ok2 {
		return false
	}
	return (pt == "*" || pt == qt) && (pa == "*" || pa == qa)
}

// Contains reports whether scope s includes scope t:
// org ⊇ everything; a workspace ⊇ itself; a resource ⊇ itself.
func (s ScopeRef) Contains(t ScopeRef) bool {
	switch s.Type {
	case ScopeOrg:
		return true
	case ScopeWorkspace:
		return t.Type == ScopeWorkspace && t.WorkspaceID == s.WorkspaceID
	case ScopeResource:
		return t.Type == ScopeResource && t.ResourceType == s.ResourceType && t.ResourceID == s.ResourceID
	}
	return false
}

// appliesTo reports whether a grant at scope s applies to resource r.
func (s ScopeRef) appliesTo(r Resource) bool {
	switch s.Type {
	case ScopeOrg:
		return true
	case ScopeWorkspace:
		return r.WorkspaceID != nil && *r.WorkspaceID == s.WorkspaceID
	case ScopeResource:
		return r.ID != "" && r.Type == s.ResourceType && r.ID == s.ResourceID
	}
	return false
}

// matching returns the grants that allow action on r.
func Matching(grants []Grant, action string, r Resource) []Grant {
	perm := Permission(r.Type, action)
	var out []Grant
	for _, g := range grants {
		if Covers(g.Permission, perm) && g.Scope.appliesTo(r) {
			out = append(out, g)
		}
	}
	return out
}

// CanGrant reports whether a holder of grants may hand out every permission in
// perms at scope target — the "no escalation" rule: you cannot give what you
// do not hold there.
func CanGrant(grants []Grant, perms []string, target ScopeRef) (ok bool, missing []string) {
	for _, p := range perms {
		held := false
		for _, g := range grants {
			if Covers(g.Permission, p) && g.Scope.Contains(target) {
				held = true
				break
			}
		}
		if !held {
			missing = append(missing, p)
		}
	}
	return len(missing) == 0, missing
}
