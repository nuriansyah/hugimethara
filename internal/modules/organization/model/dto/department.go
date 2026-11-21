package dto

// DepartmentInput creates or updates a department (Slug is create-only).
type DepartmentInput struct {
	Slug        string
	Name        string
	Description string
}
