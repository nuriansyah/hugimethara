package model

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

func EndpointFromConfig(config []byte) (string, error) {
	var values map[string]any
	if err := json.Unmarshal(config, &values); err != nil {
		return "", apperrors.BadRequest("Connector configuration is unavailable for testing.")
	}
	for _, key := range []string{"base_url", "endpoint", "webhook_url"} {
		raw, ok := values[key].(string)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil ||
			u.Fragment != "" {
			return "", apperrors.Validation(apperrors.FieldError{Field: "config", Code: "invalid_test_endpoint",
				Message: "Connector reachability tests require an HTTP(S) URL without user info or a fragment."})
		}
		return u.String(), nil
	}
	return "", apperrors.NewProblem(http.StatusUnprocessableEntity, "connector_test_unsupported", "Test unavailable",
		"This connector configuration does not provide an HTTP(S) reachability endpoint.")
}

func TcpEndpointFromConfig(config []byte) (string, error) {
	var values map[string]any
	if err := json.Unmarshal(config, &values); err != nil {
		return "", apperrors.BadRequest("Connector configuration is unavailable for testing.")
	}
	host, _ := values["host"].(string)
	portValue, ok := values["port"].(float64)
	if !ok || portValue < 1 || portValue > 65535 || portValue != float64(int(portValue)) || strings.TrimSpace(host) == "" {
		return "", apperrors.NewProblem(http.StatusUnprocessableEntity, "connector_test_unsupported", "Test unavailable",
			"This connector configuration does not provide a valid host and port for a reachability test.")
	}
	return net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(int(portValue))), nil
}
