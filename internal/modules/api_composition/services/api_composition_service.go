package services

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

	compositionmodel "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model"
	compositiondto "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/api_composition/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/api_composition/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/api_composition/repository/queries"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	mappingmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model"
	mappingsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/services"
	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
	serviceregdto "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model/dto"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

type endpointInvoker interface {
	TryEndpoint(context.Context, uuid.UUID, serviceregdto.ServiceTryInput) (serviceregmodel.ServiceTryResult, error)
}

type breakerState struct {
	failures  int
	openUntil time.Time
	updatedAt time.Time
}

type CompositionService struct {
	pool     *sqlx.DB
	az       authzsvc.Authorizer
	invoker  endpointInvoker
	breakers sync.Mutex
	breaker  map[string]breakerState
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer, invoker endpointInvoker) *CompositionService {
	return &CompositionService{pool: pool, az: az, invoker: invoker, breaker: make(map[string]breakerState)}
}

func (s *CompositionService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *CompositionService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *CompositionService) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: compositionmodel.Resource, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func (s *CompositionService) load(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*repository.CompositionRow, error) {
	var row repository.CompositionRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetComposition"), id); err != nil {
		if db.IsNoRows(err) {
			return nil, compositionmodel.ErrCompositionNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *CompositionService) List(ctx context.Context, workspaceID uuid.UUID) ([]compositionmodel.Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.CompositionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListCompositions"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]compositionmodel.Composition, len(rows))
	for i := range rows {
		model, err := rows[i].Model()
		if err != nil {
			return nil, err
		}
		out[i] = model
	}
	return out, nil
}

func (s *CompositionService) Get(ctx context.Context, id uuid.UUID) (compositionmodel.Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return compositionmodel.Composition{}, err
	}
	var out compositionmodel.Composition
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out, err = row.Model()
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return compositionmodel.Composition{}, err
	}
	return out, nil
}

func (s *CompositionService) Create(ctx context.Context, in compositiondto.CreateInput) (compositionmodel.Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return compositionmodel.Composition{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return compositionmodel.Composition{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	if in.Name == "" || len(in.Name) > 100 {
		return compositionmodel.Composition{}, compositionmodel.Validation("name", "invalid", "A composition name between 1 and 100 characters is required.")
	}
	if !compositionmodel.CompositionSlug.MatchString(in.Slug) {
		return compositionmodel.Composition{}, compositionmodel.Validation("slug", "invalid", "Use 1–63 lowercase letters, digits, or hyphens; begin with a letter or digit.")
	}
	if in.Draft == nil {
		in.Draft = map[string]any{"steps": []any{}}
	}
	if err := ValidateDraft(in.Draft); err != nil {
		return compositionmodel.Composition{}, compositionmodel.Validation("draft", "invalid", err.Error())
	}
	id := uuid.New()
	raw, err := json.Marshal(in.Draft)
	if err != nil {
		return compositionmodel.Composition{}, compositionmodel.Validation("draft", "invalid", "The composition draft must be valid JSON.")
	}
	var out compositionmodel.Composition
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertComposition"), p.OrgID, id, in.WorkspaceID, in.Name, in.Slug, raw); err != nil {
			return err
		}
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = row.Model()
		if err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "api_composition.created",
			ResourceType: compositionmodel.Resource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"name": in.Name, "slug": in.Slug}})
	}); err != nil {
		return compositionmodel.Composition{}, err
	}
	return out, nil
}

