package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/pagination"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// AuditFunc records an audit event inside tx. It is injected to avoid an
// import cycle with the audit domain (which depends on authz).
type AuditFunc func(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action, resourceType, resourceID string, details map[string]any) error

// AuthorizationService manages roles and bindings and answers permission queries.
type AuthorizationService struct {
	db    *sqlx.DB
	rbac  *RBAC
	audit AuditFunc
}

func NewService(d *sqlx.DB, rbac *RBAC, audit AuditFunc) *AuthorizationService {
	return &AuthorizationService{db: d, rbac: rbac, audit: audit}
}

// SeedOrganization copies the role templates into a new organization and
// makes owner an org_owner and member (inside the signup transaction).
func SeedOrganization(ctx context.Context, tx *sqlx.Tx, org, owner uuid.UUID) error {
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("SeedRoles"), org); err != nil {
		return fmt.Errorf("authz: seed roles: %w", err)
	}
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("SeedRolePermissions"), org); err != nil {
		return fmt.Errorf("authz: seed permissions: %w", err)
	}
	for _, key := range []string{authzmodel.OwnerRoleKey, authzmodel.MemberRoleKey} {
		if err := insertBindingByKey(ctx, tx, org, authzmodel.UserRef(owner), key, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}, uuid.Nil); err != nil {
			return err
		}
	}
	return nil
}

func insertBindingByKey(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who authzmodel.PrincipalRef, roleKey string, scope authzmodel.ScopeRef, by uuid.UUID) error {
	var role repository.RoleRow
	if err := tx.GetContext(ctx, &role, queries.SQL.Get("GetRoleByKey"), roleKey); err != nil {
		return fmt.Errorf("authz: role %s: %w", roleKey, err)
	}
	_, err := insertBinding(ctx, tx, org, who, role.ID, scope, by, nil)
	return err
}

func insertBinding(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who authzmodel.PrincipalRef, role uuid.UUID, scope authzmodel.ScopeRef, by uuid.UUID, expires *time.Time) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	var user, group, sa, ws, rt, rid, createdBy, exp any
	switch who.Type {
	case authzmodel.PrincipalUser:
		user = who.ID
	case authzmodel.PrincipalGroup:
		group = who.ID
	case authzmodel.PrincipalServiceAccount:
		sa = who.ID
	default:
		return uuid.Nil, fmt.Errorf("authz: unknown principal type %q", who.Type)
	}
	switch scope.Type {
	case authzmodel.ScopeWorkspace:
		ws = scope.WorkspaceID
	case authzmodel.ScopeResource:
		rt, rid = scope.ResourceType, scope.ResourceID
	}
	if by != uuid.Nil {
		createdBy = by
	}
	if expires != nil {
		exp = *expires
	}
	res, err := tx.ExecContext(ctx, commands.SQL.Get("InsertBinding"), id, org, role, who.Type, user, group, sa, scope.Type, ws, rt, rid, createdBy, exp)
	if err != nil {
		return uuid.Nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return uuid.Nil, nil // already bound: idempotent
	}
	return id, nil
}

// Invalidate publishes an authorization change for org (call after commit).
func (s *AuthorizationService) Invalidate(ctx context.Context, org uuid.UUID) {
	s.rbac.Invalidate(ctx, org)
}

func (s *AuthorizationService) require(ctx context.Context, action string, res authzmodel.Resource) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authzmodel.Unauthenticated()
	}
	res.OrgID = p.OrgID
	return Require(ctx, s.rbac, action, res)
}

func (s *AuthorizationService) tx(ctx context.Context, org uuid.UUID, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.db, org, fn)
}

// ResourceTypes lists the permission catalog.
func (s *AuthorizationService) ResourceTypes(ctx context.Context) ([]authzmodel.ResourceType, error) {
	if _, err := s.require(ctx, "read", authzmodel.Resource{Type: authzmodel.RoleResource}); err != nil {
		return nil, err
	}
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]authzmodel.ResourceType, 0, len(types))
	for _, t := range types {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b authzmodel.ResourceType) int { return strings.Compare(a.Type, b.Type) })
	return out, nil
}

