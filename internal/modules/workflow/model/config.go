package model

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Typed views of node and trigger configuration. Definitions stay plain JSON
// (map[string]any) so the designer can round-trip unknown keys; these parsers
// are the single interpretation shared by the validator and the engine.

// Bounds of S11 node configuration.
const (
	MaxSwitchCases        = 20
	MaxLoopIterations     = 1000
	MaxLoopConcurrency    = 10
	MaxDelay              = MaxDurationCap
	MaxTaskAssignees      = 50
	MaxFormFields         = 30
	MaxTriggerChainDepth  = 5
	MaxConcurrentExecsCap = 1000
)

var portRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// SwitchCase is one branch of a logic.switch.
type SwitchCase struct {
	Port      string `json:"port"`
	Condition string `json:"condition"`
}

// ParseSwitchCases reads config.cases.
func ParseSwitchCases(cfg map[string]any) ([]SwitchCase, error) {
	raw, ok := cfg["cases"].([]any)
	if !ok || len(raw) == 0 {
		return nil, errors.New("cases must be a non-empty array")
	}
	if len(raw) > MaxSwitchCases {
		return nil, fmt.Errorf("a switch may have at most %d cases", MaxSwitchCases)
	}
	seen := map[string]bool{}
	out := make([]SwitchCase, 0, len(raw))
	for i, c := range raw {
		m, ok := c.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cases[%d] must be an object", i)
		}
		port, _ := m["port"].(string)
		cond, _ := m["condition"].(string)
		switch {
		case !portRe.MatchString(port) || port == PortDefault:
			return nil, fmt.Errorf("cases[%d].port must be lowercase letters, digits or '_' (max 32) and not %q", i, PortDefault)
		case seen[port]:
			return nil, fmt.Errorf("cases[%d].port %q is used twice", i, port)
		case strings.TrimSpace(cond) == "":
			return nil, fmt.Errorf("cases[%d].condition is required", i)
		}
		seen[port] = true
		out = append(out, SwitchCase{Port: port, Condition: cond})
	}
	return out, nil
}

// FormField is one input of a human.input form.
type FormField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // string | text | number | boolean
	Required bool   `json:"required,omitempty"`
}

// HumanTask is the parsed configuration of human.approval / human.input.
type HumanTask struct {
	Kind              string // "approval" | "input"
	Title             string
	Instructions      string
	Users             []uuid.UUID
	Groups            []uuid.UUID
	Timeout           time.Duration // 0: waits until the execution deadline
	AllowSelfApproval bool
	Fields            []FormField
}

var fieldKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func uuidList(v any, field string) ([]uuid.UUID, error) {
	if v == nil {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("assignees.%s must be an array of ids", field)
	}
	out := make([]uuid.UUID, 0, len(raw))
	for _, x := range raw {
		s, _ := x.(string)
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("assignees.%s contains %v, which is not an id", field, x)
		}
		out = append(out, id)
	}
	return out, nil
}

// number reads a JSON number (float64) that must be a whole value.
func number(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	}
	return 0, false
}

// boolean accepts JSON booleans and the strings "true"/"false".
func boolean(v any) (bool, bool) {
	switch b := v.(type) {
	case nil:
		return false, true
	case bool:
		return b, true
	case string:
		switch b {
		case "true":
			return true, true
		case "false", "":
			return false, true
		}
	}
	return false, false
}

