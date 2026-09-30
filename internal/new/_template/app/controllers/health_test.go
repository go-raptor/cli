package controllers_test

import (
	"net/http"
	"testing"
)

// Raptor's built-in HealthController answers the probes that config/routes.yaml routes.
func TestHealthProbes(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz"} {
		if rec := app.TestGet(path); rec.Code != http.StatusOK {
			t.Errorf("GET %s: %d, want 200", path, rec.Code)
		}
	}
}

// requestid and secure set their headers on every response.
func TestResponseHeaders(t *testing.T) {
	rec := app.TestGet("/api/v1/hello")
	for _, name := range []string{"X-Request-Id", "X-Content-Type-Options", "Content-Security-Policy"} {
		if rec.Header().Get(name) == "" {
			t.Errorf("the response has no %s header", name)
		}
	}
}
