// Package mapping implements versioned JSON mapping drafts, test cases and publication.
package mapping

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

const Resource = "mapping"

var errMappingNotFound = httpx.NotFound("Mapping not found.")
var errTestNotFound = httpx.NotFound("Mapping test not found.")

type mappingRow struct {
	ID           uuid.UUID       `db:"id"`
	WorkspaceID  uuid.UUID       `db:"workspace_id"`
	Name         string          `db:"name"`
	Draft        json.RawMessage `db:"draft"`
	InputSchema  json.RawMessage `db:"input_schema"`
	OutputSchema json.RawMessage `db:"output_schema"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

type versionRow struct {
	ID          uuid.UUID       `db:"id"`
	MappingID   uuid.UUID       `db:"mapping_id"`
	Version     int             `db:"version"`
	Definition  json.RawMessage `db:"definition"`
	Checksum    string          `db:"checksum"`
	PublishedBy uuid.UUID       `db:"published_by"`
	PublishedAt time.Time       `db:"published_at"`
}

type testRow struct {
	ID        uuid.UUID       `db:"id"`
	MappingID uuid.UUID       `db:"mapping_id"`
	Name      string          `db:"name"`
	Input     json.RawMessage `db:"input"`
	Expected  json.RawMessage `db:"expected"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

type CreateInput struct {
	WorkspaceID  uuid.UUID
	Name         string
	Draft        map[string]any
	InputSchema  map[string]any
	OutputSchema map[string]any
}

type UpdateInput struct {
	Name         *string
	Draft        *map[string]any
	InputSchema  *map[string]any
	OutputSchema *map[string]any
}

type TestCreateInput struct {
	Name     string
	Input    any
	Expected any
}

type TestUpdateInput struct {
	Name     *string
	Input    any
	Expected any
}

type Service struct {
	pool *sqlx.DB
	az   authz.Authorizer
}

func NewService(pool *sqlx.DB, az authz.Authorizer) *Service {
	return &Service{pool: pool, az: az}
}

func (s *Service) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authz.Unauthenticated()
	}
	return p, nil
}

func (s *Service) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *Service) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	_, err := authz.Require(ctx, s.az, action, authz.Resource{
		Type: Resource, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func rawObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func rawAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func (r mappingRow) model() Mapping {
	draft := rawObject(r.Draft)
	if draft == nil {
		draft = map[string]any{"rules": []any{}}
	}
	return Mapping{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Draft: draft,
		InputSchema: rawObject(r.InputSchema), OutputSchema: rawObject(r.OutputSchema),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r versionRow) model() MappingVersion {
	return MappingVersion{ID: r.ID, MappingID: r.MappingID, Version: r.Version,
		Definition: rawObject(r.Definition), Checksum: r.Checksum,
		PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}
}

func (r testRow) model() MappingTest {
	return MappingTest{ID: r.ID, MappingID: r.MappingID, Name: r.Name,
		Input: rawAny(r.Input), Expected: rawAny(r.Expected),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func jsonArg(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("null")
	}
	raw, _ := json.Marshal(v)
	return raw
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []mappingRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListMappings"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]Mapping, len(rows))
	for i := range rows {
		out[i] = rows[i].model()
	}
	return out, nil
}

func (s *Service) get(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*mappingRow, error) {
	var row mappingRow
	if err := tx.GetContext(ctx, &row, q.Get("GetMapping"), id); err != nil {
		if db.IsNoRows(err) {
			return nil, errMappingNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Mapping{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return Mapping{}, err
	}
	if in.Name = trimName(in.Name); in.Name == "" {
		return Mapping{}, httpx.Validation(httpx.FieldError{Field: "name", Code: "required", Message: "A mapping name is required."})
	}
	if in.Draft == nil {
		in.Draft = map[string]any{"rules": []any{}}
	}
	if err := ValidateDraft(in.Draft); err != nil {
		return Mapping{}, httpx.Validation(httpx.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
	}
	id := uuid.New()
	var out Mapping
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		draft, _ := json.Marshal(in.Draft)
		input, _ := json.Marshal(in.InputSchema)
		output, _ := json.Marshal(in.OutputSchema)
		if in.InputSchema == nil {
			input = nil
		}
		if in.OutputSchema == nil {
			output = nil
		}
		if _, err := tx.ExecContext(ctx, q.Get("InsertMapping"), p.OrgID, id, in.WorkspaceID, in.Name,
			draft, nullableRaw(input), nullableRaw(output)); err != nil {
			return err
		}
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row.model()
		return nil
	}); err != nil {
		return Mapping{}, err
	}
	return out, nil
}

func nullableRaw(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func trimName(name string) string {
	for len(name) > 0 && (name[0] == ' ' || name[0] == '\t' || name[0] == '\n') {
		name = name[1:]
	}
	for len(name) > 0 && (name[len(name)-1] == ' ' || name[len(name)-1] == '\t' || name[len(name)-1] == '\n') {
		name = name[:len(name)-1]
	}
	return name
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Mapping{}, err
	}
	var out Mapping
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row.model()
		return nil
	}); err != nil {
		return Mapping{}, err
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Mapping{}, err
	}
	var out Mapping
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if in.Name != nil {
			name := trimName(*in.Name)
			if name == "" {
				return httpx.Validation(httpx.FieldError{Field: "name", Code: "required", Message: "A mapping name is required."})
			}
			in.Name = &name
		}
		if in.Draft != nil {
			if err := ValidateDraft(*in.Draft); err != nil {
				return httpx.Validation(httpx.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
			}
		}
		var draft, input, output any
		if in.Draft != nil {
			raw, _ := json.Marshal(*in.Draft)
			draft = json.RawMessage(raw)
		}
		if in.InputSchema != nil {
			if *in.InputSchema == nil {
				input = nil
			} else {
				raw, _ := json.Marshal(*in.InputSchema)
				input = json.RawMessage(raw)
			}
		}
		if in.OutputSchema != nil {
			if *in.OutputSchema == nil {
				output = nil
			} else {
				raw, _ := json.Marshal(*in.OutputSchema)
				output = json.RawMessage(raw)
			}
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateMapping"), id, in.Name, draft, input, output); err != nil {
			return err
		}
		row, err = s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row.model()
		return nil
	}); err != nil {
		return Mapping{}, err
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "delete", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, q.Get("DeleteMapping"), id)
		return err
	})
}

