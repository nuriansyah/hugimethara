package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// SchemaVersion is the only definition schema this engine understands.
const SchemaVersion = 1

// TriggerNodeID is the pseudo-node every graph starts from. Edges leaving it
// receive the execution input.
const TriggerNodeID = "trigger"

// Platform caps (docs/architecture/workflow-runtime.md §4 "Limits").
const (
	MaxNodes             = 200
	MaxFanOut            = 50
	DefaultMaxSteps      = 500
	MaxStepsCap          = 5000
	DefaultMaxDuration   = time.Hour
	MaxDurationCap       = 7 * 24 * time.Hour
	DefaultStepTimeout   = 30 * time.Second
	MaxStepTimeout       = 5 * time.Minute
	MaxRetryAttempts     = 10
	DefaultRetryInterval = time.Second
	MaxRetryInterval     = 10 * time.Minute
	MaxInlinePayload     = 64 << 10 // step input/output stored inline (JSONB)
	MaxExpressionLength  = 2000
)

// Definition is the declarative workflow graph stored as one immutable JSONB
// document per published version.
type Definition struct {
	SchemaVersion int     `json:"schema_version"`
	Trigger       Trigger `json:"trigger"`
	Nodes         []Node  `json:"nodes"`
	Edges         []Edge  `json:"edges"`
	Limits        Limits  `json:"limits"`
}

// Trigger starts an execution.
type Trigger struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// Node is one step of the graph.
type Node struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
	Retry     *RetryPolicy   `json:"retry,omitempty"`
	TimeoutMs int            `json:"timeout_ms,omitempty"`
	// Position is editor metadata; the engine ignores it.
	Position *Position `json:"position,omitempty"`
}

// Position is a node's location on the designer canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Edge connects two nodes. Port selects a branch of the source node
// ("true"/"false" for logic.if); empty means the default output.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Port string `json:"port,omitempty"`
}

// Limits bound one execution.
type Limits struct {
	MaxSteps           int `json:"max_steps,omitempty"`
	MaxDurationSeconds int `json:"max_duration_seconds,omitempty"`
	// MaxConcurrentExecutions caps unfinished executions of this workflow per
	// environment; 0 is unlimited. Starts beyond it are refused with
	// concurrency_limit_exceeded.
	MaxConcurrentExecutions int `json:"max_concurrent_executions,omitempty"`
}

// RetryPolicy retries a failed node with exponential or fixed backoff.
type RetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Backoff     string `json:"backoff,omitempty"` // "exponential" (default) | "fixed"
	InitialMs   int    `json:"initial_ms,omitempty"`
}

// ParseDefinition decodes a definition document.
func ParseDefinition(raw []byte) (Definition, error) {
	var d Definition
	err := json.Unmarshal(raw, &d)
	return d, err
}

// Node returns the node with id, if present.
func (d Definition) Node(id string) (Node, bool) {
	for _, n := range d.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// Inbound returns the edges that end at id.
func (d Definition) Inbound(id string) []Edge {
	var out []Edge
	for _, e := range d.Edges {
		if e.To == id {
			out = append(out, e)
		}
	}
	return out
}

// Outbound returns the edges that start at id.
func (d Definition) Outbound(id string) []Edge {
	var out []Edge
	for _, e := range d.Edges {
		if e.From == id {
			out = append(out, e)
		}
	}
	return out
}

// EffectiveMaxSteps applies the default and the platform cap.
func (l Limits) EffectiveMaxSteps() int {
	switch {
	case l.MaxSteps <= 0:
		return DefaultMaxSteps
	case l.MaxSteps > MaxStepsCap:
		return MaxStepsCap
	}
	return l.MaxSteps
}

// EffectiveMaxDuration applies the default and the platform cap.
func (l Limits) EffectiveMaxDuration() time.Duration {
	d := time.Duration(l.MaxDurationSeconds) * time.Second
	switch {
	case d <= 0:
		return DefaultMaxDuration
	case d > MaxDurationCap:
		return MaxDurationCap
	}
	return d
}

// StepTimeout is the node's execution deadline, defaulted and capped.
func (n Node) StepTimeout() time.Duration {
	d := time.Duration(n.TimeoutMs) * time.Millisecond
	switch {
	case d <= 0:
		return DefaultStepTimeout
	case d > MaxStepTimeout:
		return MaxStepTimeout
	}
	return d
}

// MaxAttempts is how many times the node may run in total (≥ 1).
func (n Node) MaxAttempts() int {
	if n.Retry == nil || n.Retry.MaxAttempts <= 1 {
		return 1
	}
	if n.Retry.MaxAttempts > MaxRetryAttempts {
		return MaxRetryAttempts
	}
	return n.Retry.MaxAttempts
}

// RetryDelay is the wait before attempt number next (2-based: the first retry
// is attempt 2).
func (n Node) RetryDelay(next int) time.Duration {
	initial := DefaultRetryInterval
	backoff := "exponential"
	if n.Retry != nil {
		if n.Retry.InitialMs > 0 {
			initial = time.Duration(n.Retry.InitialMs) * time.Millisecond
		}
		if n.Retry.Backoff != "" {
			backoff = n.Retry.Backoff
		}
	}
	d := initial
	if backoff == "exponential" {
		for i := 2; i < next && d < MaxRetryInterval; i++ {
			d *= 2
		}
	}
	if d > MaxRetryInterval {
		d = MaxRetryInterval
	}
	return d
}

// Checksum is the hex SHA-256 of the canonical JSON encoding (Go's encoder
// sorts map keys, so equal documents hash equally).
func Checksum(d Definition) (string, []byte, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}
