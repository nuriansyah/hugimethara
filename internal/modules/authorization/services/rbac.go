package services

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// MaxStaleness bounds how long cached grants are trusted when the version
// stamp cannot be confirmed (e.g. Redis unavailable). "Never rely on stale
// authorization cache indefinitely" (PRD §13).
const MaxStaleness = 30 * time.Second

// RBAC is the dynamic role-based Authorizer and ScopeResolver (ADR-0007).
type RBAC struct {
	db    *sqlx.DB
	rdb   *redis.Client // optional
	log   *slog.Logger
	now   func() time.Time
	mu    sync.Mutex
	cache map[cacheKey]cacheEntry

	typesMu sync.Mutex
	types   map[string]authzmodel.ResourceType
}

type cacheEntry struct {
	version  int64
	grants   []authzmodel.Grant
	loadedAt time.Time
}

const maxCacheEntries = 10_000

// NewRBAC builds the evaluator. rdb may be nil (version read from Postgres).
func NewRBAC(d *sqlx.DB, rdb *redis.Client, log *slog.Logger) *RBAC {
	return &RBAC{db: d, rdb: rdb, log: log, now: time.Now, cache: map[cacheKey]cacheEntry{}}
}

var _ Authorizer = (*RBAC)(nil)

var _ ScopeResolver = (*RBAC)(nil)

func versionKey(org uuid.UUID) string { return "authz:ver:" + org.String() }

// version returns the org's authz version, preferring Redis.
func (r *RBAC) version(ctx context.Context, org uuid.UUID) (int64, error) {
	if r.rdb != nil {
		if s, err := r.rdb.Get(ctx, versionKey(org)).Result(); err == nil {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				return v, nil
			}
		}
	}
	var v int64
	err := db.InTenantTx(ctx, r.db, org, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &v, queries.SQL.Get("GetVersion"), org)
	})
	if err != nil {
		return 0, fmt.Errorf("authz: read version: %w", err)
	}
	if r.rdb != nil {
		_ = r.rdb.Set(ctx, versionKey(org), v, MaxStaleness).Err()
	}
	return v, nil
}

// Grants returns the principal's grants and the version they were loaded at.
func (r *RBAC) Grants(ctx context.Context, p principal.Principal) ([]authzmodel.Grant, int64, error) {
	ver, err := r.version(ctx, p.OrgID)
	if err != nil {
		return nil, 0, err
	}
	key := cacheKey{p.OrgID, p.ID, p.Type}
	r.mu.Lock()
	e, ok := r.cache[key]
	r.mu.Unlock()
	if ok && e.version == ver && r.now().Sub(e.loadedAt) < MaxStaleness {
		return e.grants, ver, nil
	}

	query := "LoadUserGrants"
	switch p.Type {
	case principal.User, "":
	case principal.ServiceAccount:
		query = "LoadServiceAccountGrants"
	default:
		return nil, ver, nil // unknown principal kinds hold nothing
	}
	var rows []repository.GrantRow
	err = db.InTenantTx(ctx, r.db, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get(query), p.ID)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("authz: load grants: %w", err)
	}
	grants := make([]authzmodel.Grant, len(rows))
	for i, row := range rows {
		grants[i] = row.ToGrant()
	}
	r.mu.Lock()
	if len(r.cache) >= maxCacheEntries {
		clear(r.cache) // simple bound; entries are cheap to rebuild
	}
	r.cache[key] = cacheEntry{version: ver, grants: grants, loadedAt: r.now()}
	r.mu.Unlock()
	return grants, ver, nil
}

// Invalidate drops cached grants for org and publishes its current version.
// Call after committing any change to roles, bindings or memberships.
func (r *RBAC) Invalidate(ctx context.Context, org uuid.UUID) {
	r.mu.Lock()
	for k := range r.cache {
		if k.org == org {
			delete(r.cache, k)
		}
	}
	r.mu.Unlock()
	if r.rdb != nil {
		if err := r.rdb.Del(ctx, versionKey(org)).Err(); err != nil {
			r.log.WarnContext(ctx, "authz: version invalidation failed; staleness bounded by MaxStaleness", "err", err)
		}
	}
}

// BumpVersion increments the org's authz version inside tx.
func BumpVersion(ctx context.Context, tx *sqlx.Tx, org uuid.UUID) error {
	var v int64
	return tx.GetContext(ctx, &v, commands.SQL.Get("BumpVersion"), org)
}

// Evaluate implements Authorizer.
func (r *RBAC) Evaluate(ctx context.Context, in dto.AuthorizationInput) (authzmodel.AuthorizationDecision, error) {
	return r.evaluate(ctx, in, true)
}

// Check evaluates without writing to the decision log (used for simulation).
func (r *RBAC) Check(ctx context.Context, in dto.AuthorizationInput) (authzmodel.AuthorizationDecision, error) {
	return r.evaluate(ctx, in, false)
}

