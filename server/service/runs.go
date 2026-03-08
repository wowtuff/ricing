package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wowtuff/ricing/agent"
	"github.com/wowtuff/ricing/tools"
)

type Run struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	Status     string `json:"status"`
	Mode       string `json:"mode"`
	Prompt     string `json:"prompt,omitempty"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	Result     *struct {
		OutputText string `json:"output_text"`
	} `json:"result,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	LatestSeq int64 `json:"latest_seq"`
}

type CreateRun struct {
	Prompt     string
	Mode       string
	ProviderID string
	Model      string
}

type runState struct {
	mu     sync.Mutex
	run    Run
	hub    *RunHub
	ctx    context.Context
	cancel context.CancelFunc
}

type RunService struct {
	mu        sync.Mutex
	runs      map[string]*runState
	reg       *tools.Registry
	providers *ProviderService
}

func NewRunService(reg *tools.Registry, providers *ProviderService) *RunService {
	return &RunService{
		runs:      make(map[string]*runState),
		reg:       reg,
		providers: providers,
	}
}

func (s *RunService) Create(ctx context.Context, req CreateRun) (Run, error) {
	if req.ProviderID == "" {
		req.ProviderID = s.providers.DefaultID()
	}
	if !s.providers.IsConnected(req.ProviderID) {
		return Run{}, ErrProviderNotConnected
	}
	if req.Mode == "" {
		req.Mode = "auto"
	}
	if req.Model == "" {
		req.Model = "gpt-5.2-codex"
	}

	runID := newID("run")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r := Run{
		ID:         runID,
		CreatedAt:  now,
		Status:     "queued",
		Mode:       req.Mode,
		Prompt:     req.Prompt,
		ProviderID: req.ProviderID,
		Model:      req.Model,
		LatestSeq:  0,
	}

	hub := NewRunHub(runID, 2048)
	ctx2, cancel := context.WithCancel(ctx)
	st := &runState{run: r, hub: hub, ctx: ctx2, cancel: cancel}

	s.mu.Lock()
	s.runs[runID] = st
	s.mu.Unlock()

	st.publish("run.status", map[string]any{"status": "queued", "stage": req.Mode})

	go s.execute(st)

	return st.snapshot(), nil
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
	st.publish("run.status", map[string]any{"status": "running", "stage": st.run.Mode})

	var output string
	err := agent.RunStream(st.ctx, s.reg, agent.RunOptions{Model: st.run.Model}, st.run.Prompt, agent.StreamSink{
		OnDelta: func(text string) {
			st.appendDelta(text)
			st.publish("assistant.delta", map[string]any{"text": text})
		},
		OnToolCall: func(call agent.StreamToolCall) {
			st.publish("tool.call", map[string]any{
				"tool_call_id": call.ID,
				"call_id":      call.CallID,
				"name":         call.Name,
				"arguments":    call.Arguments,
			})
		},
		OnToolResult: func(res agent.ToolResult) {
			st.publish("tool.result", map[string]any{
				"tool_call_id": res.ToolCallID,
				"call_id":      res.CallID,
				"output":       res.Output,
			})
		},
	})
	if err == nil {
		output = st.getOutput()
	}

	select {
	case <-st.ctx.Done():
		st.setStatus("cancelled")
		st.publish("run.status", map[string]any{"status": "cancelled"})
		st.publish("run.completed", map[string]any{"status": "cancelled"})
		return
	default:
	}

	if err != nil {
		st.setError(err.Error())
		st.setStatus("failed")
		st.publish("error", map[string]any{"code": "run_failed", "message": err.Error()})
		st.publish("run.completed", map[string]any{"status": "failed"})
		return
	}

	st.setResult(output)
	st.setStatus("succeeded")
	st.publish("assistant.message", map[string]any{"format": "text", "text": output})
	st.publish("run.completed", map[string]any{"status": "succeeded", "result": map[string]any{"output_text": output}})
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
