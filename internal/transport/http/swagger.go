// Package api serves the Hugimethara HTTP API.
//
// @title Hugimethara API
// @version 0.1.0
// @description Unified Business Operations & Intelligence Platform API.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey sessionCookie
// @in header
// @name Cookie
// @description Send the hugi_session cookie issued after sign-in.
// @securityDefinitions.apikey apiKey
// @in header
// @name Authorization
// @description Send a service-account API key as a Bearer token.
//
//go:generate sh -c "cd ../../../ && go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo internal/transport/http/swagger.go --dir . --output api/openapi --outputTypes json --parseInternal --exclude internal/modules,web,packages,node_modules && go run ./scripts/swaggergen --input api/openapi/swagger.json --output api/openapi/openapi.yaml"
package api
