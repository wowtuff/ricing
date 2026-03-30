package toolset

import (
	"context"
	"strings"

	"github.com/wowtuff/ricing/tools"
)

type UpdatePlanTool struct{}

func (t UpdatePlanTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "update_plan",
		Description: "Record a structured plan for the current task.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{
					"type": "string",
				},
				"steps": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title": map[string]any{
								"type": "string",
							},
							"status": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"title"},
					},
				},
			},
		},
	}
}

func (t UpdatePlanTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	summary, _ := args["summary"].(string)
	return map[string]any{
		"ok":      true,
		"summary": strings.TrimSpace(summary),
	}, nil
}