// ParseHumanTask reads a human.approval or human.input node's configuration.
func ParseHumanTask(n Node) (HumanTask, error) {
	c := n.Config
	t := HumanTask{Kind: "approval"}
	if n.Type == TypeHumanInput {
		t.Kind = "input"
	}
	t.Title, _ = c["title"].(string)
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" || len([]rune(t.Title)) > 200 {
		return t, errors.New("title must be 1–200 characters")
	}
	t.Instructions, _ = c["instructions"].(string)
	if len([]rune(t.Instructions)) > 2000 {
		return t, errors.New("instructions must be at most 2000 characters")
	}
	a, ok := c["assignees"].(map[string]any)
	if !ok {
		return t, errors.New(`assignees must be an object {"users": [...], "groups": [...]}`)
	}
	var err error
	if t.Users, err = uuidList(a["users"], "users"); err != nil {
		return t, err
	}
	if t.Groups, err = uuidList(a["groups"], "groups"); err != nil {
		return t, err
	}
	switch total := len(t.Users) + len(t.Groups); {
	case total == 0:
		return t, errors.New("name at least one assignee user or group")
	case total > MaxTaskAssignees:
		return t, fmt.Errorf("at most %d assignees", MaxTaskAssignees)
	}
	if v, ok := c["timeout_seconds"]; ok && v != nil {
		secs, ok := number(v)
		if !ok || secs < 1 || time.Duration(secs)*time.Second > MaxDurationCap {
			return t, fmt.Errorf("timeout_seconds must be a whole number between 1 and %d", int(MaxDurationCap.Seconds()))
		}
		t.Timeout = time.Duration(secs) * time.Second
	}
	if t.AllowSelfApproval, ok = boolean(c["allow_self_approval"]); !ok {
		return t, errors.New("allow_self_approval must be true or false")
	}
	if t.Kind == "input" {
		raw, ok := c["fields"].([]any)
		if !ok || len(raw) == 0 || len(raw) > MaxFormFields {
			return t, fmt.Errorf("fields must be an array of 1–%d form fields", MaxFormFields)
		}
		seen := map[string]bool{}
		for i, f := range raw {
			m, ok := f.(map[string]any)
			if !ok {
				return t, fmt.Errorf("fields[%d] must be an object", i)
			}
			ff := FormField{}
			ff.Key, _ = m["key"].(string)
			ff.Label, _ = m["label"].(string)
			ff.Type, _ = m["type"].(string)
			ff.Required, _ = m["required"].(bool)
			if ff.Type == "" {
				ff.Type = "string"
			}
			if ff.Label == "" {
				ff.Label = ff.Key
			}
			switch {
			case !fieldKeyRe.MatchString(ff.Key):
				return t, fmt.Errorf("fields[%d].key must be lowercase letters, digits or '_'", i)
			case seen[ff.Key]:
				return t, fmt.Errorf("fields[%d].key %q is used twice", i, ff.Key)
			case ff.Type != "string" && ff.Type != "text" && ff.Type != "number" && ff.Type != "boolean":
				return t, fmt.Errorf("fields[%d].type must be string, text, number or boolean", i)
			}
			seen[ff.Key] = true
			t.Fields = append(t.Fields, ff)
		}
	}
	return t, nil
}

// ValidateFormData checks submitted data against the form and returns the
// normalized values (unknown keys are refused, not dropped).
func ValidateFormData(fields []FormField, data map[string]any) (map[string]any, []Issue) {
	var issues []Issue
	known := map[string]FormField{}
	for _, f := range fields {
		known[f.Key] = f
	}
	for k := range data {
		if _, ok := known[k]; !ok {
			issues = append(issues, Issue{Path: "data." + k, Code: "unknown_field", Message: "This field is not part of the form."})
		}
	}
	out := map[string]any{}
	for _, f := range fields {
		v, present := data[f.Key]
		if !present || v == nil || v == "" {
			if f.Required {
				issues = append(issues, Issue{Path: "data." + f.Key, Code: "required", Message: f.Label + " is required."})
			}
			continue
		}
		ok := false
		switch f.Type {
		case "string", "text":
			s, isStr := v.(string)
			ok = isStr && len([]rune(s)) <= 10000
		case "number":
			_, ok = v.(float64)
		case "boolean":
			_, ok = v.(bool)
		}
		if !ok {
			issues = append(issues, Issue{Path: "data." + f.Key, Code: "invalid_type", Message: f.Label + " must be a " + f.Type + "."})
			continue
		}
		out[f.Key] = v
	}
	return out, issues
}

// Delay is the parsed configuration of logic.delay.
type Delay struct {
	Fixed time.Duration // used when Until is empty
	Until string        // CEL expression returning an RFC 3339 timestamp
}

