package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	secretssvc "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/services"
)

// This file is the data-plane connector runtime used by the workflow engine.
// Phase 1 invokes the external workflow tools (S13): n8n, Make and Zapier,
// whose single operation trigger_workflow POSTs the input to the tool's
// webhook. Other built-ins keep their management features (catalog, config,
// connection test) until their invocation adapters land (S6).

// OperationTriggerWorkflow is the operation of the external workflow tools.
const OperationTriggerWorkflow = "trigger_workflow"

// invocable lists the (definition key → operations) the runtime can invoke.
var invocable = map[string][]string{
	"n8n":    {OperationTriggerWorkflow},
	"make":   {OperationTriggerWorkflow},
	"zapier": {OperationTriggerWorkflow},
}

// defaultAuthHeader is the credential header per tool when the instance
// config does not name one (auth method "header").
var defaultAuthHeader = map[string]string{"n8n": "Authorization", "make": "x-make-apikey"}

// Supports reports whether the runtime can invoke operation on connectors of
// definition key.
func Supports(key, operation string) bool {
	for _, op := range invocable[key] {
		if op == operation {
			return true
		}
	}
	return false
}

// InvocableDefinitions lists the definition keys the runtime can invoke.
func InvocableDefinitions() []string { return []string{"n8n", "make", "zapier"} }

// MaxInvokeResponse bounds what an invocation reads; larger responses fail
// instead of being truncated silently.
const MaxInvokeResponse = 1 << 20

// Invocation error codes. Retryable ones are worth another attempt; the
// workflow engine applies the node's retry policy to them.
const (
	CodeNotFound       = "connector_not_found"
	CodeDisabled       = "connector_disabled"
	CodeUnsupported    = "connector_operation_unsupported"
	CodeMisconfigured  = "connector_misconfigured"
	CodeUnavailable    = "connector_unavailable"
	CodeTimeout        = "connector_timeout"
	CodeEgressBlocked  = "egress_blocked"
	CodeUpstreamError  = "connector_upstream_error"
	CodeUpstreamReject = "connector_upstream_rejected"
	CodeTooLarge       = "payload_too_large"
)

// InvokeError is a classified invocation failure. Messages never contain
// the destination URL or credentials.
type InvokeError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *InvokeError) Error() string { return e.Code + ": " + e.Message }

func invokeErr(code string, retryable bool, format string, args ...any) *InvokeError {
	return &InvokeError{Code: code, Message: fmt.Sprintf(format, args...), Retryable: retryable}
}

// Binding is a connector instance resolved for one invocation, including its
// decrypted credential. Call Release when done.
type Binding struct {
	InstanceID uuid.UUID
	RefKey     string
	Key        string // definition key, e.g. "n8n"
	Operations []string
	Config     map[string]any
	AuthMethod string
	Credential []byte
	TimeoutMs  int
	Enabled    bool
}

// Release zeroes the credential.
func (b *Binding) Release() {
	for i := range b.Credential {
		b.Credential[i] = 0
	}
	b.Credential = nil
}

// InvokeRequest is one operation call.
type InvokeRequest struct {
	Operation      string
	Input          any
	IdempotencyKey string
	Headers        map[string]string // extra correlation headers, e.g. X-Hugi-Execution-Id
	Egress         egress.Policy
}

// InvokeResult is the upstream answer.
type InvokeResult struct {
	Status int
	Body   any
}

// Runtime resolves and invokes connector instances for the data plane.
type Runtime struct {
	secrets   *secretssvc.Opener
	userAgent string
}

// NewRuntime builds a runtime; secrets decrypts instance credentials.
func NewRuntime(secrets *secretssvc.Opener) *Runtime {
	return &Runtime{secrets: secrets, userAgent: "Hugimethara-Connector/1"}
}

// ErrBindingNotFound is returned by Resolve when no instance matches.
var ErrBindingNotFound = errors.New("connector: instance not found")

// Resolve loads the instance with refKey in workspace and environment of org
// inside tx (bound to org) and decrypts its credential.
func (r *Runtime) Resolve(ctx context.Context, tx *sqlx.Tx, org, workspace, environment uuid.UUID, refKey string) (Binding, error) {
	var row struct {
		ID         uuid.UUID     `db:"id"`
		RefKey     string        `db:"ref_key"`
		Key        string        `db:"key"`
		Operations []byte        `db:"operations"`
		Config     []byte        `db:"config"`
		AuthMethod string        `db:"auth_method"`
		SecretID   uuid.NullUUID `db:"secret_id"`
		TimeoutMs  int           `db:"timeout_ms"`
		Status     string        `db:"status"`
	}
	err := tx.GetContext(ctx, &row, `
		SELECT i.id, i.ref_key, d.key, v.operations, i.config, i.auth_method, i.secret_id, i.timeout_ms, i.status
		FROM connector_instances i
		JOIN connector_versions v ON v.id = i.connector_version_id
		JOIN connector_definitions d ON d.id = v.definition_id
		WHERE i.org_id = $1 AND i.workspace_id = $2 AND i.environment_id = $3 AND i.ref_key = $4`,
		org, workspace, environment, refKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Binding{}, ErrBindingNotFound
		}
		return Binding{}, err
	}
	b := Binding{InstanceID: row.ID, RefKey: row.RefKey, Key: row.Key, AuthMethod: row.AuthMethod,
		TimeoutMs: row.TimeoutMs, Enabled: row.Status == "enabled", Operations: OperationKeys(row.Operations)}
	if err := json.Unmarshal(row.Config, &b.Config); err != nil || b.Config == nil {
		b.Config = map[string]any{}
	}
	if row.SecretID.Valid && b.AuthMethod != "none" {
		if r.secrets == nil {
			return Binding{}, errors.New("connector: no secret opener configured")
		}
		if b.Credential, err = r.secrets.Open(ctx, tx, org, row.SecretID.UUID); err != nil {
			return Binding{}, fmt.Errorf("connector: open credential: %w", err)
		}
	}
	return b, nil
}