func (s *CompositionService) Update(ctx context.Context, id uuid.UUID, in compositiondto.UpdateInput) (compositionmodel.Composition, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return compositionmodel.Composition{}, err
	}
	var out compositionmodel.Composition
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
				return compositionmodel.Validation("name", "invalid", "A composition name between 1 and 100 characters is required.")
			}
			in.Name = &name
		}
		if in.Slug != nil {
			slug := strings.TrimSpace(strings.ToLower(*in.Slug))
			if !compositionmodel.CompositionSlug.MatchString(slug) {
				return compositionmodel.Validation("slug", "invalid", "Use 1–63 lowercase letters, digits, or hyphens; begin with a letter or digit.")
			}
			in.Slug = &slug
		}
		var draft any
		if in.Draft != nil {
			if err := ValidateDraft(*in.Draft); err != nil {
				return compositionmodel.Validation("draft", "invalid", err.Error())
			}
			raw, err := json.Marshal(*in.Draft)
			if err != nil {
				return compositionmodel.Validation("draft", "invalid", "The composition draft must be valid JSON.")
			}
			draft = json.RawMessage(raw)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateComposition"), id, in.Name, in.Slug, draft, in.Exposed); err != nil {
			return err
		}
		row, err = s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = row.Model()
		if err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "api_composition.updated",
			ResourceType: compositionmodel.Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return compositionmodel.Composition{}, err
	}
	return out, nil
}

func (s *CompositionService) Delete(ctx context.Context, id uuid.UUID) error {
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
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteComposition"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "api_composition.deleted",
			ResourceType: compositionmodel.Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *CompositionService) validateReferences(ctx context.Context, p principal.Principal, tx *sqlx.Tx, row *repository.CompositionRow, draft map[string]any) error {
	if err := ValidateDraft(draft); err != nil {
		return compositionmodel.Validation("draft", "invalid", err.Error())
	}
	var def compositionmodel.Definition
	raw, _ := json.Marshal(draft)
	if err := json.Unmarshal(raw, &def); err != nil {
		return compositionmodel.Validation("draft", "invalid", "The composition draft has an invalid shape.")
	}
	if len(def.Steps) == 0 {
		return compositionmodel.Validation("draft", "empty", "Add at least one API call step before validating or publishing.")
	}
	for _, current := range def.Steps {
		var endpointWorkspace uuid.UUID
		err := tx.GetContext(ctx, &endpointWorkspace, `
			SELECT s.workspace_id
			FROM services s JOIN service_endpoints e ON e.service_id = s.id
			WHERE s.id = $1 AND e.id = $2`, current.Call.ServiceID, current.Call.EndpointID)
		if err != nil {
			if db.IsNoRows(err) {
				return compositionmodel.Validation("draft", "reference_not_found", "A composition step references a service endpoint that no longer exists.")
			}
			return err
		}
		if endpointWorkspace != row.WorkspaceID {
			return compositionmodel.Validation("draft", "workspace_mismatch", "Composition endpoints must belong to the same workspace.")
		}
		if _, err := authzsvc.Require(ctx, s.az, "read", authzmodel.Resource{Type: "service", ID: current.Call.ServiceID,
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
				return compositionmodel.Validation("draft", "mapping_not_found", "The response mapping version does not exist.")
			}
			return err
		}
		if mappingWorkspace != row.WorkspaceID {
			return compositionmodel.Validation("draft", "workspace_mismatch", "The response mapping must belong to the same workspace.")
		}
		if _, err := authzsvc.Require(ctx, s.az, "read", authzmodel.Resource{Type: "mapping",
			ID: def.Response.MappingID, OrgID: p.OrgID, WorkspaceID: &mappingWorkspace}); err != nil {
			return err
		}
	}
	return nil
}

func (s *CompositionService) Validate(ctx context.Context, id uuid.UUID) error {
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
		model, err := row.Model()
		if err != nil {
			return err
		}
		return s.validateReferences(ctx, p, tx, row, model.Draft)
	})
}

