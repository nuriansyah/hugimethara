package authz

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Resource types owned by this domain.
const (
	RoleResource          = "role"
	AuthorizationResource = "authorization"
	OwnerRoleKey          = "org_owner"
	MemberRoleKey         = "member"
)

// AuditFunc records an audit event inside tx. It is injected to avoid an
// import cycle with the audit domain (which depends on authz).
type AuditFunc func(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action, resourceType, resourceID string, details map[string]any) error

// Service manages roles and bindings and answers permission queries.
type Service struct {
	db    *sqlx.DB
	rbac  *RBAC
	audit AuditFunc
}

func NewService(d *sqlx.DB, rbac *RBAC, audit AuditFunc) *Service {
	return &Service{db: d, rbac: rbac, audit: audit}
}

// SeedOrganization copies the role templates into a new organization and
// makes owner an org_owner and member (inside the signup transaction).
func SeedOrganization(ctx context.Context, tx *sqlx.Tx, org, owner uuid.UUID) error {
	if _, err := tx.ExecContext(ctx, q.Get("SeedRoles"), org); err != nil {
		return fmt.Errorf("authz: seed roles: %w", err)
	}
	if _, err := tx.ExecContext(ctx, q.Get("SeedRolePermissions"), org); err != nil {
		return fmt.Errorf("authz: seed permissions: %w", err)
	}
	for _, key := range []string{OwnerRoleKey, MemberRoleKey} {
		if err := insertBindingByKey(ctx, tx, org, UserRef(owner), key, ScopeRef{Type: ScopeOrg}, uuid.Nil); err != nil {
			return err
		}
	}
	return nil
}

func insertBindingByKey(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who PrincipalRef, roleKey string, scope ScopeRef, by uuid.UUID) error {
	var role roleRow
	if err := tx.GetContext(ctx, &role, q.Get("GetRoleByKey"), roleKey); err != nil {
		return fmt.Errorf("authz: role %s: %w", roleKey, err)
	}
	_, err := insertBinding(ctx, tx, org, who, role.ID, scope, by, nil)
	return err
}

func insertBinding(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who PrincipalRef, role uuid.UUID, scope ScopeRef, by uuid.UUID, expires *time.Time) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	var user, group, sa, ws, rt, rid, createdBy, exp any
	switch who.Type {
	case PrincipalUser:
		user = who.ID
	case PrincipalGroup:
		group = who.ID
	case PrincipalServiceAccount:
		sa = who.ID
	default:
		return uuid.Nil, fmt.Errorf("authz: unknown principal type %q", who.Type)
	}
	switch scope.Type {
	case ScopeWorkspace:
		ws = scope.WorkspaceID
	case ScopeResource:
		rt, rid = scope.ResourceType, scope.ResourceID
	}
	if by != uuid.Nil {
		createdBy = by
	}
	if expires != nil {
		exp = *expires
	}
	res, err := tx.ExecContext(ctx, q.Get("InsertBinding"), id, org, role, who.Type, user, group, sa, scope.Type, ws, rt, rid, createdBy, exp)
	if err != nil {
		return uuid.Nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return uuid.Nil, nil // already bound: idempotent
	}
	return id, nil
}

// Invalidate publishes an authorization change for org (call after commit).
func (s *Service) Invalidate(ctx context.Context, org uuid.UUID) { s.rbac.Invalidate(ctx, org) }

func (s *Service) require(ctx context.Context, action string, res Resource) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, Unauthenticated()
	}
	res.OrgID = p.OrgID
	return Require(ctx, s.rbac, action, res)
}

func (s *Service) tx(ctx context.Context, org uuid.UUID, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.db, org, fn)
}

// Catalog ------------------------------------------------------------------------

// ResourceTypes lists the permission catalog.
func (s *Service) ResourceTypes(ctx context.Context) ([]ResourceType, error) {
	if _, err := s.require(ctx, "read", Resource{Type: RoleResource}); err != nil {
		return nil, err
	}
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceType, 0, len(types))
	for _, t := range types {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b ResourceType) int { return strings.Compare(a.Type, b.Type) })
	return out, nil
}

