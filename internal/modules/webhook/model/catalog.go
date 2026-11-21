package model

// EventType is a platform event an outbound subscription can receive.
type EventType struct {
	Type        string
	Description string
	Example     map[string]any
}

// Catalog lists the subscribable platform events (GET /event-types). Every
// one carries a workspace_id: a subscription only receives the events of its
// own workspace, so organization-level events are not offered.
func Catalog() []EventType {
	return []EventType{
		{Type: "workflow.execution_finished", Description: "A workflow execution reached a final state.",
			Example: map[string]any{"execution_id": "…", "workflow_id": "…", "workspace_id": "…",
				"status": "succeeded", "trigger_depth": 0}},
		{Type: "workflow.published", Description: "A workflow version was published.",
			Example: map[string]any{"workflow_id": "…", "workspace_id": "…", "version": 3, "version_id": "…"}},
		{Type: "workflow.approval_requested", Description: "A workflow is waiting for an approval or manual input.",
			Example: map[string]any{"task_id": "…", "execution_id": "…", "workflow_id": "…", "workspace_id": "…",
				"node_id": "approve", "kind": "approval", "title": "Approve payment"}},
		{Type: "workflow.approval_decided", Description: "An approval task was approved, rejected or submitted.",
			Example: map[string]any{"task_id": "…", "execution_id": "…", "workflow_id": "…", "workspace_id": "…",
				"node_id": "approve", "decision": "approved", "decided_by": "…"}},
		{Type: ReceivedEvent, Description: "A verified inbound webhook was received by an endpoint.",
			Example: map[string]any{"event_id": "…", "endpoint_id": "…", "workspace_id": "…",
				"provider_event_id": "evt_123", "event_type": "payment_intent.succeeded"}},
	}
}

// KnownEventType reports whether t is in the catalog.
func KnownEventType(t string) bool {
	for _, e := range Catalog() {
		if e.Type == t {
			return true
		}
	}
	return false
}
