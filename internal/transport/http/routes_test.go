package api

import (
	"context"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

func TestBindRequestPathQueryAndBody(t *testing.T) {
	userID := uuid.New()
	pathCtx := chi.NewRouteContext()
	pathCtx.URLParams.Add("user_id", userID.String())
	pathRequest := httptest.NewRequest("GET", "/users/"+userID.String(), nil)
	pathRequest = pathRequest.WithContext(context.WithValue(pathRequest.Context(), chi.RouteCtxKey, pathCtx))

	pathDTO := reflect.New(reflect.TypeOf(dto.GetUserRequestObject{})).Elem()
	require.NoError(t, bindRequest(pathRequest, pathDTO))
	require.Equal(t, userID, pathDTO.Interface().(dto.GetUserRequestObject).UserId)

	since := "2026-01-02T03:04:05Z"
	queryRequest := httptest.NewRequest("GET", "/audit-events?since="+since+"&limit=7", nil)
	queryDTO := reflect.New(reflect.TypeOf(dto.ListAuditEventsRequestObject{})).Elem()
	require.NoError(t, bindRequest(queryRequest, queryDTO))
	query := queryDTO.Interface().(dto.ListAuditEventsRequestObject).Params
	require.Equal(t, 7, *query.Limit)
	require.Equal(t, since, query.Since.Format("2006-01-02T15:04:05Z"))

	bodyRequest := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
	bodyDTO := reflect.New(reflect.TypeOf(dto.LoginRequestObject{})).Elem()
	require.NoError(t, bindRequest(bodyRequest, bodyDTO))
	body := bodyDTO.Interface().(dto.LoginRequestObject).Body
	require.Equal(t, "user@example.com", body.Email)
	require.Equal(t, "secret", body.Password)
}

func TestBindRequestRejectsInvalidPathAndQuery(t *testing.T) {
	pathCtx := chi.NewRouteContext()
	pathCtx.URLParams.Add("user_id", "not-a-uuid")
	pathRequest := httptest.NewRequest("GET", "/users/not-a-uuid", nil)
	pathRequest = pathRequest.WithContext(context.WithValue(pathRequest.Context(), chi.RouteCtxKey, pathCtx))
	pathDTO := reflect.New(reflect.TypeOf(dto.GetUserRequestObject{})).Elem()
	require.Error(t, bindRequest(pathRequest, pathDTO))

	queryRequest := httptest.NewRequest("GET", "/mappings", nil)
	queryDTO := reflect.New(reflect.TypeOf(dto.ListMappingsRequestObject{})).Elem()
	require.Error(t, bindRequest(queryRequest, queryDTO))
}