// validatePermissions checks patterns against the catalog and normalizes them.
func (s *AuthorizationService) validatePermissions(ctx context.Context, perms []string) ([]string, error) {
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range perms {
		p = strings.TrimSpace(p)
		t, a, ok := authzmodel.SplitPerm(p)
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
			return nil, apperrors.Validation(apperrors.FieldError{Field: "permissions", Code: "unknown_permission",
				Message: fmt.Sprintf("%q is not a known permission (format resource_type:action).", p)})
		}
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

func (s *AuthorizationService) ListRoles(ctx context.Context) ([]authzmodel.Role, error) {
	p, err := s.require(ctx, "read", authzmodel.Resource{Type: authzmodel.RoleResource})
	if err != nil {
		return nil, err
	}
	var rows []repository.RoleRow
	if err := s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListRoles")) }); err != nil {
		return nil, err
	}
	out := make([]authzmodel.Role, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, nil
}

func (s *AuthorizationService) GetRole(ctx context.Context, id uuid.UUID) (authzmodel.Role, error) {
	p, err := s.require(ctx, "read", authzmodel.Resource{Type: authzmodel.RoleResource, ID: id.String()})
	if err != nil {
		return authzmodel.Role{}, err
	}
	var row repository.RoleRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &row, queries.SQL.Get("GetRole"), id) })
	if db.IsNoRows(err) {
		return authzmodel.Role{}, authzmodel.ErrRoleNotFound
	}
	return row.ToModel(), err
}

func (s *AuthorizationService) CreateRole(ctx context.Context, in dto.RoleInput) (authzmodel.Role, error) {
	p, err := s.require(ctx, "create", authzmodel.Resource{Type: authzmodel.RoleResource})
	if err != nil {
		return authzmodel.Role{}, err
	}
	in.Key = strings.ToLower(strings.TrimSpace(in.Key))
	errs := dto.ValidateRoleText(&in)
	if !authzmodel.RoleKeyRe.MatchString(in.Key) {
		errs = append(errs, apperrors.FieldError{Field: "key", Code: "invalid_key", Message: "Use 2–40 lowercase letters, digits or underscores."})
	}
	if len(errs) > 0 {
		return authzmodel.Role{}, apperrors.Validation(errs...)
	}
	perms, err := s.validatePermissions(ctx, in.Permissions)
	if err != nil {
		return authzmodel.Role{}, err
	}
	grants, _, err := s.rbac.Grants(ctx, p)
	if err != nil {
		return authzmodel.Role{}, err
	}
	if ok, missing := authzmodel.CanGrant(grants, perms, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}); !ok {
		return authzmodel.Role{}, authzmodel.Escalation(missing)
	}
	var row repository.RoleRow
	id := uuid.Must(uuid.NewV7())
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertRole"), id, p.OrgID, in.Key, in.Name, in.Description); err != nil {
			if db.IsUniqueViolation(err) {
				return apperrors.NewProblem(http.StatusConflict, "key_taken", "Conflict", "A role with this key already exists.")
			}
			return err
		}
		for _, perm := range perms {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertRolePermission"), p.OrgID, id, perm); err != nil {
				return err
			}
		}
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetRole"), id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.created", authzmodel.RoleResource, id.String(), map[string]any{"key": in.Key, "permissions": perms})
	})
	return row.ToModel(), err
}

func (s *AuthorizationService) UpdateRole(ctx context.Context, id uuid.UUID, in dto.RoleInput) (authzmodel.Role, error) {
	p, err := s.require(ctx, "update", authzmodel.Resource{Type: authzmodel.RoleResource, ID: id.String()})
	if err != nil {
		return authzmodel.Role{}, err
	}
	if errs := dto.ValidateRoleText(&in); len(errs) > 0 {
		return authzmodel.Role{}, apperrors.Validation(errs...)
	}
	perms, err := s.validatePermissions(ctx, in.Permissions)
	if err != nil {
		return authzmodel.Role{}, err
	}
	grants, _, err := s.rbac.Grants(ctx, p)
	if err != nil {
		return authzmodel.Role{}, err
	}
	var row repository.RoleRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		var before repository.RoleRow
		if err := tx.GetContext(ctx, &before, queries.SQL.Get("GetRole"), id); err != nil {
			if db.IsNoRows(err) {
				return authzmodel.ErrRoleNotFound
			}
			return err
		}
		prev := before.ToModel()
		if prev.Immutable && !slices.Equal(prev.Permissions, perms) {
			return apperrors.NewProblem(http.StatusConflict, "role_immutable", "Conflict", "The permissions of this role cannot be changed.")
		}
		var added []string
		for _, perm := range perms {
			if !slices.Contains(prev.Permissions, perm) {
				added = append(added, perm)
			}
		}
		if ok, missing := authzmodel.CanGrant(grants, added, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}); !ok {
			return authzmodel.Escalation(missing)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateRole"), id, in.Name, in.Description); err != nil {
			return err
		}
		if !slices.Equal(prev.Permissions, perms) {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteRolePermissions"), id); err != nil {
				return err
			}
			for _, perm := range perms {
				if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertRolePermission"), p.OrgID, id, perm); err != nil {
					return err
				}
			}
			if err := BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetRole"), id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.updated", authzmodel.RoleResource, id.String(), map[string]any{
			"key": prev.Key, "from": map[string]any{"name": prev.Name, "permissions": prev.Permissions},
			"to": map[string]any{"name": in.Name, "permissions": perms}})
	})
	if err == nil {
		s.Invalidate(ctx, p.OrgID)
	}
	return row.ToModel(), err
}

