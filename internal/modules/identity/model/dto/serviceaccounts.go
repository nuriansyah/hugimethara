package dto

// ServiceAccountInput creates or updates a service account.
type ServiceAccountInput struct {
	Name        string
	Description string
	Disabled    bool // update only
}
