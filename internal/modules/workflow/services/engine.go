package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository"
	"github.com/nuriansyah/hugimethara/internal/platform/clock"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
)

// EngineConfig tunes the native executor (ADR-0003).
type EngineConfig struct {
	WorkerID      string        // unique per process; fences lease ownership
	Lease         time.Duration // step lease; default 30s, heartbeated at Lease/3
	ReapInterval  time.Duration // default 5s
	RedispatchAge time.Duration // queued but undelivered for this long → re-sent; default 60s
	ReapBatch     int           // default 200
	Clock         clock.Clock
	IDs           id.Generator
	Log           *slog.Logger
	Registerer    prometheus.Registerer
}

// Engine is the data-plane executor: it claims dispatched steps, runs their
// node, records the result and advances the graph, and its reaper recovers
// from worker crashes, lost messages, due retries and deadlines. It connects
// as hugi_system; every step transaction is still bound to the step's tenant.
type Engine struct {
	pool  *sqlx.DB
	nodes Registry
	cfg   EngineConfig

	stepsTotal    *prometheus.CounterVec
	stepDuration  *prometheus.HistogramVec
	finishedTotal *prometheus.CounterVec
	recovered     *prometheus.CounterVec
}

// NewEngine builds an engine. pool must connect as hugi_system.
func NewEngine(pool *sqlx.DB, nodes Registry, cfg EngineConfig) *Engine {
	if cfg.Lease <= 0 {
		cfg.Lease = 30 * time.Second
	}
	if cfg.ReapInterval <= 0 {
		cfg.ReapInterval = 5 * time.Second
	}
	if cfg.RedispatchAge <= 0 {
		cfg.RedispatchAge = time.Minute
	}
	if cfg.ReapBatch <= 0 {
		cfg.ReapBatch = 200
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.IDs == nil {
		cfg.IDs = id.V7{}
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "worker-" + id.New().String()
	}
	e := &Engine{pool: pool, nodes: nodes, cfg: cfg,
		stepsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hugi_workflow_steps_total",
			Help: "Workflow steps finished, by node type and result."}, []string{"node_type", "result"}),
		stepDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hugi_workflow_step_duration_seconds",
			Help: "Node execution time.", Buckets: prometheus.ExponentialBuckets(0.005, 3, 10)}, []string{"node_type"}),
		finishedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hugi_workflow_executions_total",
			Help: "Workflow executions finished, by status."}, []string{"status"}),
		recovered: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hugi_workflow_reaper_total",
			Help: "Steps or executions recovered by the reaper, by reason."}, []string{"reason"}),
	}
	if cfg.Registerer != nil {
		cfg.Registerer.MustRegister(e.stepsTotal, e.stepDuration, e.finishedTotal, e.recovered)
	}
	return e
}

// claim is the state loaded when a step is leased.
type claim struct {
	step    repository.ClaimedStep
	exec    repository.LockedExecution
	def     model.Definition
	snap    snapshot
	trigger any
}

// HandleStep processes one dispatch message. It is idempotent: duplicates,
// steps already taken and steps of finished executions are acknowledged
// without effect. A returned error means "redeliver later".
func (e *Engine) HandleStep(ctx context.Context, msg StepMessage) error {
	c, ok, err := e.claim(ctx, msg)
	if err != nil || !ok {
		return err
	}
	node, found := c.def.Node(c.step.NodeID)
	exec, hasExec := e.nodes[node.Type]
	var res NodeResult
	var runErr error
	started := e.cfg.Clock.Now()
	input := model.NodeInput(c.def, node.ID, c.snap.states, c.snap.outputs, c.trigger)
	switch {
	case !found:
		runErr = &model.StepError{Code: model.ReasonInvalidState, Message: "node is not in the pinned definition"}
	case !hasExec:
		runErr = &model.StepError{Code: model.ReasonInvalidState, Message: "no executor for node type " + node.Type}
	default:
		policy, perr := egress.ParsePolicy(c.exec.Allowlist())
		if perr != nil {
			runErr = &model.StepError{Code: model.ReasonEgressBlocked, Message: perr.Error()}
			break
		}
		res, runErr = e.run(ctx, c.step.ID, exec, NodeContext{Node: node, ExecutionID: c.exec.ID, Attempt: c.step.Attempt,
			Input: input, Trigger: c.trigger, Outputs: c.snap.outputs, Egress: policy,
			IdempotencyKey: fmt.Sprintf("%s:%s::%d", c.exec.ID, node.ID, c.step.Attempt)})
	}
	e.stepDuration.WithLabelValues(c.step.NodeType).Observe(e.cfg.Clock.Now().Sub(started).Seconds())
	return e.complete(ctx, c, node, input, res, runErr)
}

