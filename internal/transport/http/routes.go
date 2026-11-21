// Package api assembles the HTTP router for hugi-api.
package api

import (
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/transport/http/handlers"
	"github.com/nuriansyah/hugimethara/internal/transport/http/middleware"
)

// MaxRequestBody is the default body cap for API routes.
const MaxRequestBody = 1 << 20

type Deps = handlers.Deps

type route struct {
	method    string
	path      string
	operation string
}

var routes = []route{
	{"GET", "/healthz", "getLiveness"},
	{"GET", "/readyz", "getReadiness"},
	{"GET", "/v1/hugimethara/system/info", "getSystemInfo"},
	{"POST", "/v1/hugimethara/organizations", "createOrganization"},
	{"POST", "/v1/hugimethara/auth/login", "login"},
	{"POST", "/v1/hugimethara/auth/logout", "logout"},
	{"GET", "/v1/hugimethara/auth/oidc/providers", "listOidcProviders"},
	{"GET", "/v1/hugimethara/auth/oidc/{idp_id}/start", "startOidcLogin"},
	{"GET", "/v1/hugimethara/auth/oidc/callback", "completeOidcLogin"},
	{"GET", "/v1/hugimethara/identity/providers", "listIdentityProviders"},
	{"POST", "/v1/hugimethara/identity/providers", "createIdentityProvider"},
	{"GET", "/v1/hugimethara/identity/providers/{provider_id}", "getIdentityProvider"},
	{"PATCH", "/v1/hugimethara/identity/providers/{provider_id}", "updateIdentityProvider"},
	{"DELETE", "/v1/hugimethara/identity/providers/{provider_id}", "deleteIdentityProvider"},
	{"POST", "/v1/hugimethara/identity/providers/{provider_id}/test", "testIdentityProvider"},
	{"GET", "/v1/hugimethara/identity/providers/{provider_id}/group-mappings", "listGroupMappings"},
	{"POST", "/v1/hugimethara/identity/providers/{provider_id}/group-mappings", "createGroupMapping"},
	{"DELETE", "/v1/hugimethara/identity/providers/{provider_id}/group-mappings/{mapping_id}", "deleteGroupMapping"},
	{"GET", "/v1/hugimethara/me", "getMe"},
	{"GET", "/v1/hugimethara/me/sessions", "listMySessions"},
	{"DELETE", "/v1/hugimethara/me/sessions/{session_id}", "revokeMySession"},
	{"GET", "/v1/hugimethara/organizations/current", "getCurrentOrganization"},
	{"PATCH", "/v1/hugimethara/organizations/current", "updateCurrentOrganization"},
	{"GET", "/v1/hugimethara/audit-events", "listAuditEvents"},
	{"GET", "/v1/hugimethara/credentials", "listCredentials"},
	{"POST", "/v1/hugimethara/credentials", "createCredential"},
	{"GET", "/v1/hugimethara/credentials/{credential_id}", "getCredential"},
	{"PATCH", "/v1/hugimethara/credentials/{credential_id}", "updateCredential"},
	{"DELETE", "/v1/hugimethara/credentials/{credential_id}", "deleteCredential"},
	{"GET", "/v1/hugimethara/connector-definitions", "listConnectorDefinitions"},
	{"GET", "/v1/hugimethara/connector-definitions/{definition_id}/versions", "listConnectorVersions"},
	{"GET", "/v1/hugimethara/connectors", "listConnectors"},
	{"POST", "/v1/hugimethara/connectors", "createConnector"},
	{"GET", "/v1/hugimethara/connectors/{connector_id}", "getConnector"},
	{"PATCH", "/v1/hugimethara/connectors/{connector_id}", "updateConnector"},
	{"DELETE", "/v1/hugimethara/connectors/{connector_id}", "deleteConnector"},
	{"POST", "/v1/hugimethara/connectors/{connector_id}/test", "testConnector"},
	{"GET", "/v1/hugimethara/connectors/{connector_id}/health", "getConnectorHealth"},
	{"GET", "/v1/hugimethara/services", "listServices"},
	{"POST", "/v1/hugimethara/services", "createService"},
	{"GET", "/v1/hugimethara/services/{service_id}", "getService"},
	{"PATCH", "/v1/hugimethara/services/{service_id}", "updateService"},
	{"DELETE", "/v1/hugimethara/services/{service_id}", "deleteService"},
	{"PUT", "/v1/hugimethara/services/{service_id}/environments/{environment_id}", "putServiceEnvironment"},
	{"DELETE", "/v1/hugimethara/services/{service_id}/environments/{environment_id}", "deleteServiceEnvironment"},
	{"POST", "/v1/hugimethara/services/{service_id}/import-openapi", "importOpenAPI"},
	{"GET", "/v1/hugimethara/services/{service_id}/endpoints", "listServiceEndpoints"},
	{"POST", "/v1/hugimethara/service-endpoints/{endpoint_id}/try", "tryServiceEndpoint"},
	{"GET", "/v1/hugimethara/mappings", "listMappings"},
	{"POST", "/v1/hugimethara/mappings", "createMapping"},
	{"GET", "/v1/hugimethara/mappings/{mapping_id}", "getMapping"},
	{"PATCH", "/v1/hugimethara/mappings/{mapping_id}", "updateMapping"},
	{"DELETE", "/v1/hugimethara/mappings/{mapping_id}", "deleteMapping"},
	{"POST", "/v1/hugimethara/mappings/{mapping_id}/publish", "publishMapping"},
	{"GET", "/v1/hugimethara/mappings/{mapping_id}/versions", "listMappingVersions"},
	{"POST", "/v1/hugimethara/mappings/{mapping_id}/test", "testMapping"},
	{"GET", "/v1/hugimethara/mappings/{mapping_id}/tests", "listMappingTests"},
	{"POST", "/v1/hugimethara/mappings/{mapping_id}/tests", "createMappingTest"},
	{"PATCH", "/v1/hugimethara/mappings/{mapping_id}/tests/{test_id}", "updateMappingTest"},
	{"DELETE", "/v1/hugimethara/mappings/{mapping_id}/tests/{test_id}", "deleteMappingTest"},
	{"GET", "/v1/hugimethara/api-compositions", "listApiCompositions"},
	{"POST", "/v1/hugimethara/api-compositions", "createApiComposition"},
	{"GET", "/v1/hugimethara/api-compositions/{composition_id}", "getApiComposition"},
	{"PATCH", "/v1/hugimethara/api-compositions/{composition_id}", "updateApiComposition"},
	{"DELETE", "/v1/hugimethara/api-compositions/{composition_id}", "deleteApiComposition"},
	{"POST", "/v1/hugimethara/api-compositions/{composition_id}/validate", "validateApiComposition"},
	{"POST", "/v1/hugimethara/api-compositions/{composition_id}/publish", "publishApiComposition"},
	{"GET", "/v1/hugimethara/api-compositions/{composition_id}/versions", "listApiCompositionVersions"},
	{"POST", "/v1/hugimethara/api-compositions/{composition_id}/execute", "executeApiComposition"},
	{"GET", "/v1/hugimethara/api-composition-executions", "listApiCompositionExecutions"},
	{"GET", "/v1/hugimethara/api-composition-executions/{execution_id}", "getApiCompositionExecution"},
	{"GET", "/v1/hugimethara/node-types", "listNodeTypes"},
	{"GET", "/v1/hugimethara/workflows", "listWorkflows"},
	{"POST", "/v1/hugimethara/workflows", "createWorkflow"},
	{"GET", "/v1/hugimethara/workflows/{workflow_id}", "getWorkflow"},
	{"PATCH", "/v1/hugimethara/workflows/{workflow_id}", "updateWorkflow"},
	{"DELETE", "/v1/hugimethara/workflows/{workflow_id}", "deleteWorkflow"},
	{"POST", "/v1/hugimethara/workflows/{workflow_id}/validate", "validateWorkflow"},
	{"POST", "/v1/hugimethara/workflows/{workflow_id}/publish", "publishWorkflow"},
	{"GET", "/v1/hugimethara/workflows/{workflow_id}/versions", "listWorkflowVersions"},
	{"GET", "/v1/hugimethara/workflows/{workflow_id}/deployments", "listWorkflowDeployments"},
	{"PUT", "/v1/hugimethara/workflows/{workflow_id}/deployments/{environment_id}", "deployWorkflow"},
	{"POST", "/v1/hugimethara/workflows/{workflow_id}/execute", "executeWorkflow"},
	{"GET", "/v1/hugimethara/workflow-executions", "listWorkflowExecutions"},
	{"GET", "/v1/hugimethara/workflow-executions/{execution_id}", "getWorkflowExecution"},
	{"GET", "/v1/hugimethara/departments", "listDepartments"},
	{"POST", "/v1/hugimethara/departments", "createDepartment"},
	{"PATCH", "/v1/hugimethara/departments/{department_id}", "updateDepartment"},
	{"DELETE", "/v1/hugimethara/departments/{department_id}", "deleteDepartment"},
	{"GET", "/v1/hugimethara/environments", "listEnvironments"},
	{"POST", "/v1/hugimethara/environments", "createEnvironment"},
	{"PATCH", "/v1/hugimethara/environments/{environment_id}", "updateEnvironment"},
	{"DELETE", "/v1/hugimethara/environments/{environment_id}", "deleteEnvironment"},
	{"GET", "/v1/hugimethara/workspaces", "listWorkspaces"},
	{"POST", "/v1/hugimethara/workspaces", "createWorkspace"},
	{"GET", "/v1/hugimethara/workspaces/{workspace}", "getWorkspace"},
	{"PATCH", "/v1/hugimethara/workspaces/{workspace}", "updateWorkspace"},
	{"POST", "/v1/hugimethara/workspaces/{workspace}/archive", "archiveWorkspace"},
	{"POST", "/v1/hugimethara/workspaces/{workspace}/restore", "restoreWorkspace"},
	{"GET", "/v1/hugimethara/workspaces/{workspace}/members", "listWorkspaceMembers"},
	{"POST", "/v1/hugimethara/workspaces/{workspace}/members", "addWorkspaceMember"},
	{"DELETE", "/v1/hugimethara/workspaces/{workspace}/members/{user_id}", "removeWorkspaceMember"},
	{"GET", "/v1/hugimethara/users", "listUsers"},
	{"GET", "/v1/hugimethara/users/{user_id}", "getUser"},
	{"PATCH", "/v1/hugimethara/users/{user_id}", "updateUser"},
	{"DELETE", "/v1/hugimethara/users/{user_id}", "removeUser"},
	{"GET", "/v1/hugimethara/invitations", "listInvitations"},
	{"POST", "/v1/hugimethara/invitations", "createInvitation"},
	{"DELETE", "/v1/hugimethara/invitations/{invitation_id}", "revokeInvitation"},
	{"GET", "/v1/hugimethara/invitation-tokens/{token}", "getInvitationPreview"},
	{"POST", "/v1/hugimethara/invitation-tokens/{token}/accept", "acceptInvitation"},
	{"GET", "/v1/hugimethara/groups", "listGroups"},
	{"POST", "/v1/hugimethara/groups", "createGroup"},
	{"GET", "/v1/hugimethara/groups/{group_id}", "getGroup"},
	{"PATCH", "/v1/hugimethara/groups/{group_id}", "updateGroup"},
	{"DELETE", "/v1/hugimethara/groups/{group_id}", "deleteGroup"},
	{"GET", "/v1/hugimethara/groups/{group_id}/members", "listGroupMembers"},
	{"POST", "/v1/hugimethara/groups/{group_id}/members", "addGroupMember"},
	{"DELETE", "/v1/hugimethara/groups/{group_id}/members/{user_id}", "removeGroupMember"},
	{"GET", "/v1/hugimethara/service-accounts", "listServiceAccounts"},
	{"POST", "/v1/hugimethara/service-accounts", "createServiceAccount"},
	{"GET", "/v1/hugimethara/service-accounts/{service_account_id}", "getServiceAccount"},
	{"PATCH", "/v1/hugimethara/service-accounts/{service_account_id}", "updateServiceAccount"},
	{"DELETE", "/v1/hugimethara/service-accounts/{service_account_id}", "deleteServiceAccount"},
	{"GET", "/v1/hugimethara/service-accounts/{service_account_id}/keys", "listApiKeys"},
	{"POST", "/v1/hugimethara/service-accounts/{service_account_id}/keys", "createApiKey"},
	{"DELETE", "/v1/hugimethara/service-accounts/{service_account_id}/keys/{key_id}", "revokeApiKey"},
	{"GET", "/v1/hugimethara/entitlements", "getEntitlements"},
	{"GET", "/v1/hugimethara/plans", "listPlans"},
	{"GET", "/v1/hugimethara/me/permissions", "getMyPermissions"},
	{"GET", "/v1/hugimethara/resource-types", "listResourceTypes"},
	{"GET", "/v1/hugimethara/roles", "listRoles"},
	{"POST", "/v1/hugimethara/roles", "createRole"},
	{"GET", "/v1/hugimethara/roles/{role_id}", "getRole"},
	{"PUT", "/v1/hugimethara/roles/{role_id}", "updateRole"},
	{"DELETE", "/v1/hugimethara/roles/{role_id}", "deleteRole"},
	{"GET", "/v1/hugimethara/role-bindings", "listRoleBindings"},
	{"POST", "/v1/hugimethara/role-bindings", "createRoleBinding"},
	{"DELETE", "/v1/hugimethara/role-bindings/{binding_id}", "deleteRoleBinding"},
	{"POST", "/v1/hugimethara/authorization/evaluate", "evaluateAccess"},
	{"GET", "/v1/hugimethara/authorization/decisions", "listAccessDecisions"},
}