// ParseDelay reads a logic.delay node's configuration.
func ParseDelay(cfg map[string]any) (Delay, error) {
	var d Delay
	d.Until, _ = cfg["until"].(string)
	d.Until = strings.TrimSpace(d.Until)
	if v, ok := cfg["seconds"]; ok && v != nil {
		secs, ok := number(v)
		if !ok || secs < 0 || time.Duration(secs)*time.Second > MaxDelay {
			return d, fmt.Errorf("seconds must be a whole number between 0 and %d", int(MaxDelay.Seconds()))
		}
		d.Fixed = time.Duration(secs) * time.Second
	} else if d.Until == "" {
		return d, errors.New("set seconds or an until expression")
	}
	return d, nil
}

// Loop is the parsed configuration of logic.loop.
type Loop struct {
	Items         string
	Body          Node // the action run per item (id = the loop's id)
	Concurrency   int
	MaxIterations int
}

// LoopBodyTypes are the actions a loop may run per item.
var LoopBodyTypes = []string{TypeActionHTTP, TypeActionTransform}

// ParseLoop reads a logic.loop node's configuration.
func ParseLoop(n Node) (Loop, error) {
	l := Loop{Concurrency: 1, MaxIterations: MaxLoopIterations}
	l.Items, _ = n.Config["items"].(string)
	body, ok := n.Config["body"].(map[string]any)
	if !ok {
		return l, errors.New(`body must be an object {"type": ..., "config": {...}}`)
	}
	bt, _ := body["type"].(string)
	allowed := false
	for _, t := range LoopBodyTypes {
		allowed = allowed || t == bt
	}
	if !allowed {
		return l, fmt.Errorf("body.type must be one of %s", strings.Join(LoopBodyTypes, ", "))
	}
	bc, _ := body["config"].(map[string]any)
	l.Body = Node{ID: n.ID, Type: bt, Config: bc, TimeoutMs: n.TimeoutMs}
	if v, ok := n.Config["concurrency"]; ok && v != nil {
		c, ok := number(v)
		if !ok || c < 1 || c > MaxLoopConcurrency {
			return l, fmt.Errorf("concurrency must be between 1 and %d", MaxLoopConcurrency)
		}
		l.Concurrency = int(c)
	}
	if v, ok := n.Config["max_iterations"]; ok && v != nil {
		m, ok := number(v)
		if !ok || m < 1 || m > MaxLoopIterations {
			return l, fmt.Errorf("max_iterations must be between 1 and %d", MaxLoopIterations)
		}
		l.MaxIterations = int(m)
	}
	return l, nil
}

// ScheduleTrigger is the parsed configuration of trigger.schedule.
type ScheduleTrigger struct {
	Cron     Cron
	Location *time.Location
	Input    map[string]any
}

// ParseScheduleTrigger reads a trigger.schedule configuration.
func ParseScheduleTrigger(cfg map[string]any) (ScheduleTrigger, error) {
	var s ScheduleTrigger
	expr, _ := cfg["cron"].(string)
	c, err := ParseCron(expr)
	if err != nil {
		return s, err
	}
	s.Cron = c
	tz, _ := cfg["timezone"].(string)
	if tz == "" {
		tz = "UTC"
	}
	if s.Location, err = time.LoadLocation(tz); err != nil {
		return s, fmt.Errorf("unknown time zone %q", tz)
	}
	if v, ok := cfg["input"]; ok && v != nil {
		m, ok := v.(map[string]any)
		if !ok {
			return s, errors.New("input must be a JSON object")
		}
		s.Input = m
	}
	return s, nil
}

// EventTrigger is the parsed configuration of trigger.event.
type EventTrigger struct {
	Event     string
	Condition string
}

// ParseEventTrigger reads a trigger.event configuration.
func ParseEventTrigger(cfg map[string]any) (EventTrigger, error) {
	var e EventTrigger
	e.Event, _ = cfg["event"].(string)
	e.Condition, _ = cfg["condition"].(string)
	for _, known := range PlatformEvents {
		if known == e.Event {
			return e, nil
		}
	}
	return e, fmt.Errorf("event must be one of %s", strings.Join(PlatformEvents, ", "))
}
