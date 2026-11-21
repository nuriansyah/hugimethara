// Package repository maps webhook rows to the model. SQL lives in commands
// (writes) and queries (reads).
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---- endpoints ----

type endpointRow struct {
	ID                 uuid.UUID    `db:"id"`
	WorkspaceID        uuid.UUID    `db:"workspace_id"`
	EnvironmentID      uuid.UUID    `db:"environment_id"`
	PublicID           string       `db:"public_id"`
	Name               string       `db:"name"`
	Description        string       `db:"description"`
	Verifier           string       `db:"verifier"`
	VerifierConfig     []byte       `db:"verifier_config"`
	SecretHint         string       `db:"secret_hint"`
	ToleranceSeconds   int          `db:"tolerance_seconds"`
	RateLimitPerMinute int          `db:"rate_limit_per_minute"`
	Enabled            bool         `db:"enabled"`
	ReceivedCount      int64        `db:"received_count"`
	RejectedCount      int64        `db:"rejected_count"`
	LastReceivedAt     sql.NullTime `db:"last_received_at"`
	LastRejectedAt     sql.NullTime `db:"last_rejected_at"`
	LastRejectedReason string       `db:"last_rejected_reason"`
	SecretRotatedAt    sql.NullTime `db:"secret_rotated_at"`
	CreatedAt          time.Time    `db:"created_at"`
	UpdatedAt          time.Time    `db:"updated_at"`
}

func (r endpointRow) model() model.Endpoint {
	var cfg model.HMACConfig
	_ = json.Unmarshal(r.VerifierConfig, &cfg)
	return model.Endpoint{ID: r.ID, WorkspaceID: r.WorkspaceID, EnvironmentID: r.EnvironmentID, PublicID: r.PublicID,
		Name: r.Name, Description: r.Description, Verifier: model.Scheme(r.Verifier), VerifierConfig: cfg,
		SecretHint: r.SecretHint, ToleranceSeconds: r.ToleranceSeconds, RateLimitPerMinute: r.RateLimitPerMinute,
		Enabled: r.Enabled, ReceivedCount: r.ReceivedCount, RejectedCount: r.RejectedCount,
		LastReceivedAt: timePtr(r.LastReceivedAt), LastRejectedAt: timePtr(r.LastRejectedAt),
		LastRejectedReason: r.LastRejectedReason, SecretRotatedAt: timePtr(r.SecretRotatedAt),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

// ListEndpoints lists a workspace's endpoints, newest first.
func ListEndpoints(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID) ([]model.Endpoint, error) {
	var rows []endpointRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListEndpoints"), workspaceID); err != nil {
		return nil, err
	}
	out := make([]model.Endpoint, len(rows))
	for i, r := range rows {
		out[i] = r.model()
	}
	return out, nil
}

// GetEndpoint returns one endpoint (sql.ErrNoRows if absent).
func GetEndpoint(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Endpoint, error) {
	var r endpointRow
	if err := tx.GetContext(ctx, &r, queries.SQL.Get("GetEndpoint"), id); err != nil {
		return model.Endpoint{}, err
	}
	return r.model(), nil
}

// EndpointSecret returns the sealed signing secret.
func EndpointSecret(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) ([]byte, error) {
	var b []byte
	err := tx.GetContext(ctx, &b, queries.SQL.Get("EndpointSecret"), id)
	return b, err
}

// NewEndpoint is an endpoint to insert.
type NewEndpoint struct {
	Endpoint  model.Endpoint
	OrgID     uuid.UUID
	Sealed    []byte
	CreatedBy uuid.UUID
}

// InsertEndpoint creates an endpoint.
func InsertEndpoint(ctx context.Context, tx *sqlx.Tx, n NewEndpoint) error {
	e := n.Endpoint
	cfg, err := json.Marshal(e.VerifierConfig)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, commands.SQL.Get("InsertEndpoint"), e.ID, n.OrgID, e.WorkspaceID, e.EnvironmentID,
		e.PublicID, e.Name, e.Description, string(e.Verifier), cfg, n.Sealed, e.SecretHint, e.ToleranceSeconds,
		e.RateLimitPerMinute, n.CreatedBy)
	return err
}

