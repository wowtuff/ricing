package api

import (
	"context"
	"net/http"
	"time"
)

type createProviderRequest struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

type connectProviderRequest struct {
	OpenBrowser string `json:"open_browser"` // server|none
}

func (s *Server) handleProviderTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"types": s.providers.Types()})
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"default_provider_id": s.providers.DefaultID(),
			"providers":           s.providers.List(),
		})
		return
	case http.MethodPost:
		var req createProviderRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", err.Error())
			return
		}
		p, err := s.providers.Create(req.Type, req.Label)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"provider": p})
		return
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type pingProviderRequest struct {
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	APIKey          string `json:"api_key"`
	URL             string `json:"url"`
}

func (s *Server) handleProviderActions(w http.ResponseWriter, r *http.Request) {
	parts, ok := pathParts(r.URL.Path, "/api/v1/providers/")
	if !ok || len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	providerID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "connect":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req connectProviderRequest
		_ = readJSON(r, &req)
		openBrowser := true
		if req.OpenBrowser == "none" {
			openBrowser = false
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()

		p, authURL, err := s.providers.Connect(ctx, providerID, openBrowser)
		if err != nil {
			writeError(w, http.StatusBadRequest, "provider_connect_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"provider": p, "auth_url": authURL})
		return

	case "ping":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req pingProviderRequest
		_ = readJSON(r, &req)
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		reply, err := s.providers.Ping(ctx, providerID, req.Model, req.APIKey, req.URL, req.ReasoningEffort)
		if err != nil {
			writeError(w, http.StatusBadRequest, "ping_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"reply": reply})
		return

	case "disconnect":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		p, err := s.providers.Disconnect(providerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "provider_disconnect_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"provider": p})
		return

	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
}
