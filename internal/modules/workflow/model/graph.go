package model

import "sort"

// NodeState is what the engine knows about a node that already has a step.
type NodeState struct {
	Status StepStatus
	Port   string // branch chosen by a succeeded branching node
}

// Plan is the outcome of Advance: nodes to enqueue now and nodes on dead paths.
type Plan struct {
	Ready   []string
	Skipped []string
}

// edgeState classifies one inbound edge.
type edgeState int

const (
	edgeUnresolved edgeState = iota
	edgeActive
	edgeInactive
)

func classify(e Edge, states map[string]NodeState) edgeState {
	if e.From == TriggerNodeID {
		return edgeActive
	}
	s, ok := states[e.From]
	if !ok {
		return edgeUnresolved
	}
	switch s.Status {
	case StepSucceeded:
		if e.Port == "" || e.Port == s.Port {
			return edgeActive
		}
		return edgeInactive
	case StepSkipped, StepFailed, StepCancelled:
		return edgeInactive
	}
	return edgeUnresolved
}

// Advance computes which nodes without a step become ready and which are
// skipped, given the states of nodes that have one. Skips propagate to a fixed
// point (dead-path elimination), so a merge after an if never deadlocks. It is
// pure: the engine applies the plan inside the execution's row lock.
func Advance(d Definition, states map[string]NodeState) Plan {
	known := make(map[string]NodeState, len(states))
	for k, v := range states {
		known[k] = v
	}
	var plan Plan
	ready := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, n := range d.Nodes {
			if _, has := known[n.ID]; has || ready[n.ID] {
				continue
			}
			in := d.Inbound(n.ID)
			if len(in) == 0 {
				continue // unreachable; rejected at publish
			}
			active, unresolved := 0, 0
			for _, e := range in {
				switch classify(e, known) {
				case edgeActive:
					active++
				case edgeUnresolved:
					unresolved++
				}
			}
			anyMode := n.Type == TypeLogicMerge && n.Config["mode"] == "any"
			switch {
			case anyMode && active > 0, unresolved == 0 && active > 0:
				ready[n.ID] = true
				plan.Ready = append(plan.Ready, n.ID)
				changed = true
			case unresolved == 0 && active == 0:
				known[n.ID] = NodeState{Status: StepSkipped}
				plan.Skipped = append(plan.Skipped, n.ID)
				changed = true
			}
		}
	}
	sort.Strings(plan.Ready)
	sort.Strings(plan.Skipped)
	return plan
}

// NodeInput builds a node's input from the outputs of its active inbound
// edges: the single source's output, the trigger input for entry nodes, or
// for merges an object keyed by source node id ("trigger" for the trigger).
func NodeInput(d Definition, nodeID string, states map[string]NodeState, outputs map[string]any, trigger any) any {
	n, _ := d.Node(nodeID)
	var sources []string
	for _, e := range d.Inbound(nodeID) {
		if classify(e, states) == edgeActive {
			sources = append(sources, e.From)
		}
	}
	value := func(id string) any {
		if id == TriggerNodeID {
			return trigger
		}
		return outputs[id]
	}
	if n.Type == TypeLogicMerge {
		merged := make(map[string]any, len(sources))
		for _, s := range sources {
			merged[s] = value(s)
		}
		return merged
	}
	if len(sources) == 0 {
		return nil
	}
	return value(sources[0])
}

// Sinks are nodes without outgoing edges; their outputs form the execution output.
func Sinks(d Definition) []string {
	var out []string
	for _, n := range d.Nodes {
		if len(d.Outbound(n.ID)) == 0 {
			out = append(out, n.ID)
		}
	}
	sort.Strings(out)
	return out
}