// UpdateEndpoint stores the editable fields.
func UpdateEndpoint(ctx context.Context, tx *sqlx.Tx, e model.Endpoint) error {
	cfg, err := json.Marshal(e.VerifierConfig)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, commands.SQL.Get("UpdateEndpoint"), e.ID, e.Name, e.Description, cfg,
		e.ToleranceSeconds, e.RateLimitPerMinute, e.Enabled)
	return err
}

// DeleteEndpoint removes an endpoint and its events.
func DeleteEndpoint(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteEndpoint"), id)
	return err
}

// RotateEndpointSecret replaces the sealed secret.
func RotateEndpointSecret(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, sealed []byte, hint string) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("RotateEndpointSecret"), id, sealed, hint)
	return err
}

// IngressTarget is the tenant an ingress public id resolves to.
type IngressTarget struct {
	EndpointID         uuid.UUID `db:"endpoint_id"`
	OrgID              uuid.UUID `db:"org_id"`
	Enabled            bool      `db:"enabled"`
	RateLimitPerMinute int       `db:"rate_limit_per_minute"`
}

// LookupIngress resolves a public id before any tenant is bound.
func LookupIngress(ctx context.Context, q sqlx.QueryerContext, publicID string) (IngressTarget, error) {
	var t IngressTarget
	err := sqlx.GetContext(ctx, q, &t, queries.SQL.Get("IngressLookup"), publicID)
	return t, err
}

// RecordReceived bumps an endpoint's received counter.
func RecordReceived(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("RecordReceived"), id)
	return err
}

// RecordRejected bumps an endpoint's rejected counter.
func RecordRejected(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, reason string) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("RecordRejected"), id, reason)
	return err
}