func (s *AuthorizationService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", authzmodel.Resource{Type: authzmodel.RoleResource, ID: id.String()})
	if err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		var before repository.RoleRow
		if err := tx.GetContext(ctx, &before, queries.SQL.Get("GetRole"), id); err != nil {
			if db.IsNoRows(err) {
				return authzmodel.ErrRoleNotFound
			}
			return err
		}
		if before.IsSystem {
			return apperrors.NewProblem(http.StatusConflict, "role_system", "Conflict", "Built-in roles cannot be deleted. Edit their permissions instead.")
		}
		if before.BindingCount > 0 {
			return apperrors.NewProblem(http.StatusConflict, "role_in_use", "Conflict",
				fmt.Sprintf("This role is assigned %d time(s). Remove the assignments first.", before.BindingCount))
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteRole"), id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.deleted", authzmodel.RoleResource, id.String(), map[string]any{"key": before.Key})
	})
}

func (s *AuthorizationService) ListBindings(ctx context.Context, f dto.BindingFilter) ([]authzmodel.Binding, error) {
	p, err := s.require(ctx, "read", authzmodel.Resource{Type: authzmodel.RoleResource})
	if err != nil {
		return nil, err
	}
	var rows []repository.BindingRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListBindings"), authzmodel.NullStr(f.Principal.Type), authzmodel.NullUUID(f.Principal.ID),
			authzmodel.NullStr(f.ScopeType), authzmodel.NullUUID(f.WorkspaceID))
	})
	out := make([]authzmodel.Binding, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, err
}

func (s *AuthorizationService) validateScope(ctx context.Context, sc authzmodel.ScopeRef) error {
	switch sc.Type {
	case authzmodel.ScopeOrg:
		return nil
	case authzmodel.ScopeWorkspace:
		if sc.WorkspaceID == uuid.Nil {
			return apperrors.Validation(apperrors.FieldError{Field: "workspace_id", Code: "required", Message: "Choose a workspace."})
		}
		return nil
	case authzmodel.ScopeResource:
		types, err := s.rbac.ResourceTypes(ctx)
		if err != nil {
			return err
		}
		if _, ok := types[sc.ResourceType]; !ok || sc.ResourceID == "" || len(sc.ResourceID) > 200 {
			return apperrors.Validation(apperrors.FieldError{Field: "resource", Code: "invalid", Message: "Choose a known resource type and a resource id."})
		}
		return nil
	}
	return apperrors.Validation(apperrors.FieldError{Field: "scope_type", Code: "invalid", Message: "Scope must be org, workspace or resource."})
}

// CreateBinding assigns a role (permission role:bind at the target scope, and
// no escalation beyond the caller's own permissions there).
func (s *AuthorizationService) CreateBinding(ctx context.Context, in dto.BindingInput) (authzmodel.Binding, error) {
	if err := s.validateScope(ctx, in.Scope); err != nil {
		return authzmodel.Binding{}, err
	}
	p, err := s.require(ctx, "bind", authzmodel.ResourceForScope(in.Scope))
	if err != nil {
		return authzmodel.Binding{}, err
	}
	var out repository.BindingRow
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		id, err := s.BindInTx(ctx, tx, p, in)
		if err != nil {
			return err
		}
		if id == uuid.Nil {
			return apperrors.NewProblem(http.StatusConflict, "already_assigned", "Conflict", "This role is already assigned at that scope.")
		}
		return tx.GetContext(ctx, &out, queries.SQL.Get("GetBinding"), id)
	})
	if err != nil {
		return authzmodel.Binding{}, err
	}
	s.Invalidate(ctx, p.OrgID)
	return out.ToModel(), nil
}

