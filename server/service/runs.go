package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wowtuff/ricing/agent"
	"github.com/wowtuff/ricing/tools"
)

type Run struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	CreatedAt  string `json:"created_at"`
	Status     string `json:"status"`
	Mode       string `json:"mode"`
	Prompt     string `json:"prompt,omitempty"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	Backend    string `json:"backend"`
	APIKey     string `json:"-"`
	URL        string `json:"url"`
	Result     *struct {
		OutputText string `json:"output_text"`
	} `json:"result,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	LatestSeq int64 `json:"latest_seq"`
}

type CreateRun struct {
	SessionID  string
	Prompt     string
	Mode       string
	ProviderID string
	Model      string
	APIKey     string
	URL        string
}

type runState struct {
	mu               sync.Mutex
	run              Run
	hub              *RunHub
	ctx              context.Context
	cancel           context.CancelFunc
	assistantEntryID string
}

type RunService struct {
	mu        sync.Mutex
	runs      map[string]*runState
	reg       *tools.Registry
	providers *ProviderService
	sessions  *SessionService
}

func NewRunService(reg *tools.Registry, providers *ProviderService, sessions *SessionService) *RunService {
	return &RunService{
		runs:      make(map[string]*runState),
		reg:       reg,
		providers: providers,
		sessions:  sessions,
	}
}

func (s *RunService) Create(ctx context.Context, req CreateRun) (Run, error) {
	sessionSnapshot, sessionReq, err := s.ensureSession(req)
	if err != nil {
		return Run{}, err
	}
	req = sessionReq
	hasCreds := req.APIKey != "" || req.URL != ""
	if !hasCreds {
		if req.ProviderID == "" {
			req.ProviderID = s.providers.DefaultID()
		}
		if !s.providers.IsConnected(req.ProviderID) {
			return Run{}, ErrProviderNotConnected
		}
	}
	if req.Model == "" {
		req.Model = firstNonEmptyString(sessionSnapshot.Session.Model, "gpt-5.2-codex")
	}
	if req.Mode == "" {
		req.Mode = firstNonEmptyString(sessionSnapshot.Session.Mode, "auto")
	}
	backend := s.providers.BackendForProvider(req.ProviderID)
	if backend == "" {
		backend = req.ProviderID
	}
	runID := newID("run")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	run := Run{
		ID:         runID,
		SessionID:  sessionSnapshot.Session.ID,
		CreatedAt:  now,
		Status:     "queued",
		Mode:       req.Mode,
		Prompt:     req.Prompt,
		ProviderID: req.ProviderID,
		Model:      req.Model,
		Backend:    backend,
		APIKey:     req.APIKey,
		URL:        req.URL,
	}
	if _, err := s.sessions.CreateEntry(run.SessionID, SessionEntry{
		RunID:   runID,
		Kind:    "user_message",
		Role:    "user",
		Status:  "done",
		Content: req.Prompt,
	}); err != nil {
		return Run{}, err
	}
	ctx2, cancel := context.WithCancel(ctx)
	st := &runState{
		run:    run,
		hub:    NewRunHub(runID, 2048),
		ctx:    ctx2,
		cancel: cancel,
	}
	s.mu.Lock()
	s.runs[runID] = st
	s.mu.Unlock()
	st.publish("run.status", map[string]any{"status": "queued", "stage": req.Mode, "session_id": run.SessionID})
	_ = s.sessions.SetStatus(run.SessionID, "queued")
	go s.execute(st)
	return st.snapshot(), nil
}

func (s *RunService) ensureSession(req CreateRun) (SessionSnapshot, CreateRun, error) {
	if req.SessionID != "" {
		snapshot, ok := s.sessions.Get(req.SessionID)
		if !ok {
			return SessionSnapshot{}, req, errors.New("session not found")
		}
		if req.Mode == "" {
			req.Mode = snapshot.Session.Mode
		}
		if req.ProviderID == "" {
			req.ProviderID = snapshot.Session.ProviderID
		}
		if req.Model == "" {
			req.Model = snapshot.Session.Model
		}
		return snapshot, req, nil
	}
	session, err := s.sessions.Create(CreateSession{
		Title:      req.Prompt,
		Mode:       req.Mode,
		ProviderID: req.ProviderID,
		Model:      req.Model,
	})
	if err != nil {
		return SessionSnapshot{}, req, err
	}
	req.SessionID = session.ID
	snapshot, _ := s.sessions.Get(session.ID)
	return snapshot, req, nil
}

func (s *RunService) Get(id string) (Run, bool) {
	s.mu.Lock()
	st, ok := s.runs[id]
	s.mu.Unlock()
	if !ok {
		return Run{}, false
	}
	return st.snapshot(), true
}