func (r *RBAC) evaluate(ctx context.Context, in dto.AuthorizationInput, audit bool) (authzmodel.AuthorizationDecision, error) {
	p := in.Principal
	if p.OrgID == uuid.Nil || p.ID == uuid.Nil {
		return authzmodel.AuthorizationDecision{Reason: authzmodel.ReasonNoPrincipal}, nil
	}
	if p.OrgID != in.Resource.OrgID {
		d := authzmodel.AuthorizationDecision{Reason: authzmodel.ReasonTenantMismatch}
		if audit {
			r.record(ctx, in, d)
		}
		return d, nil
	}
	grants, ver, err := r.Grants(ctx, p)
	if err != nil {
		return authzmodel.AuthorizationDecision{}, err
	}
	d := authzmodel.AuthorizationDecision{PolicyVersion: ver, Reason: "no_matching_binding"}
	if m := authzmodel.Matching(grants, in.Action, in.Resource); len(m) > 0 {
		d.Allowed, d.Reason = true, "granted_by_binding"
		for _, g := range m {
			rule := g.RoleKey + "@" + g.Scope.Type + "#" + g.BindingID.String()
			if g.Via == authzmodel.PrincipalGroup {
				rule += " (via group)"
			}
			d.MatchedRules = append(d.MatchedRules, rule)
		}
	}
	if audit && (!d.Allowed || r.sensitive(ctx, in.Resource.Type, in.Action)) {
		r.record(ctx, in, d)
	}
	return d, nil
}

// WorkspaceScope implements ScopeResolver.
func (r *RBAC) WorkspaceScope(ctx context.Context, p principal.Principal, action string) (authzmodel.Scope, error) {
	grants, _, err := r.Grants(ctx, p)
	if err != nil {
		return authzmodel.Scope{}, err
	}
	perm := authzmodel.Permission("workspace", action)
	var s authzmodel.Scope
	for _, g := range grants {
		if !authzmodel.Covers(g.Permission, perm) {
			continue
		}
		switch g.Scope.Type {
		case authzmodel.ScopeOrg:
			return authzmodel.Scope{All: true}, nil
		case authzmodel.ScopeWorkspace:
			if !slices.Contains(s.WorkspaceIDs, g.Scope.WorkspaceID) {
				s.WorkspaceIDs = append(s.WorkspaceIDs, g.Scope.WorkspaceID)
			}
		}
	}
	return s, nil
}

// ResourceTypes returns the registered catalog (cached; it only changes with migrations).
func (r *RBAC) ResourceTypes(ctx context.Context) (map[string]authzmodel.ResourceType, error) {
	r.typesMu.Lock()
	defer r.typesMu.Unlock()
	if r.types != nil {
		return r.types, nil
	}
	var rows []repository.ResourceTypeRow
	if err := r.db.SelectContext(ctx, &rows, queries.SQL.Get("ListResourceTypes")); err != nil {
		return nil, err // not cached: the next call retries
	}
	types := map[string]authzmodel.ResourceType{}
	for _, row := range rows {
		rt := row.ToModel()
		types[rt.Type] = rt
	}
	r.types = types
	return types, nil
}

func (r *RBAC) sensitive(ctx context.Context, resourceType, action string) bool {
	types, err := r.ResourceTypes(ctx)
	if err != nil {
		return true // fail towards auditing
	}
	return slices.Contains(types[resourceType].SensitiveActions, action)
}

// record writes the decision to authz_decisions. Failures are logged, never
// fatal: the decision itself has already been made.
func (r *RBAC) record(ctx context.Context, in dto.AuthorizationInput, d authzmodel.AuthorizationDecision) {
	var ws any
	if in.Resource.WorkspaceID != nil {
		ws = *in.Resource.WorkspaceID
	}
	var resID, reqID any
	if in.Resource.ID != "" {
		resID = in.Resource.ID
	}
	if id := logging.RequestID(ctx); id != "" {
		reqID = id
	}
	matched := d.MatchedRules
	if matched == nil {
		matched = []string{}
	}
	ptype := string(in.Principal.Type)
	if ptype == "" {
		ptype = authzmodel.PrincipalUser
	}
	wctx := context.WithoutCancel(ctx)
	err := db.InTenantTx(wctx, r.db, in.Principal.OrgID, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(wctx, commands.SQL.Get("InsertDecision"), in.Principal.OrgID, in.Principal.ID, in.Action,
			in.Resource.Type, resID, ws, d.Allowed, d.Reason, matched, d.PolicyVersion, reqID, ptype)
		return err
	})
	if err != nil {
		r.log.ErrorContext(wctx, "authz: decision audit failed", "err", err)
	}
}

// cacheKey identifies one principal's cached grant set.
type cacheKey struct {
	org, id uuid.UUID
	kind    principal.Type
}
