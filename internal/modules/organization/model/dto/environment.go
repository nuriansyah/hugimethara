package dto

// EnvironmentInput creates or updates an environment (Slug is create-only).
type EnvironmentInput struct {
	Slug            string
	Name            string
	IsProduction    bool
	EgressAllowlist []string
}
