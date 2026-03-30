package api

import "net/http"

type resolveApprovalRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"approvals": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": s.sessions.ListApprovals(sessionID)})
}

func (s *Server) handleApprovalActions(w http.ResponseWriter, r *http.Request) {
	parts, ok := pathParts(r.URL.Path, "/api/v1/approvals/")
	if !ok || len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req resolveApprovalRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	approval, err := s.sessions.ResolveApproval(parts[0], req.Decision, req.Note)
	if err != nil {
		writeError(w, http.StatusBadRequest, "approval_resolve_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approval": approval})
}
