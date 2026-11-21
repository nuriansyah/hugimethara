package services

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"cel.dev/cel-go/cel"
	"google.golang.org/protobuf/types/known/structpb"
)

// celCostLimit bounds the work one expression may do (runaway list
// comprehensions are refused instead of pinning a worker).
const celCostLimit = 1_000_000

// Expressions compiles and evaluates CEL (ADR-0010) against the workflow
// context: input (the node's input), trigger (the execution input), nodes
// ({id: {output}}) and, inside logic.filter, item. Compiled programs are
// cached; the cache is bounded by the number of distinct expressions in
// published versions.
type Expressions struct {
	env   *cel.Env
	mu    sync.RWMutex
	cache map[string]cel.Program
}

// NewExpressions builds the shared CEL environment.
func NewExpressions() (*Expressions, error) {
	env, err := cel.NewEnv(
		cel.Variable("input", cel.DynType),
		cel.Variable("trigger", cel.DynType),
		cel.Variable("nodes", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("item", cel.DynType),
	)
	if err != nil {
		return nil, err
	}
	return &Expressions{env: env, cache: map[string]cel.Program{}}, nil
}

// Check compiles expr; it is the model.ExpressionChecker used at publish.
func (x *Expressions) Check(expr string) error {
	_, err := x.program(expr)
	return err
}

func (x *Expressions) program(expr string) (cel.Program, error) {
	x.mu.RLock()
	p, ok := x.cache[expr]
	x.mu.RUnlock()
	if ok {
		return p, nil
	}
	ast, issues := x.env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	p, err := x.env.Program(ast, cel.CostLimit(celCostLimit))
	if err != nil {
		return nil, err
	}
	x.mu.Lock()
	x.cache[expr] = p
	x.mu.Unlock()
	return p, nil
}

// Eval evaluates expr and converts the result to plain JSON values.
func (x *Expressions) Eval(expr string, vars map[string]any) (any, error) {
	p, err := x.program(expr)
	if err != nil {
		return nil, err
	}
	for _, k := range []string{"input", "trigger", "item"} {
		if _, ok := vars[k]; !ok {
			vars[k] = nil
		}
	}
	if _, ok := vars["nodes"]; !ok {
		vars["nodes"] = map[string]any{}
	}
	out, _, err := p.Eval(vars)
	if err != nil {
		return nil, err
	}
	native, err := out.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		return nil, fmt.Errorf("expression result is not JSON-compatible: %w", err)
	}
	return native.(*structpb.Value).AsInterface(), nil
}

// EvalBool evaluates expr and requires a boolean result.
func (x *Expressions) EvalBool(expr string, vars map[string]any) (bool, error) {
	v, err := x.Eval(expr, vars)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, errors.New("condition did not evaluate to a boolean")
	}
	return b, nil
}
