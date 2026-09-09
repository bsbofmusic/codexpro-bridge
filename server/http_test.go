package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codexpro/bridge/core"
)

func TestAuthRejectsOversizedAuthenticatedMCPRequest(t *testing.T) {
	token := strings.Repeat("x", 32)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := Auth(next, core.Config{Token: token, MaxRequestBytes: 16}, nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(strings.Repeat("a", 17)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
	if called {
		t.Fatal("oversized request reached downstream MCP handler")
	}
}

func TestAuthRejectsUnauthenticatedBeforeBodyHandling(t *testing.T) {
	token := strings.Repeat("x", 32)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := Auth(next, core.Config{Token: token, MaxRequestBytes: 16}, nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(strings.Repeat("a", 17)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 before request-body handling, got %d", rec.Code)
	}
}
