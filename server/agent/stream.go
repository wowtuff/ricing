package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

// runoptions carries per-run overrides — backend selection, model, and credentials.
type RunOptions struct {
	Model   string
	Backend string
	APIKey  string
	URL     string
}

// streamtoolcall is a tool invocation surfaced to the caller mid-stream.
type StreamToolCall struct {
	ID        string
	CallID    string
	Name      string
	Arguments map[string]any
}

// toolresult is the output of a tool call, passed back into the conversation.
type ToolResult struct {
	ToolCallID string
	CallID     string
	Output     any
}

// streamsink is the set of callbacks the caller can hook into during a run

// all fields are optional nil callbacks are silently skipped
type StreamSink struct {
	OnDelta      func(text string)
	OnToolCall   func(call StreamToolCall)
	OnToolResult func(res ToolResult)
	ExecuteTool  func(ctx context.Context, call StreamToolCall) (ToolResult, error)
}

// runstream is the streaming entry point routes to the chatgpt websocket path or the generic rest path depending on the backend in opts
func RunStream(ctx context.Context, reg *tools.Registry, opts RunOptions, userPrompt string, sink StreamSink) error {
	backend := opts.Backend
	if backend == "" {
		backend = "chatgpt"
	}

	// use rest backend for non-chatgpt
	if backend != "chatgpt" {
		return runStreamREST(ctx, reg, opts, userPrompt, sink)
	}

	// chatgpt oauth flow
	token, err := loadCachedToken()
	if err != nil {
		return utils.LogError("provider not connected: %s", err)
	}

	model := opts.Model
	if model == "" {
		model = "gpt-5.2-codex"
	}

	originator := os.Getenv("CODEX_ORIGINATOR")
	if originator == "" {
		originator = "codex_cli_rs"
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("Origin", "https://chatgpt.com")
	headers.Set("User-Agent", "codex-cli-rs/0.1.0")
	headers.Set("originator", originator)
	if residency := os.Getenv("REQUIREMENTS_RESIDENCY"); residency != "" {
		headers.Set("x-openai-internal-codex-residency", residency)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsEndpoint, headers)
	if err != nil {
		if resp != nil {
			return utils.LogError("websocket dial error: %s | status: %d", err, resp.StatusCode)
		}
		return utils.LogError("websocket dial error: %s", err)
	}
	defer conn.Close()

	var regList []tools.ToolSpec
	if reg != nil {
		regList = reg.List()
	}
	toolSpecs := buildWSToolSpecs(regList)
	var previousResponseID *string
	input := []wsInput{{
		Type:    "message",
		Role:    "user",
		Content: []wsContent{{Type: "input_text", Text: userPrompt}},
	}}

	for {
		req := wsRequest{
			Type:               "response.create",
			Model:              model,
			Instructions:       "You are a smart agent, you provide solutions to user prompts, with no outside knowledge but from the toolset provided to you",
			PreviousResponseID: previousResponseID,
			Input:              input,
			Tools:              toolSpecs,
			ToolChoice:         "auto",
			ParallelToolCalls:  true,
			Store:              false,
			Stream:             true,
			Include:            []string{},
		}

		if err := conn.WriteJSON(req); err != nil {
			return utils.LogError("websocket write error: %s", err)
		}

		var pendingToolCalls []wsFunctionCallItem
		var responseID string
		done := false

		for !done {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return utils.LogError("websocket read error: %s", err)
			}
			var event wsEvent
			if err := json.Unmarshal(msg, &event); err != nil {
				continue
			}

			switch event.Type {
			case "response.output_text.delta":
				if sink.OnDelta != nil && event.Delta != "" {
					sink.OnDelta(event.Delta)
				}

			case "response.output_item.done":
				var item wsFunctionCallItem
				if err := json.Unmarshal(event.Item, &item); err == nil && item.Type == "function_call" {
					pendingToolCalls = append(pendingToolCalls, item)
				}

			case "response.completed":
				var resp wsResponseCompleted
				if err := json.Unmarshal(event.Response, &resp); err == nil {
					responseID = resp.ID
				}
				done = true

			case "response.failed", "response.incomplete":
				return utils.LogError("response error event: %s", event.Type)
			}
		}

		if len(pendingToolCalls) == 0 {
			return nil
		}

		input = []wsInput{}
		for _, tc := range pendingToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Arguments), &args)
			call := StreamToolCall{ID: tc.ID, CallID: tc.CallID, Name: tc.Name, Arguments: args}
			if sink.OnToolCall != nil {
				sink.OnToolCall(call)
			}
			res, resultJSON := executeStreamTool(ctx, reg, call, tc.Arguments, sink)
			if sink.OnToolResult != nil {
				sink.OnToolResult(res)
			}
			input = append(input, wsInput{Type: "function_call_output", CallID: tc.CallID, Output: resultJSON})
		}
		previousResponseID = &responseID
	}
}

