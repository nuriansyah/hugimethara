package model

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Issue is one validation finding with a stable machine-readable code.
type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ExpressionChecker compiles an expression and reports syntax/type errors. It
// is injected so the model stays free of the expression engine.
type ExpressionChecker func(expr string) error

var nodeIDRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// Validate checks a definition for publishing (workflow-runtime.md §1
// "Publish-time validation"). It returns every issue found, sorted by path, so
// the designer can show them all at once. A nil result means publishable.
func Validate(d Definition, checkExpr ExpressionChecker) []Issue {
	v := &validator{def: d, check: checkExpr}
	v.run()
	sort.SliceStable(v.issues, func(i, j int) bool { return v.issues[i].Path < v.issues[j].Path })
	return v.issues
}

type validator struct {
	def    Definition
	check  ExpressionChecker
	issues []Issue
	nodes  map[string]Node
}

func (v *validator) add(path, code, format string, args ...any) {
	v.issues = append(v.issues, Issue{Path: path, Code: code, Message: fmt.Sprintf(format, args...)})
}

func (v *validator) run() {
	d := v.def
	if d.SchemaVersion != SchemaVersion {
		v.add("schema_version", "unsupported_schema_version", "schema_version must be %d.", SchemaVersion)
	}
	v.trigger()
	v.nodeSet()
	v.edgeSet()
	if len(v.issues) > 0 {
		return // graph checks need a structurally sound document
	}
	v.graph()
	v.limits()
}

func (v *validator) trigger() {
	t, ok := LookupNodeType(v.def.Trigger.Type)
	switch {
	case !ok || t.Kind != KindTrigger:
		v.add("trigger.type", "unknown_trigger", "Unknown trigger type %q.", v.def.Trigger.Type)
	case !t.Available:
		v.add("trigger.type", "trigger_unavailable", "Trigger %q is not available yet (%s).", t.Type, t.Unavailable)
	}
	cfg := v.def.Trigger.Config
	switch v.def.Trigger.Type {
	case TypeTriggerSchedule:
		if _, err := ParseScheduleTrigger(cfg); err != nil {
			v.add("trigger.config", "invalid_schedule", "%s.", err.Error())
		}
	case TypeTriggerEvent:
		if _, err := ParseEventTrigger(cfg); err != nil {
			v.add("trigger.config.event", "invalid_event", "%s.", err.Error())
		}
		v.expr("trigger.config.condition", cfg["condition"], false)
	case TypeTriggerWebhook:
		if _, err := ParseWebhookTrigger(cfg); err != nil {
			v.add("trigger.config.endpoint_id", "invalid_endpoint", "%s.", err.Error())
		}
		v.expr("trigger.config.condition", cfg["condition"], false)
	}
}

func (v *validator) nodeSet() {
	v.nodes = map[string]Node{}
	if len(v.def.Nodes) == 0 {
		v.add("nodes", "empty_workflow", "Add at least one node.")
	}
	if len(v.def.Nodes) > MaxNodes {
		v.add("nodes", "too_many_nodes", "A workflow may have at most %d nodes.", MaxNodes)
	}
	for i, n := range v.def.Nodes {
		p := fmt.Sprintf("nodes[%d]", i)
		if !nodeIDRe.MatchString(n.ID) || n.ID == TriggerNodeID {
			v.add(p+".id", "invalid_node_id",
				"Node ids start with a letter, use letters, digits, '_' or '-', are at most 64 characters and are not %q.", TriggerNodeID)
			continue
		}
		if _, dup := v.nodes[n.ID]; dup {
			v.add(p+".id", "duplicate_node_id", "Node id %q is used more than once.", n.ID)
			continue
		}
		v.nodes[n.ID] = n
		t, ok := LookupNodeType(n.Type)
		switch {
		case !ok || t.Kind == KindTrigger:
			v.add(p+".type", "unknown_node_type", "Unknown node type %q.", n.Type)
			continue
		case !t.Available:
			v.add(p+".type", "node_type_unavailable", "Node type %q is not available yet (%s).", n.Type, t.Unavailable)
			continue
		}
		if n.TimeoutMs < 0 || n.TimeoutMs > int(MaxStepTimeout.Milliseconds()) {
			v.add(p+".timeout_ms", "invalid_timeout", "timeout_ms must be between 0 and %d.", MaxStepTimeout.Milliseconds())
		}
		if r := n.Retry; r != nil {
			if r.MaxAttempts < 1 || r.MaxAttempts > MaxRetryAttempts {
				v.add(p+".retry.max_attempts", "invalid_retry", "max_attempts must be between 1 and %d.", MaxRetryAttempts)
			}
			if r.Backoff != "" && r.Backoff != "exponential" && r.Backoff != "fixed" {
				v.add(p+".retry.backoff", "invalid_retry", "backoff must be exponential or fixed.")
			}
			if r.InitialMs < 0 || r.InitialMs > int(MaxRetryInterval.Milliseconds()) {
				v.add(p+".retry.initial_ms", "invalid_retry", "initial_ms must be between 0 and %d.", MaxRetryInterval.Milliseconds())
			}
		}
		v.config(p+".config", n)
	}
}