// PublicOperations lists operations that do not require an authenticated
// principal. This is intentionally explicit because it is a security policy.
func PublicOperations() (map[string]bool, error) {
	return map[string]bool{
		"GetLiveness": true, "GetReadiness": true, "GetSystemInfo": true,
		"CreateOrganization": true, "Login": true, "Logout": true,
		"GetInvitationPreview": true, "AcceptInvitation": true,
		"ListOidcProviders": true, "StartOidcLogin": true, "CompleteOidcLogin": true,
	}, nil
}

// NewRouter returns the root handler for the API listener.
func NewRouter(d Deps) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.Observe(middleware.NewHTTPMetrics(d.Registerer)),
		middleware.Recoverer,
		middleware.SecurityHeaders,
		middleware.CORS(d.TrustedOrigins),
		middleware.ClientInfoMiddleware(d.TrustedProxies),
		middleware.OriginGuard(d.TrustedOrigins),
		middleware.MaxBody(MaxRequestBody),
		d.Identity.Middleware,
	)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		apperrors.WriteError(w, req, apperrors.NotFound("No route matches "+req.Method+" "+req.URL.Path+"."))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		apperrors.WriteError(w, req, apperrors.NewProblem(http.StatusMethodNotAllowed, "method_not_allowed",
			"Method not allowed", req.Method+" is not supported on this route."))
	})

	public, err := PublicOperations()
	if err != nil {
		return nil, err
	}
	srv := handlers.NewServer(d)
	for _, spec := range routes {
		handler, err := newOperationHandler(srv, spec, public[exportedOperation(spec.operation)])
		if err != nil {
			return nil, err
		}
		r.Method(spec.method, spec.path, handler)
	}
	return r, nil
}