func (s *Service) ListVersions(ctx context.Context, id uuid.UUID) ([]MappingVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []MappingVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []versionRow
		if err := tx.SelectContext(ctx, &rows, q.Get("ListMappingVersions"), id); err != nil {
			return err
		}
		out = make([]MappingVersion, len(rows))
		for i := range rows {
			out[i] = rows[i].model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Publish(ctx context.Context, id uuid.UUID) (MappingVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return MappingVersion{}, err
	}
	var out MappingVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "publish", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		draft := row.model().Draft
		if err := ValidateDraft(draft); err != nil {
			return httpx.Validation(httpx.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
		}
		tests, err := s.listTestsTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(tests) == 0 {
			return httpx.Validation(httpx.FieldError{Field: "tests", Code: "required", Message: "Add at least one passing test before publishing."})
		}
		for _, test := range tests {
			actual, err := Apply(draft, test.Input)
			if err != nil {
				return httpx.Validation(httpx.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
			}
			if !jsonEqual(actual, test.Expected) {
				return httpx.Validation(httpx.FieldError{Field: "tests", Code: "failed", Message: "Stored test \"" + test.Name + "\" does not pass."})
			}
		}
		definition, _ := json.Marshal(draft)
		sum := sha256.Sum256(definition)
		checksum := hex.EncodeToString(sum[:])
		var version int
		if err := tx.GetContext(ctx, &version, q.Get("NextMappingVersion"), id); err != nil {
			return err
		}
		versionID := uuid.New()
		if _, err := tx.ExecContext(ctx, q.Get("InsertMappingVersion"), p.OrgID, versionID, id, version,
			json.RawMessage(definition), checksum, p.ID); err != nil {
			return err
		}
		var vr versionRow
		if err := tx.GetContext(ctx, &vr,
			`SELECT id, mapping_id, version, definition, checksum, published_by, published_at FROM mapping_versions WHERE id = $1`, versionID); err != nil {
			return err
		}
		out = vr.model()
		return nil
	}); err != nil {
		return MappingVersion{}, err
	}
	return out, nil
}

func jsonEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var av, bv any
	_ = json.Unmarshal(ab, &av)
	_ = json.Unmarshal(bb, &bv)
	return reflectJSONEqual(av, bv)
}

func reflectJSONEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	var av, bv any
	_ = json.Unmarshal(ab, &av)
	_ = json.Unmarshal(bb, &bv)
	return string(mustJSON(av)) == string(mustJSON(bv))
}

func mustJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func (s *Service) Test(ctx context.Context, id uuid.UUID, input any) (TestResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return TestResult{}, err
	}
	result := TestResult{}
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "test", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		draft := row.model().Draft
		actual, err := Apply(draft, input)
		if err != nil {
			return httpx.Validation(httpx.FieldError{Field: "input", Code: "invalid", Message: err.Error()})
		}
		result.Output = actual
		tests, err := s.listTestsTx(ctx, tx, id)
		if err != nil {
			return err
		}
		for _, test := range tests {
			testActual, err := Apply(draft, test.Input)
			if err != nil {
				result.StoredTestsFailed = append(result.StoredTestsFailed, TestFailure{Name: test.Name, Actual: nil, Expected: test.Expected})
				continue
			}
			if jsonEqual(testActual, test.Expected) {
				result.StoredTestsPassed++
			} else {
				result.StoredTestsFailed = append(result.StoredTestsFailed, TestFailure{Name: test.Name, Actual: testActual, Expected: test.Expected})
			}
		}
		return nil
	}); err != nil {
		return TestResult{}, err
	}
	return result, nil
}

func (s *Service) listTestsTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) ([]MappingTest, error) {
	var rows []testRow
	if err := tx.SelectContext(ctx, &rows, q.Get("ListMappingTests"), id); err != nil {
		return nil, err
	}
	out := make([]MappingTest, len(rows))
	for i := range rows {
		out[i] = rows[i].model()
	}
	return out, nil
}

func (s *Service) ListTests(ctx context.Context, id uuid.UUID) ([]MappingTest, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []MappingTest
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out, err = s.listTestsTx(ctx, tx, id)
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) CreateTest(ctx context.Context, id uuid.UUID, in TestCreateInput) (MappingTest, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return MappingTest{}, err
	}
	if trimName(in.Name) == "" {
		return MappingTest{}, httpx.Validation(httpx.FieldError{Field: "name", Code: "required", Message: "A test name is required."})
	}
	in.Name = trimName(in.Name)
	var out MappingTest
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		testID := uuid.New()
		if _, err := tx.ExecContext(ctx, q.Get("InsertMappingTest"), p.OrgID, testID, id, in.Name,
			jsonArg(in.Input), jsonArg(in.Expected)); err != nil {
			return err
		}
		var tr testRow
		if err := tx.GetContext(ctx, &tr, q.Get("GetMappingTest"), testID); err != nil {
			return err
		}
		out = tr.model()
		return nil
	}); err != nil {
		return MappingTest{}, err
	}
	return out, nil
}

func (s *Service) UpdateTest(ctx context.Context, mappingID, testID uuid.UUID, in TestUpdateInput) (MappingTest, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return MappingTest{}, err
	}
	var out MappingTest
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, mappingID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, mappingID.String()); err != nil {
			return err
		}
		var tr testRow
		if err := tx.GetContext(ctx, &tr, q.Get("GetMappingTest"), testID); err != nil {
			if db.IsNoRows(err) {
				return errTestNotFound
			}
			return err
		}
		if tr.MappingID != mappingID {
			return errTestNotFound
		}
		var name any
		if in.Name != nil {
			name = trimName(*in.Name)
			if trimName(*in.Name) == "" {
				return httpx.Validation(httpx.FieldError{Field: "name", Code: "required", Message: "A test name is required."})
			}
		}
		var input, expected any
		if in.Input != nil {
			input = jsonArg(in.Input)
		}
		if in.Expected != nil {
			expected = jsonArg(in.Expected)
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateMappingTest"), testID, name, input, expected); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &tr, q.Get("GetMappingTest"), testID); err != nil {
			return err
		}
		out = tr.model()
		return nil
	}); err != nil {
		return MappingTest{}, err
	}
	return out, nil
}

func (s *Service) DeleteTest(ctx context.Context, mappingID, testID uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, mappingID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, mappingID.String()); err != nil {
			return err
		}
		var tr testRow
		if err := tx.GetContext(ctx, &tr, q.Get("GetMappingTest"), testID); err != nil {
			if db.IsNoRows(err) {
				return errTestNotFound
			}
			return err
		}
		if tr.MappingID != mappingID {
			return errTestNotFound
		}
		_, err = tx.ExecContext(ctx, q.Get("DeleteMappingTest"), testID)
		return err
	})
}
