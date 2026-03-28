package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Session struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Mode             string `json:"mode"`
	Status           string `json:"status"`
	ProviderID       string `json:"provider_id"`
	Model            string `json:"model"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	LatestPreview    string `json:"latest_preview,omitempty"`
	LatestEntryID    string `json:"latest_entry_id,omitempty"`
	LatestSeq        int64  `json:"latest_seq"`
	PendingApprovals int    `json:"pending_approvals"`
	AttachmentCount  int    `json:"attachment_count"`
}

type SessionEntry struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id,omitempty"`
	Seq       int64          `json:"seq"`
	Kind      string         `json:"kind"`
	Role      string         `json:"role,omitempty"`
	Status    string         `json:"status"`
	Title     string         `json:"title,omitempty"`
	Content   string         `json:"content,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	Meta      map[string]any `json:"meta,omitempty"`
}

type Attachment struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

type PendingApproval struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id,omitempty"`
	EntryID   string         `json:"entry_id,omitempty"`
	ToolName  string         `json:"tool_name"`
	Summary   string         `json:"summary"`
	Status    string         `json:"status"`
	Decision  string         `json:"decision,omitempty"`
	Note      string         `json:"note,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type SessionSnapshot struct {
	Session     Session           `json:"session"`
	Entries     []SessionEntry    `json:"entries"`
	Attachments []Attachment      `json:"attachments"`
	Approvals   []PendingApproval `json:"approvals"`
}

type sessionState struct {
	mu       sync.Mutex
	snapshot SessionSnapshot
	hub      *SessionHub
}

type approvalWaiter struct {
	resolved chan struct{}
}

type CreateSession struct {
	Title      string
	Mode       string
	ProviderID string
	Model      string
}

type SessionService struct {
	mu        sync.Mutex
	root      string
	sessions  map[string]*sessionState
	approvals map[string]*approvalWaiter
	catalog   *SessionHub
}

func NewSessionService(root string) *SessionService {
	root = defaultSessionRoot(root)
	_ = os.MkdirAll(root, 0o755)
	s := &SessionService{
		root:      root,
		sessions:  make(map[string]*sessionState),
		approvals: make(map[string]*approvalWaiter),
		catalog:   NewSessionHub("sessions", 2048),
	}
	s.load()
	return s
}

func defaultSessionRoot(root string) string {
	if strings.TrimSpace(root) != "" {
		return root
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ricing_sessions"
	}
	return filepath.Join(home, ".ricing", "sessions")
}

func (s *SessionService) load() {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		statePath := filepath.Join(s.root, entry.Name(), "state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var snapshot SessionSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}
		hub := NewSessionHub(snapshot.Session.ID, 2048)
		hub.SetNextSeq(snapshot.Session.LatestSeq)
		st := &sessionState{
			snapshot: snapshot,
			hub:      hub,
		}
		s.sessions[snapshot.Session.ID] = st
		for _, approval := range snapshot.Approvals {
			if approval.Status == "pending" {
				s.approvals[approval.ID] = &approvalWaiter{resolved: make(chan struct{})}
			}
		}
	}
}

func (s *SessionService) Root() string {
	return s.root
}

func (s *SessionService) CatalogHub() *SessionHub {
	return s.catalog
}

func (s *SessionService) List() []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Session, 0, len(s.sessions))
	for _, st := range s.sessions {
		st.mu.Lock()
		out = append(out, st.snapshot.Session)
		st.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].ID > out[j].ID
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (s *SessionService) Create(req CreateSession) (Session, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if req.Mode == "" {
		req.Mode = "auto"
	}
	session := Session{
		ID:         newID("sess"),
		Title:      normalizeSessionTitle(req.Title),
		Mode:       req.Mode,
		Status:     "idle",
		ProviderID: req.ProviderID,
		Model:      req.Model,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if session.Title == "" {
		session.Title = "new session"
	}
	st := &sessionState{
		snapshot: SessionSnapshot{
			Session:     session,
			Entries:     []SessionEntry{},
			Attachments: []Attachment{},
			Approvals:   []PendingApproval{},
		},
		hub: NewSessionHub(session.ID, 2048),
	}
	s.mu.Lock()
	s.sessions[session.ID] = st
	s.mu.Unlock()
	if err := s.saveState(st); err != nil {
		return Session{}, err
	}
	s.catalog.Publish("session.updated", map[string]any{"session": session})
	return session, nil
}

func (s *SessionService) Get(sessionID string) (SessionSnapshot, bool) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return SessionSnapshot{}, false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return cloneSnapshot(st.snapshot), true
}

func (s *SessionService) Hub(sessionID string) (*SessionHub, bool) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return nil, false
	}
	return st.hub, true
}

func (s *SessionService) SetMode(sessionID, mode string) (Session, error) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return Session{}, fmt.Errorf("session not found")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.snapshot.Session.Mode = mode
	st.snapshot.Session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.saveStateLocked(st); err != nil {
		return Session{}, err
	}
	s.publishSessionUpdate(st)
	return st.snapshot.Session, nil
}

func (s *SessionService) SetStatus(sessionID, status string) error {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.snapshot.Session.Status = status
	st.snapshot.Session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.saveStateLocked(st); err != nil {
		return err
	}
	s.publishSessionUpdate(st)
	return nil
}

func (s *SessionService) CreateEntry(sessionID string, entry SessionEntry) (SessionEntry, error) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return SessionEntry{}, fmt.Errorf("session not found")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	entry.ID = newID("entry")
	entry.SessionID = sessionID
	entry.CreatedAt = now
	entry.UpdatedAt = now
	entry.Status = firstNonEmptyString(entry.Status, "done")
	st.snapshot.Session.LatestSeq++
	entry.Seq = st.snapshot.Session.LatestSeq
	st.snapshot.Entries = append(st.snapshot.Entries, entry)
	s.touchSessionLocked(st, entry)
	if err := s.saveStateLocked(st); err != nil {
		return SessionEntry{}, err
	}
	st.hub.Publish("entry.created", map[string]any{"entry": entry})
	s.publishSessionUpdate(st)
	return entry, nil
}

func (s *SessionService) UpdateEntry(sessionID, entryID string, fn func(*SessionEntry)) (SessionEntry, error) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return SessionEntry{}, fmt.Errorf("session not found")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for i := range st.snapshot.Entries {
		if st.snapshot.Entries[i].ID != entryID {
			continue
		}
		fn(&st.snapshot.Entries[i])
		st.snapshot.Entries[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		s.touchSessionLocked(st, st.snapshot.Entries[i])
		if err := s.saveStateLocked(st); err != nil {
			return SessionEntry{}, err
		}
		st.hub.Publish("entry.updated", map[string]any{"entry": st.snapshot.Entries[i]})
		s.publishSessionUpdate(st)
		return st.snapshot.Entries[i], nil
	}
	return SessionEntry{}, fmt.Errorf("entry not found")
}

func (s *SessionService) AppendEntryContent(sessionID, entryID, chunk string) (SessionEntry, error) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return SessionEntry{}, fmt.Errorf("session not found")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for i := range st.snapshot.Entries {
		if st.snapshot.Entries[i].ID != entryID {
			continue
		}
		st.snapshot.Entries[i].Content += chunk
		st.snapshot.Entries[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		s.touchSessionLocked(st, st.snapshot.Entries[i])
		if err := s.saveStateLocked(st); err != nil {
			return SessionEntry{}, err
		}
		st.hub.Publish("entry.delta", map[string]any{"entry_id": entryID, "text": chunk, "entry": st.snapshot.Entries[i]})
		s.publishSessionUpdate(st)
		return st.snapshot.Entries[i], nil
	}
	return SessionEntry{}, fmt.Errorf("entry not found")
}

func (s *SessionService) AddAttachment(sessionID, name string, r io.Reader) (Attachment, error) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return Attachment{}, fmt.Errorf("session not found")
	}
	attachmentID := newID("file")
	ext := filepath.Ext(name)
	fileName := attachmentID + ext
	dir := filepath.Join(s.root, sessionID, "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Attachment{}, err
	}
	path := filepath.Join(dir, fileName)
	f, err := os.Create(path)
	if err != nil {
		return Attachment{}, err
	}
	size, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return Attachment{}, copyErr
	}
	if closeErr != nil {
		return Attachment{}, closeErr
	}
	attachment := Attachment{
		ID:        attachmentID,
		SessionID: sessionID,
		Name:      name,
		Path:      path,
		Size:      size,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.snapshot.Attachments = append(st.snapshot.Attachments, attachment)
	st.snapshot.Session.AttachmentCount = len(st.snapshot.Attachments)
	st.snapshot.Session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.saveStateLocked(st); err != nil {
		return Attachment{}, err
	}
	st.hub.Publish("session.updated", map[string]any{"attachment": attachment})
	s.publishSessionUpdate(st)
	return attachment, nil
}

func (s *SessionService) ListApprovals(sessionID string) []PendingApproval {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]PendingApproval, len(st.snapshot.Approvals))
	copy(out, st.snapshot.Approvals)
	return out
}

func (s *SessionService) CreateApproval(sessionID, runID, entryID, toolName, summary string, args map[string]any) (PendingApproval, error) {
	s.mu.Lock()
	st, ok := s.sessions[sessionID]
	s.mu.Unlock()
	if !ok {
		return PendingApproval{}, fmt.Errorf("session not found")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	approval := PendingApproval{
		ID:        newID("approval"),
		SessionID: sessionID,
		RunID:     runID,
		EntryID:   entryID,
		ToolName:  toolName,
		Summary:   summary,
		Status:    "pending",
		Arguments: args,
		CreatedAt: now,
		UpdatedAt: now,
	}
	st.mu.Lock()
	st.snapshot.Approvals = append(st.snapshot.Approvals, approval)
	st.snapshot.Session.PendingApprovals = countPendingApprovals(st.snapshot.Approvals)
	st.snapshot.Session.UpdatedAt = now
	if err := s.saveStateLocked(st); err != nil {
		st.mu.Unlock()
		return PendingApproval{}, err
	}
	st.mu.Unlock()
	s.mu.Lock()
	s.approvals[approval.ID] = &approvalWaiter{resolved: make(chan struct{})}
	s.mu.Unlock()
	st.hub.Publish("approval.updated", map[string]any{"approval": approval})
	s.publishSessionUpdate(st)
	return approval, nil
}

func (s *SessionService) ResolveApproval(approvalID, decision, note string) (PendingApproval, error) {
	s.mu.Lock()
	waiter, waiterOK := s.approvals[approvalID]
	var target *sessionState
	for _, st := range s.sessions {
		st.mu.Lock()
		found := false
		for i := range st.snapshot.Approvals {
			if st.snapshot.Approvals[i].ID == approvalID {
				now := time.Now().UTC().Format(time.RFC3339Nano)
				st.snapshot.Approvals[i].Decision = decision
				st.snapshot.Approvals[i].Note = note
				st.snapshot.Approvals[i].UpdatedAt = now
				if decision == "approve" {
					st.snapshot.Approvals[i].Status = "approved"
				} else {
					st.snapshot.Approvals[i].Status = "rejected"
				}
				st.snapshot.Session.PendingApprovals = countPendingApprovals(st.snapshot.Approvals)
				st.snapshot.Session.UpdatedAt = now
				target = st
				found = true
				break
			}
		}
		if found {
			approval := copyApproval(target.snapshot.Approvals, approvalID)
			saveErr := s.saveStateLocked(target)
			target.mu.Unlock()
			if saveErr != nil {
				s.mu.Unlock()
				return PendingApproval{}, saveErr
			}
			if waiterOK {
				close(waiter.resolved)
				delete(s.approvals, approvalID)
			}
			s.mu.Unlock()
			target.hub.Publish("approval.updated", map[string]any{"approval": approval})
			s.publishSessionUpdate(target)
			return approval, nil
		}
		st.mu.Unlock()
	}
	s.mu.Unlock()
	return PendingApproval{}, fmt.Errorf("approval not found")
}

func (s *SessionService) WaitForApproval(ctx context.Context, approvalID string) (PendingApproval, error) {
	s.mu.Lock()
	waiter, ok := s.approvals[approvalID]
	s.mu.Unlock()
	if !ok {
		return PendingApproval{}, fmt.Errorf("approval not found")
	}
	select {
	case <-ctx.Done():
		return PendingApproval{}, ctx.Err()
	case <-waiter.resolved:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.sessions {
		st.mu.Lock()
		approval := copyApproval(st.snapshot.Approvals, approvalID)
		st.mu.Unlock()
		if approval.ID != "" {
			return approval, nil
		}
	}
	return PendingApproval{}, fmt.Errorf("approval not found")
}

func (s *SessionService) publishSessionUpdate(st *sessionState) {
	st.mu.Lock()
	session := st.snapshot.Session
	st.mu.Unlock()
	st.hub.Publish("session.updated", map[string]any{"session": session})
	s.catalog.Publish("session.updated", map[string]any{"session": session})
}

func (s *SessionService) touchSessionLocked(st *sessionState, entry SessionEntry) {
	st.snapshot.Session.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	st.snapshot.Session.LatestEntryID = entry.ID
	if entry.Content != "" {
		st.snapshot.Session.LatestPreview = trimPreview(entry.Content)
	}
	if st.snapshot.Session.Title == "new session" && entry.Kind == "user_message" && entry.Content != "" {
		st.snapshot.Session.Title = normalizeSessionTitle(entry.Content)
	}
}

func normalizeSessionTitle(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	input = strings.ReplaceAll(input, "\n", " ")
	runes := []rune(input)
	if len(runes) > 48 {
		return string(runes[:48]) + "..."
	}
	return input
}

func trimPreview(input string) string {
	input = strings.TrimSpace(strings.ReplaceAll(input, "\n", " "))
	runes := []rune(input)
	if len(runes) > 80 {
		return string(runes[:80]) + "..."
	}
	return input
}

func countPendingApprovals(list []PendingApproval) int {
	count := 0
	for _, approval := range list {
		if approval.Status == "pending" {
			count++
		}
	}
	return count
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func copyApproval(list []PendingApproval, approvalID string) PendingApproval {
	for _, approval := range list {
		if approval.ID == approvalID {
			return approval
		}
	}
	return PendingApproval{}
}

func cloneSnapshot(snapshot SessionSnapshot) SessionSnapshot {
	out := snapshot
	out.Entries = append([]SessionEntry(nil), snapshot.Entries...)
	out.Attachments = append([]Attachment(nil), snapshot.Attachments...)
	out.Approvals = append([]PendingApproval(nil), snapshot.Approvals...)
	return out
}

func (s *SessionService) saveState(st *sessionState) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	return s.saveStateLocked(st)
}

func (s *SessionService) saveStateLocked(st *sessionState) error {
	dir := filepath.Join(s.root, st.snapshot.Session.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st.snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), data, 0o644)
}