func (v *validator) expr(path string, raw any, required bool) {
	s, _ := raw.(string)
	s = strings.TrimSpace(s)
	if raw != nil {
		if _, ok := raw.(string); !ok {
			v.add(path, "invalid_expression", "Expressions must be strings.")
			return
		}
	}
	if s == "" {
		if required {
			v.add(path, "required", "This expression is required.")
		}
		return
	}
	if len(s) > MaxExpressionLength {
		v.add(path, "expression_too_long", "Expressions are limited to %d characters.", MaxExpressionLength)
		return
	}
	if v.check != nil {
		if err := v.check(s); err != nil {
			v.add(path, "invalid_expression", "%s", err.Error())
		}
	}
}

func (v *validator) config(path string, n Node) {
	c := n.Config
	switch n.Type {
	case TypeActionHTTP:
		method, _ := c["method"].(string)
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			v.add(path+".method", "invalid_method", "method must be GET, POST, PUT, PATCH or DELETE.")
		}
		raw, _ := c["url"].(string)
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			v.add(path+".url", "invalid_url", "url must be an absolute http or https URL.")
		} else if u.User != nil {
			v.add(path+".url", "invalid_url", "Put credentials in headers from the vault, not in the URL.")
		}
		if h, ok := c["headers"]; ok && h != nil {
			hm, ok := h.(map[string]any)
			if !ok {
				v.add(path+".headers", "invalid_headers", "headers must be an object of strings.")
			}
			for k, val := range hm {
				if _, ok := val.(string); !ok || strings.ContainsAny(k, "\r\n:") || k == "" {
					v.add(path+".headers", "invalid_headers", "Header %q must be a valid name with a string value.", k)
				}
			}
		}
		v.expr(path+".body_expression", c["body_expression"], false)
	case TypeActionTransform:
		if _, ok := c["rules"].([]any); !ok {
			v.add(path+".rules", "required", "rules must be an array of mapping rules.")
		}
	case TypeLogicIf:
		v.expr(path+".condition", c["condition"], true)
	case TypeLogicFilter:
		v.expr(path+".items", c["items"], false)
		v.expr(path+".condition", c["condition"], true)
	case TypeLogicMerge:
		if m, ok := c["mode"]; ok && m != "all" && m != "any" && m != nil && m != "" {
			v.add(path+".mode", "invalid_mode", "mode must be all or any.")
		}
	case TypeLogicSwitch:
		cases, err := ParseSwitchCases(c)
		if err != nil {
			v.add(path+".cases", "invalid_cases", "%s.", err.Error())
		}
		for i, sc := range cases {
			v.expr(fmt.Sprintf("%s.cases[%d].condition", path, i), sc.Condition, true)
		}
	case TypeLogicLoop:
		l, err := ParseLoop(n)
		if err != nil {
			v.add(path, "invalid_loop", "%s.", err.Error())
			return
		}
		v.expr(path+".items", c["items"], false)
		v.config(path+".body.config", l.Body)
	case TypeLogicDelay:
		d, err := ParseDelay(c)
		if err != nil {
			v.add(path, "invalid_delay", "%s.", err.Error())
			return
		}
		v.expr(path+".until", c["until"], false)
		if d.Until == "" && d.Fixed > v.def.Limits.EffectiveMaxDuration() {
			v.add(path+".seconds", "timeout_exceeds_execution_limit",
				"The delay is longer than the execution may run (limits.max_duration_seconds = %d).",
				int(v.def.Limits.EffectiveMaxDuration().Seconds()))
		}
	case TypeHumanApproval, TypeHumanInput:
		t, err := ParseHumanTask(n)
		if err != nil {
			v.add(path, "invalid_task", "%s.", err.Error())
			return
		}
		if t.Timeout > v.def.Limits.EffectiveMaxDuration() {
			v.add(path+".timeout_seconds", "timeout_exceeds_execution_limit",
				"The task timeout is longer than the execution may run (limits.max_duration_seconds = %d).",
				int(v.def.Limits.EffectiveMaxDuration().Seconds()))
		}
	}
}