// validatePermissions checks patterns against the catalog and normalizes them.
func (s *Service) validatePermissions(ctx context.Context, perms []string) ([]string, error) {
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range perms {
		p = strings.TrimSpace(p)
		t, a, ok := splitPerm(p)
		valid := ok
		if ok && t != "*" {
			rt, known := types[t]
			valid = known && (a == "*" || slices.Contains(rt.Actions, a))
		} else if ok && a != "*" {
			// "*:action" is valid when at least one type has the action.
			valid = false
			for _, rt := range types {
				if slices.Contains(rt.Actions, a) {
					valid = true
					break
				}
			}
		}
		if !valid {
			return nil, httpx.Validation(httpx.FieldError{Field: "permissions", Code: "unknown_permission",
				Message: fmt.Sprintf("%q is not a known permission (format resource_type:action).", p)})
		}
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

func escalation(missing []string) *httpx.Problem {
	p := httpx.NewProblem(http.StatusForbidden, "privilege_escalation", "Not allowed",
		"You can only grant permissions you hold yourself at that scope. Missing: "+strings.Join(missing, ", ")+".")
	p.Extensions = map[string]any{"missing_permissions": missing}
	return p
}

// Roles ----------------------------------------------------------------------------

var roleKeyRe = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

// RoleInput creates or updates a role (Key is create-only).
type RoleInput struct {
	Key         string
	Name        string
	Description string
	Permissions []string
}

var errRoleNotFound = httpx.NotFound("Role not found.")

func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	p, err := s.require(ctx, "read", Resource{Type: RoleResource})
	if err != nil {
		return nil, err
	}
	var rows []roleRow
	if err := s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &rows, q.Get("ListRoles")) }); err != nil {
		return nil, err
	}
	out := make([]Role, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

func (s *Service) GetRole(ctx context.Context, id uuid.UUID) (Role, error) {
	p, err := s.require(ctx, "read", Resource{Type: RoleResource, ID: id.String()})
	if err != nil {
		return Role{}, err
	}
	var row roleRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &row, q.Get("GetRole"), id) })
	if db.IsNoRows(err) {
		return Role{}, errRoleNotFound
	}
	return row.toModel(), err
}

func validateRoleText(in *RoleInput) []httpx.FieldError {
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	var errs []httpx.FieldError
	if n := len([]rune(in.Name)); n < 1 || n > 100 {
		errs = append(errs, httpx.FieldError{Field: "name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	if len([]rune(in.Description)) > 500 {
		errs = append(errs, httpx.FieldError{Field: "description", Code: "invalid_length", Message: "Description must be at most 500 characters."})
	}
	return errs
}

func (s *Service) CreateRole(ctx context.Context, in RoleInput) (Role, error) {
	p, err := s.require(ctx, "create", Resource{Type: RoleResource})
	if err != nil {
		return Role{}, err
	}
	in.Key = strings.ToLower(strings.TrimSpace(in.Key))
	errs := validateRoleText(&in)
	if !roleKeyRe.MatchString(in.Key) {
		errs = append(errs, httpx.FieldError{Field: "key", Code: "invalid_key", Message: "Use 2–40 lowercase letters, digits or underscores."})
	}
	if len(errs) > 0 {
		return Role{}, httpx.Validation(errs...)
	}
	perms, err := s.validatePermissions(ctx, in.Permissions)
	if err != nil {
		return Role{}, err
	}
	grants, _, err := s.rbac.Grants(ctx, p)
	if err != nil {
		return Role{}, err
	}
	if ok, missing := CanGrant(grants, perms, ScopeRef{Type: ScopeOrg}); !ok {
		return Role{}, escalation(missing)
	}
	var row roleRow
	id := uuid.Must(uuid.NewV7())
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("InsertRole"), id, p.OrgID, in.Key, in.Name, in.Description); err != nil {
			if db.IsUniqueViolation(err) {
				return httpx.NewProblem(http.StatusConflict, "key_taken", "Conflict", "A role with this key already exists.")
			}
			return err
		}
		for _, perm := range perms {
			if _, err := tx.ExecContext(ctx, q.Get("InsertRolePermission"), p.OrgID, id, perm); err != nil {
				return err
			}
		}
		if err := tx.GetContext(ctx, &row, q.Get("GetRole"), id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.created", RoleResource, id.String(), map[string]any{"key": in.Key, "permissions": perms})
	})
	return row.toModel(), err
}

