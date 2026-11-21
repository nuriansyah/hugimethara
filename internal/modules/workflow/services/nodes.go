package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	mappingsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/services"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
)

// NodeContext is everything a node may read. Nodes are pure functions of it
// plus their external side effect; they never touch the database.
type NodeContext struct {
	Node           model.Node
	ExecutionID    uuid.UUID
	Attempt        int
	Input          any            // output of the active upstream node(s)
	Trigger        any            // execution input
	Outputs        map[string]any // outputs of succeeded nodes, by id
	Egress         egress.Policy  // the execution's environment policy snapshot
	IdempotencyKey string         // {execution}:{node}:{iteration}:{attempt}
}

// vars is the CEL activation for this node.
func (c NodeContext) vars() map[string]any {
	nodes := make(map[string]any, len(c.Outputs))
	for id, out := range c.Outputs {
		nodes[id] = map[string]any{"output": out}
	}
	return map[string]any{"input": c.Input, "trigger": c.Trigger, "nodes": nodes}
}

// NodeResult is a node's output and, for branching nodes, the chosen port.
type NodeResult struct {
	Output any
	Port   string
}

// NodeExecutor runs one node type (workflow-runtime.md §2).
type NodeExecutor interface {
	Type() string
	Execute(ctx context.Context, c NodeContext) (NodeResult, error)
}

// Registry maps node types to executors.
type Registry map[string]NodeExecutor

// NewRegistry returns the executors for every available S10 node type.
func NewRegistry(x *Expressions) Registry {
	r := Registry{}
	for _, n := range []NodeExecutor{
		httpNode{x}, transformNode{}, ifNode{x}, parallelNode{}, mergeNode{}, filterNode{x},
	} {
		r[n.Type()] = n
	}
	return r
}

func failure(code, format string, args ...any) *model.StepError {
	return &model.StepError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func retryable(code, format string, args ...any) *model.StepError {
	e := failure(code, format, args...)
	e.Retryable = true
	return e
}

// ---- logic ----------------------------------------------------------------------

type ifNode struct{ x *Expressions }

func (ifNode) Type() string { return model.TypeLogicIf }
func (n ifNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	cond, _ := c.Node.Config["condition"].(string)
	ok, err := n.x.EvalBool(cond, c.vars())
	if err != nil {
		return NodeResult{}, failure(model.ReasonNodeFailed, "condition: %v", err)
	}
	port := model.PortFalse
	if ok {
		port = model.PortTrue
	}
	return NodeResult{Output: c.Input, Port: port}, nil
}

type parallelNode struct{}

func (parallelNode) Type() string { return model.TypeLogicParallel }
func (parallelNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	return NodeResult{Output: c.Input}, nil
}

type mergeNode struct{}

func (mergeNode) Type() string { return model.TypeLogicMerge }
func (mergeNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	return NodeResult{Output: c.Input}, nil // the engine already keyed inputs by source
}

type filterNode struct{ x *Expressions }

func (filterNode) Type() string { return model.TypeLogicFilter }
func (n filterNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	vars := c.vars()
	items := c.Input
	if expr, _ := c.Node.Config["items"].(string); strings.TrimSpace(expr) != "" {
		v, err := n.x.Eval(expr, vars)
		if err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "items: %v", err)
		}
		items = v
	}
	list, ok := items.([]any)
	if !ok {
		return NodeResult{}, failure(model.ReasonNodeFailed, "filter input is not an array")
	}
	cond, _ := c.Node.Config["condition"].(string)
	out := make([]any, 0, len(list))
	for _, item := range list {
		vars["item"] = item
		keep, err := n.x.EvalBool(cond, vars)
		if err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "condition: %v", err)
		}
		if keep {
			out = append(out, item)
		}
	}
	return NodeResult{Output: out}, nil
}

// ---- actions --------------------------------------------------------------------

type transformNode struct{}

func (transformNode) Type() string { return model.TypeActionTransform }
func (transformNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	rules, _ := c.Node.Config["rules"].([]any)
	out, err := mappingsvc.Apply(map[string]any{"rules": rules}, c.Input)
	if err != nil {
		return NodeResult{}, failure(model.ReasonNodeFailed, "transform: %v", err)
	}
	return NodeResult{Output: out}, nil
}

// maxHTTPResponse bounds what an HTTP node reads; anything larger fails the
// step instead of being truncated silently.
const maxHTTPResponse = 1 << 20

type httpNode struct{ x *Expressions }

func (httpNode) Type() string { return model.TypeActionHTTP }

func (n httpNode) Execute(ctx context.Context, c NodeContext) (NodeResult, error) {
	cfg := c.Node.Config
	method, _ := cfg["method"].(string)
	target, _ := cfg["url"].(string)

	var body io.Reader
	payload := cfg["body"]
	if expr, _ := cfg["body_expression"].(string); strings.TrimSpace(expr) != "" {
		var err error
		if payload, err = n.x.Eval(expr, c.vars()); err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "body_expression: %v", err)
		}
	}
	if payload != nil && method != http.MethodGet && method != http.MethodDelete {
		raw, err := json.Marshal(payload)
		if err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "body is not JSON-serializable")
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return NodeResult{}, failure(model.ReasonNodeFailed, "build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Idempotency-Key", c.IdempotencyKey)
	if hs, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range hs {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}
	deadline, _ := ctx.Deadline()
	client := egress.NewClient(egress.Options{Policy: c.Egress, Timeout: time.Until(deadline),
		MaxResponseBytes: maxHTTPResponse})
	resp, err := client.Do(req)
	if err != nil {
		switch {
		case egress.IsBlocked(err):
			return NodeResult{}, failure(model.ReasonEgressBlocked,
				"The destination is not allowed by the environment's egress policy.")
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
			return NodeResult{}, retryable(model.ReasonStepTimeout, "The request did not complete in time.")
		}
		return NodeResult{}, retryable(model.ReasonNodeFailed, "request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponse+1))
	if err != nil {
		return NodeResult{}, retryable(model.ReasonNodeFailed, "read response: %v", err)
	}
	if len(raw) > maxHTTPResponse {
		return NodeResult{}, failure(model.ReasonPayloadTooLarge, "The response exceeds %d bytes.", maxHTTPResponse)
	}
	var parsed any = string(raw)
	if mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); strings.HasSuffix(mt, "json") && len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = string(raw)
		}
	}
	out := map[string]any{"status": resp.StatusCode, "body": parsed,
		"headers": map[string]any{"content-type": resp.Header.Get("Content-Type")}}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return NodeResult{Output: out}, retryable(model.ReasonNodeFailed, "upstream answered %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		return NodeResult{Output: out}, failure(model.ReasonNodeFailed, "upstream answered %d", resp.StatusCode)
	}
	return NodeResult{Output: out}, nil
}
