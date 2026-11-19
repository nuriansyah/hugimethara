package organization

import (
	"context"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
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
func validateShortSlug(field, slug string) *httpx.FieldError {
	if !shortSlugRe.MatchString(slug) || strings.Contains(slug, "--") {
		return &httpx.FieldError{Field: field, Code: "invalid_slug",
			Message: "Use 1–40 lowercase letters, digits or single hyphens, starting and ending with a letter or digit."}
	}
	return nil
}

func validateText(field string, v string, minLen, maxLen int, what string) *httpx.FieldError {
	n := len([]rune(strings.TrimSpace(v)))
	if n < minLen || n > maxLen {
		msg := what + " must be at most " + itoa(maxLen) + " characters."
		if minLen > 0 {
			msg = what + " must be " + itoa(minLen) + "–" + itoa(maxLen) + " characters."
		}
		return &httpx.FieldError{Field: field, Code: "invalid_length", Message: msg}
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

func collect(errs ...*httpx.FieldError) error {
	var out []httpx.FieldError
	for _, e := range errs {
		if e != nil {
			out = append(out, *e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return httpx.Validation(out...)
}

// authorize checks action on a resource in the caller's organization.
func (s *Service) authorize(ctx context.Context, action, resourceType, id string, attrs map[string]any) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authz.Unauthenticated()
	}
	return authz.Require(ctx, s.az, action, authz.Resource{Type: resourceType, ID: id, OrgID: p.OrgID, Attributes: attrs})
}

// tx runs fn in a transaction bound to the caller's organization.
func (s *Service) tx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func rowsAffected(res interface{ RowsAffected() (int64, error) }, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func conflict(code, detail string) *httpx.Problem {
	return httpx.NewProblem(409, code, "Conflict", detail)
}