func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, in RoleInput) (Role, error) {
	p, err := s.require(ctx, "update", Resource{Type: RoleResource, ID: id.String()})
	if err != nil {
		return Role{}, err
	}
	if errs := validateRoleText(&in); len(errs) > 0 {
		return Role{}, httpx.Validation(errs...)
	}
	perms, err := s.validatePermissions(ctx, in.Permissions)
	if err != nil {
		return Role{}, err
	}
	grants, _, err := s.rbac.Grants(ctx, p)
	if err != nil {
		return Role{}, err
	}
	var row roleRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		var before roleRow
		if err := tx.GetContext(ctx, &before, q.Get("GetRole"), id); err != nil {
			if db.IsNoRows(err) {
				return errRoleNotFound
			}
			return err
		}
		prev := before.toModel()
		if prev.Immutable && !slices.Equal(prev.Permissions, perms) {
			return httpx.NewProblem(http.StatusConflict, "role_immutable", "Conflict", "The permissions of this role cannot be changed.")
		}
		var added []string
		for _, perm := range perms {
			if !slices.Contains(prev.Permissions, perm) {
				added = append(added, perm)
			}
		}
		if ok, missing := CanGrant(grants, added, ScopeRef{Type: ScopeOrg}); !ok {
			return escalation(missing)
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateRole"), id, in.Name, in.Description); err != nil {
			return err
		}
		if !slices.Equal(prev.Permissions, perms) {
			if _, err := tx.ExecContext(ctx, q.Get("DeleteRolePermissions"), id); err != nil {
				return err
			}
			for _, perm := range perms {
				if _, err := tx.ExecContext(ctx, q.Get("InsertRolePermission"), p.OrgID, id, perm); err != nil {
					return err
				}
			}
			if err := BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		if err := tx.GetContext(ctx, &row, q.Get("GetRole"), id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.updated", RoleResource, id.String(), map[string]any{
			"key": prev.Key, "from": map[string]any{"name": prev.Name, "permissions": prev.Permissions},
			"to": map[string]any{"name": in.Name, "permissions": perms}})
	})
	if err == nil {
		s.Invalidate(ctx, p.OrgID)
	}
	return row.toModel(), err
}

func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", Resource{Type: RoleResource, ID: id.String()})
	if err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		var before roleRow
		if err := tx.GetContext(ctx, &before, q.Get("GetRole"), id); err != nil {
			if db.IsNoRows(err) {
				return errRoleNotFound
			}
			return err
		}
		if before.IsSystem {
			return httpx.NewProblem(http.StatusConflict, "role_system", "Conflict", "Built-in roles cannot be deleted. Edit their permissions instead.")
		}
		if before.BindingCount > 0 {
			return httpx.NewProblem(http.StatusConflict, "role_in_use", "Conflict",
				fmt.Sprintf("This role is assigned %d time(s). Remove the assignments first.", before.BindingCount))
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteRole"), id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.deleted", RoleResource, id.String(), map[string]any{"key": before.Key})
	})
}

// Bindings ----------------------------------------------------------------------------

// BindingFilter narrows ListBindings.
type BindingFilter struct {
	Principal   PrincipalRef // zero = everyone; Type alone filters by kind
	ScopeType   string
	WorkspaceID uuid.UUID
}

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Service) ListBindings(ctx context.Context, f BindingFilter) ([]Binding, error) {
	p, err := s.require(ctx, "read", Resource{Type: RoleResource})
	if err != nil {
		return nil, err
	}
	var rows []bindingRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListBindings"), nullStr(f.Principal.Type), nullUUID(f.Principal.ID),
			nullStr(f.ScopeType), nullUUID(f.WorkspaceID))
	})
	out := make([]Binding, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, err
}