// runstreamrest handles the agentic loop for all non-chatgpt backends, it calls Complete in a loop, forwarding deltas and tool calls to the sink
func runStreamREST(ctx context.Context, reg *tools.Registry, opts RunOptions, userPrompt string, sink StreamSink) error {
	cfg := Config{
		Backend: opts.Backend,
		Model:   opts.Model,
		APIKey:  opts.APIKey,
		URL:     opts.URL,
	}

	var backend Backend
	var err error

	switch cfg.Backend {
	case "openai":
		backend, err = restBackendFn("https://api.openai.com/v1", cfg.Model, cfg.APIKey)
	case "anthropic":
		backend, err = anthropic(cfg.Model, cfg.APIKey)
	case "gemini":
		backend, err = gemini(cfg.Model, cfg.APIKey)
	case "openrouter":
		backend, err = restBackendFn("https://openrouter.ai/api/v1", cfg.Model, cfg.APIKey)
	case "ollama", "lmstudio", "local":
		backend, err = restBackendFn(cfg.URL, cfg.Model, "")
	default:
		backend, err = restBackendFn(cfg.URL, cfg.Model, cfg.APIKey)
	}

	if err != nil {
		return err
	}

	var specs []tools.ToolSpec
	if reg != nil {
		specs = reg.List()
	}
	messages := []Message{{Role: "user", Content: userPrompt}}

	for {
		result, err := backend.Complete(ctx, messages, specs)
		if err != nil {
			return err
		}

		if result.Content != "" && sink.OnDelta != nil {
			sink.OnDelta(result.Content)
		}

		if len(result.ToolCalls) == 0 {
			return nil
		}

		for _, tc := range result.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Arguments), &args)
			call := StreamToolCall{ID: tc.ID, CallID: tc.ID, Name: tc.Name, Arguments: args}
			if sink.OnToolCall != nil {
				sink.OnToolCall(call)
			}
			res, output := executeStreamTool(ctx, reg, call, tc.Arguments, sink)
			if sink.OnToolResult != nil {
				sink.OnToolResult(res)
			}

			messages = append(messages, Message{
				Role:      "assistant",
				ToolCalls: []ToolCall{tc},
			})
			messages = append(messages, Message{
				Role:       "tool",
				Content:    output,
				ToolCallID: tc.ID,
			})
		}
	}
}

func executeStreamTool(ctx context.Context, reg *tools.Registry, call StreamToolCall, rawArgs string, sink StreamSink) (ToolResult, string) {
	if sink.ExecuteTool != nil {
		res, err := sink.ExecuteTool(ctx, call)
		if err != nil {
			payload := map[string]any{"error": err.Error()}
			resultJSON, _ := json.Marshal(payload)
			return ToolResult{ToolCallID: call.ID, CallID: call.CallID, Output: payload}, string(resultJSON)
		}
		if res.ToolCallID == "" {
			res.ToolCallID = call.ID
		}
		if res.CallID == "" {
			res.CallID = call.CallID
		}
		resultJSON, err := json.Marshal(res.Output)
		if err != nil {
			payload := map[string]any{"error": err.Error()}
			fallback, _ := json.Marshal(payload)
			return ToolResult{ToolCallID: call.ID, CallID: call.CallID, Output: payload}, string(fallback)
		}
		return res, string(resultJSON)
	}
	resultJSON := executeToolByName(ctx, reg, ToolCall{ID: call.CallID, Name: call.Name, Arguments: rawArgs})
	var out any
	_ = json.Unmarshal([]byte(resultJSON), &out)
	return ToolResult{ToolCallID: call.ID, CallID: call.CallID, Output: out}, resultJSON
}
