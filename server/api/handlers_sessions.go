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
		writeError(w, http.StatusNotImplemented, "not_ready", "session messages are not wired yet")
	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}
