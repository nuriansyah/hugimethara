package organization

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSlug(t *testing.T) {
	for _, ok := range []string{"acme", "acme-corp", "a1b", "x2-y3-z4"} {
		require.Nil(t, ValidateSlug("slug", ok), ok)
	}
	for _, bad := range []string{"", "ab", "-acme", "acme-", "Acme", "ac--me", "acme_corp", "admin", "api"} {
		require.NotNil(t, ValidateSlug("slug", bad), bad)
	}
}

func TestSuggestSlug(t *testing.T) {
	require.Equal(t, "pt-maju-jaya-tbk", SuggestSlug("PT. Maju Jaya, Tbk"))
	require.Equal(t, "acme", SuggestSlug("  Acme!!  "))
}

func TestSettingsValidation(t *testing.T) {
	require.Empty(t, DefaultSettings().Validate(""))
	require.Len(t, Settings{SessionIdleTimeoutMinutes: 1, SessionAbsoluteTimeoutHours: 1}.Validate(""), 1)
	errs := Settings{SessionIdleTimeoutMinutes: 600, SessionAbsoluteTimeoutHours: 2}.Validate("settings.")
	require.Len(t, errs, 1)
	require.Equal(t, "exceeds_absolute", errs[0].Code)
}

func TestParseSettingsDefaults(t *testing.T) {
	require.Equal(t, DefaultSettings(), ParseSettings(nil))
	require.Equal(t, 30, ParseSettings([]byte(`{"session_idle_timeout_minutes":30}`)).SessionIdleTimeoutMinutes)
	require.Equal(t, DefaultAbsoluteHours, ParseSettings([]byte(`{"session_idle_timeout_minutes":30}`)).SessionAbsoluteTimeoutHours)
}
