package services

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository"
	"github.com/nuriansyah/hugimethara/internal/platform/clock"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Execution history page size bounds.
const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

// WorkflowService implements the control-plane use cases. It runs as
// hugi_app: every transaction is tenant-bound and every operation is
// authorized against the workflow's workspace.
type WorkflowService struct {
	pool  *sqlx.DB
	az    authzsvc.Authorizer
	expr  *Expressions
	clock clock.Clock
	ids   id.Generator
}

// NewService builds the workflow service.
func NewService(pool *sqlx.DB, az authzsvc.Authorizer, expr *Expressions) *WorkflowService {
	return &WorkflowService{pool: pool, az: az, expr: expr, clock: clock.System{}, ids: id.V7{}}
}

func (s *WorkflowService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *WorkflowService) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: model.ResourceType, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

// load fetches a workflow and authorizes action on it.
func (s *WorkflowService) load(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string, id uuid.UUID) (model.Workflow, error) {
	w, err := repository.GetWorkflow(ctx, tx, id)
	if db.IsNoRows(err) {
		return model.Workflow{}, model.ErrWorkflowNotFound
	}
	if err != nil {
		return model.Workflow{}, err
	}
	return w, s.authorize(ctx, p, action, w.WorkspaceID, id.String())
}

func (s *WorkflowService) audit(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	w model.Workflow, details map[string]any) error {
	return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: action, ResourceType: model.ResourceType,
		ResourceID: w.ID.String(), WorkspaceID: &w.WorkspaceID, Details: details})
}

func validation(field, code, msg string) error {
	return problem.Validation(problem.FieldError{Field: field, Code: code, Message: msg})
}

// NodeTypes returns the node-type catalog.
func (s *WorkflowService) NodeTypes(ctx context.Context) ([]model.NodeType, error) {
	if _, err := s.principal(ctx); err != nil {
		return nil, err
	}
	return model.Catalog(), nil
}

// List returns the workflows of a workspace.
func (s *WorkflowService) List(ctx context.Context, workspaceID uuid.UUID) ([]model.Workflow, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var out []model.Workflow
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		out, err = repository.ListWorkflows(ctx, tx, workspaceID)
		return err
	})
	return out, err
}

// Get returns one workflow.
func (s *WorkflowService) Get(ctx context.Context, id uuid.UUID) (model.Workflow, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Workflow{}, err
	}
	var out model.Workflow
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		out, err = s.load(ctx, tx, p, "read", id)
		return err
	})
	return out, err
}

// Create creates a workflow draft.
func (s *WorkflowService) Create(ctx context.Context, in dto.CreateInput) (model.Workflow, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Workflow{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return model.Workflow{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	in.Description = strings.TrimSpace(in.Description)
	switch {
	case in.Name == "" || len([]rune(in.Name)) > 100:
		return model.Workflow{}, validation("name", "invalid_length", "Name must be 1–100 characters.")
	case !slugRe.MatchString(in.Slug):
		return model.Workflow{}, validation("slug", "invalid_slug",
			"Use 1–63 lowercase letters, digits or hyphens, starting with a letter or digit.")
	case len([]rune(in.Description)) > 500:
		return model.Workflow{}, validation("description", "invalid_length", "Description must be at most 500 characters.")
	}
	draft := model.EmptyDefinition()
	if in.Draft != nil {
		draft = *in.Draft
	}
	var out model.Workflow
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		out, err = repository.InsertWorkflow(ctx, tx, repository.NewWorkflow{ID: s.ids.New(), OrgID: p.OrgID,
			WorkspaceID: in.WorkspaceID, Name: in.Name, Slug: in.Slug, Desc: in.Description, Draft: draft,
			CreatedBy: p.ID})
		if db.IsUniqueViolation(err) {
			return model.ErrSlugTaken
		}
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "workflow.created", out, map[string]any{"name": out.Name, "slug": out.Slug})
	})
	return out, err
}

