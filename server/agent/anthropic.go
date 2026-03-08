package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wowtuff/ricing/tools"
)

type AnthropicBackend struct {
	model  string
	apiKey string
}

func anthropic(model, apiKey string) (*AnthropicBackend, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for anthropic backend")
	}
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &AnthropicBackend{model: model, apiKey: apiKey}, nil
}

func (b *AnthropicBackend) Complete(ctx context.Context, messages []Message, specs []tools.ToolSpec) (*CompletionResult, error) {
	var anthropicMessages []map[string]any
	var systemPrompt string

	for _, m := range messages {
		switch m.Role {
		case "system":
			systemPrompt = m.Content
		case "user":
			anthropicMessages = append(anthropicMessages, map[string]any{
				"role":    "user",
				"content": m.Content,
			})
		case "assistant":
			if len(m.ToolCalls) > 0 {
				var content []map[string]any
				if m.Content != "" {
					content = append(content, map[string]any{
						"type": "text",
						"text": m.Content,
					})
				}
				for _, tc := range m.ToolCalls {
					var input map[string]any
					json.Unmarshal([]byte(tc.Arguments), &input)
					content = append(content, map[string]any{
						"type":  "tool_use",
						"id":    tc.ID,
						"name":  tc.Name,
						"input": input,
					})
				}
				anthropicMessages = append(anthropicMessages, map[string]any{
					"role":    "assistant",
					"content": content,
				})
			} else {
				anthropicMessages = append(anthropicMessages, map[string]any{
					"role":    "assistant",
					"content": m.Content,
				})
			}
		case "tool":
			anthropicMessages = append(anthropicMessages, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
		}
	}

	body := map[string]any{
		"model":      b.model,
		"max_tokens": 4096,
		"messages":   anthropicMessages,
		"tools":      toAnthropicTools(specs),
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", b.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("anthropic api error %d: %v", resp.StatusCode, errBody)
	}

	var result struct {
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Input any    `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var text string
	var toolCalls []ToolCall

	for _, block := range result.Content {
		switch block.Type {
		case "text":
			text += block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(args),
			})
		}
	}

	return &CompletionResult{
		Content:   text,
		ToolCalls: toolCalls,
	}, nil
}

func toAnthropicTools(specs []tools.ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		out = append(out, map[string]any{
			"name":         spec.Name,
			"description":  spec.Description,
			"input_schema": spec.ParamSchema,
		})
	}
	return out
}
