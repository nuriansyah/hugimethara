package model

import "fmt"

// StepStatus is the lifecycle state of one execution step (§3).
type StepStatus string

const (
	StepPending        StepStatus = "pending"
	StepQueued         StepStatus = "queued"
	StepRunning        StepStatus = "running"
	StepWaiting        StepStatus = "waiting" // delay, approval or manual input
	StepSucceeded      StepStatus = "succeeded"
	StepFailed         StepStatus = "failed"
	StepSkipped        StepStatus = "skipped"
	StepCancelled      StepStatus = "cancelled"
	StepRetryScheduled StepStatus = "retry_scheduled"
)

// Terminal reports whether no further transition is possible.
func (s StepStatus) Terminal() bool {
	switch s {
	case StepSucceeded, StepFailed, StepSkipped, StepCancelled:
		return true
	}
	return false
}

// StepEvent drives step transitions.
type StepEvent string

const (
	EvDispatch      StepEvent = "dispatch"       // pending → queued
	EvClaim         StepEvent = "claim"          // queued → running (lease acquired)
	EvSucceed       StepEvent = "succeed"        // running → succeeded
	EvFail          StepEvent = "fail"           // running → failed (no attempts left)
	EvScheduleRetry StepEvent = "schedule_retry" // running → retry_scheduled
	EvRetryDue      StepEvent = "retry_due"      // retry_scheduled → queued
	EvLeaseExpired  StepEvent = "lease_expired"  // running → queued (worker died)
	EvSkip          StepEvent = "skip"           // pending → skipped (dead path)
	EvCancel        StepEvent = "cancel"         // any non-terminal → cancelled
	EvSuspend       StepEvent = "suspend"        // running → waiting (timer or human task)
	EvResume        StepEvent = "resume"         // waiting → succeeded (timer fired, task decided)
)

type stepKey struct {
	from StepStatus
	ev   StepEvent
}

// stepTable is the complete step state machine. Anything not listed is an
// invalid transition — the engine refuses it rather than guessing.
var stepTable = map[stepKey]StepStatus{
	{StepPending, EvDispatch}:        StepQueued,
	{StepPending, EvSkip}:            StepSkipped,
	{StepPending, EvCancel}:          StepCancelled,
	{StepQueued, EvClaim}:            StepRunning,
	{StepQueued, EvCancel}:           StepCancelled,
	{StepRunning, EvSucceed}:         StepSucceeded,
	{StepRunning, EvFail}:            StepFailed,
	{StepRunning, EvScheduleRetry}:   StepRetryScheduled,
	{StepRunning, EvLeaseExpired}:    StepQueued,
	{StepRunning, EvCancel}:          StepCancelled,
	{StepRunning, EvSuspend}:         StepWaiting,
	{StepWaiting, EvResume}:          StepSucceeded,
	{StepWaiting, EvFail}:            StepFailed,
	{StepWaiting, EvCancel}:          StepCancelled,
	{StepRetryScheduled, EvRetryDue}: StepQueued,
	{StepRetryScheduled, EvCancel}:   StepCancelled,
}

// StepTransition returns the state reached by applying ev in from.
func StepTransition(from StepStatus, ev StepEvent) (StepStatus, error) {
	to, ok := stepTable[stepKey{from, ev}]
	if !ok {
		return from, fmt.Errorf("workflow: invalid step transition %s --%s-->", from, ev)
	}
	return to, nil
}

// ExecutionStatus is the lifecycle state of an execution.
type ExecutionStatus string

const (
	ExecPending   ExecutionStatus = "pending"
	ExecRunning   ExecutionStatus = "running"
	ExecSucceeded ExecutionStatus = "succeeded"
	ExecFailed    ExecutionStatus = "failed"
	ExecCancelled ExecutionStatus = "cancelled"
	ExecTimedOut  ExecutionStatus = "timed_out"
)

// Terminal reports whether the execution has finished.
func (s ExecutionStatus) Terminal() bool {
	switch s {
	case ExecSucceeded, ExecFailed, ExecCancelled, ExecTimedOut:
		return true
	}
	return false
}

// ExecutionEvent drives execution transitions.
type ExecutionEvent string

const (
	ExStart    ExecutionEvent = "start"    // first step claimed
	ExComplete ExecutionEvent = "complete" // every step terminal, none failed
	ExFail     ExecutionEvent = "fail"     // a step failed or a limit was exceeded
	ExCancel   ExecutionEvent = "cancel"
	ExTimeout  ExecutionEvent = "timeout" // deadline_at passed
	ExRerun    ExecutionEvent = "rerun"   // re-run from the failed step
)

type execKey struct {
	from ExecutionStatus
	ev   ExecutionEvent
}

var execTable = map[execKey]ExecutionStatus{
	{ExecPending, ExStart}:    ExecRunning,
	{ExecPending, ExFail}:     ExecFailed,
	{ExecPending, ExCancel}:   ExecCancelled,
	{ExecPending, ExTimeout}:  ExecTimedOut,
	{ExecRunning, ExComplete}: ExecSucceeded,
	{ExecRunning, ExFail}:     ExecFailed,
	{ExecRunning, ExCancel}:   ExecCancelled,
	{ExecRunning, ExTimeout}:  ExecTimedOut,
	{ExecFailed, ExRerun}:     ExecRunning,
	{ExecTimedOut, ExRerun}:   ExecRunning,
}

// ExecutionTransition returns the state reached by applying ev in from.
func ExecutionTransition(from ExecutionStatus, ev ExecutionEvent) (ExecutionStatus, error) {
	to, ok := execTable[execKey{from, ev}]
	if !ok {
		return from, fmt.Errorf("workflow: invalid execution transition %s --%s-->", from, ev)
	}
	return to, nil
}

// Failure reason codes recorded in execution and step errors. They are part of
// the API contract: clients branch on them.
const (
	ReasonNodeFailed       = "node_failed"
	ReasonStepTimeout      = "step_timeout"
	ReasonMaxStepsExceeded = "max_steps_exceeded"
	ReasonDeadlineExceeded = "execution_deadline_exceeded"
	ReasonPayloadTooLarge  = "payload_too_large"
	ReasonEgressBlocked    = "egress_blocked"
	ReasonInvalidState     = "invalid_state"
	ReasonUpstreamFailed   = "upstream_failed"
	ReasonLoopLimit        = "loop_limit_exceeded"
	ReasonConcurrencyLimit = "concurrency_limit_exceeded"
	ReasonCancelled        = "cancelled"
	ReasonTaskExpired      = "task_expired"
	ReasonInvalidConfig    = "invalid_config"
)

// StepError is the structured error stored on a failed step or execution.
type StepError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	NodeID    string `json:"node_id,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

func (e *StepError) Error() string { return e.Code + ": " + e.Message }
