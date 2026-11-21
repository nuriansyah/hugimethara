package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	mappingmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

type MappingService struct {
	pool *sqlx.DB
	az   authzsvc.Authorizer
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer) *MappingService {
	return &MappingService{pool: pool, az: az}
}

func (s *MappingService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *MappingService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *MappingService) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: mappingmodel.Resource, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func (s *MappingService) List(ctx context.Context, workspaceID uuid.UUID) ([]mappingmodel.Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.MappingRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListMappings"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]mappingmodel.Mapping, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *MappingService) get(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*repository.MappingRow, error) {
	var row repository.MappingRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetMapping"), id); err != nil {
		if db.IsNoRows(err) {
			return nil, mappingmodel.ErrMappingNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *MappingService) Create(ctx context.Context, in dto.CreateInput) (mappingmodel.Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.Mapping{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return mappingmodel.Mapping{}, err
	}
	if in.Name = mappingmodel.TrimName(in.Name); in.Name == "" {
		return mappingmodel.Mapping{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A mapping name is required."})
	}
	if in.Draft == nil {
		in.Draft = map[string]any{"rules": []any{}}
	}
	if err := ValidateDraft(in.Draft); err != nil {
		return mappingmodel.Mapping{}, problem.Validation(problem.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
	}
	id := uuid.New()
	var out mappingmodel.Mapping
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
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMapping"), p.OrgID, id, in.WorkspaceID, in.Name,
			draft, mappingmodel.NullableRaw(input), mappingmodel.NullableRaw(output)); err != nil {
			return err
		}
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row.Model()
		return nil
	}); err != nil {
		return mappingmodel.Mapping{}, err
	}
	return out, nil
}

func (s *MappingService) Get(ctx context.Context, id uuid.UUID) (mappingmodel.Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.Mapping{}, err
	}
	var out mappingmodel.Mapping
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row.Model()
		return nil
	}); err != nil {
		return mappingmodel.Mapping{}, err
	}
	return out, nil
}

func (s *MappingService) Update(ctx context.Context, id uuid.UUID, in dto.UpdateInput) (mappingmodel.Mapping, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.Mapping{}, err
	}
	var out mappingmodel.Mapping
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if in.Name != nil {
			name := mappingmodel.TrimName(*in.Name)
			if name == "" {
				return problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A mapping name is required."})
			}
			in.Name = &name
		}
		if in.Draft != nil {
			if err := ValidateDraft(*in.Draft); err != nil {
				return problem.Validation(problem.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
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
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateMapping"), id, in.Name, draft, input, output); err != nil {
			return err
		}
		row, err = s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row.Model()
		return nil
	}); err != nil {
		return mappingmodel.Mapping{}, err
	}
	return out, nil
}

func (s *MappingService) Delete(ctx context.Context, id uuid.UUID) error {
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
		_, err = tx.ExecContext(ctx, commands.SQL.Get("DeleteMapping"), id)
		return err
	})
}

func (s *MappingService) ListVersions(ctx context.Context, id uuid.UUID) ([]mappingmodel.MappingVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []mappingmodel.MappingVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []repository.VersionRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListMappingVersions"), id); err != nil {
			return err
		}
		out = make([]mappingmodel.MappingVersion, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *MappingService) Publish(ctx context.Context, id uuid.UUID) (mappingmodel.MappingVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.MappingVersion{}, err
	}
	var out mappingmodel.MappingVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "publish", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		draft := row.Model().Draft
		if err := ValidateDraft(draft); err != nil {
			return problem.Validation(problem.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
		}
		tests, err := s.listTestsTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(tests) == 0 {
			return problem.Validation(problem.FieldError{Field: "tests", Code: "required", Message: "Add at least one passing test before publishing."})
		}
		for _, test := range tests {
			actual, err := Apply(draft, test.Input)
			if err != nil {
				return problem.Validation(problem.FieldError{Field: "draft", Code: "invalid", Message: err.Error()})
			}
			if !mappingmodel.JsonEqual(actual, test.Expected) {
				return problem.Validation(problem.FieldError{Field: "tests", Code: "failed", Message: "Stored test \"" + test.Name + "\" does not pass."})
			}
		}
		definition, _ := json.Marshal(draft)
		sum := sha256.Sum256(definition)
		checksum := hex.EncodeToString(sum[:])
		var version int
		if err := tx.GetContext(ctx, &version, queries.SQL.Get("NextMappingVersion"), id); err != nil {
			return err
		}
		versionID := uuid.New()
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMappingVersion"), p.OrgID, versionID, id, version,
			json.RawMessage(definition), checksum, p.ID); err != nil {
			return err
		}
		var vr repository.VersionRow
		if err := tx.GetContext(ctx, &vr,
			`SELECT id, mapping_id, version, definition, checksum, published_by, published_at FROM mapping_versions WHERE id = $1`, versionID); err != nil {
			return err
		}
		out = vr.Model()
		return nil
	}); err != nil {
		return mappingmodel.MappingVersion{}, err
	}
	return out, nil
}

