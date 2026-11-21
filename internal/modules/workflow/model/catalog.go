package model

// Node type identifiers (workflow-runtime.md §2).
const (
	TypeTriggerManual      = "trigger.manual"
	TypeTriggerSchedule    = "trigger.schedule"
	TypeTriggerWebhook     = "trigger.webhook"
	TypeTriggerEvent       = "trigger.event"
	TypeActionHTTP         = "action.http"
	TypeActionConnector    = "action.connector"
	TypeActionTransform    = "action.transform"
	TypeActionComposition  = "action.composition"
	TypeActionDBQuery      = "action.db_query"
	TypeActionNotify       = "action.notify"
	TypeActionRecordCreate = "action.record_create"
	TypeActionRecordUpdate = "action.record_update"
	TypeLogicIf            = "logic.if"
	TypeLogicSwitch        = "logic.switch"
	TypeLogicParallel      = "logic.parallel"
	TypeLogicMerge         = "logic.merge"
	TypeLogicFilter        = "logic.filter"
	TypeLogicLoop          = "logic.loop"
	TypeLogicDelay         = "logic.delay"
	TypeHumanApproval      = "human.approval"
	TypeHumanInput         = "human.input"
)

// Ports of branching nodes.
const (
	PortTrue      = "true"
	PortFalse     = "false"
	PortDefault   = "default"   // logic.switch when no case matches
	PortApproved  = "approved"  // human.approval
	PortRejected  = "rejected"  // human.approval
	PortSubmitted = "submitted" // human.input
	PortTimeout   = "timeout"   // human task expired (needs a timeout configured)
)

// NodeKind groups node types in the designer palette.
type NodeKind string

const (
	KindTrigger NodeKind = "trigger"
	KindAction  NodeKind = "action"
	KindLogic   NodeKind = "logic"
	KindHuman   NodeKind = "human"
)

// ConfigField describes one configuration field for the designer's form.
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"` // string | text | expression | json | select | number | boolean
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
	Ports       []string      `json:"ports"` // empty: single default output; logic.switch ports come from its cases
	Available   bool          `json:"available"`
	Unavailable string        `json:"unavailable_reason,omitempty"`
	Fields      []ConfigField `json:"fields"`
}

const connectorRuntimePending = "needs the connector invocation runtime, which is not implemented yet (S6)"

var humanTaskFields = []ConfigField{
	{Key: "title", Label: "Title", Kind: "string", Required: true, Description: "Shown in the approvals inbox."},
	{Key: "instructions", Label: "Instructions", Kind: "text"},
	{Key: "assignees", Label: "Assignees", Kind: "json", Required: true,
		Description: `{"users": ["<user id>"], "groups": ["<group id>"]} — anyone listed (or in a listed group) may act.`},
	{Key: "timeout_seconds", Label: "Timeout (seconds)", Kind: "number",
		Description: "Optional. When it elapses the task expires and the timeout branch runs."},
}