// EnvironmentExists reports whether an environment is visible in the tenant.
func EnvironmentExists(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (bool, error) {
	var got uuid.UUID
	err := tx.GetContext(ctx, &got, queries.SQL.Get("EnvironmentInOrg"), id)
	if db.IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

// ---- events ----

type eventRow struct {
	ID              uuid.UUID      `db:"id"`
	WorkspaceID     uuid.UUID      `db:"workspace_id"`
	EndpointID      uuid.UUID      `db:"endpoint_id"`
	ProviderEventID sql.NullString `db:"provider_event_id"`
	EventType       string         `db:"event_type"`
	ContentType     string         `db:"content_type"`
	Headers         []byte         `db:"headers"`
	Payload         []byte         `db:"payload"`
	RawBody         sql.NullString `db:"raw_body"`
	PayloadSize     int            `db:"payload_size"`
	SourceIP        string         `db:"source_ip"`
	ReceivedAt      time.Time      `db:"received_at"`
}

func (r eventRow) model() model.Event {
	h := map[string]string{}
	_ = json.Unmarshal(r.Headers, &h)
	return model.Event{ID: r.ID, WorkspaceID: r.WorkspaceID, EndpointID: r.EndpointID,
		ProviderEventID: r.ProviderEventID.String, EventType: r.EventType, ContentType: r.ContentType, Headers: h,
		Payload: r.Payload, RawBody: r.RawBody.String, PayloadSize: r.PayloadSize, SourceIP: r.SourceIP,
		ReceivedAt: r.ReceivedAt}
}

// NewEvent is a verified request to store.
type NewEvent struct {
	Event model.Event
	OrgID uuid.UUID
}

// InsertEvent stores a verified event; inserted is false for a duplicate
// provider event id.
func InsertEvent(ctx context.Context, tx *sqlx.Tx, n NewEvent) (inserted bool, err error) {
	e := n.Event
	headers, err := json.Marshal(e.Headers)
	if err != nil {
		return false, err
	}
	var id uuid.UUID
	err = tx.GetContext(ctx, &id, commands.SQL.Get("InsertEvent"), e.ID, n.OrgID, e.WorkspaceID, e.EndpointID,
		nullString(e.ProviderEventID), e.EventType, e.ContentType, headers, nullJSON(e.Payload), nullString(e.RawBody),
		e.PayloadSize, e.SourceIP)
	if db.IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

// ListEvents lists inbound events without their bodies.
func ListEvents(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID, endpointID *uuid.UUID, limit int) ([]model.Event, error) {
	var rows []eventRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListEvents"), workspaceID, endpointID, limit); err != nil {
		return nil, err
	}
	out := make([]model.Event, len(rows))
	for i, r := range rows {
		out[i] = r.model()
	}
	return out, nil
}

// GetEvent returns one event with its body.
func GetEvent(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Event, error) {
	var r eventRow
	if err := tx.GetContext(ctx, &r, queries.SQL.Get("GetEvent"), id); err != nil {
		return model.Event{}, err
	}
	return r.model(), nil
}

// ---- subscriptions ----

type subscriptionRow struct {
	ID             uuid.UUID    `db:"id"`
	WorkspaceID    uuid.UUID    `db:"workspace_id"`
	EnvironmentID  uuid.UUID    `db:"environment_id"`
	Name           string       `db:"name"`
	URL            string       `db:"url"`
	EventTypes     db.TextArray `db:"event_types"`
	SecretHint     string       `db:"secret_hint"`
	Enabled        bool         `db:"enabled"`
	DisabledReason string       `db:"disabled_reason"`
	FailingSince   sql.NullTime `db:"failing_since"`
	LastSuccessAt  sql.NullTime `db:"last_success_at"`
	CreatedAt      time.Time    `db:"created_at"`
	UpdatedAt      time.Time    `db:"updated_at"`
}

func (r subscriptionRow) model() model.Subscription {
	types := []string(r.EventTypes)
	if types == nil {
		types = []string{}
	}
	return model.Subscription{ID: r.ID, WorkspaceID: r.WorkspaceID, EnvironmentID: r.EnvironmentID, Name: r.Name,
		URL: r.URL, EventTypes: types, SecretHint: r.SecretHint, Enabled: r.Enabled, DisabledReason: r.DisabledReason,
		FailingSince: timePtr(r.FailingSince), LastSuccessAt: timePtr(r.LastSuccessAt), CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt}
}

// ListSubscriptions lists a workspace's subscriptions.
func ListSubscriptions(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID) ([]model.Subscription, error) {
	var rows []subscriptionRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListSubscriptions"), workspaceID); err != nil {
		return nil, err
	}
	out := make([]model.Subscription, len(rows))
	for i, r := range rows {
		out[i] = r.model()
	}
	return out, nil
}

// GetSubscription returns one subscription.
func GetSubscription(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Subscription, error) {
	var r subscriptionRow
	if err := tx.GetContext(ctx, &r, queries.SQL.Get("GetSubscription"), id); err != nil {
		return model.Subscription{}, err
	}
	return r.model(), nil
}

// InsertSubscription creates a subscription.
func InsertSubscription(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID, s model.Subscription, sealed []byte,
	createdBy uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertSubscription"), s.ID, orgID, s.WorkspaceID, s.EnvironmentID,
		s.Name, s.URL, s.EventTypes, sealed, s.SecretHint, createdBy)
	return err
}

// UpdateSubscription stores the editable fields.
func UpdateSubscription(ctx context.Context, tx *sqlx.Tx, s model.Subscription) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateSubscription"), s.ID, s.Name, s.URL, s.EventTypes, s.Enabled)
	return err
}

// DeleteSubscription removes a subscription and its deliveries.
func DeleteSubscription(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteSubscription"), id)
	return err
}

// RotateSubscriptionSecret replaces the sealed secret.
func RotateSubscriptionSecret(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, sealed []byte, hint string) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("RotateSubscriptionSecret"), id, sealed, hint)
	return err
}

// ---- deliveries ----