func (s *MappingService) Test(ctx context.Context, id uuid.UUID, input any) (mappingmodel.TestResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.TestResult{}, err
	}
	result := mappingmodel.TestResult{}
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "test", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		draft := row.Model().Draft
		actual, err := Apply(draft, input)
		if err != nil {
			return problem.Validation(problem.FieldError{Field: "input", Code: "invalid", Message: err.Error()})
		}
		result.Output = actual
		tests, err := s.listTestsTx(ctx, tx, id)
		if err != nil {
			return err
		}
		for _, test := range tests {
			testActual, err := Apply(draft, test.Input)
			if err != nil {
				result.StoredTestsFailed = append(result.StoredTestsFailed, mappingmodel.TestFailure{Name: test.Name, Actual: nil, Expected: test.Expected})
				continue
			}
			if mappingmodel.JsonEqual(testActual, test.Expected) {
				result.StoredTestsPassed++
			} else {
				result.StoredTestsFailed = append(result.StoredTestsFailed, mappingmodel.TestFailure{Name: test.Name, Actual: testActual, Expected: test.Expected})
			}
		}
		return nil
	}); err != nil {
		return mappingmodel.TestResult{}, err
	}
	return result, nil
}

func (s *MappingService) listTestsTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) ([]mappingmodel.MappingTest, error) {
	var rows []repository.TestRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListMappingTests"), id); err != nil {
		return nil, err
	}
	out := make([]mappingmodel.MappingTest, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *MappingService) ListTests(ctx context.Context, id uuid.UUID) ([]mappingmodel.MappingTest, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []mappingmodel.MappingTest
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

func (s *MappingService) CreateTest(ctx context.Context, id uuid.UUID, in dto.TestCreateInput) (mappingmodel.MappingTest, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.MappingTest{}, err
	}
	if mappingmodel.TrimName(in.Name) == "" {
		return mappingmodel.MappingTest{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A test name is required."})
	}
	in.Name = mappingmodel.TrimName(in.Name)
	var out mappingmodel.MappingTest
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		testID := uuid.New()
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMappingTest"), p.OrgID, testID, id, in.Name,
			mappingmodel.JsonArg(in.Input), mappingmodel.JsonArg(in.Expected)); err != nil {
			return err
		}
		var tr repository.TestRow
		if err := tx.GetContext(ctx, &tr, queries.SQL.Get("GetMappingTest"), testID); err != nil {
			return err
		}
		out = tr.Model()
		return nil
	}); err != nil {
		return mappingmodel.MappingTest{}, err
	}
	return out, nil
}

func (s *MappingService) UpdateTest(ctx context.Context, mappingID, testID uuid.UUID, in dto.TestUpdateInput) (mappingmodel.MappingTest, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return mappingmodel.MappingTest{}, err
	}
	var out mappingmodel.MappingTest
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.get(ctx, tx, mappingID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, mappingID.String()); err != nil {
			return err
		}
		var tr repository.TestRow
		if err := tx.GetContext(ctx, &tr, queries.SQL.Get("GetMappingTest"), testID); err != nil {
			if db.IsNoRows(err) {
				return mappingmodel.ErrTestNotFound
			}
			return err
		}
		if tr.MappingID != mappingID {
			return mappingmodel.ErrTestNotFound
		}
		var name any
		if in.Name != nil {
			name = mappingmodel.TrimName(*in.Name)
			if mappingmodel.TrimName(*in.Name) == "" {
				return problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A test name is required."})
			}
		}
		var input, expected any
		if in.Input != nil {
			input = mappingmodel.JsonArg(in.Input)
		}
		if in.Expected != nil {
			expected = mappingmodel.JsonArg(in.Expected)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateMappingTest"), testID, name, input, expected); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &tr, queries.SQL.Get("GetMappingTest"), testID); err != nil {
			return err
		}
		out = tr.Model()
		return nil
	}); err != nil {
		return mappingmodel.MappingTest{}, err
	}
	return out, nil
}

func (s *MappingService) DeleteTest(ctx context.Context, mappingID, testID uuid.UUID) error {
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
		var tr repository.TestRow
		if err := tx.GetContext(ctx, &tr, queries.SQL.Get("GetMappingTest"), testID); err != nil {
			if db.IsNoRows(err) {
				return mappingmodel.ErrTestNotFound
			}
			return err
		}
		if tr.MappingID != mappingID {
			return mappingmodel.ErrTestNotFound
		}
		_, err = tx.ExecContext(ctx, commands.SQL.Get("DeleteMappingTest"), testID)
		return err
	})
}