func newOperationHandler(server any, spec route, public bool) (http.Handler, error) {
	name := exportedOperation(spec.operation)
	method := reflect.ValueOf(server).MethodByName(name)
	if !method.IsValid() {
		return nil, fmt.Errorf("api: handler %s is not implemented", name)
	}
	methodType := method.Type()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if methodType.NumIn() != 2 || methodType.In(0) != contextType ||
		methodType.NumOut() != 2 || methodType.Out(1) != errorType {
		return nil, fmt.Errorf("api: handler %s has an incompatible signature", name)
	}
	if methodType.In(1).Kind() != reflect.Struct {
		return nil, fmt.Errorf("api: handler %s request must be a struct", name)
	}
	if _, ok := methodType.Out(0).MethodByName("Visit" + name + "Response"); !ok {
		return nil, fmt.Errorf("api: handler %s response does not implement its response visitor", name)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := reflect.New(methodType.In(1)).Elem()
		if err := bindRequest(r, request); err != nil {
			apperrors.WriteError(w, r, apperrors.BadRequest(err.Error()))
			return
		}
		if !public {
			if _, ok := principal.From(r.Context()); !ok {
				apperrors.WriteError(w, r, authzmodel.Unauthenticated())
				return
			}
		}
		result := method.Call([]reflect.Value{reflect.ValueOf(r.Context()), request})
		if !result[1].IsNil() {
			apperrors.WriteError(w, r, result[1].Interface().(error))
			return
		}
		if isNil(result[0]) {
			apperrors.WriteError(w, r, fmt.Errorf("api: handler %s returned an empty response", name))
			return
		}
		response := result[0]
		visitor := response.MethodByName("Visit" + name + "Response")
		if !visitor.IsValid() {
			apperrors.WriteError(w, r, fmt.Errorf("api: handler %s response visitor is missing", name))
			return
		}
		writeResult := visitor.Call([]reflect.Value{reflect.ValueOf(w)})
		if !writeResult[0].IsNil() {
			apperrors.WriteError(w, r, writeResult[0].Interface().(error))
		}
	}), nil
}