type deliveryRow struct {
	ID               uuid.UUID     `db:"id"`
	WorkspaceID      uuid.UUID     `db:"workspace_id"`
	SubscriptionID   uuid.UUID     `db:"subscription_id"`
	SubscriptionName string        `db:"subscription_name"`
	EventID          string        `db:"event_id"`
	EventType        string        `db:"event_type"`
	Payload          []byte        `db:"payload"`
	ReplayOf         uuid.NullUUID `db:"replay_of"`
	Status           string        `db:"status"`
	Attempts         int           `db:"attempts"`
	NextAttemptAt    sql.NullTime  `db:"next_attempt_at"`
	LastAttemptAt    sql.NullTime  `db:"last_attempt_at"`
	ResponseStatus   sql.NullInt64 `db:"response_status"`
	ResponseExcerpt  string        `db:"response_excerpt"`
	LatencyMS        sql.NullInt64 `db:"latency_ms"`
	Error            string        `db:"error"`
	CreatedAt        time.Time     `db:"created_at"`
	FinishedAt       sql.NullTime  `db:"finished_at"`
}

func (r deliveryRow) model() model.Delivery {
	d := model.Delivery{ID: r.ID, WorkspaceID: r.WorkspaceID, SubscriptionID: r.SubscriptionID,
		SubscriptionName: r.SubscriptionName, EventID: r.EventID, EventType: r.EventType, Payload: r.Payload,
		Status: model.DeliveryStatus(r.Status), Attempts: r.Attempts, NextAttemptAt: timePtr(r.NextAttemptAt),
		LastAttemptAt: timePtr(r.LastAttemptAt), ResponseStatus: intPtr(r.ResponseStatus),
		ResponseExcerpt: r.ResponseExcerpt, LatencyMS: intPtr(r.LatencyMS), Error: r.Error, CreatedAt: r.CreatedAt,
		FinishedAt: timePtr(r.FinishedAt)}
	if r.ReplayOf.Valid {
		v := r.ReplayOf.UUID
		d.ReplayOf = &v
	}
	return d
}

// ListDeliveries lists deliveries without payloads, newest first.
func ListDeliveries(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID, subscriptionID *uuid.UUID, status string,
	limit int) ([]model.Delivery, error) {
	var rows []deliveryRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDeliveries"), workspaceID, subscriptionID, status,
		limit); err != nil {
		return nil, err
	}
	out := make([]model.Delivery, len(rows))
	for i, r := range rows {
		out[i] = r.model()
	}
	return out, nil
}

// GetDelivery returns one delivery with its payload and attempt log.
func GetDelivery(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Delivery, error) {
	var r deliveryRow
	if err := tx.GetContext(ctx, &r, queries.SQL.Get("GetDelivery"), id); err != nil {
		return model.Delivery{}, err
	}
	d := r.model()
	var attempts []struct {
		Attempt         int           `db:"attempt"`
		ResponseStatus  sql.NullInt64 `db:"response_status"`
		ResponseExcerpt string        `db:"response_excerpt"`
		LatencyMS       int           `db:"latency_ms"`
		Error           string        `db:"error"`
		AttemptedAt     time.Time     `db:"attempted_at"`
	}
	if err := tx.SelectContext(ctx, &attempts, queries.SQL.Get("ListAttempts"), id); err != nil {
		return model.Delivery{}, err
	}
	d.AttemptLog = make([]model.Attempt, len(attempts))
	for i, a := range attempts {
		d.AttemptLog[i] = model.Attempt{Attempt: a.Attempt, ResponseStatus: intPtr(a.ResponseStatus),
			ResponseExcerpt: a.ResponseExcerpt, LatencyMS: a.LatencyMS, Error: a.Error, AttemptedAt: a.AttemptedAt}
	}
	return d, nil
}

// NewDelivery is a delivery to enqueue.
type NewDelivery struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	WorkspaceID    uuid.UUID
	SubscriptionID uuid.UUID
	EventID        string
	EventType      string
	Payload        []byte
	ReplayOf       *uuid.UUID
}

// InsertDelivery enqueues a delivery, due now. A fan-out of an event already
// delivered to the subscription inserts nothing (inserted=false).
func InsertDelivery(ctx context.Context, tx *sqlx.Tx, n NewDelivery) (bool, error) {
	if n.ReplayOf != nil {
		_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertReplay"), n.ID, n.OrgID, n.WorkspaceID, n.SubscriptionID,
			n.EventID, n.EventType, n.Payload, *n.ReplayOf)
		return err == nil, err
	}
	res, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDelivery"), n.ID, n.OrgID, n.WorkspaceID, n.SubscriptionID,
		n.EventID, n.EventType, n.Payload)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	return rows == 1, err
}

