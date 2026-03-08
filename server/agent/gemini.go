package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wowtuff/ricing/tools"
)

type GeminiBackend struct {
	model  string
	apiKey string
}

func gemini(model, apiKey string) (*GeminiBackend, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for gemini backend")
	}
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiBackend{model: model, apiKey: apiKey}, nil
}

func (b *GeminiBackend) Complete(ctx context.Context, messages []Message, specs []tools.ToolSpec) (*CompletionResult, error) {
	var contents []map[string]any
	var systemInstruction string

	for _, m := range messages {
		switch m.Role {
		case "system":
			systemInstruction = m.Content
		case "user":
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"text": m.Content},
				},
			})
		case "assistant":
			if len(m.ToolCalls) > 0 {
				var parts []map[string]any
				if m.Content != "" {
					parts = append(parts, map[string]any{"text": m.Content})
				}
				for _, tc := range m.ToolCalls {
					var args map[string]any
					json.Unmarshal([]byte(tc.Arguments), &args)
					parts = append(parts, map[string]any{
						"functionCall": map[string]any{
							"name": tc.Name,
							"args": args,
						},
					})
				}
				contents = append(contents, map[string]any{
					"role":  "model",
					"parts": parts,
				})
			} else {
				contents = append(contents, map[string]any{
					"role": "model",
					"parts": []map[string]any{
						{"text": m.Content},
					},
				})
			}
		case "tool":
			var output any
			if err := json.Unmarshal([]byte(m.Content), &output); err != nil {
				output = m.Content
			}
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{
						"functionResponse": map[string]any{
							"name":     m.ToolCallID,
							"response": output,
						},
					},
				},
			})
		}
	}

	body := map[string]any{
		"contents": contents,
		"tools": []map[string]any{
			{"functionDeclarations": toGeminiTools(specs)},
		},
	}
	if systemInstruction != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemInstruction}},
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", b.model, b.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("gemini api error %d: %v", resp.StatusCode, errBody)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	var text string
	var toolCalls []ToolCall

	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, ToolCall{
				ID:        part.FunctionCall.Name,
				Name:      part.FunctionCall.Name,
				Arguments: string(args),
			})
		} else {
			text += part.Text
		}
	}

	return &CompletionResult{
		Content:   text,
		ToolCalls: toolCalls,
	}, nil
}

func toGeminiTools(specs []tools.ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		out = append(out, map[string]any{
			"name":        spec.Name,
			"description": spec.Description,
			"parameters":  spec.ParamSchema,
		})
	}
	return out
}
