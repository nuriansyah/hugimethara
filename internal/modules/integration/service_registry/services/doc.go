// Package services implements the integration service registry use cases. service.go declares the
// ports (interfaces) other layers depend on; the concrete implementation
// enforces authorization, tenancy, audit and outbox events per operation.
package services
