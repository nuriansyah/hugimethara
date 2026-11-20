package mapping

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiMapping(m Mapping) dto.Mapping {
	var input, output *map[string]interface{}
	if m.InputSchema != nil {
		v := m.InputSchema
		input = &v
	}
	if m.OutputSchema != nil {
		v := m.OutputSchema
		output = &v
	}
	return dto.Mapping{Id: m.ID, WorkspaceId: m.WorkspaceID, Name: m.Name, Draft: m.Draft,
		InputSchema: input, OutputSchema: output, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func apiVersion(v MappingVersion) dto.MappingVersion {
	return dto.MappingVersion{Id: v.ID, MappingId: v.MappingID, Version: v.Version,
		Definition: v.Definition, Checksum: v.Checksum, PublishedBy: v.PublishedBy, PublishedAt: v.PublishedAt}
}

func apiTest(t MappingTest) dto.MappingTest {
	return dto.MappingTest{Id: t.ID, MappingId: t.MappingID, Name: t.Name,
		Input: t.Input, Expected: t.Expected, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

// @Summary List mappings in a workspace
// @ID listMappings
// @Tags mappings
// @Param workspace_id query string true "workspace_id"
// @Success 200 {object} dto.ListMappings200JSONResponse "Mappings."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings [get]
func (h *HTTP) ListMappings(ctx context.Context, req dto.ListMappingsRequestObject) (dto.ListMappingsResponseObject, error) {
	rows, err := h.svc.List(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := dto.ListMappings200JSONResponse{Data: make([]dto.Mapping, len(rows))}
	for i := range rows {
		out.Data[i] = apiMapping(rows[i])
	}
	return out, nil
}

// @Summary Create a mapping draft
// @ID createMapping
// @Tags mappings
// @Param body body dto.MappingCreate true "Request body"
// @Success 201 {object} dto.Mapping "Created mapping."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings [post]
func (h *HTTP) CreateMapping(ctx context.Context, req dto.CreateMappingRequestObject) (dto.CreateMappingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := CreateInput{WorkspaceID: req.Body.WorkspaceId, Name: req.Body.Name}
	if req.Body.Draft != nil {
		in.Draft = *req.Body.Draft
	}
	if req.Body.InputSchema != nil {
		in.InputSchema = *req.Body.InputSchema
	}
	if req.Body.OutputSchema != nil {
		in.OutputSchema = *req.Body.OutputSchema
	}
	row, err := h.svc.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return dto.CreateMapping201JSONResponse(apiMapping(row)), nil
}

// @Summary Get a mapping draft
// @ID getMapping
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Success 200 {object} dto.Mapping "Mapping"
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id} [get]
func (h *HTTP) GetMapping(ctx context.Context, req dto.GetMappingRequestObject) (dto.GetMappingResponseObject, error) {
	row, err := h.svc.Get(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	return dto.GetMapping200JSONResponse(apiMapping(row)), nil
}

// @Summary Update a mapping draft
// @ID updateMapping
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Param body body dto.MappingUpdate true "Request body"
// @Success 200 {object} dto.Mapping "Updated mapping"
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id} [patch]
func (h *HTTP) UpdateMapping(ctx context.Context, req dto.UpdateMappingRequestObject) (dto.UpdateMappingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := UpdateInput{Name: req.Body.Name}
	if req.Body.Draft != nil {
		draft := *req.Body.Draft
		in.Draft = &draft
	}
	if req.Body.InputSchema != nil {
		schema := *req.Body.InputSchema
		in.InputSchema = &schema
	}
	if req.Body.OutputSchema != nil {
		schema := *req.Body.OutputSchema
		in.OutputSchema = &schema
	}
	row, err := h.svc.Update(ctx, uuid.UUID(req.MappingId), in)
	if err != nil {
		return nil, err
	}
	return dto.UpdateMapping200JSONResponse(apiMapping(row)), nil
}

// @Summary Delete a mapping and its versions
// @ID deleteMapping
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id} [delete]
func (h *HTTP) DeleteMapping(ctx context.Context, req dto.DeleteMappingRequestObject) (dto.DeleteMappingResponseObject, error) {
	if err := h.svc.Delete(ctx, uuid.UUID(req.MappingId)); err != nil {
		return nil, err
	}
	return dto.DeleteMapping204Response{}, nil
}

// @Summary Publish a tested immutable mapping version
// @ID publishMapping
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Success 201 {object} dto.MappingVersion "Published mapping version."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/publish [post]
func (h *HTTP) PublishMapping(ctx context.Context, req dto.PublishMappingRequestObject) (dto.PublishMappingResponseObject, error) {
	row, err := h.svc.Publish(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	return dto.PublishMapping201JSONResponse(apiVersion(row)), nil
}

// @Summary List immutable mapping versions
// @ID listMappingVersions
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Success 200 {object} dto.ListMappingVersions200JSONResponse "Mapping versions."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/versions [get]
func (h *HTTP) ListMappingVersions(ctx context.Context, req dto.ListMappingVersionsRequestObject) (dto.ListMappingVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	out := dto.ListMappingVersions200JSONResponse{Data: make([]dto.MappingVersion, len(rows))}
	for i := range rows {
		out.Data[i] = apiVersion(rows[i])
	}
	return out, nil
}

// @Summary Evaluate mapping input ad hoc or run stored tests
// @ID testMapping
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Param body body dto.MappingTestRequest true "Request body"
// @Success 200 {object} dto.MappingTestResult "Mapping evaluation result."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/test [post]
func (h *HTTP) TestMapping(ctx context.Context, req dto.TestMappingRequestObject) (dto.TestMappingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	result, err := h.svc.Test(ctx, uuid.UUID(req.MappingId), req.Body.Input)
	if err != nil {
		return nil, err
	}
	failed := make([]struct {
		Actual   interface{} `json:"actual"`
		Expected interface{} `json:"expected"`
		Name     string      `json:"name"`
	}, len(result.StoredTestsFailed))
	for i := range result.StoredTestsFailed {
		failed[i].Name = result.StoredTestsFailed[i].Name
		failed[i].Actual = result.StoredTestsFailed[i].Actual
		failed[i].Expected = result.StoredTestsFailed[i].Expected
	}
	return dto.TestMapping200JSONResponse{Output: result.Output, StoredTestsPassed: result.StoredTestsPassed, StoredTestsFailed: failed}, nil
}

// @Summary List stored mapping test cases
// @ID listMappingTests
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Success 200 {object} dto.ListMappingTests200JSONResponse "Test cases."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/tests [get]
func (h *HTTP) ListMappingTests(ctx context.Context, req dto.ListMappingTestsRequestObject) (dto.ListMappingTestsResponseObject, error) {
	rows, err := h.svc.ListTests(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	out := dto.ListMappingTests200JSONResponse{Data: make([]dto.MappingTest, len(rows))}
	for i := range rows {
		out.Data[i] = apiTest(rows[i])
	}
	return out, nil
}

// @Summary Create a mapping test case
// @ID createMappingTest
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Param body body dto.MappingTestCreate true "Request body"
// @Success 201 {object} dto.MappingTest "Test case."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/tests [post]
func (h *HTTP) CreateMappingTest(ctx context.Context, req dto.CreateMappingTestRequestObject) (dto.CreateMappingTestResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateTest(ctx, uuid.UUID(req.MappingId), TestCreateInput{Name: req.Body.Name, Input: req.Body.Input, Expected: req.Body.Expected})
	if err != nil {
		return nil, err
	}
	return dto.CreateMappingTest201JSONResponse(apiTest(row)), nil
}

// @Summary Update a mapping test case
// @ID updateMappingTest
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Param test_id path string true "test_id"
// @Param body body dto.MappingTestUpdate true "Request body"
// @Success 200 {object} dto.MappingTest "Updated test"
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/tests/{test_id} [patch]
func (h *HTTP) UpdateMappingTest(ctx context.Context, req dto.UpdateMappingTestRequestObject) (dto.UpdateMappingTestResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := TestUpdateInput{Name: req.Body.Name, Input: req.Body.Input, Expected: req.Body.Expected}
	row, err := h.svc.UpdateTest(ctx, uuid.UUID(req.MappingId), uuid.UUID(req.TestId), in)
	if err != nil {
		return nil, err
	}
	return dto.UpdateMappingTest200JSONResponse(apiTest(row)), nil
}

// @Summary Delete a mapping test case
// @ID deleteMappingTest
// @Tags mappings
// @Param mapping_id path string true "mapping_id"
// @Param test_id path string true "test_id"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/mappings/{mapping_id}/tests/{test_id} [delete]
func (h *HTTP) DeleteMappingTest(ctx context.Context, req dto.DeleteMappingTestRequestObject) (dto.DeleteMappingTestResponseObject, error) {
	if err := h.svc.DeleteTest(ctx, uuid.UUID(req.MappingId), uuid.UUID(req.TestId)); err != nil {
		return nil, err
	}
	return dto.DeleteMappingTest204Response{}, nil
}
