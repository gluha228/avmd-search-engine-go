package httpserver

import (
	"net/http"
	"strings"
)

const (
	corsAllowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Accept, Accept-Language, Authorization, Content-Type, X-Requested-With"
)

func withCORS(allowedOriginsConfig string) func(http.Handler) http.Handler {
	allowedOrigins := parseAllowedOrigins(allowedOriginsConfig)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if allowOrigin(origin, allowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseAllowedOrigins(config string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, item := range strings.Split(config, ",") {
		origin := strings.TrimSpace(item)
		if origin == "" {
			continue
		}
		result[origin] = struct{}{}
	}
	return result
}

func allowOrigin(origin string, allowedOrigins map[string]struct{}) bool {
	if _, ok := allowedOrigins["*"]; ok {
		return true
	}
	_, ok := allowedOrigins[origin]
	return ok
}