func (e *Engine) claim(ctx context.Context, msg StepMessage) (claim, bool, error) {
	var c claim
	claimed := false
	err := db.InTenantTx(ctx, e.pool, msg.OrgID, func(tx *sqlx.Tx) error {
		step, ok, err := repository.ClaimStep(ctx, tx, msg.StepID, e.cfg.Lease, e.cfg.WorkerID)
		if err != nil || !ok {
			return err
		}
		exec, err := repository.LockExecution(ctx, tx, step.ExecutionID)
		if err != nil {
			return err
		}
		if model.ExecutionStatus(exec.Status).Terminal() {
			_, err := repository.CompleteStep(ctx, tx, step.ID, e.cfg.WorkerID,
				repository.StepResult{Status: model.StepCancelled})
			return err
		}
		if err := repository.StartExecution(ctx, tx, exec.ID); err != nil {
			return err
		}
		def, err := repository.EngineDefinition(ctx, tx, exec.VersionID)
		if err != nil {
			return err
		}
		snap, err := loadSnapshot(ctx, tx, exec.ID)
		if err != nil {
			return err
		}
		var trigger any
		if len(exec.Input) > 0 {
			if err := json.Unmarshal(exec.Input, &trigger); err != nil {
				return err
			}
		}
		c = claim{step: step, exec: exec, def: def, snap: snap, trigger: trigger}
		claimed = true
		return nil
	})
	return c, claimed, err
}

// run executes the node under its timeout while heartbeating the lease.
func (e *Engine) run(ctx context.Context, stepID uuid.UUID, n NodeExecutor, nc NodeContext) (NodeResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, nc.Node.StepTimeout())
	defer cancel()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(e.cfg.Lease / 3)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				ok, err := repository.ExtendLease(ctx, e.pool, stepID, e.cfg.Lease, e.cfg.WorkerID)
				if err == nil && !ok {
					cancel() // lease lost: another worker owns the step now
					return
				}
			}
		}
	}()
	res, err := n.Execute(runCtx, nc)
	if err == nil && runCtx.Err() != nil {
		err = &model.StepError{Code: model.ReasonStepTimeout, Message: "The node exceeded its timeout.", Retryable: true}
	}
	return res, err
}

func asStepError(err error, nodeID string) *model.StepError {
	var se *model.StepError
	if errors.As(err, &se) {
		cp := *se
		cp.NodeID = nodeID
		return &cp
	}
	return &model.StepError{Code: model.ReasonNodeFailed, Message: err.Error(), NodeID: nodeID}
}

