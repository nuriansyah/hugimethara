package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
)

// S11 node executors: switch, loop, delay and human tasks.

type switchNode struct{ x *Expressions }

func (switchNode) Type() string { return model.TypeLogicSwitch }
func (n switchNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	cases, err := model.ParseSwitchCases(c.Node.Config)
	if err != nil {
		return NodeResult{}, failure(model.ReasonInvalidConfig, "%v", err)
	}
	vars := c.vars()
	for _, sc := range cases {
		ok, err := n.x.EvalBool(sc.Condition, vars)
		if err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "case %q: %v", sc.Port, err)
		}
		if ok {
			return NodeResult{Output: c.Input, Port: sc.Port}, nil
		}
	}
	return NodeResult{Output: c.Input, Port: model.PortDefault}, nil
}

// loopNode runs one action per array item with bounded concurrency. Each item
// gets its own idempotency key ({execution}:{node}:{index}:{attempt}), so a
// retried loop lets idempotent upstreams deduplicate the items that already
// succeeded. The node's timeout bounds the whole loop.
type loopNode struct {
	x    *Expressions
	body Registry
}

func (loopNode) Type() string { return model.TypeLogicLoop }
func (n loopNode) Execute(ctx context.Context, c NodeContext) (NodeResult, error) {
	l, err := model.ParseLoop(c.Node)
	if err != nil {
		return NodeResult{}, failure(model.ReasonInvalidConfig, "%v", err)
	}
	items := c.Input
	if strings.TrimSpace(l.Items) != "" {
		if items, err = n.x.Eval(l.Items, c.vars()); err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "items: %v", err)
		}
	}
	list, ok := items.([]any)
	if !ok {
		return NodeResult{}, failure(model.ReasonNodeFailed, "loop items are not an array")
	}
	if len(list) > l.MaxIterations {
		return NodeResult{}, failure(model.ReasonLoopLimit,
			"The loop has %d items; the limit is %d (max_iterations).", len(list), l.MaxIterations)
	}
	exec, ok := n.body[l.Body.Type]
	if !ok {
		return NodeResult{}, failure(model.ReasonInvalidState, "no executor for loop body %s", l.Body.Type)
	}
	results := make([]any, len(list))
	errs := make([]error, len(list))
	sem := make(chan struct{}, l.Concurrency)
	var wg sync.WaitGroup
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i, item := range list {
		sem <- struct{}{}
		if runCtx.Err() != nil {
			<-sem
			break
		}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			ic := c
			ic.Node = l.Body
			ic.Input = item
			ic.IdempotencyKey = fmt.Sprintf("%s:%s:%d:%d", c.ExecutionID, c.Node.ID, i, c.Attempt)
			res, err := exec.Execute(runCtx, ic)
			if err != nil {
				errs[i] = err
				cancel() // stop scheduling; the step fails as a whole
				return
			}
			results[i] = res.Output
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err == nil {
			continue
		}
		var se *model.StepError
		if errors.As(err, &se) {
			cp := *se
			cp.Message = fmt.Sprintf("item %d: %s", i, se.Message)
			return NodeResult{Output: results}, &cp
		}
		return NodeResult{Output: results}, failure(model.ReasonNodeFailed, "item %d: %v", i, err)
	}
	if ctx.Err() != nil {
		return NodeResult{}, retryable(model.ReasonStepTimeout, "The loop did not finish within the node timeout.")
	}
	return NodeResult{Output: results}, nil
}

type delayNode struct{ x *Expressions }

func (delayNode) Type() string { return model.TypeLogicDelay }
func (n delayNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	d, err := model.ParseDelay(c.Node.Config)
	if err != nil {
		return NodeResult{}, failure(model.ReasonInvalidConfig, "%v", err)
	}
	wake := c.Now.Add(d.Fixed)
	if d.Until != "" {
		v, err := n.x.Eval(d.Until, c.vars())
		if err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "until: %v", err)
		}
		s, _ := v.(string)
		if wake, err = time.Parse(time.RFC3339, s); err != nil {
			return NodeResult{}, failure(model.ReasonNodeFailed, "until did not return an RFC 3339 timestamp")
		}
		if wake.Sub(c.Now) > model.MaxDelay {
			return NodeResult{}, failure(model.ReasonNodeFailed, "until is more than %s away", model.MaxDelay)
		}
	}
	if !wake.After(c.Now) {
		return NodeResult{Output: c.Input}, nil
	}
	return NodeResult{Suspend: &Suspension{WakeAt: &wake}}, nil
}

type humanNode struct{ typ string }

func (h humanNode) Type() string { return h.typ }
func (h humanNode) Execute(_ context.Context, c NodeContext) (NodeResult, error) {
	t, err := model.ParseHumanTask(c.Node)
	if err != nil {
		return NodeResult{}, failure(model.ReasonInvalidConfig, "%v", err)
	}
	s := &Suspension{Task: &t}
	if t.Timeout > 0 {
		due := c.Now.Add(t.Timeout)
		s.WakeAt = &due
	}
	return NodeResult{Suspend: s}, nil
}