// Update edits a workflow's name, description, draft or status.
func (s *WorkflowService) Update(ctx context.Context, id uuid.UUID, in dto.UpdateInput) (model.Workflow, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Workflow{}, err
	}
	var out model.Workflow
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		w, err := s.load(ctx, tx, p, "update", id)
		if err != nil {
			return err
		}
		changed := []string{}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" || len([]rune(name)) > 100 {
				return validation("name", "invalid_length", "Name must be 1–100 characters.")
			}
			w.Name, changed = name, append(changed, "name")
		}
		if in.Description != nil {
			desc := strings.TrimSpace(*in.Description)
			if len([]rune(desc)) > 500 {
				return validation("description", "invalid_length", "Description must be at most 500 characters.")
			}
			w.Description, changed = desc, append(changed, "description")
		}
		if in.Draft != nil {
			raw, err := json.Marshal(in.Draft)
			if err != nil || len(raw) > 512<<10 {
				return validation("draft", "too_large", "The draft must be a JSON document of at most 512 KiB.")
			}
			w.Draft, changed = *in.Draft, append(changed, "draft")
		}
		if in.Status != nil {
			switch *in.Status {
			case "active", "paused", "archived":
			default:
				return validation("status", "invalid_status", "status must be active, paused or archived.")
			}
			if *in.Status == "active" && w.LatestVersion == nil {
				return validation("status", "not_published", "Publish a version before activating the workflow.")
			}
			w.Status, changed = *in.Status, append(changed, "status")
		}
		out, err = repository.UpdateWorkflow(ctx, tx, w)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "workflow.updated", out, map[string]any{"changed": changed})
	})
	return out, err
}

// Delete removes a workflow and its history.
func (s *WorkflowService) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		w, err := s.load(ctx, tx, p, "delete", id)
		if err != nil {
			return err
		}
		if _, err := repository.DeleteWorkflow(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "workflow.deleted", w, map[string]any{"name": w.Name})
	})
}

// Validate checks the draft and returns every issue (empty means publishable).
func (s *WorkflowService) Validate(ctx context.Context, id uuid.UUID) ([]model.Issue, error) {
	w, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return model.Validate(w.Draft, s.expr.Check), nil
}

// Publish freezes the draft as the next immutable version.
func (s *WorkflowService) Publish(ctx context.Context, id uuid.UUID) (model.Version, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Version{}, err
	}
	var out model.Version
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		if err := repository.LockWorkflow(ctx, tx, id); err != nil {
			if db.IsNoRows(err) {
				return model.ErrWorkflowNotFound
			}
			return err
		}
		w, err := s.load(ctx, tx, p, "publish", id)
		if err != nil {
			return err
		}
		if issues := model.Validate(w.Draft, s.expr.Check); len(issues) > 0 {
			return model.InvalidDefinition(issues)
		}
		sum, raw, err := model.Checksum(w.Draft)
		if err != nil {
			return err
		}
		next := 1
		latest, err := repository.LatestVersion(ctx, tx, id)
		switch {
		case err == nil:
			if latest.Checksum == sum {
				return model.ErrNothingToPublish
			}
			next = latest.Version + 1
		case !db.IsNoRows(err):
			return err
		}
		out, err = repository.InsertVersion(ctx, tx, s.ids.New(), p.OrgID, id, next, raw, sum, p.ID)
		if err != nil {
			return err
		}
		if err := repository.ActivateWorkflow(ctx, tx, id); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, p, "workflow.published", w,
			map[string]any{"version": out.Version, "checksum": sum}); err != nil {
			return err
		}
		_, err = outbox.Publish(ctx, tx, outbox.Event{OrgID: p.OrgID, Subject: "hugi.events.workflow.published",
			Payload: map[string]any{"workflow_id": id, "version": out.Version, "version_id": out.ID}})
		return err
	})
	return out, err
}

// ListVersions lists published versions, newest first.
func (s *WorkflowService) ListVersions(ctx context.Context, id uuid.UUID) ([]model.Version, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Version
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		if _, err := s.load(ctx, tx, p, "read", id); err != nil {
			return err
		}
		out, err = repository.ListVersions(ctx, tx, id)
		return err
	})
	return out, err
}

// ListDeployments lists the environments a workflow is deployed to.
func (s *WorkflowService) ListDeployments(ctx context.Context, id uuid.UUID) ([]model.Deployment, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Deployment
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		if _, err := s.load(ctx, tx, p, "read", id); err != nil {
			return err
		}
		out, err = repository.ListDeployments(ctx, tx, id)
		return err
	})
	return out, err
}