// OperationKeys extracts the operation keys of a version's operations JSON.
func OperationKeys(raw []byte) []string {
	var ops []struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal(raw, &ops)
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		if op.Key != "" {
			out = append(out, op.Key)
		}
	}
	return out
}

// Invoke runs req against b. The caller's context deadline bounds the call
// together with the instance timeout.
func (r *Runtime) Invoke(ctx context.Context, b Binding, req InvokeRequest) (InvokeResult, error) {
	if !b.Enabled {
		return InvokeResult{}, invokeErr(CodeDisabled, false, "Connector %q is disabled.", b.RefKey)
	}
	declared := false
	for _, op := range b.Operations {
		declared = declared || op == req.Operation
	}
	if !declared || !Supports(b.Key, req.Operation) {
		return InvokeResult{}, invokeErr(CodeUnsupported, false,
			"Connector %q (%s) cannot run operation %q.", b.RefKey, b.Key, req.Operation)
	}
	return r.triggerWorkflow(ctx, b, req)
}

func (r *Runtime) triggerWorkflow(ctx context.Context, b Binding, req InvokeRequest) (InvokeResult, error) {
	raw, _ := b.Config["webhook_url"].(string)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return InvokeResult{}, invokeErr(CodeMisconfigured, false, "Connector %q has no valid webhook_url.", b.RefKey)
	}
	payload := req.Input
	if payload == nil {
		payload = map[string]any{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return InvokeResult{}, invokeErr(CodeMisconfigured, false, "The input is not JSON-serializable.")
	}
	timeout := time.Duration(b.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return InvokeResult{}, invokeErr(CodeMisconfigured, false, "Connector %q has no valid webhook_url.", b.RefKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", r.userAgent)
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if b.AuthMethod == "header" {
		if len(b.Credential) == 0 {
			return InvokeResult{}, invokeErr(CodeMisconfigured, false,
				"Connector %q uses header authentication but has no credential.", b.RefKey)
		}
		name, _ := b.Config["auth_header"].(string)
		if name = strings.TrimSpace(name); name == "" {
			name = defaultAuthHeader[b.Key]
		}
		if name == "" || strings.ContainsAny(name, "\r\n: ") {
			return InvokeResult{}, invokeErr(CodeMisconfigured, false, "Connector %q has an invalid auth_header.", b.RefKey)
		}
		httpReq.Header.Set(name, string(b.Credential))
	}

	deadline, _ := ctx.Deadline()
	client := egress.NewClient(egress.Options{Policy: req.Egress, Timeout: time.Until(deadline),
		MaxRedirects: 3, MaxResponseBytes: MaxInvokeResponse})
	resp, err := client.Do(httpReq)
	if err != nil {
		switch {
		case egress.IsBlocked(err):
			return InvokeResult{}, invokeErr(CodeEgressBlocked, false,
				"The destination is not allowed by the environment's egress policy.")
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
			return InvokeResult{}, invokeErr(CodeTimeout, true, "%s did not answer in time.", toolName(b.Key))
		}
		return InvokeResult{}, invokeErr(CodeUnavailable, true, "%s is unreachable.", toolName(b.Key))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxInvokeResponse+1))
	if err != nil {
		return InvokeResult{}, invokeErr(CodeUnavailable, true, "%s closed the connection.", toolName(b.Key))
	}
	if len(data) > MaxInvokeResponse {
		return InvokeResult{}, invokeErr(CodeTooLarge, false, "The response exceeds %d bytes.", MaxInvokeResponse)
	}
	var parsed any = string(data)
	if mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); strings.HasSuffix(mt, "json") && len(data) > 0 {
		if json.Unmarshal(data, &parsed) != nil {
			parsed = string(data)
		}
	}
	out := InvokeResult{Status: resp.StatusCode, Body: parsed}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return out, invokeErr(CodeUpstreamError, true, "%s answered %d.", toolName(b.Key), resp.StatusCode)
	case resp.StatusCode >= 400:
		return out, invokeErr(CodeUpstreamReject, false, "%s answered %d.", toolName(b.Key), resp.StatusCode)
	}
	return out, nil
}

func toolName(key string) string {
	switch key {
	case "n8n":
		return "n8n"
	case "make":
		return "Make"
	case "zapier":
		return "Zapier"
	}
	return "The connector"
}
