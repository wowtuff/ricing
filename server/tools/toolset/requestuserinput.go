package toolset

import (
	"context"
	"strings"

	"github.com/wowtuff/ricing/tools"
)

type RequestUserInputTool struct{}

func (t RequestUserInputTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "request_user_input",
		Description: "Ask the user one concise multiple-choice question and wait for their answer. Use this when a short selection would unblock the task.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"header": map[string]any{
					"type": "string",
				},
				"question": map[string]any{
					"type": "string",
				},
				"options": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{
								"type": "string",
							},
							"label": map[string]any{
								"type": "string",
							},
							"description": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"label"},
					},
				},
			},
			"required": []string{"question", "options"},
		},
	}
}

func (t RequestUserInputTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	question, _ := args["question"].(string)
	header, _ := args["header"].(string)
	return map[string]any{
		"ok":       true,
		"header":   strings.TrimSpace(header),
		"question": strings.TrimSpace(question),
	}, nil
}
