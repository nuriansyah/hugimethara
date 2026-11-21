package model

// Node type identifiers (workflow-runtime.md §2).
const (
	TypeTriggerManual     = "trigger.manual"
	TypeTriggerSchedule   = "trigger.schedule"
	TypeTriggerWebhook    = "trigger.webhook"
	TypeTriggerEvent      = "trigger.event"
	TypeActionHTTP        = "action.http"
	TypeActionConnector   = "action.connector"
	TypeActionTransform   = "action.transform"
	TypeActionComposition = "action.composition"
	TypeLogicIf           = "logic.if"
	TypeLogicParallel     = "logic.parallel"
	TypeLogicMerge        = "logic.merge"
	TypeLogicFilter       = "logic.filter"
)

// Ports of branching nodes.
const (
	PortTrue  = "true"
	PortFalse = "false"
)

// NodeKind groups node types in the designer palette.
type NodeKind string

const (
	KindTrigger NodeKind = "trigger"
	KindAction  NodeKind = "action"
	KindLogic   NodeKind = "logic"
)

// ConfigField describes one configuration field for the designer's form.
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"` // string | text | expression | json | select | number
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

// NodeType is a catalog entry. Unavailable types are listed so the designer can
// show them, but validation rejects them with a reason code until their
// runtime ships.
type NodeType struct {
	Type        string        `json:"type"`
	Kind        NodeKind      `json:"kind"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Ports       []string      `json:"ports"` // empty: single default output
	Available   bool          `json:"available"`
	Unavailable string        `json:"unavailable_reason,omitempty"`
	Fields      []ConfigField `json:"fields"`
}

// Catalog is the ordered node-type catalog (GET /node-types).
func Catalog() []NodeType {
	return []NodeType{
		{Type: TypeTriggerManual, Kind: KindTrigger, Label: "Manual trigger", Available: true,
			Description: "Started from the API or the designer with a JSON input."},
		{Type: TypeTriggerSchedule, Kind: KindTrigger, Label: "Schedule", Unavailable: "planned for S11",
			Description: "Cron schedule in a time zone."},
		{Type: TypeTriggerWebhook, Kind: KindTrigger, Label: "Webhook", Unavailable: "planned for S11/S12",
			Description: "Inbound signed webhook."},
		{Type: TypeTriggerEvent, Kind: KindTrigger, Label: "Platform event", Unavailable: "planned for S11",
			Description: "Reacts to a platform domain event."},
		{Type: TypeActionHTTP, Kind: KindAction, Label: "HTTP request", Available: true,
			Description: "Calls an HTTP endpoint through the environment's egress policy.",
			Fields: []ConfigField{
				{Key: "method", Label: "Method", Kind: "select", Required: true,
					Options: []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
				{Key: "url", Label: "URL", Kind: "string", Required: true, Description: "http(s) URL."},
				{Key: "headers", Label: "Headers", Kind: "json", Description: "Object of header values."},
				{Key: "body", Label: "Body", Kind: "json", Description: "Static JSON body."},
				{Key: "body_expression", Label: "Body expression", Kind: "expression",
					Description: "CEL expression evaluated against input, trigger and nodes; overrides body."},
			}},
		{Type: TypeActionTransform, Kind: KindAction, Label: "Transform", Available: true,
			Description: "Reshapes the input with mapping rules (JSONPath, transforms, CEL).",
			Fields:      []ConfigField{{Key: "rules", Label: "Mapping rules", Kind: "json", Required: true}}},
		{Type: TypeActionConnector, Kind: KindAction, Label: "Connector operation",
			Unavailable: "connector invocation runtime is not implemented yet (S6)",
			Description: "Invokes an operation of a configured connector instance."},
		{Type: TypeActionComposition, Kind: KindAction, Label: "API composition",
			Unavailable: "system-principal composition invocation is not implemented yet",
			Description: "Runs a published API composition."},
		{Type: TypeLogicIf, Kind: KindLogic, Label: "If", Available: true, Ports: []string{PortTrue, PortFalse},
			Description: "Routes to the true or false branch; the other branch is skipped.",
			Fields:      []ConfigField{{Key: "condition", Label: "Condition", Kind: "expression", Required: true}}},
		{Type: TypeLogicParallel, Kind: KindLogic, Label: "Parallel", Available: true,
			Description: "Fans the input out to every outgoing branch concurrently."},
		{Type: TypeLogicMerge, Kind: KindLogic, Label: "Merge", Available: true,
			Description: "Joins branches; output is an object keyed by source node id.",
			Fields: []ConfigField{{Key: "mode", Label: "Mode", Kind: "select", Options: []string{"all", "any"},
				Description: "all: wait for every active branch; any: continue on the first."}}},
		{Type: TypeLogicFilter, Kind: KindLogic, Label: "Filter", Available: true,
			Description: "Keeps the array items for which the condition holds (item is bound per element).",
			Fields: []ConfigField{
				{Key: "items", Label: "Items expression", Kind: "expression",
					Description: "CEL expression returning the array; defaults to input."},
				{Key: "condition", Label: "Condition", Kind: "expression", Required: true},
			}},
	}
}

// LookupNodeType returns the catalog entry for t.
func LookupNodeType(t string) (NodeType, bool) {
	for _, nt := range Catalog() {
		if nt.Type == t {
			return nt, true
		}
	}
	return NodeType{}, false
}
