package api

import (
	"net/http"
)

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"protocol": map[string]any{
			"v":  1,
			"ws": "/api/v1/ws",
		},
		"capabilities": map[string]any{
			"modes":          []string{"plan", "build", "auto"},
			"run_streaming":  true,
			"provider_types": s.providers.Types(),
		},
	})
}
