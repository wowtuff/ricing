package toolset

import (
	"context"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type (
	MultiplyTool struct{}
)

func (m MultiplyTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "multiply",
		Description: "Multiply two numbers together.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{
					"type":        "number",
					"description": "The first number",
				},
				"b": map[string]any{
					"type":        "number",
					"description": "The second number",
				},
			},
			"required": []string{"a", "b"},
		},
	}
}

func (m MultiplyTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	a, ok := args["a"].(float64)
	if !ok {
		return nil, utils.LogError("invalid or missing parameter: a")
	}
	b, ok := args["b"].(float64)
	if !ok {
		return nil, utils.LogError("invalid or missing parameter: b")
	}

	return map[string]any{
		"result": a * b,
	}, nil
}
