package api

import (
	"fmt"
	"net/http"

	"github.com/wowtuff/ricing/service"
)

type createSessionRequest struct {
	Title      string `json:"title"`
	Mode       string `json:"mode"`
	Thinking   string `json:"thinking"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

type updateModeRequest struct {
	Mode string `json:"mode"`
}

type updateThinkingRequest struct {
	Thinking string `json:"thinking"`
}

type updateEntryRequest struct {
	Content string `json:"content"`
}

type answerQuestionRequest struct {
	OptionID string `json:"option_id"`
}

type createMessageRequest struct {
	Prompt string `json:"prompt"`
	Mode   string `json:"mode"`
	LLM    struct {
		ProviderID      string `json:"provider_id"`
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
		APIKey          string `json:"api_key"`
		URL             string `json:"url"`
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
			Thinking:   req.Thinking,
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
		switch r.Method {
		case http.MethodGet:
			snapshot, ok := s.sessions.Get(sessionID)
			if !ok {
				writeError(w, http.StatusNotFound, "session_not_found", "session not found")
				return
			}
			writeJSON(w, http.StatusOK, snapshot)
		case http.MethodDelete:
			session, err := s.sessions.Delete(sessionID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "session_delete_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"session": session, "deleted": true})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	case "entries":
		if len(parts) > 2 {
			entryID := parts[2]
			if r.Method != http.MethodPut {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			var req updateEntryRequest
			if err := readJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "bad_json", err.Error())
				return
			}
			entry, err := s.sessions.UpdateUserEntry(sessionID, entryID, req.Content)
			if err != nil {
				writeError(w, http.StatusBadRequest, "entry_update_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"entry": entry})
			return
		}
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
	case "thinking":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req updateThinkingRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", err.Error())
			return
		}
		session, err := s.sessions.SetThinking(sessionID, req.Thinking)
		if err != nil {
			writeError(w, http.StatusBadRequest, "thinking_update_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case "questions":
		if len(parts) < 3 {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req answerQuestionRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", err.Error())
			return
		}
		entry, answer, err := s.sessions.AnswerQuestion(sessionID, parts[2], req.OptionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "question_answer_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"entry": entry, "answer": answer})
	case "attachments":
		if len(parts) > 2 {
			attachmentID := parts[2]
			attachmentAction := ""
			if len(parts) > 3 {
				attachmentAction = parts[3]
			}
			switch attachmentAction {
			case "":
				switch r.Method {
				case http.MethodGet:
					attachment, ok := s.sessions.GetAttachment(sessionID, attachmentID)
					if !ok {
						writeError(w, http.StatusNotFound, "attachment_not_found", "attachment not found")
						return
					}
					writeJSON(w, http.StatusOK, map[string]any{"attachment": attachment})
					return
				case http.MethodDelete:
					attachment, err := s.sessions.RemoveAttachment(sessionID, attachmentID)
					if err != nil {
						writeError(w, http.StatusBadRequest, "attachment_remove_failed", err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]any{"attachment": attachment, "deleted": true})
					return
				default:
					writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
					return
				}
			case "content":
				if r.Method != http.MethodGet {
					writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
					return
				}
				attachment, ok := s.sessions.GetAttachment(sessionID, attachmentID)
				if !ok {
					writeError(w, http.StatusNotFound, "attachment_not_found", "attachment not found")
					return
				}
				disposition := "inline"
				if r.URL.Query().Get("download") == "1" {
					disposition = "attachment"
				}
				w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, attachment.Name))
				http.ServeFile(w, r, attachment.Path)
				return
			default:
				writeError(w, http.StatusNotFound, "not_found", "not found")
				return
			}
		}
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
			SessionID:       sessionID,
			Prompt:          req.Prompt,
			Mode:            req.Mode,
			ReasoningEffort: req.LLM.ReasoningEffort,
			ProviderID:      req.LLM.ProviderID,
			Model:           req.LLM.Model,
			APIKey:          req.LLM.APIKey,
			URL:             req.LLM.URL,
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
