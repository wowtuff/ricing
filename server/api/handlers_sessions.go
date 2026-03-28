package api

import (
	"net/http"

	"github.com/wowtuff/ricing/service"
)

type createSessionRequest struct {
	Title      string `json:"title"`
	Mode       string `json:"mode"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

type updateModeRequest struct {
	Mode string `json:"mode"`
}

type createMessageRequest struct {
	Prompt string `json:"prompt"`
	Mode   string `json:"mode"`
	LLM    struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		APIKey     string `json:"api_key"`
		URL        string `json:"url"`
	} `json:"llm"`
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"sessions": s.sessions.List()})
	case http.MethodPost:
		var req createSessionRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", err.Error())
			return
		}
		session, err := s.sessions.Create(service.CreateSession{
			Title:      req.Title,
			Mode:       req.Mode,
			ProviderID: req.ProviderID,
			Model:      req.Model,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "session_create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"session": session})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleSessionActions(w http.ResponseWriter, r *http.Request) {
	parts, ok := pathParts(r.URL.Path, "/api/v1/sessions/")
	if !ok || len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	sessionID := parts[0]
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
		snapshot, ok := s.sessions.Get(sessionID)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "session not found")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	case "entries":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		snapshot, ok := s.sessions.Get(sessionID)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "session not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"entries": snapshot.Entries})
	case "mode":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req updateModeRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", err.Error())
			return
		}
		session, err := s.sessions.SetMode(sessionID, req.Mode)
		if err != nil {
			writeError(w, http.StatusBadRequest, "mode_update_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case "attachments":
		if r.Method == http.MethodGet {
			snapshot, ok := s.sessions.Get(sessionID)
			if !ok {
				writeError(w, http.StatusNotFound, "session_not_found", "session not found")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"attachments": snapshot.Attachments})
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "bad_multipart", err.Error())
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing_file", "file is required")
			return
		}
		defer file.Close()
		attachment, err := s.sessions.AddAttachment(sessionID, header.Filename, file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "attachment_create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment": attachment})
	case "messages":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req createMessageRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", err.Error())
			return
		}
		if req.Prompt == "" {
			writeError(w, http.StatusBadRequest, "missing_prompt", "prompt is required")
			return
		}
		run, err := s.runs.Create(r.Context(), service.CreateRun{
			SessionID:  sessionID,
			Prompt:     req.Prompt,
			Mode:       req.Mode,
			ProviderID: req.LLM.ProviderID,
			Model:      req.LLM.Model,
			APIKey:     req.LLM.APIKey,
			URL:        req.LLM.URL,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "message_create_failed", err.Error())
			return
		}
		snapshot, _ := s.sessions.Get(sessionID)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"run":     run,
			"session": snapshot.Session,
			"ws_url":  "/api/v1/ws",
		})
	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}