func (s *RunService) Cancel(id string) (Run, error) {
	s.mu.Lock()
	st, ok := s.runs[id]
	s.mu.Unlock()
	if !ok {
		return Run{}, errors.New("run not found")
	}
	st.cancel()
	return st.snapshot(), nil
}

func (s *RunService) Hub(id string) (*RunHub, bool) {
	s.mu.Lock()
	st, ok := s.runs[id]
	s.mu.Unlock()
	if !ok {
		return nil, false
	}
	return st.hub, true
}

func (s *RunService) execute(st *runState) {
	st.setStatus("running")
	st.publish("run.status", map[string]any{"status": "running", "stage": st.run.Mode, "session_id": st.run.SessionID})
	_ = s.sessions.SetStatus(st.run.SessionID, "running")
	assistantEntry, entryErr := s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
		RunID:   st.run.ID,
		Kind:    "assistant_message",
		Role:    "assistant",
		Status:  "streaming",
		Content: "",
	})
	if entryErr != nil {
		st.setError(entryErr.Error())
		st.setStatus("failed")
		st.publish("error", map[string]any{"code": "session_entry_failed", "message": entryErr.Error()})
		_ = s.sessions.SetStatus(st.run.SessionID, "failed")
		return
	}
	st.assistantEntryID = assistantEntry.ID
	var output string
	err := agent.RunStream(st.ctx, s.reg, agent.RunOptions{
		Model:   st.run.Model,
		Backend: st.run.Backend,
		APIKey:  st.run.APIKey,
		URL:     st.run.URL,
	}, s.buildPrompt(st), agent.StreamSink{
		OnDelta: func(text string) {
			st.appendDelta(text)
			_, _ = s.sessions.AppendEntryContent(st.run.SessionID, st.assistantEntryID, text)
			st.publish("assistant.delta", map[string]any{"text": text, "session_id": st.run.SessionID})
		},
		OnToolCall: func(call agent.StreamToolCall) {
			st.publish("tool.call", map[string]any{
				"tool_call_id": call.ID,
				"call_id":      call.CallID,
				"name":         call.Name,
				"arguments":    call.Arguments,
				"session_id":   st.run.SessionID,
			})
		},
		OnToolResult: func(res agent.ToolResult) {
			st.publish("tool.result", map[string]any{
				"tool_call_id": res.ToolCallID,
				"call_id":      res.CallID,
				"output":       res.Output,
				"session_id":   st.run.SessionID,
			})
		},
		ExecuteTool: func(ctx context.Context, call agent.StreamToolCall) (agent.ToolResult, error) {
			return s.executeTool(ctx, st, call)
		},
	})
	if err == nil {
		output = st.getOutput()
	}
	select {
	case <-st.ctx.Done():
		st.setStatus("cancelled")
		st.publish("run.status", map[string]any{"status": "cancelled", "session_id": st.run.SessionID})
		st.publish("run.completed", map[string]any{"status": "cancelled", "session_id": st.run.SessionID})
		_, _ = s.sessions.UpdateEntry(st.run.SessionID, st.assistantEntryID, func(entry *SessionEntry) {
			entry.Status = "cancelled"
		})
		_ = s.sessions.SetStatus(st.run.SessionID, "cancelled")
		return
	default:
	}
	if err != nil {
		st.setError(err.Error())
		st.setStatus("failed")
		st.publish("error", map[string]any{"code": "run_failed", "message": err.Error(), "session_id": st.run.SessionID})
		st.publish("run.completed", map[string]any{"status": "failed", "session_id": st.run.SessionID})
		_, _ = s.sessions.UpdateEntry(st.run.SessionID, st.assistantEntryID, func(entry *SessionEntry) {
			entry.Status = "failed"
			if entry.Content == "" {
				entry.Content = err.Error()
			}
		})
		_ = s.sessions.SetStatus(st.run.SessionID, "failed")
		return
	}
	st.setResult(output)
	st.setStatus("succeeded")
	st.publish("assistant.message", map[string]any{"format": "text", "text": output, "session_id": st.run.SessionID})
	st.publish("run.completed", map[string]any{"status": "succeeded", "result": map[string]any{"output_text": output}, "session_id": st.run.SessionID})
	_, _ = s.sessions.UpdateEntry(st.run.SessionID, st.assistantEntryID, func(entry *SessionEntry) {
		entry.Status = "done"
	})
	_ = s.sessions.SetStatus(st.run.SessionID, "idle")
}