func bindRequest(r *http.Request, request reflect.Value) error {
	requestType := request.Type()
	for i := 0; i < request.NumField(); i++ {
		field := request.Field(i)
		fieldType := requestType.Field(i)
		switch fieldType.Name {
		case "Body":
			if field.Kind() != reflect.Pointer {
				return fmt.Errorf("request body has an unsupported type")
			}
			body := reflect.New(field.Type().Elem())
			err := json.NewDecoder(r.Body).Decode(body.Interface())
			if err == io.EOF {
				continue
			}
			if err != nil {
				return fmt.Errorf("The request body could not be parsed as JSON of the expected shape.")
			}
			field.Set(body)
		case "Params":
			if err := bindQuery(r, field); err != nil {
				return err
			}
		default:
			value := chi.URLParam(r, snakeCase(fieldType.Name))
			if value == "" {
				return fmt.Errorf("The path parameter %q is required.", snakeCase(fieldType.Name))
			}
			if err := setText(field, value); err != nil {
				return fmt.Errorf("The path parameter %q is invalid.", snakeCase(fieldType.Name))
			}
		}
	}
	return nil
}

func bindQuery(r *http.Request, params reflect.Value) error {
	paramsType := params.Type()
	query := r.URL.Query()
	for i := 0; i < params.NumField(); i++ {
		field := params.Field(i)
		fieldType := paramsType.Field(i)
		tag := strings.Split(fieldType.Tag.Get("form"), ",")[0]
		if tag == "" {
			tag = snakeCase(fieldType.Name)
		}
		values, ok := query[tag]
		if !ok || len(values) == 0 {
			if field.Kind() != reflect.Pointer && !strings.Contains(fieldType.Tag.Get("form"), "omitempty") {
				return fmt.Errorf("The query parameter %q is required.", tag)
			}
			continue
		}
		if field.Kind() == reflect.Slice {
			slice := reflect.MakeSlice(field.Type(), 0, len(values))
			for _, value := range values {
				item := reflect.New(field.Type().Elem()).Elem()
				if err := setText(item, value); err != nil {
					return fmt.Errorf("The query parameter %q is invalid.", tag)
				}
				slice = reflect.Append(slice, item)
			}
			field.Set(slice)
			continue
		}
		if err := setText(field, values[0]); err != nil {
			return fmt.Errorf("The query parameter %q is invalid.", tag)
		}
	}
	return nil
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func setText(dst reflect.Value, value string) error {
	if dst.Kind() == reflect.Pointer {
		dst.Set(reflect.New(dst.Type().Elem()))
		return setText(dst.Elem(), value)
	}
	if dst.CanAddr() && dst.Addr().Type().Implements(textUnmarshalerType) {
		return dst.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value))
	}
	switch dst.Kind() {
	case reflect.String:
		dst.SetString(value)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		dst.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetFloat(parsed)
	default:
		return fmt.Errorf("unsupported parameter type %s", dst.Type())
	}
	return nil
}

func snakeCase(value string) string {
	var out strings.Builder
	for i, r := range value {
		if unicode.IsUpper(r) && i > 0 {
			out.WriteByte('_')
		}
		out.WriteRune(unicode.ToLower(r))
	}
	return out.String()
}

func exportedOperation(operation string) string {
	if operation == "" {
		return ""
	}
	return strings.ToUpper(operation[:1]) + operation[1:]
}

func isNil(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