// BindingInput assigns a role.
type BindingInput struct {
	Principal PrincipalRef
	RoleID    uuid.UUID // or RoleKey
	RoleKey   string
	Scope     ScopeRef
	ExpiresAt *time.Time
}

func scopeResource(scope ScopeRef) Resource {
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

func (s *Service) validateScope(ctx context.Context, sc ScopeRef) error {
	switch sc.Type {
	case ScopeOrg:
		return nil
	case ScopeWorkspace:
		if sc.WorkspaceID == uuid.Nil {
			return httpx.Validation(httpx.FieldError{Field: "workspace_id", Code: "required", Message: "Choose a workspace."})
		}
		return nil
	case ScopeResource:
		types, err := s.rbac.ResourceTypes(ctx)
		if err != nil {
			return err
		}
		if _, ok := types[sc.ResourceType]; !ok || sc.ResourceID == "" || len(sc.ResourceID) > 200 {
			return httpx.Validation(httpx.FieldError{Field: "resource", Code: "invalid", Message: "Choose a known resource type and a resource id."})
		}
		return nil
	}
	return httpx.Validation(httpx.FieldError{Field: "scope_type", Code: "invalid", Message: "Scope must be org, workspace or resource."})
}

// CreateBinding assigns a role (permission role:bind at the target scope, and
// no escalation beyond the caller's own permissions there).
func (s *Service) CreateBinding(ctx context.Context, in BindingInput) (Binding, error) {
	if err := s.validateScope(ctx, in.Scope); err != nil {
		return Binding{}, err
	}
	p, err := s.require(ctx, "bind", scopeResource(in.Scope))
	if err != nil {
		return Binding{}, err
	}
	var out bindingRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		id, err := s.BindInTx(ctx, tx, p, in)
		if err != nil {
			return err
		}
		if id == uuid.Nil {
			return httpx.NewProblem(http.StatusConflict, "already_assigned", "Conflict", "This role is already assigned at that scope.")
		}
		return tx.GetContext(ctx, &out, q.Get("GetBinding"), id)
	})
	if err != nil {
		return Binding{}, err
	}
	s.Invalidate(ctx, p.OrgID)
	return out.toModel(), nil
}

// BindInTx creates a binding inside the caller's transaction after the
// no-escalation check, bumping the authz version and auditing. Other domains
// (workspace membership) call it after their own permission check; they must
// call Invalidate after commit. Returns uuid.Nil if already bound.
func (s *Service) BindInTx(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, in BindingInput) (uuid.UUID, error) {
	var role roleRow
	var err error
	if in.RoleID != uuid.Nil {
		err = tx.GetContext(ctx, &role, q.Get("GetRole"), in.RoleID)
	} else {
		err = tx.GetContext(ctx, &role, q.Get("GetRoleByKey"), in.RoleKey)
	}
	if db.IsNoRows(err) {
		return uuid.Nil, httpx.Validation(httpx.FieldError{Field: "role", Code: "not_found", Message: "Role not found."})
	}
	if err != nil {
		return uuid.Nil, err
	}
	model := role.toModel()
	grants, _, err := s.rbac.Grants(ctx, caller)
	if err != nil {
		return uuid.Nil, err
	}
	if ok, missing := CanGrant(grants, model.Permissions, in.Scope); !ok {
		return uuid.Nil, escalation(missing)
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return uuid.Nil, httpx.Validation(httpx.FieldError{Field: "expires_at", Code: "in_past", Message: "Expiry must be in the future."})
	}
	if in.Principal.ID == uuid.Nil {
		return uuid.Nil, httpx.Validation(httpx.FieldError{Field: "principal", Code: "required", Message: "Choose who gets the role."})
	}
	id, err := insertBinding(ctx, tx, caller.OrgID, in.Principal, role.ID, in.Scope, caller.ID, in.ExpiresAt)
	if db.IsForeignKeyViolation(err) {
		field, msg := "user_id", "This user is not a member of the organization (or the workspace does not exist)."
		switch in.Principal.Type {
		case PrincipalGroup:
			field, msg = "group_id", "This group does not exist (or the workspace does not exist)."
		case PrincipalServiceAccount:
			field, msg = "service_account_id", "This service account does not exist (or the workspace does not exist)."
		}
		return uuid.Nil, httpx.Validation(httpx.FieldError{Field: field, Code: "not_found", Message: msg})
	}
	if err != nil || id == uuid.Nil {
		return id, err
	}
	if err := BumpVersion(ctx, tx, caller.OrgID); err != nil {
		return uuid.Nil, err
	}
	return id, s.audit(ctx, tx, caller.OrgID, "role.assigned", RoleResource, role.ID.String(), map[string]any{
		"binding_id": id, "principal": principalDetails(in.Principal), "role": model.Key, "scope": scopeDetails(in.Scope)})
}

