package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/codexpro/bridge/core"
)

type HealthFunc func() map[string]any

func Auth(next http.Handler, config core.Config, health HealthFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			payload := map[string]any{"ok": true, "service": "codexpro-bridge"}
			if health != nil {
				payload = health()
			}
			_ = json.NewEncoder(w).Encode(payload)
			return
		}
		if r.URL.Path == "/mcp" && !config.AllowAnonymous && !validToken(r, config.Token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/mcp" && config.MaxRequestBytes > 0 {
			limit := int64(config.MaxRequestBytes)
			if r.ContentLength > limit {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

func validToken(r *http.Request, expected string) bool {
	supplied := r.URL.Query().Get("codexpro_token")
	if supplied == "" {
		authorization := r.Header.Get("Authorization")
		if len(authorization) >= 7 && strings.EqualFold(authorization[:7], "bearer ") {
			supplied = authorization[7:]
		}
	}
	return supplied != "" && len(supplied) == len(expected) && subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) == 1
}