// Catalog is the ordered node-type catalog (GET /node-types).
func Catalog() []NodeType {
	return []NodeType{
		{Type: TypeTriggerManual, Kind: KindTrigger, Label: "Manual trigger", Available: true,
			Description: "Started from the API or the designer with a JSON input."},
		{Type: TypeTriggerSchedule, Kind: KindTrigger, Label: "Schedule", Available: true,
			Description: "Starts on a cron schedule in a time zone; missed fires are skipped, not replayed.",
			Fields: []ConfigField{
				{Key: "cron", Label: "Cron expression", Kind: "string", Required: true,
					Description: "Five fields: minute hour day-of-month month day-of-week, e.g. 0 9 * * 1-5."},
				{Key: "timezone", Label: "Time zone", Kind: "string", Description: "IANA name, e.g. Asia/Jakarta. Default UTC."},
				{Key: "input", Label: "Input", Kind: "json", Description: "JSON object passed as the execution input."},
			}},
		{Type: TypeTriggerWebhook, Kind: KindTrigger, Label: "Webhook", Available: true,
			Description: "Starts when a webhook endpoint of this workspace receives a verified request; " +
				"input is {event_id, endpoint_id, provider_event_id, event_type, payload}.",
			Fields: []ConfigField{
				{Key: "endpoint_id", Label: "Webhook endpoint", Kind: "webhook_endpoint", Required: true,
					Description: "An inbound endpoint in this workspace, in the environment the workflow is deployed to."},
				{Key: "condition", Label: "Condition", Kind: "expression",
					Description: "Optional CEL over input (e.g. input.event_type == 'payment_intent.succeeded'); the execution starts only when true."},
			}},
		{Type: TypeTriggerEvent, Kind: KindTrigger, Label: "Platform event", Available: true,
			Description: "Starts when a platform event is emitted in this organization; input is {event, data}.",
			Fields: []ConfigField{
				{Key: "event", Label: "Event", Kind: "select", Required: true, Options: PlatformEvents},
				{Key: "condition", Label: "Condition", Kind: "expression",
					Description: "Optional CEL over input.event and input.data; the execution starts only when true."},
			}},
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
		{Type: TypeActionConnector, Kind: KindAction, Label: "Connector operation", Available: true,
			Description: "Invokes an operation of a connector instance of this workspace in the deployment's environment. " +
				"Phase 1 runs trigger_workflow of n8n, Make and Zapier connectors; other connectors still need the S6 runtime.",
			Fields: []ConfigField{
				{Key: "connector", Label: "Connector", Kind: "connector", Required: true,
					Description: "Ref key of the connector instance; it resolves per environment, so dev and prod can point at different webhooks."},
				{Key: "operation", Label: "Operation", Kind: "select", Required: true, Options: []string{"trigger_workflow"}},
				{Key: "input", Label: "Input", Kind: "json", Description: "Static JSON sent to the tool. Defaults to the node's input."},
				{Key: "input_expression", Label: "Input expression", Kind: "expression",
					Description: "CEL expression evaluated against input, trigger and nodes; overrides input."},
			}},
		{Type: TypeActionDBQuery, Kind: KindAction, Label: "Database query", Unavailable: connectorRuntimePending,
			Description: "Runs a parameterized query through a PostgreSQL or MySQL connector."},
		{Type: TypeActionNotify, Kind: KindAction, Label: "Notify", Unavailable: connectorRuntimePending,
			Description: "Sends an email, Slack or Microsoft Teams message through a connector."},
		{Type: TypeActionRecordCreate, Kind: KindAction, Label: "Create record", Unavailable: connectorRuntimePending,
			Description: "Creates a record through a connector operation."},
		{Type: TypeActionRecordUpdate, Kind: KindAction, Label: "Update record", Unavailable: connectorRuntimePending,
			Description: "Updates a record through a connector operation."},
		{Type: TypeActionComposition, Kind: KindAction, Label: "API composition",
			Unavailable: "system-principal composition invocation is not implemented yet",
			Description: "Runs a published API composition."},
		{Type: TypeLogicIf, Kind: KindLogic, Label: "If", Available: true, Ports: []string{PortTrue, PortFalse},
			Description: "Routes to the true or false branch; the other branch is skipped.",
			Fields:      []ConfigField{{Key: "condition", Label: "Condition", Kind: "expression", Required: true}}},
		{Type: TypeLogicSwitch, Kind: KindLogic, Label: "Switch", Available: true, Ports: []string{PortDefault},
			Description: "Routes to the first case whose condition holds, else to default; other branches are skipped.",
			Fields: []ConfigField{{Key: "cases", Label: "Cases", Kind: "json", Required: true,
				Description: `[{"port": "high", "condition": "input.amount > 1000.0"}] — each port becomes an output.`}}},
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
		{Type: TypeLogicLoop, Kind: KindLogic, Label: "Loop (for each)", Available: true,
			Description: "Runs one action for every item of an array (the item is the action's input); output is the results in order.",
			Fields: []ConfigField{
				{Key: "items", Label: "Items expression", Kind: "expression",
					Description: "CEL expression returning the array; defaults to input."},
				{Key: "body", Label: "Body action", Kind: "json", Required: true,
					Description: `{"type": "action.http" | "action.transform", "config": {…}}`},
				{Key: "concurrency", Label: "Concurrency", Kind: "number", Description: "1–10 items at a time; default 1."},
				{Key: "max_iterations", Label: "Max iterations", Kind: "number",
					Description: "More items than this fails the step (loop_limit_exceeded). Default and cap 1000."},
			}},
		{Type: TypeLogicDelay, Kind: KindLogic, Label: "Delay", Available: true,
			Description: "Waits without holding a worker, then passes its input on.",
			Fields: []ConfigField{
				{Key: "seconds", Label: "Seconds", Kind: "number", Description: "Fixed wait."},
				{Key: "until", Label: "Until expression", Kind: "expression",
					Description: "CEL returning an RFC 3339 timestamp; overrides seconds. Past times continue at once."},
			}},
		{Type: TypeHumanApproval, Kind: KindHuman, Label: "Approval", Available: true,
			Ports:       []string{PortApproved, PortRejected, PortTimeout},
			Description: "Creates an approval task and waits for an assignee to approve or reject it.",
			Fields: append(append([]ConfigField{}, humanTaskFields...), ConfigField{Key: "allow_self_approval",
				Label: "Allow self-approval", Kind: "boolean",
				Description: "Off by default: whoever started the execution cannot approve it."})},
		{Type: TypeHumanInput, Kind: KindHuman, Label: "Manual input", Available: true,
			Ports:       []string{PortSubmitted, PortTimeout},
			Description: "Creates a form task and waits for an assignee to submit it; output is the submitted data.",
			Fields: append(append([]ConfigField{}, humanTaskFields...), ConfigField{Key: "fields", Label: "Form fields",
				Kind: "json", Required: true,
				Description: `[{"key": "invoice_no", "label": "Invoice number", "type": "string|number|boolean|text", "required": true}]`})},
	}
}

// PlatformEvents are the event names a trigger.event can listen to (the
// subject after "hugi.events.").
var PlatformEvents = []string{
	"workflow.execution_finished",
	"workflow.published",
	"workflow.approval_requested",
	"workflow.approval_decided",
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

// NodePorts returns the output ports of n. Most types have fixed ports; a
// switch has one per case plus default.
func NodePorts(n Node) []string {
	if n.Type == TypeLogicSwitch {
		cases, _ := ParseSwitchCases(n.Config)
		out := make([]string, 0, len(cases)+1)
		for _, c := range cases {
			out = append(out, c.Port)
		}
		return append(out, PortDefault)
	}
	t, _ := LookupNodeType(n.Type)
	return t.Ports
}