// complete records the result and advances the graph in one transaction
// under the execution lock.
func (e *Engine) complete(ctx context.Context, c claim, node model.Node, input any, res NodeResult, runErr error) error {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return err
	}
	var outputJSON []byte
	if res.Output != nil || runErr == nil {
		if outputJSON, err = json.Marshal(res.Output); err != nil {
			runErr = &model.StepError{Code: model.ReasonNodeFailed, Message: "node output is not JSON-serializable"}
			outputJSON = nil
		}
	}
	if len(outputJSON) > model.MaxInlinePayload {
		runErr = &model.StepError{Code: model.ReasonPayloadTooLarge,
			Message: "The node output exceeds the inline payload limit (64 KiB)."}
		outputJSON = nil
	}
	if len(inputJSON) > model.MaxInlinePayload {
		inputJSON = nil
	}
	return db.InTenantTx(ctx, e.pool, c.exec.OrgID, func(tx *sqlx.Tx) error {
		exec, err := repository.LockExecution(ctx, tx, c.exec.ID)
		if err != nil {
			return err
		}
		if model.ExecutionStatus(exec.Status).Terminal() {
			_, err := repository.CompleteStep(ctx, tx, c.step.ID, e.cfg.WorkerID,
				repository.StepResult{Status: model.StepCancelled, Input: inputJSON})
			return err
		}
		if runErr != nil {
			se := asStepError(runErr, node.ID)
			if se.Retryable && c.step.Attempt < node.MaxAttempts() {
				at := e.cfg.Clock.Now().Add(node.RetryDelay(c.step.Attempt + 1))
				ok, err := repository.ScheduleRetry(ctx, tx, c.step.ID, e.cfg.WorkerID, inputJSON, se, at)
				if err == nil && ok {
					e.stepsTotal.WithLabelValues(node.Type, "retry_scheduled").Inc()
				}
				return err
			}
			ok, err := repository.CompleteStep(ctx, tx, c.step.ID, e.cfg.WorkerID, repository.StepResult{
				Status: model.StepFailed, Input: inputJSON, Output: outputJSON, Error: se})
			if err != nil || !ok {
				return err // lease lost: the new owner reports the outcome
			}
			e.stepsTotal.WithLabelValues(node.Type, "failed").Inc()
			e.finishedTotal.WithLabelValues(string(model.ExecFailed)).Inc()
			return finish(ctx, tx, exec, model.ExecFailed, nil, se)
		}
		ok, err := repository.CompleteStep(ctx, tx, c.step.ID, e.cfg.WorkerID, repository.StepResult{
			Status: model.StepSucceeded, Input: inputJSON, Output: outputJSON, Port: res.Port})
		if err != nil || !ok {
			return err
		}
		e.stepsTotal.WithLabelValues(node.Type, "succeeded").Inc()
		pr, err := progress(ctx, tx, e.cfg.IDs, exec, c.def)
		if err == nil && pr.finished != "" {
			e.finishedTotal.WithLabelValues(string(pr.finished)).Inc()
		}
		return err
	})
}

// RunReaper recovers stuck work until ctx is cancelled. Every worker runs
// one; SKIP LOCKED makes them cooperate instead of contend.
func (e *Engine) RunReaper(ctx context.Context) error {
	t := time.NewTicker(e.cfg.ReapInterval)
	defer t.Stop()
	for {
		if err := e.ReapOnce(ctx); err != nil && ctx.Err() == nil {
			e.cfg.Log.ErrorContext(ctx, "workflow reaper pass failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// ReapOnce performs one recovery pass.
func (e *Engine) ReapOnce(ctx context.Context) error {
	return db.InTx(ctx, e.pool, func(tx *sqlx.Tx) error {
		type pass struct {
			reason string
			fn     func() ([]repository.StepRef, error)
		}
		for _, p := range []pass{
			{"lease_expired", func() ([]repository.StepRef, error) {
				return repository.RequeueExpiredLeases(ctx, tx, e.cfg.ReapBatch)
			}},
			{"retry_due", func() ([]repository.StepRef, error) {
				return repository.RequeueDueRetries(ctx, tx, e.cfg.ReapBatch)
			}},
			{"redispatch", func() ([]repository.StepRef, error) {
				return repository.RedispatchStale(ctx, tx, e.cfg.ReapBatch, e.cfg.RedispatchAge)
			}},
		} {
			refs, err := p.fn()
			if err != nil {
				return fmt.Errorf("%s: %w", p.reason, err)
			}
			for _, r := range refs {
				if err := dispatch(ctx, tx, r.OrgID, r.ID); err != nil {
					return err
				}
			}
			if len(refs) > 0 {
				e.recovered.WithLabelValues(p.reason).Add(float64(len(refs)))
				e.cfg.Log.InfoContext(ctx, "workflow reaper requeued steps", "reason", p.reason, "count", len(refs))
			}
		}
		expired, err := repository.ExpireDeadlines(ctx, tx, e.cfg.ReapBatch)
		if err != nil {
			return fmt.Errorf("deadlines: %w", err)
		}
		for _, id := range expired {
			if err := repository.CancelOpenSteps(ctx, tx, id); err != nil {
				return err
			}
		}
		if len(expired) > 0 {
			e.recovered.WithLabelValues("deadline").Add(float64(len(expired)))
			e.finishedTotal.WithLabelValues(string(model.ExecTimedOut)).Add(float64(len(expired)))
		}
		return nil
	})
}
