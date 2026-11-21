package model

import (
	"regexp"
	"strings"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Resource types owned by this domain.
const (
	DepartmentResource  = "department"
	EnvironmentResource = "environment"
	WorkspaceResource   = "workspace"
	UserResource        = "user"
)

var shortSlugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$`)

// validateShortSlug accepts 1–40 lowercase letters, digits and single hyphens.
func ValidateShortSlug(field, slug string) *problem.FieldError {
	if !shortSlugRe.MatchString(slug) || strings.Contains(slug, "--") {
		return &problem.FieldError{Field: field, Code: "invalid_slug",
			Message: "Use 1–40 lowercase letters, digits or single hyphens, starting and ending with a letter or digit."}
	}
	return nil
}

func ValidateText(field string, v string, minLen, maxLen int, what string) *problem.FieldError {
	n := len([]rune(strings.TrimSpace(v)))
	if n < minLen || n > maxLen {
		msg := what + " must be at most " + itoa(maxLen) + " characters."
		if minLen > 0 {
			msg = what + " must be " + itoa(minLen) + "–" + itoa(maxLen) + " characters."
		}
		return &problem.FieldError{Field: field, Code: "invalid_length", Message: msg}
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func Collect(errs ...*problem.FieldError) error {
	var out []problem.FieldError
	for _, e := range errs {
		if e != nil {
			out = append(out, *e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return problem.Validation(out...)
}

func RowsAffected(res interface{ RowsAffected() (int64, error) }, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func Conflict(code, detail string) *problem.Problem {
	return problem.NewProblem(409, code, "Conflict", detail)
}
