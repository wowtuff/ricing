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

type RunOptions struct {
	Model string
}

type StreamToolCall struct {
	ID        string
	CallID    string
	Name      string
	Arguments map[string]any
}

type ToolResult struct {
	ToolCallID string
	CallID     string
	Output     any
}

type StreamSink struct {
	OnDelta      func(text string)
	OnToolCall   func(call StreamToolCall)
	OnToolResult func(res ToolResult)
}

func RunStream(ctx context.Context, reg *tools.Registry, opts RunOptions, userPrompt string, sink StreamSink) error {
	// only use cached token; connect via provider API first
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

	toolSpecs := buildWSToolSpecs(reg.List())
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
			if sink.OnToolCall != nil {
				sink.OnToolCall(StreamToolCall{ID: tc.ID, CallID: tc.CallID, Name: tc.Name, Arguments: args})
			}
			resultJSON := executeToolByName(ctx, reg, ToolCall{ID: tc.CallID, Name: tc.Name, Arguments: tc.Arguments})
			var out any
			_ = json.Unmarshal([]byte(resultJSON), &out)
			if sink.OnToolResult != nil {
				sink.OnToolResult(ToolResult{ToolCallID: tc.ID, CallID: tc.CallID, Output: out})
			}
			input = append(input, wsInput{Type: "function_call_output", CallID: tc.CallID, Output: resultJSON})
		}
		previousResponseID = &responseID
	}
}