func principalDetails(pr PrincipalRef) map[string]any {
	return map[string]any{"type": pr.Type, "id": pr.ID}
}

func scopeDetails(sc ScopeRef) map[string]any {
	m := map[string]any{"type": sc.Type}
	if sc.Type == ScopeWorkspace {
		m["workspace_id"] = sc.WorkspaceID
	}
	if sc.Type == ScopeResource {
		m["resource_type"], m["resource_id"] = sc.ResourceType, sc.ResourceID
	}
	return m
}

// DeleteBinding removes an assignment (role:bind at its scope). The last
// organization owner cannot be removed.
func (s *Service) DeleteBinding(ctx context.Context, id uuid.UUID) error {
	p, ok := principal.From(ctx)
	if !ok {
		return Unauthenticated()
	}
	var b bindingRow
	err := s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &b, q.Get("GetBinding"), id) })
	if db.IsNoRows(err) {
		return httpx.NotFound("Assignment not found.")
	}
	if err != nil {
		return err
	}
	model := b.toModel()
	if _, err := s.require(ctx, "bind", scopeResource(model.Scope)); err != nil {
		return err
	}
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		if err := s.guardLastOwner(ctx, tx, model); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteBinding"), id); err != nil {
			return err
		}
		if err := BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.unassigned", RoleResource, model.RoleID.String(), map[string]any{
			"binding_id": id, "principal": principalDetails(model.Principal), "role": model.RoleKey, "scope": scopeDetails(model.Scope)})
	})
	if err == nil {
		s.Invalidate(ctx, p.OrgID)
	}
	return err
}

func (s *Service) guardLastOwner(ctx context.Context, tx *sqlx.Tx, b Binding) error {
	if b.RoleKey != OwnerRoleKey || b.Scope.Type != ScopeOrg || b.Principal.Type != PrincipalUser {
		return nil
	}
	return ensureOwnerRemains(ctx, tx, b.ID, nil)
}

var errLastOwner = httpx.NewProblem(http.StatusConflict, "last_owner", "Conflict",
	"An organization needs at least one owner. Make someone else an owner first.")

func ensureOwnerRemains(ctx context.Context, tx *sqlx.Tx, excludeBinding uuid.UUID, excludeUsers []uuid.UUID) error {
	if excludeUsers == nil {
		excludeUsers = []uuid.UUID{}
	}
	var others int64
	if err := tx.GetContext(ctx, &others, q.Get("CountOtherOwners"), excludeBinding, excludeUsers); err != nil {
		return err
	}
	if others == 0 {
		return errLastOwner
	}
	return nil
}

// EnsureOwnerRemainsWithout fails with 409 last_owner when no active direct
// owner would remain once user loses access (removal or suspension).
func EnsureOwnerRemainsWithout(ctx context.Context, tx *sqlx.Tx, user uuid.UUID) error {
	return ensureOwnerRemains(ctx, tx, uuid.Nil, []uuid.UUID{user})
}