func (v *validator) edgeSet() {
	seen := map[Edge]bool{}
	for i, e := range v.def.Edges {
		p := fmt.Sprintf("edges[%d]", i)
		if e.From != TriggerNodeID {
			if _, ok := v.nodes[e.From]; !ok {
				v.add(p+".from", "unknown_node", "Edge source %q does not exist.", e.From)
				continue
			}
		}
		if _, ok := v.nodes[e.To]; !ok {
			v.add(p+".to", "unknown_node", "Edge target %q does not exist.", e.To)
			continue
		}
		if e.From == e.To {
			v.add(p, "self_loop", "A node cannot connect to itself; use a loop container.")
		}
		if seen[e] {
			v.add(p, "duplicate_edge", "This edge is defined more than once.")
		}
		seen[e] = true
		var ports []string
		if e.From != TriggerNodeID {
			ports = NodePorts(v.nodes[e.From])
			if e.Port == PortTimeout && isHuman(v.nodes[e.From].Type) {
				if t, err := ParseHumanTask(v.nodes[e.From]); err == nil && t.Timeout == 0 {
					v.add(p+".port", "timeout_not_configured",
						"Node %q has no timeout_seconds, so its timeout branch can never run.", e.From)
				}
			}
		}
		if !validPort(ports, e.Port) {
			if len(ports) == 0 {
				v.add(p+".port", "invalid_port", "Node %q has a single output; remove the port.", e.From)
			} else {
				v.add(p+".port", "invalid_port", "Port must be one of %s.", strings.Join(ports, ", "))
			}
		}
	}
}

func isHuman(t string) bool { return t == TypeHumanApproval || t == TypeHumanInput }

func validPort(ports []string, port string) bool {
	if len(ports) == 0 {
		return port == ""
	}
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

func (v *validator) graph() {
	d := v.def
	if len(d.Outbound(TriggerNodeID)) == 0 {
		v.add("edges", "no_entry", "Connect the trigger to at least one node.")
		return
	}
	for id, n := range v.nodes {
		in := d.Inbound(id)
		out := d.Outbound(id)
		switch {
		case n.Type == TypeLogicMerge && len(in) < 2:
			v.add("nodes."+id, "merge_needs_inputs", "Merge %q needs at least two incoming edges.", id)
		case n.Type != TypeLogicMerge && len(in) > 1:
			v.add("nodes."+id, "join_requires_merge", "Node %q has %d incoming edges; join branches with a merge node.", id, len(in))
		}
		if len(out) > MaxFanOut {
			v.add("nodes."+id, "too_many_branches", "Node %q fans out to more than %d branches.", id, MaxFanOut)
		}
	}
	// Acyclic (Kahn) and reachable from the trigger.
	indeg := map[string]int{}
	for id := range v.nodes {
		indeg[id] = 0
	}
	for _, e := range d.Edges {
		if e.From != TriggerNodeID {
			indeg[e.To]++
		}
	}
	queue := []string{}
	for id, n := range indeg {
		if n == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, e := range d.Outbound(id) {
			indeg[e.To]--
			if indeg[e.To] == 0 {
				queue = append(queue, e.To)
			}
		}
	}
	if visited != len(v.nodes) {
		v.add("edges", "cycle", "The graph has a cycle; loops must use an explicit loop container.")
		return
	}
	reach := map[string]bool{}
	stack := []string{TriggerNodeID}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, e := range d.Outbound(id) {
			if !reach[e.To] {
				reach[e.To] = true
				stack = append(stack, e.To)
			}
		}
	}
	ids := make([]string, 0, len(v.nodes))
	for id := range v.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !reach[id] {
			v.add("nodes."+id, "unreachable", "Node %q is not reachable from the trigger.", id)
		}
	}
}

func (v *validator) limits() {
	l := v.def.Limits
	if l.MaxSteps < 0 || l.MaxSteps > MaxStepsCap {
		v.add("limits.max_steps", "limit_out_of_range", "max_steps must be between 1 and %d.", MaxStepsCap)
	}
	if l.MaxDurationSeconds < 0 || l.MaxDurationSeconds > int(MaxDurationCap.Seconds()) {
		v.add("limits.max_duration_seconds", "limit_out_of_range",
			"max_duration_seconds must be between 1 and %d.", int(MaxDurationCap.Seconds()))
	}
	if l.MaxConcurrentExecutions < 0 || l.MaxConcurrentExecutions > MaxConcurrentExecsCap {
		v.add("limits.max_concurrent_executions", "limit_out_of_range",
			"max_concurrent_executions must be between 0 (unlimited) and %d.", MaxConcurrentExecsCap)
	}
	if l.MaxSteps > 0 && l.MaxSteps < len(v.def.Nodes) {
		v.add("limits.max_steps", "limit_below_graph", "max_steps (%d) is smaller than the number of nodes (%d).",
			l.MaxSteps, len(v.def.Nodes))
	}
}