// BindInTx creates a binding inside the caller's transaction after the
// no-escalation check, bumping the authz version and auditing. Other domains
// (workspace membership) call it after their own permission check; they must
// call Invalidate after commit. Returns uuid.Nil if already bound.
func (s *AuthorizationService) BindInTx(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, in dto.BindingInput) (uuid.UUID, error) {
	var role repository.RoleRow
	var err error
	if in.RoleID != uuid.Nil {
		err = tx.GetContext(ctx, &role, queries.SQL.Get("GetRole"), in.RoleID)
	} else {
		err = tx.GetContext(ctx, &role, queries.SQL.Get("GetRoleByKey"), in.RoleKey)
	}
	if db.IsNoRows(err) {
		return uuid.Nil, apperrors.Validation(apperrors.FieldError{Field: "role", Code: "not_found", Message: "Role not found."})
	}
	if err != nil {
		return uuid.Nil, err
	}
	model := role.ToModel()
	grants, _, err := s.rbac.Grants(ctx, caller)
	if err != nil {
		return uuid.Nil, err
	}
	if ok, missing := authzmodel.CanGrant(grants, model.Permissions, in.Scope); !ok {
		return uuid.Nil, authzmodel.Escalation(missing)
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return uuid.Nil, apperrors.Validation(apperrors.FieldError{Field: "expires_at", Code: "in_past", Message: "Expiry must be in the future."})
	}
	if in.Principal.ID == uuid.Nil {
		return uuid.Nil, apperrors.Validation(apperrors.FieldError{Field: "principal", Code: "required", Message: "Choose who gets the role."})
	}
	id, err := insertBinding(ctx, tx, caller.OrgID, in.Principal, role.ID, in.Scope, caller.ID, in.ExpiresAt)
	if db.IsForeignKeyViolation(err) {
		field, msg := "user_id", "This user is not a member of the organization (or the workspace does not exist)."
		switch in.Principal.Type {
		case authzmodel.PrincipalGroup:
			field, msg = "group_id", "This group does not exist (or the workspace does not exist)."
		case authzmodel.PrincipalServiceAccount:
			field, msg = "service_account_id", "This service account does not exist (or the workspace does not exist)."
		}
		return uuid.Nil, apperrors.Validation(apperrors.FieldError{Field: field, Code: "not_found", Message: msg})
	}
	if err != nil || id == uuid.Nil {
		return id, err
	}
	if err := BumpVersion(ctx, tx, caller.OrgID); err != nil {
		return uuid.Nil, err
	}
	return id, s.audit(ctx, tx, caller.OrgID, "role.assigned", authzmodel.RoleResource, role.ID.String(), map[string]any{
		"binding_id": id, "principal": authzmodel.PrincipalDetails(in.Principal), "role": model.Key, "scope": authzmodel.ScopeDetails(in.Scope)})
}

// DeleteBinding removes an assignment (role:bind at its scope). The last
// organization owner cannot be removed.
func (s *AuthorizationService) DeleteBinding(ctx context.Context, id uuid.UUID) error {
	p, ok := principal.From(ctx)
	if !ok {
		return authzmodel.Unauthenticated()
	}
	var b repository.BindingRow
	err := s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &b, queries.SQL.Get("GetBinding"), id) })
	if db.IsNoRows(err) {
		return apperrors.NotFound("Assignment not found.")
	}
	if err != nil {
		return err
	}
	model := b.ToModel()
	if _, err := s.require(ctx, "bind", authzmodel.ResourceForScope(model.Scope)); err != nil {
		return err
	}
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		if err := s.guardLastOwner(ctx, tx, model); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteBinding"), id); err != nil {
			return err
		}
		if err := BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.OrgID, "role.unassigned", authzmodel.RoleResource, model.RoleID.String(), map[string]any{
			"binding_id": id, "principal": authzmodel.PrincipalDetails(model.Principal), "role": model.RoleKey, "scope": authzmodel.ScopeDetails(model.Scope)})
	})
	if err == nil {
		s.Invalidate(ctx, p.OrgID)
	}
	return err
}