func (s *CompositionService) ListVersions(ctx context.Context, id uuid.UUID) ([]compositionmodel.Version, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []compositionmodel.Version
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []repository.VersionRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListCompositionVersions"), id); err != nil {
			return err
		}
		out = make([]compositionmodel.Version, len(rows))
		for i := range rows {
			out[i], err = rows[i].Model()
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

func (s *CompositionService) Publish(ctx context.Context, id uuid.UUID) (compositionmodel.Version, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return compositionmodel.Version{}, err
	}
	var out compositionmodel.Version
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.load(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "publish", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		composition, err := row.Model()
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
		if err := tx.GetContext(ctx, &next, queries.SQL.Get("NextCompositionVersion"), id); err != nil {
			return err
		}
		versionID := uuid.New()
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertCompositionVersion"), p.OrgID, versionID, id,
			next, json.RawMessage(raw), checksum, p.ID); err != nil {
			return err
		}
		var version repository.VersionRow
		if err := tx.GetContext(ctx, &version, queries.SQL.Get("GetCompositionVersion"), id, next); err != nil {
			return err
		}
		out, err = version.Model()
		if err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "api_composition.published",
			ResourceType: compositionmodel.Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"version": next, "checksum": checksum}})
	}); err != nil {
		return compositionmodel.Version{}, err
	}
	return out, nil
}

func (s *CompositionService) Execute(ctx context.Context, id uuid.UUID, in compositiondto.ExecuteInput) (compositionmodel.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return compositionmodel.Execution{}, err
	}
	var row repository.CompositionRow
	var version repository.VersionRow
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
			return compositionmodel.Validation("environment_id", "required", "An execution environment is required.")
		}
		if err := tx.GetContext(ctx, new(uuid.UUID),
			`SELECT id FROM environments WHERE id = $1 AND org_id = $2`, in.EnvironmentID, p.OrgID); err != nil {
			if db.IsNoRows(err) {
				return compositionmodel.Validation("environment_id", "not_found", "The execution environment does not exist.")
			}
			return err
		}
		if err := tx.GetContext(ctx, &version, `
			SELECT id, composition_id, version, definition, checksum, published_by, published_at
			FROM api_composition_versions WHERE composition_id = $1
			ORDER BY version DESC LIMIT 1`, id); err != nil {
			if db.IsNoRows(err) {
				return compositionmodel.Validation("composition_id", "unpublished", "Publish the composition before executing it.")
			}
			return err
		}
		return nil
	}); err != nil {
		return compositionmodel.Execution{}, err
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
		return compositionmodel.Execution{}, fmt.Errorf("composition: encode step summary: %w", err)
	}
	if len(summary) > compositionmodel.MaxAggregateBytes {
		status = "failed"
		errorText = compositionmodel.PtrString("Composition step output exceeds the 5 MiB aggregate limit.")
		summary = json.RawMessage(`{}`)
		result = nil
	}
	var output []byte
	if result != nil {
		output, err = json.Marshal(result)
		if err != nil {
			return compositionmodel.Execution{}, fmt.Errorf("composition: encode response: %w", err)
		}
		if len(output)+len(summary) > compositionmodel.MaxAggregateBytes {
			status = "failed"
			errorText = compositionmodel.PtrString("Composition output exceeds the 5 MiB aggregate limit.")
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
	out := compositionmodel.Execution{ID: executionID, CompositionID: id, VersionID: version.ID,
		EnvironmentID: in.EnvironmentID, StartedAt: now, DurationMs: duration,
		Status: status, StepSummary: summary, Output: result, Error: errorText, RequestID: requestID}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.tenantTx(persistCtx, p, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(persistCtx, commands.SQL.Get("InsertCompositionExecution"),
			p.OrgID, executionID, row.WorkspaceID, id, version.ID, in.EnvironmentID,
			now, duration, status, json.RawMessage(summary), compositionmodel.NullableJSON(output), errorText, requestID)
		if err != nil {
			return err
		}
		return auditsvc.Record(persistCtx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "api_composition.executed",
			ResourceType: compositionmodel.Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"execution_id": executionID.String(), "version": version.Version,
				"status": status, "duration_ms": duration}})
	}); err != nil {
		return compositionmodel.Execution{}, err
	}
	return out, nil
}