// CheckCanHandOut enforces the no-escalation rule for indirect grants: before
// a user joins a group, the caller must be able to grant every role the group
// holds, at the scope it holds it.
func (s *Service) CheckCanHandOut(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, target PrincipalRef) error {
	var rows []grantRow
	if err := tx.SelectContext(ctx, &rows, q.Get("PrincipalBindingGrants"), target.Type, target.ID); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	grants, _, err := s.rbac.Grants(ctx, caller)
	if err != nil {
		return err
	}
	var missing []string
	for _, r := range rows {
		g := r.toGrant()
		if ok, m := CanGrant(grants, []string{g.Permission}, g.Scope); !ok {
			for _, p := range m {
				if !slices.Contains(missing, p) {
					missing = append(missing, p)
				}
			}
		}
	}
	if len(missing) > 0 {
		return escalation(missing)
	}
	return nil
}

// RefOf converts a request principal into a binding principal.
func RefOf(p principal.Principal) PrincipalRef {
	if p.Type == principal.ServiceAccount {
		return PrincipalRef{Type: PrincipalServiceAccount, ID: p.ID}
	}
	return UserRef(p.ID)
}

// RemoveWorkspaceAccessInTx removes all of a user's workspace-scope bindings
// in a workspace (caller already authorized). Returns how many were removed.
func (s *Service) RemoveWorkspaceAccessInTx(ctx context.Context, tx *sqlx.Tx, org, workspace, user uuid.UUID) (int64, error) {
	res, err := tx.ExecContext(ctx, q.Get("DeleteWorkspaceBindings"), workspace, user)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		if err := BumpVersion(ctx, tx, org); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// IsOrgOwner reports whether user holds org_owner at organization scope.
func IsOrgOwner(ctx context.Context, tx *sqlx.Tx, user uuid.UUID) (bool, error) {
	var ok bool
	err := tx.GetContext(ctx, &ok, q.Get("IsOrgOwner"), user)
	return ok, err
}

// Permissions -----------------------------------------------------------------------------

// Effective lists concrete permissions held at organization scope and, per
// workspace, those held only through workspace-scope bindings.
type Effective struct {
	Organization []string
	Workspaces   map[uuid.UUID][]string
}

// expand turns permission patterns into concrete "type:action" strings.
func expand(types map[string]ResourceType, patterns []string) []string {
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

// MyPermissions returns the caller's effective permissions (for UI gating;
// the API still authorizes every request).
func (s *Service) MyPermissions(ctx context.Context) (Effective, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Effective{}, Unauthenticated()
	}
	grants, _, err := s.rbac.Grants(ctx, p)
	if err != nil {
		return Effective{}, err
	}
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return Effective{}, err
	}
	var orgPatterns []string
	wsPatterns := map[uuid.UUID][]string{}
	for _, g := range grants {
		switch g.Scope.Type {
		case ScopeOrg:
			orgPatterns = append(orgPatterns, g.Permission)
		case ScopeWorkspace:
			wsPatterns[g.Scope.WorkspaceID] = append(wsPatterns[g.Scope.WorkspaceID], g.Permission)
		}
	}
	out := Effective{Organization: expand(types, orgPatterns), Workspaces: map[uuid.UUID][]string{}}
	for ws, pats := range wsPatterns {
		out.Workspaces[ws] = expand(types, pats)
	}
	if out.Organization == nil {
		out.Organization = []string{}
	}
	return out, nil
}

// Simulate answers "could user U do action A on resource R?" without logging
// the answer as a real decision (permission authorization:simulate).
func (s *Service) Simulate(ctx context.Context, who PrincipalRef, action string, res Resource) (AuthorizationDecision, error) {
	p, err := s.require(ctx, "simulate", Resource{Type: AuthorizationResource})
	if err != nil {
		return AuthorizationDecision{}, err
	}
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return AuthorizationDecision{}, err
	}
	if rt, ok := types[res.Type]; !ok || !slices.Contains(rt.Actions, action) {
		return AuthorizationDecision{}, httpx.Validation(httpx.FieldError{Field: "action", Code: "unknown_permission",
			Message: fmt.Sprintf("%s:%s is not a known permission.", res.Type, action)})
	}
	res.OrgID = p.OrgID
	kind := principal.User
	if who.Type == PrincipalServiceAccount {
		kind = principal.ServiceAccount
	}
	return s.rbac.Check(ctx, AuthorizationInput{Principal: principal.Principal{ID: who.ID, Type: kind, OrgID: p.OrgID},
		Action: action, Resource: res})
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

