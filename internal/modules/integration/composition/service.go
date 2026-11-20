// Package composition implements versioned, bounded API composition DAGs.
package composition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cel.dev/cel-go/cel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping"
	servicereg "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

const (
	Resource          = "api_composition"
	maxAggregateBytes = 5 << 20
	defaultDeadline   = 30 * time.Second
	defaultStepLimit  = 20 * time.Second
)

var errCompositionNotFound = httpx.NotFound("API composition not found.")
var errExecutionNotFound = httpx.NotFound("API composition execution not found.")

type endpointInvoker interface {
	TryEndpoint(context.Context, uuid.UUID, servicereg.ServiceTryInput) (servicereg.ServiceTryResult, error)
}

type compositionRow struct {
	ID          uuid.UUID       `db:"id"`
	WorkspaceID uuid.UUID       `db:"workspace_id"`
	Name        string          `db:"name"`
	Slug        string          `db:"slug"`
	Draft       json.RawMessage `db:"draft"`
	Exposed     bool            `db:"exposed"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

type versionRow struct {
	ID            uuid.UUID       `db:"id"`
	CompositionID uuid.UUID       `db:"composition_id"`
	Version       int             `db:"version"`
	Definition    json.RawMessage `db:"definition"`
	Checksum      string          `db:"checksum"`
	PublishedBy   uuid.UUID       `db:"published_by"`
	PublishedAt   time.Time       `db:"published_at"`
}

type executionRow struct {
	ID            uuid.UUID       `db:"id"`
	CompositionID uuid.UUID       `db:"composition_id"`
	VersionID     uuid.UUID       `db:"version_id"`
	EnvironmentID uuid.UUID       `db:"environment_id"`
	StartedAt     time.Time       `db:"started_at"`
	DurationMs    int             `db:"duration_ms"`
	Status        string          `db:"status"`
	StepSummary   json.RawMessage `db:"step_summary"`
	Output        json.RawMessage `db:"output"`
	Error         *string         `db:"error"`
	RequestID     string          `db:"request_id"`
	WorkspaceID   uuid.UUID       `db:"workspace_id"`
}

type breakerState struct {
	failures  int
	openUntil time.Time
	updatedAt time.Time
}

type endpointResponseError struct {
	status int
}

func (e endpointResponseError) Error() string { return "endpoint returned an unsuccessful status" }

type Service struct {
	pool     *sqlx.DB
	az       authz.Authorizer
	invoker  endpointInvoker
	breakers sync.Mutex
	breaker  map[string]breakerState
}

func NewService(pool *sqlx.DB, az authz.Authorizer, invoker endpointInvoker) *Service {
	return &Service{pool: pool, az: az, invoker: invoker, breaker: make(map[string]breakerState)}
}

func (s *Service) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authz.Unauthenticated()
	}
	return p, nil
}

func (s *Service) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *Service) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	_, err := authz.Require(ctx, s.az, action, authz.Resource{
		Type: Resource, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func (r compositionRow) model() (Composition, error) {
	var draft map[string]any
	if err := json.Unmarshal(r.Draft, &draft); err != nil {
		return Composition{}, fmt.Errorf("composition: decode draft: %w", err)
	}
	if draft == nil {
		return Composition{}, errors.New("composition: stored draft is not a JSON object")
	}
	return Composition{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		Draft: draft, Exposed: r.Exposed, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}, nil
}

func (r versionRow) model() (Version, error) {
	var definition map[string]any
	if err := json.Unmarshal(r.Definition, &definition); err != nil {
		return Version{}, fmt.Errorf("composition: decode version: %w", err)
	}
	if definition == nil {
		return Version{}, errors.New("composition: stored version is not a JSON object")
	}
	return Version{ID: r.ID, CompositionID: r.CompositionID, Version: r.Version,
		Definition: definition, Checksum: r.Checksum, PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}, nil
}

func (r executionRow) model() (Execution, error) {
	output, err := rawJSONValue(r.Output)
	if err != nil {
		return Execution{}, err
	}
	return Execution{ID: r.ID, CompositionID: r.CompositionID, VersionID: r.VersionID,
		EnvironmentID: r.EnvironmentID, StartedAt: r.StartedAt, DurationMs: r.DurationMs,
		Status: r.Status, StepSummary: r.StepSummary, Output: output,
		Error: r.Error, RequestID: r.RequestID}, nil
}

func rawJSONValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("composition: decode JSON value: %w", err)
	}
	return value, nil
}

func (s *Service) load(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*compositionRow, error) {
	var row compositionRow
	if err := tx.GetContext(ctx, &row, q.Get("GetComposition"), id); err != nil {
		if db.IsNoRows(err) {
			return nil, errCompositionNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []compositionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListCompositions"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]Composition, len(rows))
	for i := range rows {
		model, err := rows[i].model()
		if err != nil {
			return nil, err
		}
		out[i] = model
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Composition{}, err
	}
	var out Composition
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out, err = row.model()
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return Composition{}, err
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Composition{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return Composition{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	if in.Name == "" || len(in.Name) > 100 {
		return Composition{}, validation("name", "invalid", "A composition name between 1 and 100 characters is required.")
	}
	if !compositionSlug.MatchString(in.Slug) {
		return Composition{}, validation("slug", "invalid", "Use 1–63 lowercase letters, digits, or hyphens; begin with a letter or digit.")
	}
	if in.Draft == nil {
		in.Draft = map[string]any{"steps": []any{}}
	}
	if err := ValidateDraft(in.Draft); err != nil {
		return Composition{}, validation("draft", "invalid", err.Error())
	}
	id := uuid.New()
	raw, err := json.Marshal(in.Draft)
	if err != nil {
		return Composition{}, validation("draft", "invalid", "The composition draft must be valid JSON.")
	}
	var out Composition
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("InsertComposition"), p.OrgID, id, in.WorkspaceID, in.Name, in.Slug, raw); err != nil {
			return err
		}
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = row.model()
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "api_composition.created",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"name": in.Name, "slug": in.Slug}})
	}); err != nil {
		return Composition{}, err
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Composition{}, err
	}
	var out Composition
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" || len(name) > 100 {
				return validation("name", "invalid", "A composition name between 1 and 100 characters is required.")
			}
			in.Name = &name
		}
		if in.Slug != nil {
			slug := strings.TrimSpace(strings.ToLower(*in.Slug))
			if !compositionSlug.MatchString(slug) {
				return validation("slug", "invalid", "Use 1–63 lowercase letters, digits, or hyphens; begin with a letter or digit.")
			}
			in.Slug = &slug
		}
		var draft any
		if in.Draft != nil {
			if err := ValidateDraft(*in.Draft); err != nil {
				return validation("draft", "invalid", err.Error())
			}
			raw, err := json.Marshal(*in.Draft)
			if err != nil {
				return validation("draft", "invalid", "The composition draft must be valid JSON.")
			}
			draft = json.RawMessage(raw)
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateComposition"), id, in.Name, in.Slug, draft, in.Exposed); err != nil {
			return err
		}
		row, err = s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = row.model()
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "api_composition.updated",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return Composition{}, err
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "delete", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteComposition"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "api_composition.deleted",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *Service) validateReferences(ctx context.Context, p principal.Principal, tx *sqlx.Tx, row *compositionRow, draft map[string]any) error {
	if err := ValidateDraft(draft); err != nil {
		return validation("draft", "invalid", err.Error())
	}
	var def definition
	raw, _ := json.Marshal(draft)
	if err := json.Unmarshal(raw, &def); err != nil {
		return validation("draft", "invalid", "The composition draft has an invalid shape.")
	}
	if len(def.Steps) == 0 {
		return validation("draft", "empty", "Add at least one API call step before validating or publishing.")
	}
	for _, current := range def.Steps {
		var endpointWorkspace uuid.UUID
		err := tx.GetContext(ctx, &endpointWorkspace, `
			SELECT s.workspace_id
			FROM services s JOIN service_endpoints e ON e.service_id = s.id
			WHERE s.id = $1 AND e.id = $2`, current.Call.ServiceID, current.Call.EndpointID)
		if err != nil {
			if db.IsNoRows(err) {
				return validation("draft", "reference_not_found", "A composition step references a service endpoint that no longer exists.")
			}
			return err
		}
		if endpointWorkspace != row.WorkspaceID {
			return validation("draft", "workspace_mismatch", "Composition endpoints must belong to the same workspace.")
		}
		if _, err := authz.Require(ctx, s.az, "read", authz.Resource{Type: "service", ID: current.Call.ServiceID,
			OrgID: p.OrgID, WorkspaceID: &endpointWorkspace}); err != nil {
			return err
		}
	}
	if def.Response.MappingID != "" {
		var mappingWorkspace uuid.UUID
		err := tx.GetContext(ctx, &mappingWorkspace, `
			SELECT m.workspace_id
			FROM mappings m JOIN mapping_versions v ON v.mapping_id = m.id
			WHERE m.id = $1 AND v.version = $2`, def.Response.MappingID, def.Response.MappingVersion)
		if err != nil {
			if db.IsNoRows(err) {
				return validation("draft", "mapping_not_found", "The response mapping version does not exist.")
			}
			return err
		}
		if mappingWorkspace != row.WorkspaceID {
			return validation("draft", "workspace_mismatch", "The response mapping must belong to the same workspace.")
		}
		if _, err := authz.Require(ctx, s.az, "read", authz.Resource{Type: "mapping",
			ID: def.Response.MappingID, OrgID: p.OrgID, WorkspaceID: &mappingWorkspace}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Validate(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		model, err := row.model()
		if err != nil {
			return err
		}
		return s.validateReferences(ctx, p, tx, row, model.Draft)
	})
}

func (s *Service) ListVersions(ctx context.Context, id uuid.UUID) ([]Version, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []Version
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []versionRow
		if err := tx.SelectContext(ctx, &rows, q.Get("ListCompositionVersions"), id); err != nil {
			return err
		}
		out = make([]Version, len(rows))
		for i := range rows {
			out[i], err = rows[i].model()
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Publish(ctx context.Context, id uuid.UUID) (Version, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Version{}, err
	}
	var out Version
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "publish", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		composition, err := row.model()
		if err != nil {
			return err
		}
		definition := composition.Draft
		if err := s.validateReferences(ctx, p, tx, row, definition); err != nil {
			return err
		}
		raw, err := json.Marshal(definition)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		checksum := hex.EncodeToString(sum[:])
		var next int
		if err := tx.GetContext(ctx, &next, q.Get("NextCompositionVersion"), id); err != nil {
			return err
		}
		versionID := uuid.New()
		if _, err := tx.ExecContext(ctx, q.Get("InsertCompositionVersion"), p.OrgID, versionID, id,
			next, json.RawMessage(raw), checksum, p.ID); err != nil {
			return err
		}
		var version versionRow
		if err := tx.GetContext(ctx, &version, q.Get("GetCompositionVersion"), id, next); err != nil {
			return err
		}
		out, err = version.model()
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "api_composition.published",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"version": next, "checksum": checksum}})
	}); err != nil {
		return Version{}, err
	}
	return out, nil
}

func (s *Service) Execute(ctx context.Context, id uuid.UUID, in ExecuteInput) (Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Execution{}, err
	}
	var row compositionRow
	var version versionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		rowPtr, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		row = *rowPtr
		if err := s.authorize(ctx, p, "execute", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if in.EnvironmentID == uuid.Nil {
			return validation("environment_id", "required", "An execution environment is required.")
		}
		if err := tx.GetContext(ctx, new(uuid.UUID),
			`SELECT id FROM environments WHERE id = $1 AND org_id = $2`, in.EnvironmentID, p.OrgID); err != nil {
			if db.IsNoRows(err) {
				return validation("environment_id", "not_found", "The execution environment does not exist.")
			}
			return err
		}
		if err := tx.GetContext(ctx, &version, `
			SELECT id, composition_id, version, definition, checksum, published_by, published_at
			FROM api_composition_versions WHERE composition_id = $1
			ORDER BY version DESC LIMIT 1`, id); err != nil {
			if db.IsNoRows(err) {
				return validation("composition_id", "unpublished", "Publish the composition before executing it.")
			}
			return err
		}
		return nil
	}); err != nil {
		return Execution{}, err
	}

	now := time.Now().UTC()
	result, stepResults, runErr := s.run(ctx, p.OrgID, version, in)
	executionID := uuid.New()
	status := "succeeded"
	var errorText *string
	if runErr != nil {
		status = "failed"
		message := runErr.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		errorText = &message
	}
	summary, err := json.Marshal(stepResults)
	if err != nil {
		return Execution{}, fmt.Errorf("composition: encode step summary: %w", err)
	}
	if len(summary) > maxAggregateBytes {
		status = "failed"
		errorText = ptrString("Composition step output exceeds the 5 MiB aggregate limit.")
		summary = json.RawMessage(`{}`)
		result = nil
	}
	var output []byte
	if result != nil {
		output, err = json.Marshal(result)
		if err != nil {
			return Execution{}, fmt.Errorf("composition: encode response: %w", err)
		}
		if len(output)+len(summary) > maxAggregateBytes {
			status = "failed"
			errorText = ptrString("Composition output exceeds the 5 MiB aggregate limit.")
			output = nil
			result = nil
		}
	}
	if status == "failed" {
		result = nil
	}
	duration := int(time.Since(now).Milliseconds())
	requestID := in.RequestID
	if len(requestID) > 200 {
		requestID = requestID[:200]
	}
	out := Execution{ID: executionID, CompositionID: id, VersionID: version.ID,
		EnvironmentID: in.EnvironmentID, StartedAt: now, DurationMs: duration,
		Status: status, StepSummary: summary, Output: result, Error: errorText, RequestID: requestID}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.tenantTx(persistCtx, p, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(persistCtx, q.Get("InsertCompositionExecution"),
			p.OrgID, executionID, row.WorkspaceID, id, version.ID, in.EnvironmentID,
			now, duration, status, json.RawMessage(summary), nullableJSON(output), errorText, requestID)
		if err != nil {
			return err
		}
		return audit.Record(persistCtx, tx, audit.Event{OrgID: p.OrgID, Action: "api_composition.executed",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"execution_id": executionID.String(), "version": version.Version,
				"status": status, "duration_ms": duration}})
	}); err != nil {
		return Execution{}, err
	}
	return out, nil
}

func (s *Service) run(ctx context.Context, orgID uuid.UUID, version versionRow, in ExecuteInput) (any, map[string]StepResult, error) {
	var def definition
	if err := json.Unmarshal(version.Definition, &def); err != nil {
		return nil, map[string]StepResult{}, fmt.Errorf("published composition version is invalid")
	}
	deadline := defaultDeadline
	if def.Limits.TimeoutMs > 0 {
		deadline = time.Duration(def.Limits.TimeoutMs) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	request := in.Request
	if request == nil {
		request = map[string]any{}
	}
	results := make(map[string]StepResult, len(def.Steps))
	remaining := make(map[string]step, len(def.Steps))
	for _, current := range def.Steps {
		remaining[current.ID] = current
	}
	concurrency := def.Limits.MaxConcurrency
	if concurrency < 1 {
		concurrency = maxConcurrency
	}
	for len(remaining) > 0 {
		if err := runCtx.Err(); err != nil {
			return nil, results, errors.New("composition execution deadline exceeded")
		}
		ready := make([]step, 0)
		for _, current := range def.Steps {
			if _, pending := remaining[current.ID]; !pending {
				continue
			}
			depsDone := true
			for _, dep := range current.DependsOn {
				if _, exists := results[dep]; !exists {
					depsDone = false
					break
				}
			}
			if depsDone {
				ready = append(ready, current)
			}
		}
		if len(ready) == 0 {
			return nil, results, errors.New("composition execution could not resolve the dependency graph")
		}
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrency)
		runErrors := make(map[string]error)
		stepResults := make(map[string]StepResult, len(ready))
		for _, current := range ready {
			current := current
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				got, err := s.executeStep(runCtx, orgID, in.EnvironmentID, current, request, results)
				mu.Lock()
				stepResults[current.ID] = got
				if err != nil {
					runErrors[current.ID] = err
				}
				mu.Unlock()
			}()
		}
		wg.Wait()
		for _, current := range ready {
			got := stepResults[current.ID]
			if err := runErrors[current.ID]; err != nil {
				switch current.OnError {
				case "null":
					got = StepResult{Status: got.Status, Body: nil, Attempts: got.Attempts, Error: err.Error()}
				case "default":
					got = StepResult{Status: got.Status, Body: current.Default, Attempts: got.Attempts, Error: err.Error()}
				case "fallback_step":
					fallback, ok := results[current.FallbackStep]
					if !ok {
						return nil, results, fmt.Errorf("step %q failed and its fallback result is unavailable", current.ID)
					}
					got = StepResult{Status: got.Status, Body: fallback.Body, Attempts: got.Attempts, Error: err.Error()}
				default:
					got.Error = err.Error()
					results[current.ID] = got
					return nil, results, fmt.Errorf("step %q failed: %s", current.ID, err.Error())
				}
			}
			results[current.ID] = got
			delete(remaining, current.ID)
		}
		aggregate, err := json.Marshal(results)
		if err != nil {
			return nil, results, fmt.Errorf("composition step output is not valid JSON")
		}
		if len(aggregate) > maxAggregateBytes {
			return nil, results, errors.New("composition step output exceeds the 5 MiB aggregate limit")
		}
	}
	source := map[string]any{"request": request, "steps": results}
	if def.Response.MappingID != "" {
		mapped, err := s.applyResponseMapping(runCtx, orgID, def.Response.MappingID, def.Response.MappingVersion, source)
		if err != nil {
			return nil, results, fmt.Errorf("response mapping failed")
		}
		return mapped, results, nil
	}
	return source, results, nil
}

func (s *Service) executeStep(ctx context.Context, orgID, environmentID uuid.UUID, current step,
	request map[string]any, prior map[string]StepResult) (StepResult, error) {
	for _, dep := range current.DependsOn {
		if prior[dep].Skipped {
			return StepResult{Skipped: true}, nil
		}
	}
	stepData := make(map[string]any, len(prior))
	for id, result := range prior {
		stepData[id] = map[string]any{"status": result.Status, "body": result.Body,
			"duration_ms": result.DurationMs, "skipped": result.Skipped, "error": result.Error}
	}
	if current.When != "" {
		ok, err := evalCondition(current.When, map[string]any{"steps": stepData, "request": request})
		if err != nil {
			return StepResult{}, fmt.Errorf("when expression could not be evaluated")
		}
		if !ok {
			return StepResult{Skipped: true}, nil
		}
	}
	values := map[string]any{"request": request, "steps": stepData}
	resolved, err := resolveMap(current.Call.Input, values)
	if err != nil {
		return StepResult{}, errors.New("step input reference is invalid")
	}
	pathValues := stringMap(resolved["path_values"])
	query := stringMap(resolved["query"])
	headers := stringMap(resolved["headers"])
	call := servicereg.ServiceTryInput{EnvironmentID: environmentID, PathValues: pathValues,
		Query: query, Headers: headers, Body: resolved["body"]}
	endpointID, _ := uuid.Parse(current.Call.EndpointID)
	key := orgID.String() + ":" + current.Call.ServiceID + ":" + environmentID.String()
	attempts := current.Retries + 1
	var result servicereg.ServiceTryResult
	var lastErr error
	start := time.Now()
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return StepResult{Attempts: attempt - 1}, errors.New("step deadline exceeded")
		}
		if !s.breakerAllow(key, time.Now()) {
			return StepResult{Attempts: attempt - 1}, errors.New("service circuit breaker is open")
		}
		stepLimit := defaultStepLimit
		if current.TimeoutMs > 0 {
			stepLimit = time.Duration(current.TimeoutMs) * time.Millisecond
		}
		stepCtx, cancel := context.WithTimeout(ctx, stepLimit)
		result, lastErr = s.invoker.TryEndpoint(stepCtx, endpointID, call)
		cancel()
		if lastErr == nil && (result.Status < http.StatusOK || result.Status >= http.StatusMultipleChoices) {
			lastErr = endpointResponseError{status: result.Status}
		}
		if lastErr == nil {
			s.breakerSuccess(key)
			return StepResult{Status: result.Status, Body: result.Body, DurationMs: result.DurationMs,
				Attempts: attempt}, nil
		}
		if statusErr, ok := lastErr.(endpointResponseError); ok && statusErr.status < http.StatusInternalServerError {
			return StepResult{Status: result.Status, Body: result.Body,
				DurationMs: result.DurationMs, Attempts: attempt}, lastErr
		}
		s.breakerFailure(key, time.Now())
		if attempt < attempts {
			delay := time.Duration(1<<(attempt-1)) * 100 * time.Millisecond
			if delay > time.Second {
				delay = time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return StepResult{Attempts: attempt}, errors.New("step deadline exceeded")
			case <-timer.C:
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("endpoint request failed")
	}
	return StepResult{Status: result.Status, Body: result.Body,
		DurationMs: int(time.Since(start).Milliseconds()), Attempts: attempts}, errors.New("endpoint request failed")
}

func (s *Service) applyResponseMapping(ctx context.Context, orgID uuid.UUID, mappingID string, version int, source any) (any, error) {
	id, err := uuid.Parse(mappingID)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	err = db.InTenantTx(ctx, s.pool, orgID, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &raw, `
			SELECT v.definition FROM mapping_versions v
			JOIN mappings m ON m.org_id = v.org_id AND m.id = v.mapping_id
			WHERE m.id = $1 AND v.version = $2`, id, version)
	})
	if err != nil {
		return nil, err
	}
	var draft map[string]any
	if err := json.Unmarshal(raw, &draft); err != nil {
		return nil, err
	}
	return mapping.Apply(draft, source)
}

func evalCondition(expression string, values map[string]any) (bool, error) {
	ast, err := compileCondition(expression)
	if err != nil {
		return false, err
	}
	env, err := cel.NewEnv(cel.Variable("steps", cel.DynType), cel.Variable("request", cel.DynType))
	if err != nil {
		return false, err
	}
	program, err := env.Program(ast, cel.CostTracking(nil), cel.CostLimit(10000))
	if err != nil {
		return false, err
	}
	value, _, err := program.Eval(values)
	if err != nil {
		return false, err
	}
	result, ok := value.Value().(bool)
	if !ok {
		return false, errors.New("when expression must return a boolean")
	}
	return result, nil
}

func resolveMap(input map[string]any, source any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		resolved, err := resolveValue(value, source)
		if err != nil {
			return nil, err
		}
		out[key] = resolved
	}
	return out, nil
}

func resolveValue(value any, source any) (any, error) {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "$.") || typed == "$" {
			resolved, err := mapping.ResolveJSONPath(source, typed)
			if err != nil {
				return nil, err
			}
			return resolved, nil
		}
		return typed, nil
	case map[string]any:
		return resolveMap(typed, source)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			resolved, err := resolveValue(typed[i], source)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return value, nil
	}
}

func stringMap(value any) map[string]string {
	input, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, raw := range input {
		switch v := raw.(type) {
		case string:
			out[key] = v
		case nil:
			continue
		default:
			out[key] = fmt.Sprint(v)
		}
	}
	return out
}

func (s *Service) breakerAllow(key string, now time.Time) bool {
	s.breakers.Lock()
	defer s.breakers.Unlock()
	state := s.breaker[key]
	if !state.openUntil.IsZero() {
		if now.Before(state.openUntil) {
			return false
		}
		delete(s.breaker, key)
		return true
	}
	if !state.updatedAt.IsZero() && now.Sub(state.updatedAt) >= 30*time.Second {
		delete(s.breaker, key)
	}
	return true
}

func (s *Service) breakerFailure(key string, now time.Time) {
	s.breakers.Lock()
	defer s.breakers.Unlock()
	state := s.breaker[key]
	if !state.updatedAt.IsZero() && now.Sub(state.updatedAt) >= 30*time.Second {
		state.failures = 0
	}
	state.failures++
	state.updatedAt = now
	if state.failures >= 5 {
		state.openUntil = now.Add(30 * time.Second)
	}
	s.breaker[key] = state
}

func (s *Service) breakerSuccess(key string) {
	s.breakers.Lock()
	defer s.breakers.Unlock()
	delete(s.breaker, key)
}

func (s *Service) ListExecutions(ctx context.Context, workspaceID uuid.UUID, compositionID *uuid.UUID) ([]Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	if compositionID != nil {
		var row compositionRow
		if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			var err error
			ptr, err := s.load(ctx, tx, *compositionID)
			if err != nil {
				return err
			}
			row = *ptr
			if row.WorkspaceID != workspaceID {
				return errCompositionNotFound
			}
			return s.authorize(ctx, p, "read", row.WorkspaceID, row.ID.String())
		}); err != nil {
			return nil, err
		}
	}
	var rows []executionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListCompositionExecutions"), workspaceID, compositionID)
	}); err != nil {
		return nil, err
	}
	out := make([]Execution, len(rows))
	for i := range rows {
		model, err := rows[i].model()
		if err != nil {
			return nil, err
		}
		out[i] = model
	}
	return out, nil
}

func (s *Service) GetExecution(ctx context.Context, id uuid.UUID) (Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Execution{}, err
	}
	var row executionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &row, q.Get("GetCompositionExecution"), id); err != nil {
			if db.IsNoRows(err) {
				return errExecutionNotFound
			}
			return err
		}
		return s.authorize(ctx, p, "read", row.WorkspaceID, row.CompositionID.String())
	}); err != nil {
		return Execution{}, err
	}
	return row.model()
}

func validation(field, code, message string) error {
	return httpx.Validation(httpx.FieldError{Field: field, Code: code, Message: message})
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func ptrString(value string) *string { return &value }
