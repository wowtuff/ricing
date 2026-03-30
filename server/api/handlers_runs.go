package api

import (
	"errors"
	"net/http"

	"github.com/wowtuff/ricing/service"
)

type createRunRequest struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
	Mode      string `json:"mode"`
	LLM       struct {
		ProviderID      string `json:"provider_id"`
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
		APIKey          string `json:"api_key"`
		URL             string `json:"url"`
	} `json:"llm"`
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req createRunRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "missing_prompt", "prompt is required")
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "auto"
	}
	providerID := req.LLM.ProviderID
	if providerID == "" {
		providerID = s.providers.DefaultID()
	}
	model := req.LLM.Model
	if model == "" {
		model = "gpt-5.2-codex"
	}

	run, err := s.runs.Create(r.Context(), service.CreateRun{
		SessionID:       req.SessionID,
		Prompt:          req.Prompt,
		Mode:            mode,
		ReasoningEffort: req.LLM.ReasoningEffort,
		ProviderID:      providerID,
		Model:           model,
		APIKey:          req.LLM.APIKey,
		URL:             req.LLM.URL,
	})
	if err != nil {
		if errors.Is(err, service.ErrProviderNotConnected) {
			writeError(w, http.StatusConflict, "provider_not_connected", "provider not linked. click connect to authenticate.")
			return
		}
		writeError(w, http.StatusBadRequest, "run_create_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"run":    run,
		"ws_url": "/api/v1/ws",
	})
}

func (s *Server) handleRunActions(w http.ResponseWriter, r *http.Request) {
	parts, ok := pathParts(r.URL.Path, "/api/v1/runs/")
	if !ok || len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	runID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		run, ok := s.runs.Get(runID)
		if !ok {
			writeError(w, http.StatusNotFound, "run_not_found", "run not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"run": run})
		return

	case "cancel":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		run, err := s.runs.Cancel(runID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cancel_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"run": run})
		return

	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
}