func (s *AuthorizationService) guardLastOwner(ctx context.Context, tx *sqlx.Tx, b authzmodel.Binding) error {
	if b.RoleKey != authzmodel.OwnerRoleKey || b.Scope.Type != authzmodel.ScopeOrg || b.Principal.Type != authzmodel.PrincipalUser {
		return nil
	}
	return ensureOwnerRemains(ctx, tx, b.ID, nil)
}

func ensureOwnerRemains(ctx context.Context, tx *sqlx.Tx, excludeBinding uuid.UUID, excludeUsers []uuid.UUID) error {
	if excludeUsers == nil {
		excludeUsers = []uuid.UUID{}
	}
	var others int64
	if err := tx.GetContext(ctx, &others, queries.SQL.Get("CountOtherOwners"), excludeBinding, excludeUsers); err != nil {
		return err
	}
	if others == 0 {
		return authzmodel.ErrLastOwner
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
func (s *AuthorizationService) CheckCanHandOut(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, target authzmodel.PrincipalRef) error {
	var rows []repository.GrantRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("PrincipalBindingGrants"), target.Type, target.ID); err != nil {
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
		g := r.ToGrant()
		if ok, m := authzmodel.CanGrant(grants, []string{g.Permission}, g.Scope); !ok {
			for _, p := range m {
				if !slices.Contains(missing, p) {
					missing = append(missing, p)
				}
			}
		}
	}
	if len(missing) > 0 {
		return authzmodel.Escalation(missing)
	}
	return nil
}

// RemoveWorkspaceAccessInTx removes all of a user's workspace-scope bindings
// in a workspace (caller already authorized). Returns how many were removed.
func (s *AuthorizationService) RemoveWorkspaceAccessInTx(ctx context.Context, tx *sqlx.Tx, org, workspace, user uuid.UUID) (int64, error) {
	res, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteWorkspaceBindings"), workspace, user)
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
	err := tx.GetContext(ctx, &ok, queries.SQL.Get("IsOrgOwner"), user)
	return ok, err
}

// MyPermissions returns the caller's effective permissions (for UI gating;
// the API still authorizes every request).
func (s *AuthorizationService) MyPermissions(ctx context.Context) (authzmodel.Effective, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return authzmodel.Effective{}, authzmodel.Unauthenticated()
	}
	grants, _, err := s.rbac.Grants(ctx, p)
	if err != nil {
		return authzmodel.Effective{}, err
	}
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return authzmodel.Effective{}, err
	}
	var orgPatterns []string
	wsPatterns := map[uuid.UUID][]string{}
	for _, g := range grants {
		switch g.Scope.Type {
		case authzmodel.ScopeOrg:
			orgPatterns = append(orgPatterns, g.Permission)
		case authzmodel.ScopeWorkspace:
			wsPatterns[g.Scope.WorkspaceID] = append(wsPatterns[g.Scope.WorkspaceID], g.Permission)
		}
	}
	out := authzmodel.Effective{Organization: authzmodel.Expand(types, orgPatterns), Workspaces: map[uuid.UUID][]string{}}
	for ws, pats := range wsPatterns {
		out.Workspaces[ws] = authzmodel.Expand(types, pats)
	}
	if out.Organization == nil {
		out.Organization = []string{}
	}
	return out, nil
}

// Simulate answers "could user U do action A on resource R?" without logging
// the answer as a real decision (permission authorization:simulate).
func (s *AuthorizationService) Simulate(ctx context.Context, who authzmodel.PrincipalRef, action string, res authzmodel.Resource) (authzmodel.AuthorizationDecision, error) {
	p, err := s.require(ctx, "simulate", authzmodel.Resource{Type: authzmodel.AuthorizationResource})
	if err != nil {
		return authzmodel.AuthorizationDecision{}, err
	}
	types, err := s.rbac.ResourceTypes(ctx)
	if err != nil {
		return authzmodel.AuthorizationDecision{}, err
	}
	if rt, ok := types[res.Type]; !ok || !slices.Contains(rt.Actions, action) {
		return authzmodel.AuthorizationDecision{}, apperrors.Validation(apperrors.FieldError{Field: "action", Code: "unknown_permission",
			Message: fmt.Sprintf("%s:%s is not a known permission.", res.Type, action)})
	}
	res.OrgID = p.OrgID
	kind := principal.User
	if who.Type == authzmodel.PrincipalServiceAccount {
		kind = principal.ServiceAccount
	}
	return s.rbac.Check(ctx, dto.AuthorizationInput{Principal: principal.Principal{ID: who.ID, Type: kind, OrgID: p.OrgID},
		Action: action, Resource: res})
}