// Deploy pins a published version to an environment (or disables it).
func (s *WorkflowService) Deploy(ctx context.Context, id, environmentID uuid.UUID, in dto.DeployInput) (model.Deployment, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Deployment{}, err
	}
	var out model.Deployment
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		w, err := s.load(ctx, tx, p, "deploy", id)
		if err != nil {
			return err
		}
		v, err := repository.GetVersion(ctx, tx, in.VersionID)
		if db.IsNoRows(err) || (err == nil && v.WorkflowID != id) {
			return validation("version_id", "not_found", "The version does not belong to this workflow.")
		}
		if err != nil {
			return err
		}
		if _, err := repository.EnvironmentAllowlist(ctx, tx, environmentID); err != nil {
			if db.IsNoRows(err) {
				return validation("environment_id", "not_found", "The environment does not exist.")
			}
			return err
		}
		var nextFire *time.Time
		if v.Definition.Trigger.Type == model.TypeTriggerSchedule && in.Enabled {
			sched, err := model.ParseScheduleTrigger(v.Definition.Trigger.Config)
			if err != nil {
				return validation("version_id", "invalid_schedule", err.Error())
			}
			if next := sched.Cron.Next(s.clock.Now(), sched.Location); !next.IsZero() {
				nextFire = &next
			}
		}
		if err := repository.UpsertDeployment(ctx, tx, p.OrgID, id, environmentID, v.ID, in.Enabled, p.ID,
			v.Definition.Trigger, nextFire); err != nil {
			return err
		}
		if out, err = repository.GetDeployment(ctx, tx, id, environmentID); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "workflow.deployed", w, map[string]any{
			"environment_id": environmentID, "version": v.Version, "enabled": in.Enabled})
	})
	return out, err
}

// Execute starts a manual execution of the version deployed to the
// environment. The first steps are queued and dispatched in the same
// transaction; the worker takes it from there.
func (s *WorkflowService) Execute(ctx context.Context, id uuid.UUID, in dto.ExecuteInput) (model.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Execution{}, err
	}
	input := []byte(in.Input)
	if len(input) == 0 {
		input = []byte("{}")
	}
	var probe map[string]any
	if json.Unmarshal(input, &probe) != nil {
		return model.Execution{}, validation("input", "invalid_input", "input must be a JSON object.")
	}
	if len(input) > model.MaxInlinePayload {
		return model.Execution{}, validation("input", "too_large", "input must be at most 64 KiB.")
	}
	if len(in.IdempotencyKey) > 200 {
		return model.Execution{}, validation("idempotency_key", "too_long", "idempotency_key must be at most 200 characters.")
	}
	var execID uuid.UUID
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		w, err := s.load(ctx, tx, p, "execute", id)
		if err != nil {
			return err
		}
		if w.Status == "paused" || w.Status == "archived" {
			return problem.NewProblem(409, "workflow_"+w.Status, "Workflow is "+w.Status,
				"Activate the workflow before executing it.")
		}
		dep, err := repository.GetDeployment(ctx, tx, id, in.EnvironmentID)
		if db.IsNoRows(err) || (err == nil && !dep.Enabled) {
			return model.ErrNotDeployed
		}
		if err != nil {
			return err
		}
		v, err := repository.GetVersion(ctx, tx, dep.VersionID)
		if err != nil {
			return err
		}
		var created bool
		execID, created, err = startExecution(ctx, tx, s.ids, startRequest{OrgID: p.OrgID,
			WorkspaceID: w.WorkspaceID, WorkflowID: id, EnvironmentID: in.EnvironmentID, Version: v,
			TriggerType: model.TypeTriggerManual, TriggerRef: p.ID.String(), IdempotencyKey: in.IdempotencyKey,
			Input: input, CreatedBy: p.ID, RequestID: logging.RequestID(ctx), Now: s.clock.Now()})
		if err != nil || !created {
			return err // idempotent replay returns the original execution
		}
		return s.audit(ctx, tx, p, "workflow.executed", w, map[string]any{
			"execution_id": execID, "version": v.Version, "environment_id": in.EnvironmentID})
	})
	if err != nil {
		return model.Execution{}, err
	}
	return s.GetExecution(ctx, execID)
}

// ListExecutions returns the execution history of a workspace.
func (s *WorkflowService) ListExecutions(ctx context.Context, f dto.ExecutionFilter) ([]model.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", f.WorkspaceID, ""); err != nil {
		return nil, err
	}
	switch f.Status {
	case "", "pending", "running", "succeeded", "failed", "cancelled", "timed_out":
	default:
		return nil, validation("status", "invalid_status", "Unknown execution status.")
	}
	if f.Limit <= 0 {
		f.Limit = defaultHistoryLimit
	}
	if f.Limit > maxHistoryLimit {
		f.Limit = maxHistoryLimit
	}
	var out []model.Execution
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		out, err = repository.ListExecutions(ctx, tx, f.WorkspaceID, f.WorkflowID, f.Status, f.Limit)
		return err
	})
	return out, err
}

// GetExecution returns an execution with every step's input, output and error.
func (s *WorkflowService) GetExecution(ctx context.Context, executionID uuid.UUID) (model.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Execution{}, err
	}
	var out model.Execution
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		out, err = repository.GetExecution(ctx, tx, executionID)
		if db.IsNoRows(err) {
			return model.ErrExecutionNotFound
		}
		if err != nil {
			return err
		}
		return s.authorize(ctx, p, "read", out.WorkspaceID, out.WorkflowID.String())
	})
	return out, err
}