func (s *RunService) executeTool(ctx context.Context, st *runState, call agent.StreamToolCall) (agent.ToolResult, error) {
	if call.Name == "update_plan" {
		entry, err := s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "plan",
			Status:  "done",
			Title:   firstNonEmptyString(asString(call.Arguments["summary"]), "plan"),
			Content: renderPlan(call.Arguments),
			Meta:    cloneMap(call.Arguments),
		})
		if err != nil {
			return agent.ToolResult{}, err
		}
		return agent.ToolResult{
			ToolCallID: call.ID,
			CallID:     call.CallID,
			Output: map[string]any{
				"ok":       true,
				"entry_id": entry.ID,
			},
		}, nil
	}
	toolCallEntry, err := s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
		RunID:   st.run.ID,
		Kind:    "tool_call",
		Status:  "done",
		Title:   call.Name,
		Content: summarizeToolCall(call),
		Meta:    cloneMap(call.Arguments),
	})
	if err != nil {
		return agent.ToolResult{}, err
	}
	if st.run.Mode == "plan" && isMutatingCall(call) {
		blocked := map[string]any{"ok": false, "blocked": true, "reason": "mutating tools are disabled in plan mode"}
		_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "system",
			Status:  "done",
			Title:   "plan mode",
			Content: "mutating action blocked while the session is in plan mode",
			Meta: map[string]any{
				"tool_name": call.Name,
				"entry_id":  toolCallEntry.ID,
			},
		})
		return agent.ToolResult{ToolCallID: call.ID, CallID: call.CallID, Output: blocked}, nil
	}
	if needsApproval(call) {
		approval, err := s.sessions.CreateApproval(st.run.SessionID, st.run.ID, toolCallEntry.ID, call.Name, summarizeToolCall(call), cloneMap(call.Arguments))
		if err != nil {
			return agent.ToolResult{}, err
		}
		_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "approval",
			Status:  "pending",
			Title:   approval.ToolName,
			Content: approval.Summary,
			Meta: map[string]any{
				"approval_id": approval.ID,
			},
		})
		resolution, err := s.sessions.WaitForApproval(ctx, approval.ID)
		if err != nil {
			return agent.ToolResult{}, err
		}
		if resolution.Status != "approved" {
			_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
				RunID:   st.run.ID,
				Kind:    "approval",
				Status:  "rejected",
				Title:   approval.ToolName,
				Content: firstNonEmptyString(resolution.Note, "user rejected the action"),
				Meta: map[string]any{
					"approval_id": approval.ID,
				},
			})
			blocked := map[string]any{"ok": false, "rejected": true, "reason": firstNonEmptyString(resolution.Note, "user rejected the action")}
			return agent.ToolResult{ToolCallID: call.ID, CallID: call.CallID, Output: blocked}, nil
		}
		_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "approval",
			Status:  "approved",
			Title:   approval.ToolName,
			Content: firstNonEmptyString(resolution.Note, "action approved"),
			Meta: map[string]any{
				"approval_id": approval.ID,
			},
		})
	}
	output, err := s.runTool(ctx, call)
	if err != nil {
		_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "tool_result",
			Status:  "failed",
			Title:   call.Name,
			Content: err.Error(),
		})
		return agent.ToolResult{}, err
	}
	_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
		RunID:   st.run.ID,
		Kind:    "tool_result",
		Status:  "done",
		Title:   call.Name,
		Content: formatToolOutput(output),
		Meta: map[string]any{
			"tool_call_entry_id": toolCallEntry.ID,
		},
	})
	if change := describeChange(call, output); change != "" {
		_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "change",
			Status:  "done",
			Title:   "change",
			Content: change,
		})
	}
	if verify := describeVerification(call, output); verify != "" {
		_, _ = s.sessions.CreateEntry(st.run.SessionID, SessionEntry{
			RunID:   st.run.ID,
			Kind:    "verification",
			Status:  "done",
			Title:   "verification",
			Content: verify,
		})
	}
	return agent.ToolResult{
		ToolCallID: call.ID,
		CallID:     call.CallID,
		Output:     output,
	}, nil
}

func (s *RunService) runTool(ctx context.Context, call agent.StreamToolCall) (map[string]any, error) {
	tool, err := s.reg.Get(call.Name)
	if err != nil {
		return nil, err
	}
	result, err := tool.Run(ctx, cloneMap(call.Arguments))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *RunService) buildPrompt(st *runState) string {
	snapshot, ok := s.sessions.Get(st.run.SessionID)
	if !ok {
		return st.run.Prompt
	}
	parts := []string{modePrompt(st.run.Mode)}
	attachmentContext := renderAttachmentContext(snapshot.Attachments)
	if attachmentContext != "" {
		parts = append(parts, attachmentContext)
	}
	parts = append(parts, "user request:\n"+st.run.Prompt)
	return strings.Join(parts, "\n\n")
}

func modePrompt(mode string) string {
	switch mode {
	case "plan":
		return "You are in plan mode. Do not perform mutating actions. Use update_plan before finalizing a plan. Ask the user for clarification in normal assistant text when needed."
	case "build":
		return "You are in build mode. You may use tools, but mutating actions can require approval. Use update_plan when the work benefits from a visible plan."
	default:
		return "You are in auto mode. Use update_plan when it helps the user follow the work. Prefer read-only inspection before mutating actions."
	}
}

