package testdb

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ErrUnsafeDSN is returned when DATABASE_TEST_URL does not look ephemeral.
var ErrUnsafeDSN = fmt.Errorf("DATABASE_TEST_URL rejected: host must be a local/CI test host and database name must contain \"test\"")

// RequireURL returns DATABASE_TEST_URL after validating it is an ephemeral test DSN.
// Allowed hosts: localhost, 127.0.0.1, ::1, postgres (compose service).
// Database name must contain "test" (case-insensitive), e.g. base_golang_test.
func RequireURL() (string, error) {
	raw := strings.TrimSpace(os.Getenv("DATABASE_TEST_URL"))
	if raw == "" {
		return "", fmt.Errorf("DATABASE_TEST_URL is not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse DATABASE_TEST_URL: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("DATABASE_TEST_URL scheme must be postgres")
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "localhost", "127.0.0.1", "::1", "postgres":
	default:
		return "", fmt.Errorf("%w (host=%q)", ErrUnsafeDSN, host)
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" || !strings.Contains(strings.ToLower(dbName), "test") {
		return "", fmt.Errorf("%w (database=%q)", ErrUnsafeDSN, dbName)
	}
	// Never log the raw DSN from callers; return it only for driver use.
	return raw, nil
}