type decisionCursor struct {
	T  time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}

// ListDecisions pages through the decision log (authorization:read_decisions).
func (s *Service) ListDecisions(ctx context.Context, allowed *bool, page httpx.Page) ([]Decision, string, error) {
	p, err := s.require(ctx, "read_decisions", Resource{Type: AuthorizationResource})
	if err != nil {
		return nil, "", err
	}
	limit := min(max(page.Limit, 1), httpx.MaxPageLimit)
	var curT, curID any
	if page.Cursor != "" {
		var c decisionCursor
		if err := httpx.DecodeCursor(page.Cursor, &c); err != nil {
			return nil, "", err
		}
		curT, curID = c.T, c.ID
	}
	var allowedArg any
	if allowed != nil {
		allowedArg = *allowed
	}
	var rows []Decision
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListDecisions"), allowedArg, curT, curID, limit+1)
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next, _ = httpx.EncodeCursor(decisionCursor{T: last.OccurredAt, ID: last.ID})
	}
	for i := range rows {
		_ = json.Unmarshal(rows[i].MatchedRaw, &rows[i].MatchedRules)
	}
	return rows, next, nil
}

// GrantCreatorInTx makes the creator of a workspace (a user or a service
// account) its workspace_admin (creation itself was authorized by workspace:create).
func GrantCreatorInTx(ctx context.Context, tx *sqlx.Tx, creator principal.Principal, workspace uuid.UUID) error {
	if err := insertBindingByKey(ctx, tx, creator.OrgID, RefOf(creator), "workspace_admin",
		ScopeRef{Type: ScopeWorkspace, WorkspaceID: workspace}, creator.ID); err != nil {
		return err
	}
	return BumpVersion(ctx, tx, creator.OrgID)
}

// CheckGrantRole loads role id and verifies the caller could grant it at
// scope (no escalation). Used to validate grants that are applied later, such
// as the roles an invitation hands out when it is accepted.
func (s *Service) CheckGrantRole(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, roleID uuid.UUID, scope ScopeRef) (Role, error) {
	var row roleRow
	if err := tx.GetContext(ctx, &row, q.Get("GetRole"), roleID); err != nil {
		if db.IsNoRows(err) {
			return Role{}, httpx.Validation(httpx.FieldError{Field: "role_id", Code: "not_found", Message: "Role not found."})
		}
		return Role{}, err
	}
	role := row.toModel()
	grants, _, err := s.rbac.Grants(ctx, caller)
	if err != nil {
		return Role{}, err
	}
	if ok, missing := CanGrant(grants, role.Permissions, scope); !ok {
		return Role{}, escalation(missing)
	}
	return role, nil
}

// BindPreauthorizedInTx inserts a binding whose grant was authorized earlier
// (e.g. by CheckGrantRole when an invitation was created). by is recorded as
// the creator. Idempotent. The caller bumps the version and invalidates.
func BindPreauthorizedInTx(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who PrincipalRef, roleID uuid.UUID, scope ScopeRef, by uuid.UUID) error {
	_, err := insertBinding(ctx, tx, org, who, roleID, scope, by, nil)
	return err
}

// BindRoleKeyInTx is BindPreauthorizedInTx for a role identified by key
// (e.g. the baseline "member" role every member holds).
func BindRoleKeyInTx(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who PrincipalRef, roleKey string, scope ScopeRef, by uuid.UUID) error {
	return insertBindingByKey(ctx, tx, org, who, roleKey, scope, by)
}