func renderAttachmentContext(attachments []Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("available attachments:")
	for _, attachment := range attachments {
		builder.WriteString("\n- ")
		builder.WriteString(attachment.Name)
		content := readAttachmentSnippet(attachment.Path, attachment.Size)
		if content != "" {
			builder.WriteString("\n```")
			builder.WriteString("\n")
			builder.WriteString(content)
			builder.WriteString("\n```")
		}
	}
	return builder.String()
}

func readAttachmentSnippet(path string, size int64) string {
	if size > 16384 {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".pdf", ".zip", ".gz":
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 2000 {
		return string(runes[:2000])
	}
	return text
}

func (st *runState) publish(typ string, data any) {
	ev := st.hub.Publish(typ, data)
	st.mu.Lock()
	st.run.LatestSeq = ev.Seq
	st.mu.Unlock()
}

func (st *runState) snapshot() Run {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.run
}

func (st *runState) setStatus(status string) {
	st.mu.Lock()
	st.run.Status = status
	st.mu.Unlock()
}

func (st *runState) setError(msg string) {
	st.mu.Lock()
	st.run.Error = &struct {
		Message string `json:"message"`
	}{Message: msg}
	st.mu.Unlock()
}

func (st *runState) setResult(text string) {
	st.mu.Lock()
	st.run.Result = &struct {
		OutputText string `json:"output_text"`
	}{OutputText: text}
	st.mu.Unlock()
}

func (st *runState) appendDelta(text string) {
	st.mu.Lock()
	if st.run.Result == nil {
		st.run.Result = &struct {
			OutputText string `json:"output_text"`
		}{OutputText: ""}
	}
	st.run.Result.OutputText += text
	st.mu.Unlock()
}

func (st *runState) getOutput() string {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.run.Result == nil {
		return ""
	}
	return st.run.Result.OutputText
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func renderPlan(args map[string]any) string {
	summary := strings.TrimSpace(asString(args["summary"]))
	lines := []string{}
	if summary != "" && summary != "<nil>" {
		lines = append(lines, summary)
	}
	if rawSteps, ok := args["steps"].([]interface{}); ok {
		for _, rawStep := range rawSteps {
			stepMap, ok := rawStep.(map[string]any)
			if !ok {
				continue
			}
			title := strings.TrimSpace(asString(stepMap["title"]))
			status := strings.TrimSpace(asString(stepMap["status"]))
			if title == "" || title == "<nil>" {
				continue
			}
			if status != "" && status != "<nil>" {
				lines = append(lines, "- ["+status+"] "+title)
			} else {
				lines = append(lines, "- "+title)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func summarizeToolCall(call agent.StreamToolCall) string {
	payload, _ := json.Marshal(call.Arguments)
	return call.Name + " " + string(payload)
}

func formatToolOutput(output map[string]any) string {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", output)
	}
	return string(data)
}

func isMutatingCall(call agent.StreamToolCall) bool {
	switch call.Name {
	case "apply_patch", "install_package", "set_color_mode":
		return true
	case "cmd":
		command := strings.ToLower(strings.TrimSpace(asString(call.Arguments["command"])))
		if command == "" {
			return true
		}
		switch command {
		case "ls", "pwd", "cat", "find", "rg", "grep", "head", "tail", "wc", "echo":
			return false
		case "git":
			args, _ := call.Arguments["args"].([]interface{})
			if len(args) == 0 {
				return true
			}
			sub := strings.ToLower(strings.TrimSpace(asString(args[0])))
			return sub != "status" && sub != "diff" && sub != "show" && sub != "log"
		default:
			return true
		}
	default:
		return false
	}
}

func needsApproval(call agent.StreamToolCall) bool {
	return isMutatingCall(call)
}

func describeChange(call agent.StreamToolCall, output map[string]any) string {
	switch call.Name {
	case "apply_patch":
		return fmt.Sprintf("updated %s", asString(output["file_path"]))
	case "install_package":
		return "package installation attempted"
	case "set_color_mode":
		return fmt.Sprintf("color mode set to %s", asString(output["mode"]))
	case "cmd":
		if isMutatingCall(call) {
			return "executed mutating command"
		}
	}
	return ""
}

func describeVerification(call agent.StreamToolCall, output map[string]any) string {
	if call.Name != "cmd" {
		return ""
	}
	command := strings.ToLower(strings.TrimSpace(asString(call.Arguments["command"])))
	switch command {
	case "go", "npm", "pnpm", "yarn", "pytest", "cargo":
		status := strings.TrimSpace(asString(output["status"]))
		if status == "" || status == "<nil>" {
			status = "unknown"
		}
		return "verification command finished with " + status
	}
	return ""
}
