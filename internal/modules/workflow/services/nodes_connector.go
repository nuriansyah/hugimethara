package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	connectorsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/services"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
)

// StepScope is the tenant scope of a step, given to executors that load
// tenant data before they run.
type StepScope struct {
	OrgID         uuid.UUID
	WorkspaceID   uuid.UUID
	EnvironmentID uuid.UUID
}

// Preparer is implemented by executors that need tenant data resolved before
// Execute (which never touches the database). Prepare runs in a short
// tenant-bound transaction; its result is NodeContext.Prepared and release,
// when non-nil, runs after Execute (e.g. to zero a credential).
type Preparer interface {
	Prepare(ctx context.Context, tx *sqlx.Tx, scope StepScope, n model.Node) (prepared any, release func(), err error)
}

// WithConnectors registers action.connector backed by rt.
func (r Registry) WithConnectors(x *Expressions, rt *connectorsvc.Runtime) Registry {
	if rt != nil {
		r[model.TypeActionConnector] = connectorNode{x: x, rt: rt}
	}
	return r
}

type connectorNode struct {
	x  *Expressions
	rt *connectorsvc.Runtime
}

func (connectorNode) Type() string { return model.TypeActionConnector }

func (n connectorNode) Prepare(ctx context.Context, tx *sqlx.Tx, scope StepScope, node model.Node) (any, func(), error) {
	a, err := model.ParseConnectorAction(node.Config)
	if err != nil {
		return nil, nil, failure(model.ReasonInvalidConfig, "%v", err)
	}
	b, err := n.rt.Resolve(ctx, tx, scope.OrgID, scope.WorkspaceID, scope.EnvironmentID, a.Connector)
	if errors.Is(err, connectorsvc.ErrBindingNotFound) {
		return nil, nil, failure(connectorsvc.CodeNotFound,
			"No connector instance %q exists in this workspace for the execution's environment.", a.Connector)
	}
	if err != nil {
		return nil, nil, retryable(model.ReasonNodeFailed, "The connector could not be resolved.")
	}
	return &b, b.Release, nil
}

func (n connectorNode) Execute(ctx context.Context, c NodeContext) (NodeResult, error) {
	a, err := model.ParseConnectorAction(c.Node.Config)
	if err != nil {
		return NodeResult{}, failure(model.ReasonInvalidConfig, "%v", err)
	}
	b, ok := c.Prepared.(*connectorsvc.Binding)
	if !ok || b == nil {
		return NodeResult{}, failure(model.ReasonInvalidState, "the connector was not resolved")
	}
	input := c.Input
	if v, ok := c.Node.Config["input"]; ok && v != nil {
		input = v
	}
	if expr, _ := c.Node.Config["input_expression"].(string); strings.TrimSpace(expr) != "" {
		if input, err = n.x.Eval(expr, c.vars()); err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "input_expression: %v", err)
		}
	}
	res, err := n.rt.Invoke(ctx, *b, connectorsvc.InvokeRequest{
		Operation: a.Operation, Input: input, IdempotencyKey: c.IdempotencyKey, Egress: c.Egress,
		Headers: map[string]string{"X-Hugi-Execution-Id": c.ExecutionID.String(), "X-Hugi-Node-Id": c.Node.ID},
	})
	var out any
	if res.Status != 0 {
		out = map[string]any{"connector": a.Connector, "operation": a.Operation, "status": res.Status, "body": res.Body}
	}
	if err != nil {
		var ie *connectorsvc.InvokeError
		if errors.As(err, &ie) {
			return NodeResult{Output: out}, &model.StepError{Code: ie.Code, Message: ie.Message, Retryable: ie.Retryable}
		}
		return NodeResult{Output: out}, retryable(model.ReasonNodeFailed, "connector call failed")
	}
	return NodeResult{Output: out}, nil
}