// MatchingSubscription is a fan-out target.
type MatchingSubscription struct {
	ID          uuid.UUID `db:"id"`
	WorkspaceID uuid.UUID `db:"workspace_id"`
}

// MatchingSubscriptions lists the enabled subscriptions of a workspace that
// receive eventType.
func MatchingSubscriptions(ctx context.Context, tx *sqlx.Tx, orgID, workspaceID uuid.UUID, eventType string) ([]MatchingSubscription, error) {
	var out []MatchingSubscription
	err := tx.SelectContext(ctx, &out, queries.SQL.Get("MatchingSubscriptions"), orgID, workspaceID, eventType)
	return out, err
}

// ClaimedDelivery is a delivery claimed for one attempt.
type ClaimedDelivery struct {
	ID             uuid.UUID `db:"id"`
	OrgID          uuid.UUID `db:"org_id"`
	SubscriptionID uuid.UUID `db:"subscription_id"`
	EventID        string    `db:"event_id"`
	EventType      string    `db:"event_type"`
	Payload        []byte    `db:"payload"`
	Attempt        int       `db:"attempts"`
	CreatedAt      time.Time `db:"created_at"`
}

// ClaimDueDeliveries claims up to limit due deliveries, leasing each for lease.
func ClaimDueDeliveries(ctx context.Context, tx *sqlx.Tx, limit int, lease time.Duration) ([]ClaimedDelivery, error) {
	var out []ClaimedDelivery
	err := tx.SelectContext(ctx, &out, commands.SQL.Get("ClaimDueDeliveries"), limit, lease.Seconds())
	return out, err
}

// Target is where and how a delivery is sent.
type Target struct {
	ID        uuid.UUID    `db:"id"`
	OrgID     uuid.UUID    `db:"org_id"`
	URL       string       `db:"url"`
	Sealed    []byte       `db:"secret_ciphertext"`
	Enabled   bool         `db:"enabled"`
	Allowlist db.TextArray `db:"egress_allowlist"`
}

// DeliveryTarget loads a subscription's destination and secret.
func DeliveryTarget(ctx context.Context, q sqlx.QueryerContext, subscriptionID uuid.UUID) (Target, error) {
	var t Target
	err := sqlx.GetContext(ctx, q, &t, queries.SQL.Get("DeliveryTarget"), subscriptionID)
	return t, err
}

// AttemptResult is the outcome of one attempt.
type AttemptResult struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	DeliveryID      uuid.UUID
	Attempt         int
	ResponseStatus  *int
	ResponseExcerpt string
	LatencyMS       int
	Error           string
	// Status is the delivery's new state; NextAttemptAt is set when pending.
	Status        model.DeliveryStatus
	NextAttemptAt *time.Time
}

// RecordAttempt logs an attempt and moves the delivery to its next state. It
// reports false when the claim was lost (another worker re-claimed it).
func RecordAttempt(ctx context.Context, tx *sqlx.Tx, a AttemptResult) (bool, error) {
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertAttempt"), a.ID, a.OrgID, a.DeliveryID, a.Attempt,
		a.ResponseStatus, a.ResponseExcerpt, a.LatencyMS, a.Error); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, commands.SQL.Get("FinishAttempt"), a.DeliveryID, a.Attempt, string(a.Status),
		a.NextAttemptAt, a.ResponseStatus, a.ResponseExcerpt, a.LatencyMS, a.Error)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SubscriptionSucceeded ends a subscription's failure streak.
func SubscriptionSucceeded(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("SubscriptionSucceeded"), id)
	return err
}

// SubscriptionFailed starts or continues the failure streak and returns its start.
func SubscriptionFailed(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (time.Time, error) {
	var since time.Time
	err := tx.GetContext(ctx, &since, commands.SQL.Get("SubscriptionFailed"), id)
	return since, err
}

// DisableSubscription disables a subscription with a reason; it reports
// whether it was enabled before.
func DisableSubscription(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, reason string) (bool, error) {
	res, err := tx.ExecContext(ctx, commands.SQL.Get("DisableSubscription"), id, reason)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
