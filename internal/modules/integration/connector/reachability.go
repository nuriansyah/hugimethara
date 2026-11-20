package connector

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nuriansyah/hugimethara/internal/platform/egress"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
)

func endpointFromConfig(config []byte) (string, error) {
	var values map[string]any
	if err := json.Unmarshal(config, &values); err != nil {
		return "", httpx.BadRequest("Connector configuration is unavailable for testing.")
	}
	for _, key := range []string{"base_url", "endpoint", "webhook_url"} {
		raw, ok := values[key].(string)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil ||
			u.Fragment != "" {
			return "", httpx.Validation(httpx.FieldError{Field: "config", Code: "invalid_test_endpoint",
				Message: "Connector reachability tests require an HTTP(S) URL without user info or a fragment."})
		}
		return u.String(), nil
	}
	return "", httpx.NewProblem(http.StatusUnprocessableEntity, "connector_test_unsupported", "Test unavailable",
		"This connector configuration does not provide an HTTP(S) reachability endpoint.")
}

func tcpEndpointFromConfig(config []byte) (string, error) {
	var values map[string]any
	if err := json.Unmarshal(config, &values); err != nil {
		return "", httpx.BadRequest("Connector configuration is unavailable for testing.")
	}
	host, _ := values["host"].(string)
	portValue, ok := values["port"].(float64)
	if !ok || portValue < 1 || portValue > 65535 || portValue != float64(int(portValue)) || strings.TrimSpace(host) == "" {
		return "", httpx.NewProblem(http.StatusUnprocessableEntity, "connector_test_unsupported", "Test unavailable",
			"This connector configuration does not provide a valid host and port for a reachability test.")
	}
	return net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(int(portValue))), nil
}

func parseEnvironmentAllowlist(encoded []byte) (egress.Policy, error) {
	var allowlist []string
	if err := json.Unmarshal(encoded, &allowlist); err != nil {
		return egress.Policy{}, errors.New("connector: invalid environment egress allowlist")
	}
	if allowlist == nil {
		return egress.Policy{}, errors.New("connector: invalid environment egress allowlist")
	}
	return egress.ParsePolicy(allowlist)
}

func checkTCP(ctx context.Context, address string, policy egress.Policy, timeoutMs int) TestResult {
	started := time.Now()
	result := TestResult{CheckedAt: started.UTC(), Message: "Could not reach the configured endpoint."}
	if timeoutMs < 100 || timeoutMs > 120000 {
		timeoutMs = 10000
	}
	dialer := &net.Dialer{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
		Control: func(_, resolved string, _ syscall.RawConn) error {
			addr, err := netip.ParseAddrPort(resolved)
			if err != nil || !policy.Permits(addr.Addr()) {
				return errors.Join(egress.ErrBlockedDestination, errors.New("resolved destination is not allowed"))
			}
			return nil
		},
	}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	result.LatencyMs = int(time.Since(started).Milliseconds())
	if err != nil {
		if errors.Is(err, egress.ErrBlockedDestination) {
			result.ErrorCode = "destination_blocked"
			result.Message = "The endpoint resolves to a destination that is not allowed."
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.ErrorCode = "timeout"
			result.Message = "The endpoint did not respond before the timeout."
		} else {
			result.ErrorCode = "unreachable"
		}
		return result
	}
	_ = conn.Close()
	result.OK = true
	result.Message = "The endpoint is reachable."
	return result
}

func checkEndpoint(ctx context.Context, endpoint string, policy egress.Policy, timeoutMs int) TestResult {
	started := time.Now()
	result := TestResult{CheckedAt: started.UTC(), Message: "Could not reach the configured endpoint."}
	if timeoutMs < 100 || timeoutMs > 120000 {
		timeoutMs = 10000
	}
	client := egress.NewClient(egress.Options{
		Policy: policy, Timeout: time.Duration(timeoutMs) * time.Millisecond,
		MaxRedirects: 3, MaxResponseBytes: 1024,
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		result.ErrorCode = "invalid_endpoint"
		return result
	}
	response, err := client.Do(request)
	result.LatencyMs = int(time.Since(started).Milliseconds())
	if err != nil {
		if errors.Is(err, egress.ErrBlockedDestination) {
			result.ErrorCode = "destination_blocked"
			result.Message = "The endpoint resolves to a destination that is not allowed."
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.ErrorCode = "timeout"
			result.Message = "The endpoint did not respond before the timeout."
		} else {
			result.ErrorCode = "unreachable"
		}
		return result
	}
	_ = response.Body.Close()
	result.OK = response.StatusCode < http.StatusInternalServerError
	if result.OK {
		result.Message = "The endpoint is reachable."
	} else {
		result.ErrorCode = "endpoint_error"
		result.Message = "The endpoint returned a server error."
	}
	return result
}
