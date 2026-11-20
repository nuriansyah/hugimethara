package mapping

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/transport/http/apigen"
)

type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiMapping(m Mapping) apigen.Mapping {
	var input, output *map[string]interface{}
	if m.InputSchema != nil {
		v := m.InputSchema
		input = &v
	}
	if m.OutputSchema != nil {
		v := m.OutputSchema
		output = &v
	}
	return apigen.Mapping{Id: m.ID, WorkspaceId: m.WorkspaceID, Name: m.Name, Draft: m.Draft,
		InputSchema: input, OutputSchema: output, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func apiVersion(v MappingVersion) apigen.MappingVersion {
	return apigen.MappingVersion{Id: v.ID, MappingId: v.MappingID, Version: v.Version,
		Definition: v.Definition, Checksum: v.Checksum, PublishedBy: v.PublishedBy, PublishedAt: v.PublishedAt}
}

func apiTest(t MappingTest) apigen.MappingTest {
	return apigen.MappingTest{Id: t.ID, MappingId: t.MappingID, Name: t.Name,
		Input: t.Input, Expected: t.Expected, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func (h *HTTP) ListMappings(ctx context.Context, req apigen.ListMappingsRequestObject) (apigen.ListMappingsResponseObject, error) {
	rows, err := h.svc.List(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := apigen.ListMappings200JSONResponse{Data: make([]apigen.Mapping, len(rows))}
	for i := range rows {
		out.Data[i] = apiMapping(rows[i])
	}
	return out, nil
}

func (h *HTTP) CreateMapping(ctx context.Context, req apigen.CreateMappingRequestObject) (apigen.CreateMappingResponseObject, error) {
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
	return apigen.CreateMapping201JSONResponse(apiMapping(row)), nil
}

func (h *HTTP) GetMapping(ctx context.Context, req apigen.GetMappingRequestObject) (apigen.GetMappingResponseObject, error) {
	row, err := h.svc.Get(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	return apigen.GetMapping200JSONResponse(apiMapping(row)), nil
}

func (h *HTTP) UpdateMapping(ctx context.Context, req apigen.UpdateMappingRequestObject) (apigen.UpdateMappingResponseObject, error) {
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
	return apigen.UpdateMapping200JSONResponse(apiMapping(row)), nil
}

func (h *HTTP) DeleteMapping(ctx context.Context, req apigen.DeleteMappingRequestObject) (apigen.DeleteMappingResponseObject, error) {
	if err := h.svc.Delete(ctx, uuid.UUID(req.MappingId)); err != nil {
		return nil, err
	}
	return apigen.DeleteMapping204Response{}, nil
}

func (h *HTTP) PublishMapping(ctx context.Context, req apigen.PublishMappingRequestObject) (apigen.PublishMappingResponseObject, error) {
	row, err := h.svc.Publish(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	return apigen.PublishMapping201JSONResponse(apiVersion(row)), nil
}

func (h *HTTP) ListMappingVersions(ctx context.Context, req apigen.ListMappingVersionsRequestObject) (apigen.ListMappingVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	out := apigen.ListMappingVersions200JSONResponse{Data: make([]apigen.MappingVersion, len(rows))}
	for i := range rows {
		out.Data[i] = apiVersion(rows[i])
	}
	return out, nil
}

func (h *HTTP) TestMapping(ctx context.Context, req apigen.TestMappingRequestObject) (apigen.TestMappingResponseObject, error) {
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
	return apigen.TestMapping200JSONResponse{Output: result.Output, StoredTestsPassed: result.StoredTestsPassed, StoredTestsFailed: failed}, nil
}

func (h *HTTP) ListMappingTests(ctx context.Context, req apigen.ListMappingTestsRequestObject) (apigen.ListMappingTestsResponseObject, error) {
	rows, err := h.svc.ListTests(ctx, uuid.UUID(req.MappingId))
	if err != nil {
		return nil, err
	}
	out := apigen.ListMappingTests200JSONResponse{Data: make([]apigen.MappingTest, len(rows))}
	for i := range rows {
		out.Data[i] = apiTest(rows[i])
	}
	return out, nil
}

func (h *HTTP) CreateMappingTest(ctx context.Context, req apigen.CreateMappingTestRequestObject) (apigen.CreateMappingTestResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateTest(ctx, uuid.UUID(req.MappingId), TestCreateInput{Name: req.Body.Name, Input: req.Body.Input, Expected: req.Body.Expected})
	if err != nil {
		return nil, err
	}
	return apigen.CreateMappingTest201JSONResponse(apiTest(row)), nil
}

func (h *HTTP) UpdateMappingTest(ctx context.Context, req apigen.UpdateMappingTestRequestObject) (apigen.UpdateMappingTestResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := TestUpdateInput{Name: req.Body.Name, Input: req.Body.Input, Expected: req.Body.Expected}
	row, err := h.svc.UpdateTest(ctx, uuid.UUID(req.MappingId), uuid.UUID(req.TestId), in)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateMappingTest200JSONResponse(apiTest(row)), nil
}

func (h *HTTP) DeleteMappingTest(ctx context.Context, req apigen.DeleteMappingTestRequestObject) (apigen.DeleteMappingTestResponseObject, error) {
	if err := h.svc.DeleteTest(ctx, uuid.UUID(req.MappingId), uuid.UUID(req.TestId)); err != nil {
		return nil, err
	}
	return apigen.DeleteMappingTest204Response{}, nil
}