func (s *CompositionService) run(ctx context.Context, orgID uuid.UUID, version repository.VersionRow, in compositiondto.ExecuteInput) (any, map[string]compositionmodel.StepResult, error) {
	var def compositionmodel.Definition
	if err := json.Unmarshal(version.Definition, &def); err != nil {
		return nil, map[string]compositionmodel.StepResult{}, fmt.Errorf("published composition version is invalid")
	}
	deadline := compositionmodel.DefaultDeadline
	if def.Limits.TimeoutMs > 0 {
		deadline = time.Duration(def.Limits.TimeoutMs) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	request := in.Request
	if request == nil {
		request = map[string]any{}
	}
	results := make(map[string]compositionmodel.StepResult, len(def.Steps))
	remaining := make(map[string]compositionmodel.Step, len(def.Steps))
	for _, current := range def.Steps {
		remaining[current.ID] = current
	}
	concurrency := def.Limits.MaxConcurrency
	if concurrency < 1 {
		concurrency = compositionmodel.MaxConcurrency
	}
	for len(remaining) > 0 {
		if err := runCtx.Err(); err != nil {
			return nil, results, errors.New("composition execution deadline exceeded")
		}
		ready := make([]compositionmodel.Step, 0)
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
		stepResults := make(map[string]compositionmodel.StepResult, len(ready))
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
					got = compositionmodel.StepResult{Status: got.Status, Body: nil, Attempts: got.Attempts, Error: err.Error()}
				case "default":
					got = compositionmodel.StepResult{Status: got.Status, Body: current.Default, Attempts: got.Attempts, Error: err.Error()}
				case "fallback_step":
					fallback, ok := results[current.FallbackStep]
					if !ok {
						return nil, results, fmt.Errorf("step %q failed and its fallback result is unavailable", current.ID)
					}
					got = compositionmodel.StepResult{Status: got.Status, Body: fallback.Body, Attempts: got.Attempts, Error: err.Error()}
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
		if len(aggregate) > compositionmodel.MaxAggregateBytes {
			return nil, results, errors.New("composition step output exceeds the 5 MiB aggregate limit")
		}
	}
	source := map[string]any{"request": request, "steps": stepValues(results)}
	if def.Response.MappingID != "" {
		mapped, err := s.applyResponseMapping(runCtx, orgID, def.Response.MappingID, def.Response.MappingVersion, source)
		if err != nil {
			return nil, results, fmt.Errorf("response mapping failed")
		}
		return mapped, results, nil
	}
	return source, results, nil
}

func (s *CompositionService) executeStep(ctx context.Context, orgID, environmentID uuid.UUID, current compositionmodel.Step,
	request map[string]any, prior map[string]compositionmodel.StepResult) (compositionmodel.StepResult, error) {
	for _, dep := range current.DependsOn {
		if prior[dep].Skipped {
			return compositionmodel.StepResult{Skipped: true}, nil
		}
	}
	stepData := stepValues(prior)
	if current.When != "" {
		ok, err := evalCondition(current.When, map[string]any{"steps": stepData, "request": request})
		if err != nil {
			return compositionmodel.StepResult{}, fmt.Errorf("when expression could not be evaluated")
		}
		if !ok {
			return compositionmodel.StepResult{Skipped: true}, nil
		}
	}
	values := map[string]any{"request": request, "steps": stepData}
	resolved, err := resolveMap(current.Call.Input, values)
	if err != nil {
		return compositionmodel.StepResult{}, errors.New("step input reference is invalid")
	}
	pathValues := compositionmodel.StringMap(resolved["path_values"])
	query := compositionmodel.StringMap(resolved["query"])
	headers := compositionmodel.StringMap(resolved["headers"])
	call := serviceregdto.ServiceTryInput{EnvironmentID: environmentID, PathValues: pathValues,
		Query: query, Headers: headers, Body: resolved["body"]}
	endpointID, _ := uuid.Parse(current.Call.EndpointID)
	key := orgID.String() + ":" + current.Call.ServiceID + ":" + environmentID.String()
	attempts := current.Retries + 1
	var result serviceregmodel.ServiceTryResult
	var lastErr error
	start := time.Now()
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return compositionmodel.StepResult{Attempts: attempt - 1}, errors.New("step deadline exceeded")
		}
		if !s.breakerAllow(key, time.Now()) {
			return compositionmodel.StepResult{Attempts: attempt - 1}, errors.New("service circuit breaker is open")
		}
		stepLimit := compositionmodel.DefaultStepLimit
		if current.TimeoutMs > 0 {
			stepLimit = time.Duration(current.TimeoutMs) * time.Millisecond
		}
		stepCtx, cancel := context.WithTimeout(ctx, stepLimit)
		result, lastErr = s.invoker.TryEndpoint(stepCtx, endpointID, call)
		cancel()
		if lastErr == nil && (result.Status < http.StatusOK || result.Status >= http.StatusMultipleChoices) {
			lastErr = compositionmodel.EndpointResponseError{Status: result.Status}
		}
		if lastErr == nil {
			s.breakerSuccess(key)
			return compositionmodel.StepResult{Status: result.Status, Body: result.Body, DurationMs: result.DurationMs,
				Attempts: attempt}, nil
		}
		if statusErr, ok := lastErr.(compositionmodel.EndpointResponseError); ok && statusErr.Status < http.StatusInternalServerError {
			return compositionmodel.StepResult{Status: result.Status, Body: result.Body,
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
				return compositionmodel.StepResult{Attempts: attempt}, errors.New("step deadline exceeded")
			case <-timer.C:
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("endpoint request failed")
	}
	return compositionmodel.StepResult{Status: result.Status, Body: result.Body,
		DurationMs: int(time.Since(start).Milliseconds()), Attempts: attempts}, errors.New("endpoint request failed")
}

// stepValues renders step results as plain JSON values. Conditions, input
// references and the response mapping address them as $.steps.<id>.*, so the
// values must be maps — not StepResult structs — for JSONPath to traverse.
func stepValues(results map[string]compositionmodel.StepResult) map[string]any {
	out := make(map[string]any, len(results))
	for id, result := range results {
		out[id] = map[string]any{"status": result.Status, "body": result.Body,
			"duration_ms": result.DurationMs, "skipped": result.Skipped, "error": result.Error}
	}
	return out
}

func (s *CompositionService) applyResponseMapping(ctx context.Context, orgID uuid.UUID, mappingID string, version int, source any) (any, error) {
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
	return mappingsvc.Apply(draft, source)
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
			resolved, err := mappingmodel.ResolveJSONPath(source, typed)
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

func (s *CompositionService) breakerAllow(key string, now time.Time) bool {
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

func (s *CompositionService) breakerFailure(key string, now time.Time) {
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

func (s *CompositionService) breakerSuccess(key string) {
	s.breakers.Lock()
	defer s.breakers.Unlock()
	delete(s.breaker, key)
}

func (s *CompositionService) ListExecutions(ctx context.Context, workspaceID uuid.UUID, compositionID *uuid.UUID) ([]compositionmodel.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	if compositionID != nil {
		var row repository.CompositionRow
		if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			var err error
			ptr, err := s.load(ctx, tx, *compositionID)
			if err != nil {
				return err
			}
			row = *ptr
			if row.WorkspaceID != workspaceID {
				return compositionmodel.ErrCompositionNotFound
			}
			return s.authorize(ctx, p, "read", row.WorkspaceID, row.ID.String())
		}); err != nil {
			return nil, err
		}
	}
	var rows []repository.ExecutionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListCompositionExecutions"), workspaceID, compositionID)
	}); err != nil {
		return nil, err
	}
	out := make([]compositionmodel.Execution, len(rows))
	for i := range rows {
		model, err := rows[i].Model()
		if err != nil {
			return nil, err
		}
		out[i] = model
	}
	return out, nil
}

func (s *CompositionService) GetExecution(ctx context.Context, id uuid.UUID) (compositionmodel.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return compositionmodel.Execution{}, err
	}
	var row repository.ExecutionRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetCompositionExecution"), id); err != nil {
			if db.IsNoRows(err) {
				return compositionmodel.ErrExecutionNotFound
			}
			return err
		}
		return s.authorize(ctx, p, "read", row.WorkspaceID, row.CompositionID.String())
	}); err != nil {
		return compositionmodel.Execution{}, err
	}
	return row.Model()
}
