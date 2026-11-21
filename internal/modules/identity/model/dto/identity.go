package dto

// SignupInput creates an organization and its first owner.
type SignupInput struct {
	OrganizationName string
	OrganizationSlug string
	DisplayName      string
	Email            string
	Password         string
}

// LoginInput authenticates with email and password.
type LoginInput struct {
	Email            string
	Password         string
	OrganizationSlug string // required when the user belongs to several orgs
}
