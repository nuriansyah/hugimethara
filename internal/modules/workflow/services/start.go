package services

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
)

// startRequest is everything needed to start an execution, whoever starts it:
// the control plane (manual, as hugi_app) or the trigger runner (schedule and
// event, as hugi_system).
type startRequest struct {
	OrgID, WorkspaceID, WorkflowID, EnvironmentID uuid.UUID
	Version                                       model.Version
	TriggerType, TriggerRef, IdempotencyKey       string
	Input                                         []byte
	CreatedBy                                     uuid.UUID
	RequestID                                     string
	TriggerDepth                                  int
	Now                                           time.Time
}

// ConcurrencyLimited is returned when a workflow already has
// limits.max_concurrent_executions unfinished executions in the environment.
func ConcurrencyLimited(limit int) *apperrors.Problem {
	return apperrors.NewProblem(http.StatusTooManyRequests, model.ReasonConcurrencyLimit,
		"Too many running executions",
		"This workflow already has its maximum of "+strconv.Itoa(limit)+
			" unfinished executions in this environment (limits.max_concurrent_executions).")
}

// startExecution inserts an execution pinned to req.Version and queues its
// first steps in tx. created is false for an idempotent replay, whose id is
// the original execution.
func startExecution(ctx context.Context, tx *sqlx.Tx, ids id.Generator, req startRequest) (uuid.UUID, bool, error) {
	def := req.Version.Definition
	if limit := def.Limits.MaxConcurrentExecutions; limit > 0 {
		// Serialize starts of this workflow+environment so the count is exact.
		if err := repository.LockStarts(ctx, tx, req.WorkflowID, req.EnvironmentID); err != nil {
			return uuid.Nil, false, err
		}
		n, err := repository.CountActiveExecutions(ctx, tx, req.WorkflowID, req.EnvironmentID)
		if err != nil {
			return uuid.Nil, false, err
		}
		if n >= limit {
			if req.IdempotencyKey != "" {
				existing, err := repository.ExecutionByKey(ctx, tx, req.WorkflowID, req.IdempotencyKey)
				if err == nil {
					return existing, false, nil
				}
				if !db.IsNoRows(err) {
					return uuid.Nil, false, err
				}
			}
			return uuid.Nil, false, ConcurrencyLimited(limit)
		}
	}
	allow, err := repository.EnvironmentAllowlist(ctx, tx, req.EnvironmentID)
	if err != nil {
		return uuid.Nil, false, err
	}
	execID, created, err := repository.InsertExecution(ctx, tx, repository.NewExecution{
		ID: ids.New(), OrgID: req.OrgID, WorkspaceID: req.WorkspaceID, WorkflowID: req.WorkflowID,
		VersionID: req.Version.ID, EnvironmentID: req.EnvironmentID, TriggerType: req.TriggerType,
		TriggerRef: req.TriggerRef, IdempotencyKey: req.IdempotencyKey, Input: req.Input, EgressAllowlist: allow,
		MaxSteps: def.Limits.EffectiveMaxSteps(), CreatedBy: req.CreatedBy, RequestID: req.RequestID,
		DeadlineAt: req.Now.Add(def.Limits.EffectiveMaxDuration()), TriggerDepth: req.TriggerDepth,
	})
	if err != nil || !created {
		return execID, false, err
	}
	exec, err := repository.LockExecution(ctx, tx, execID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if _, err := progress(ctx, tx, ids, exec, def); err != nil {
		return uuid.Nil, false, err
	}
	return execID, true, nil
}