// ListDecisions pages through the decision log (authorization:read_decisions).
func (s *AuthorizationService) ListDecisions(ctx context.Context, allowed *bool, page pagination.Page) ([]authzmodel.Decision, string, error) {
	p, err := s.require(ctx, "read_decisions", authzmodel.Resource{Type: authzmodel.AuthorizationResource})
	if err != nil {
		return nil, "", err
	}
	limit := min(max(page.Limit, 1), pagination.MaxPageLimit)
	var curT, curID any
	if page.Cursor != "" {
		var c authzmodel.DecisionCursor
		if err := pagination.DecodeCursor(page.Cursor, &c); err != nil {
			return nil, "", err
		}
		curT, curID = c.T, c.ID
	}
	var allowedArg any
	if allowed != nil {
		allowedArg = *allowed
	}
	var rows []authzmodel.Decision
	err = s.tx(ctx, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDecisions"), allowedArg, curT, curID, limit+1)
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next, _ = pagination.EncodeCursor(authzmodel.DecisionCursor{T: last.OccurredAt, ID: last.ID})
	}
	for i := range rows {
		_ = json.Unmarshal(rows[i].MatchedRaw, &rows[i].MatchedRules)
	}
	return rows, next, nil
}

// GrantCreatorInTx makes the creator of a workspace (a user or a service
// account) its workspace_admin (creation itself was authorized by workspace:create).
func GrantCreatorInTx(ctx context.Context, tx *sqlx.Tx, creator principal.Principal, workspace uuid.UUID) error {
	if err := insertBindingByKey(ctx, tx, creator.OrgID, authzmodel.RefOf(creator), "workspace_admin",
		authzmodel.ScopeRef{Type: authzmodel.ScopeWorkspace, WorkspaceID: workspace}, creator.ID); err != nil {
		return err
	}
	return BumpVersion(ctx, tx, creator.OrgID)
}

// CheckGrantRole loads role id and verifies the caller could grant it at
// scope (no escalation). Used to validate grants that are applied later, such
// as the roles an invitation hands out when it is accepted.
func (s *AuthorizationService) CheckGrantRole(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, roleID uuid.UUID, scope authzmodel.ScopeRef) (authzmodel.Role, error) {
	var row repository.RoleRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetRole"), roleID); err != nil {
		if db.IsNoRows(err) {
			return authzmodel.Role{}, apperrors.Validation(apperrors.FieldError{Field: "role_id", Code: "not_found", Message: "Role not found."})
		}
		return authzmodel.Role{}, err
	}
	role := row.ToModel()
	grants, _, err := s.rbac.Grants(ctx, caller)
	if err != nil {
		return authzmodel.Role{}, err
	}
	if ok, missing := authzmodel.CanGrant(grants, role.Permissions, scope); !ok {
		return authzmodel.Role{}, authzmodel.Escalation(missing)
	}
	return role, nil
}

// BindPreauthorizedInTx inserts a binding whose grant was authorized earlier
// (e.g. by CheckGrantRole when an invitation was created). by is recorded as
// the creator. Idempotent. The caller bumps the version and invalidates.
func BindPreauthorizedInTx(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who authzmodel.PrincipalRef, roleID uuid.UUID, scope authzmodel.ScopeRef, by uuid.UUID) error {
	_, err := insertBinding(ctx, tx, org, who, roleID, scope, by, nil)
	return err
}

// BindRoleKeyInTx is BindPreauthorizedInTx for a role identified by key
// (e.g. the baseline "member" role every member holds).
func BindRoleKeyInTx(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, who authzmodel.PrincipalRef, roleKey string, scope authzmodel.ScopeRef, by uuid.UUID) error {
	return insertBindingByKey(ctx, tx, org, who, roleKey, scope, by)
}
