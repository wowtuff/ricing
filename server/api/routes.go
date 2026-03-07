package api

import (
	"net/http"
	"strings"
)

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// core
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/meta", s.handleMeta)
	mux.HandleFunc("/api/v1/tools", s.handleTools)

	// providers
	mux.HandleFunc("/api/v1/providers/types", s.handleProviderTypes)
	mux.HandleFunc("/api/v1/providers", s.handleProviders)
	mux.HandleFunc("/api/v1/providers/", s.handleProviderActions)

	// runs
	mux.HandleFunc("/api/v1/runs", s.handleRuns)
	mux.HandleFunc("/api/v1/runs/", s.handleRunActions)

	// websocket
	mux.HandleFunc("/api/v1/ws", s.handleWS)

	// stubs (reserve shapes)
	mux.HandleFunc("/api/v1/assets", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/assets/", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/snapshots", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/snapshots/", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/queue", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/system", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/skills", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/wallpapers", s.handleNotImplemented)
}

func pathParts(path, prefix string) ([]string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return nil, false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return []string{}, true
	}
	return strings.Split(rest, "/"), true
}
